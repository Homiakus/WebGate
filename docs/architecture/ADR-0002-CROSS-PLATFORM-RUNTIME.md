# ADR-0002 — Cross-Platform Runtime Architecture

- Status: **ACCEPTED**
- Date: **2026-08-29**
- Scope: Windows, Android, Linux, macOS
- Related: `ADR-0001-BROWSER-ENGINE.md`, `RESILIENCE_CROSS_PLATFORM_AUDIT.md`

## Context

WebGate must preserve the same fail-closed protected-browser model on desktop and mobile without forcing unrelated device traffic through its transport.

The existing desktop design used a Rust client and supervised transport sidecars. Android exposes different process/lifecycle/security primitives and should not be forced into the desktop execution model.

Servo is the canonical browser engine and is available on Windows, Linux, macOS and Android. Outline SDK supports generated mobile libraries as well as side-service and Go-library integration. Android also provides `VpnService` per-app routing as a fallback.

## Decision

WebGate uses one shared security core and platform-specific runtime adapters.

```text
WebGate Shared Rust Core
  ├─ browser contract
  ├─ Servo adapter
  ├─ signed policy/config
  ├─ deep-link parser
  ├─ SecureAcces client
  ├─ device registry protocol
  ├─ transport orchestration
  ├─ readiness/failure state machine
  └─ observability contracts

Platform Runtime
  ├─ Windows
  ├─ Android
  ├─ Linux
  └─ macOS
```

### Browser

Servo is primary on all supported platforms.

A platform compatibility browser may exist only behind an explicit policy-controlled fallback. There is no automatic browser-engine failover.

### Transport

Split protocol/provider behavior from execution model:

```text
TransportProvider
  +
TransportRuntime
```

Runtime options:

```text
Sidecar
InProcess
PlatformVpnService
```

Desktop may prefer supervised sidecars. Android normal mode prefers an in-process/generated mobile library with a loopback local proxy.

### Android normal mode

```text
Servo AAR
   ↓ HTTP/HTTPS proxy
127.0.0.1:<ephemeral>
   ↓
Outline MobileProxy / compatible provider
   ↓
protected remote path
```

The system default route remains unchanged.

### Android fallback mode

If a provider requires TUN/IP semantics, WebGate may use Android `VpnService` restricted to WebGate's own package with `addAllowedApplication()`.

This is an explicit compatibility mode, not the default.

### Device identity

Policy/update signing and device identity use distinct key purposes.

Canonical device proof supports **ES256 / ECDSA P-256** so hardware-backed keys can be used in Android Keystore/StrongBox, Windows TPM/CNG and Apple Secure Enclave.

Ed25519 remains preferred for compact application-controlled policy/update signatures and may remain an optional software device-key algorithm.

### Android platform shell

Use a deliberately thin Kotlin/Java layer for:

- Activity/process lifecycle;
- Android App Links;
- Keystore;
- foreground service management where required;
- VpnService compatibility adapter;
- notifications and OS permission prompts.

Business/security policy remains in the Rust core.

## Invariants

1. No supported platform may silently route protected content directly when the WebGate transport is unavailable.
2. The browser proxy/network configuration must be established before protected navigation.
3. Platform UI code cannot grant authorization or weaken signed policy.
4. Transport implementation changes cannot require browser/business-code rewrites.
5. Server-side SecureAcces remains authoritative for protected resources.
6. Device private keys are generated locally and are never distributed in bootstrap files.
7. Hardware-backed device keys are preferred and their security level is visible to server policy.
8. Android system VPN mode is optional; normal WebGate mode must remain application-scoped.
9. Platform process/lifecycle recovery must revalidate policy/session/device state rather than trust restored browser memory.
10. Servo upgrades are qualified as part of WebGate releases, not updated independently.

## Consequences

### Positive

- Android becomes a first-class target rather than a port of desktop code.
- most security-sensitive logic remains shared and testable in Rust;
- system-wide VPN is avoided on Android in normal use;
- existing external VPNs are less likely to conflict with WebGate normal mode;
- hardware-backed identity becomes possible across major target platforms;
- transport providers can use their most appropriate packaging model on each OS;
- Servo compatibility testing is shared across platforms.

### Negative

- Android requires a small Kotlin/JNI/AAR integration layer;
- transport providers must support more than one runtime model;
- key algorithm negotiation becomes part of the device protocol;
- platform-specific CI and lifecycle testing are mandatory;
- macOS/Linux packaging cannot simply reuse Windows updater logic.

## Platform tiers

```text
Tier 1  Windows x86_64
Tier 1  Android arm64 after Android M1 acceptance
Tier 2  Linux x86_64/aarch64
Tier 2  macOS arm64/x86_64
Tier 3  OpenHarmony research
Deferred iOS
```

## Android acceptance gate

Android becomes Tier 1 only when all of the following pass on physical arm64 devices:

- Servo AAR loads the target documents site;
- local proxy cannot be bypassed;
- transport failure fails closed;
- process kill/restart safely restores state;
- Wi-Fi/mobile transitions recover;
- verified App Links open the expected opaque resource;
- Android Keystore-backed device proof works;
- SecureAcces revocation is enforced;
- APK upgrade preserves expected device identity;
- clearing app data requires re-enrollment;
- normal proxy mode does not alter unrelated applications' routing.

## Supersession

Any existing WebGate documentation that implies "transport sidecar" is a universal cross-platform requirement is superseded by this ADR. Sidecar is one `TransportRuntime`, not the architecture itself.
