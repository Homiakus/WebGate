# ADR-0002 — Cross-Platform Runtime Architecture

- Status: **ACCEPTED / AMENDED**
- Original date: **2026-08-29**
- Amended: **2026-09-08**
- Scope: Windows, Android, Linux, macOS
- Related: `ADR-0001-BROWSER-ENGINE.md`, `ADR-0003-SERVO-PROCESS-ISOLATION.md`, `ADR-0004-ANDROID-CLIENT-SECURITY-ARCHITECTURE.md`, `RESILIENCE_CROSS_PLATFORM_AUDIT.md`

## Context

WebGate must preserve the same fail-closed protected-browser model on desktop and mobile without forcing unrelated device traffic through its transport.

The desktop design and the Android runtime share security semantics but must not be forced into one process/execution topology. Android exposes different process-isolation, lifecycle, Keystore, Binder and network-observation primitives and is therefore treated as a first-class runtime rather than a port of the desktop shell.

The original revision of this ADR described Android primarily as a Servo AAR connected to a loopback proxy. That remains a possible compatibility lane, but is no longer the canonical STRICT Android security architecture. Android-specific details are now owned by `ADR-0004-ANDROID-CLIENT-SECURITY-ARCHITECTURE.md`.

## Decision

WebGate uses one shared security core and platform-specific runtime adapters.

```text
WebGate Shared Security Core
  |- signed policy/config
  |- CanonicalDestination
  |- deep-link/resource identity
  |- SecureAcces contracts
  |- device registry protocol
  |- canonical Transport Supervisor
  |- readiness/evidence state machine
  |- resource/admission policy
  |- release verification
  `- observability contracts

Platform Runtime
  |- Windows
  |- Android
  |- Linux
  `- macOS
```

Security semantics are shared; process topology is platform-specific.

## Browser/runtime model

WebGate no longer defines one universal browser engine as the sole platform architecture.

The target model is renderer-agnostic at the BrowserCapsule/session boundary and supports explicit qualified renderer lanes:

```text
STRICT  -> WGBR / strongest capability isolation
COMPAT  -> Servo after measured runtime/security qualification
PUBLIC  -> platform/system browser surfaces for non-protected content only
```

There is no automatic protected-content fallback into an unqualified system browser/WebView.

On Android, `ADR-0004` is authoritative: STRICT rendering runs in an Android isolated process with zero direct network capability, while the trusted host owns policy, authorization, transport and durable authority.

## Transport model

Protocol/provider behavior is separated from execution model:

```text
TransportProvider
       +
TransportRuntime
```

Runtime options may include:

```text
Sidecar
InProcess
PlatformVpnService
```

but those are packaging/execution choices, not separate security models.

All platforms consume one canonical Transport Supervisor semantics. A platform adapter may not fork failover/routing truth into an independent state machine.

### Desktop

Desktop may use supervised sidecars or in-process providers according to qualification and sandbox constraints.

### Android normal mode

Android normal mode uses application-scoped in-process transport owned by the trusted Android host:

```text
Isolated protected renderer
       |
       | capability IPC
       v
Trusted Android host/broker
       |
       v
Canonical Transport Supervisor
       |
       v
qualified protected remote path
```

The Android system default route remains unchanged.

### Android compatibility VPN mode

If a provider requires TUN/IP semantics, WebGate may use Android `VpnService` as an explicit compatibility mode, preferably restricted to WebGate's own package with Android per-app VPN controls where available.

This is not the normal/default WebGate product mode and must not be entered silently if it changes device network semantics.

## Device identity

Policy/update signing and device identity use distinct key purposes.

Canonical device proof supports **ES256 / ECDSA P-256** so hardware-backed keys can be used in Android Keystore/StrongBox, Windows TPM/CNG and Apple Secure Enclave where available.

Ed25519 remains appropriate for compact application-controlled policy/release signatures where the trust architecture specifies it, but platform device identity is not forced to use an exportable software Ed25519 seed.

Android production identity is owned by the Android Keystore backend defined in ADR-0004; the existing file-backed Rust keystore is not the production Android private-key store.

## Android platform shell

The Kotlin/Java layer is deliberately thin but owns Android framework boundaries:

- Activity/process lifecycle observations;
- Jetpack Compose UI;
- Android App Links;
- Android Keystore and attestation calls;
- Binder/AIDL process binding;
- SharedMemory lifecycle where required;
- foreground-service management where justified;
- optional `VpnService` compatibility adapter;
- notifications and OS permission prompts;
- connectivity callbacks.

Business/security policy remains in shared Rust core/contracts. Kotlin UI state cannot grant protected-session authority.

## Android process model

The canonical Android STRICT path is:

```text
Trusted Host Process
  |- identity
  |- policy
  |- authorization
  |- canonical destination broker
  |- transport
  |- persistence
  |- resource governor
  `- compositor
          |
          | typed Binder/AIDL + bounded SharedMemory
          v
Isolated WGBR Renderer Service
  |- no own Android permissions
  |- no direct network capability
  |- no durable authority
  `- typed bounded DisplayList/resource requests
```

This is stronger than relying on a renderer to voluntarily send all network requests to a localhost proxy.

## Lifecycle model

Platform lifecycle is an observation input, not the authoritative security state machine.

On Android, saved Activity/ViewModel state may restore navigation/UI hints but cannot restore:

- authorization validity;
- transport readiness;
- route evidence;
- renderer readiness;
- active capability handles;
- `OPEN`.

Process recreation must revalidate identity, policy, route, authorization and renderer evidence.

## Network-change model

Mobile network changes are explicitly modeled.

Android introduces a monotonic `network_epoch`; material path changes invalidate route/transport evidence from older epochs. Wi-Fi/cellular/VPN/captive-portal transitions therefore cannot leave stale `Ready` state visible as current truth.

## Invariants

1. No supported platform may silently route protected content directly when the WebGate transport is unavailable.
2. Protected rendering cannot begin until the required platform transport/broker boundary is established and evidenced.
3. Platform UI code cannot grant authorization or weaken signed policy.
4. Transport implementation changes cannot require browser/business-code rewrites across platform boundaries.
5. Server-side SecureAcces remains authoritative for protected resources.
6. Device private keys are generated locally and are never distributed in bootstrap files.
7. Hardware-backed device keys are preferred where supported and their verified security level may be exposed to server policy.
8. Android system VPN mode is optional; normal WebGate mode remains application-scoped.
9. Platform process/lifecycle recovery revalidates policy/session/device/route state rather than trusting restored browser/UI memory.
10. Renderer upgrades are qualified as part of WebGate releases, not assumed safe because an upstream engine version changed.
11. Android STRICT renderer has zero direct network capability.
12. No platform may create a second independent authoritative failover state machine.
13. Resource and IPC bounds are part of readiness, not only performance tuning.
14. A system browser/WebView is never an implicit protected-content fallback.

## Consequences

### Positive

- Android becomes a first-class target rather than a desktop port;
- most security-sensitive semantics remain shared and testable in Rust;
- Android can use kernel/framework-enforced isolated-process capability reduction;
- normal Android mode avoids a system-wide VPN and is less likely to conflict with unrelated apps;
- hardware-backed identity becomes possible across major target platforms;
- transport providers can use their most appropriate execution packaging;
- renderer strategy can evolve from Servo toward WGBR without changing application authorization semantics;
- process death and network changes become explicit evidence transitions rather than hidden UI edge cases.

### Negative

- Android requires Kotlin + Rust + Binder/AIDL + SharedMemory integration;
- renderer/host IPC becomes a security-critical protocol;
- transport providers must support more than one runtime model;
- key algorithm/security-level negotiation becomes part of device policy;
- platform-specific CI and physical-device lifecycle testing are mandatory;
- Android OEM background/memory behavior must be qualified explicitly;
- macOS/Linux packaging still cannot simply reuse Windows updater/runtime assumptions.

## Platform tiers

```text
Tier 1 target  Windows x86_64
Tier 1 target  Android arm64 after Android Tier-1 acceptance
Tier 2 target  Linux x86_64/aarch64
Tier 2 target  macOS arm64/x86_64
Research       OpenHarmony
Deferred       iOS
```

## Android acceptance gate

The detailed Android gate is owned by `ADR-0004` and `../implementation/ANDROID_CLIENT_IMPLEMENTATION_PLAN.md`.

At minimum Android becomes Tier-1 only when physical arm64 devices prove:

- signed APK/AAB install and upgrade preserve expected device identity;
- Android Keystore-backed proof and server-verified attestation work;
- verified App Links resolve opaque resources safely;
- STRICT renderer runs in `isolatedProcess` and direct network attempts fail;
- renderer/host IPC and SharedMemory are bounded and adversarially tested;
- transport failure fails closed;
- process kill/restart cannot resurrect `OPEN` or authorization;
- Wi-Fi/mobile transitions invalidate stale route proof and recover safely;
- SecureAcces revocation is enforced;
- clearing app data requires re-enrollment;
- normal mode does not alter unrelated applications' routing;
- optional VPN mode remains explicit and scoped;
- release-binary qualification uses the exact signed distribution artifact.

## Supersession

Any existing WebGate documentation that implies either of the following is universal is superseded:

```text
"transport sidecar is required on every platform"
"Android protected runtime = Servo AAR -> localhost proxy"
```

Sidecar is one `TransportRuntime`. Servo is one renderer lane. Android STRICT security architecture is defined by ADR-0004.
