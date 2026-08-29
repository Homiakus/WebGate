# WebGate — Resilience and Cross-Platform Audit

Research baseline: **2026-08-29**

Scope: production resilience, platform portability, Android feasibility, browser/transport lifecycle, secure storage, failover, updates, and operational failure modes.

## Executive verdict

WebGate is architecturally suitable for a cross-platform product, but only if the project stops treating the Windows process model as universal.

The recommended product architecture is:

```text
                         WebGate Core
                            Rust
                              |
          +-------------------+-------------------+
          |                   |                   |
      Browser API         Policy/Auth         Transport API
          |                   |                   |
        Servo           SecureAcces client    provider SPI
          |                                       |
   +------+------+                     +----------+----------+
   |      |      |                     |                     |
Windows Linux  macOS                 Desktop               Mobile
   |      |      |                   sidecar             in-process/AAR
   +------+------+                     |                     |
          |                       Outline/AWG/Xray       Outline MobileProxy
          |                                             or VpnService fallback
        Android
     Kotlin shell
          |
      Servo AAR
          |
    Rust shared core
```

The product should have **one security model and several execution models**.

Canonical support order:

1. **Windows x86_64** — Tier 1.
2. **Android arm64** — Tier 1 after the Android M1 gate passes.
3. **Linux x86_64/aarch64** — Tier 2, straightforward technically.
4. **macOS arm64/x86_64** — Tier 2, with signing/notarization and Keychain/Secure Enclave work.
5. **iOS** — not in the initial support contract; revisit separately.

Servo upstream currently states support for Windows, macOS, Linux, Android and OpenHarmony, and current releases publish Android artifacts including an AAR. Servo also now ships an LTS line for embedders. This makes a shared Servo browser strategy realistic, but Servo is still pre-1.0 and its embedding API evolves quickly, so WebGate must pin and qualify releases rather than follow `main`.

---

# 1. Current architecture: strengths

The existing WebGate design already contains several unusually strong resilience properties:

- application-scoped networking rather than changing the OS default route;
- fail-closed browser behavior;
- browser/transport separation;
- two independent relay failure domains;
- independent primary and fallback transport families;
- signed bootstrap/policy/update model;
- device revocation separated from SecureAcces account revocation;
- authoritative server-side authorization through SecureAcces;
- origin server does not require a static public IP or inbound port forwarding;
- protected links are identifiers rather than long-lived bearer credentials.

These properties should remain platform-independent invariants.

---

# 2. Critical findings

## F-01 — Desktop sidecars are not a universal transport architecture

Severity: **HIGH architectural risk**

The current plan describes a supervised Go `webgate-transport` sidecar. This is excellent on Windows/Linux and feasible on macOS, but it should not be the transport contract itself.

Android needs a different execution model. Outline SDK officially supports:

- generated mobile libraries for Android/iOS/macOS using `gomobile bind`;
- side services on desktop and Android;
- a MobileProxy model exposing a local forward proxy.

For WebGate Android, an AAR/mobile-library integration is preferable to depending on subprocess semantics.

### Required correction

Split:

```text
TransportProvider       logical protocol/provider behavior
TransportRuntime        where/how provider executes
```

Suggested model:

```rust
enum RuntimeKind {
    InProcess,
    Sidecar,
    PlatformVpnService,
}
```

The browser must see the same `LocalProxyEndpoint` regardless of runtime kind.

---

## F-02 — Android can preserve WebGate's no-system-VPN design

Severity: **POSITIVE / important design finding**

Servo gained HTTP proxy support and current networking code supports HTTP and HTTPS proxy preferences. Outline MobileProxy is explicitly designed to run a local forward proxy inside mobile applications.

Therefore the preferred Android path is:

```text
Telegram App Link
      ↓
WebGate Activity
      ↓
Servo AAR
      ↓
127.0.0.1:<ephemeral HTTP proxy>
      ↓
Outline MobileProxy / protected transport
      ↓
Relay
```

No Android VPN permission is required for the normal mode.

Consequences:

- unrelated Android apps remain untouched;
- WebGate can coexist with another system VPN better than a VpnService-based implementation;
- the user avoids Android's VPN consent dialog for normal mode;
- no permanent VPN notification is required merely to display a document when the app is foregrounded;
- the same fail-closed proxy model can be reused across desktop and Android.

This should become the canonical Android design.

---

## F-03 — Android VpnService is a valuable compatibility fallback

Severity: **MEDIUM / fallback capability**

Android `VpnService.Builder.addAllowedApplication()` can restrict a VPN to WebGate's package only. Other applications then use the network normally.

Use this only when a transport requires IP/TUN semantics or Servo/proxy integration cannot satisfy a production requirement.

```text
Android VpnService
      |
      +-- allowed app: WebGate only
      |
      +-- other apps: normal networking
```

Trade-offs:

- only one active VPN service per Android user/profile;
- may conflict with the user's existing VPN;
- requires explicit user approval;
- long-running VPN needs foreground-service behavior/notification;
- always-on VPN can be supported but should not be WebGate's default UX.

Therefore:

```text
Android normal mode      = app-local proxy
Android compatibility    = per-app VpnService
Android full-device VPN  = out of scope by default
```

---

## F-04 — Device identity algorithm must be hardware-portable

Severity: **HIGH security/design finding**

The earlier design assumed an Ed25519 device key on every platform. That is simple cryptographically but does not align cleanly with the strongest native hardware key stores.

Current platform capabilities strongly favor **P-256** for non-exportable hardware-backed signing:

- Android Keystore / StrongBox supports ECDSA P-256 and can keep key material outside the application process;
- Apple's Secure Enclave exposes P-256 signing/key agreement;
- Windows Microsoft Platform Crypto Provider uses TPM-backed key storage and supports ECDSA P-256.

### Recommendation

Separate trust-key purposes:

```text
Policy/update signing        Ed25519
Server signing roots         Ed25519 where appropriate
Device proof-of-possession   ES256 / ECDSA P-256 preferred
```

Device registry schema must contain:

```text
device_id
account_id
key_algorithm   # ES256, optional legacy ED25519
public_key
hardware_level  # SOFTWARE / TEE / STRONGBOX / TPM / SECURE_ENCLAVE / UNKNOWN
attestation     # optional future capability
status
```

This change gives WebGate a credible path to non-exportable device identities across Windows, Android and Apple platforms.

Ed25519 may remain a supported software-key profile where platform hardware APIs do not provide P-256 or hardware binding is unavailable.

---

## F-05 — Servo Android is viable but not yet a drop-in Android WebView replacement

Severity: **MEDIUM**

Servo currently publishes Android builds/AARs and Android servoshell is actively maintained. However the JNI embedding surface is still evolving; upstream still tracks richer JavaScript/native bridge capabilities.

For WebGate this is mostly acceptable because arbitrary page-to-native JavaScript bridging should be avoided anyway.

WebGate should deliberately require only a **narrow embedding contract**:

- create/destroy Servo view;
- navigate/load request;
- lifecycle suspend/resume;
- resize/render/input/IME;
- navigation decision callbacks;
- download/file-picker broker where required;
- cookie/site-data management required for SecureAcces session UX;
- proxy configuration before any protected navigation.

Do **not** architect the site around `addJavascriptInterface`-style native APIs.

This reduces both dependency on unfinished Servo Android APIs and the browser-to-native attack surface.

---

## F-06 — Servo release management is a reliability boundary

Severity: **HIGH operational risk**

Servo now provides an LTS line, but regular monthly releases may contain breaking embedding API changes. WebGate must never track Servo `main` in production.

Required policy:

```text
production -> pinned Servo LTS patch release
staging    -> next LTS/current stable qualification
research   -> latest release/nightly
```

Upgrade gate:

1. dependency/security scan;
2. upstream security-notes review;
3. compile all platforms;
4. WPT subset used by WebGate;
5. site compatibility suite;
6. visual regressions;
7. transport escape tests;
8. 24h/72h soak tests;
9. rollout canary;
10. staged production release.

---

# 3. Platform support matrix

| Capability | Windows | Android | Linux | macOS |
|---|---|---|---|---|
| Servo engine | Strong | Active / viable | Strong | Strong |
| App-local HTTP proxy | Yes | Yes | Yes | Yes |
| System route untouched | Yes | Yes | Yes | Yes |
| Native secure secret store | DPAPI/CNG/TPM | Keystore/TEE/StrongBox | Secret Service/TPM optional | Keychain/Secure Enclave |
| Hardware device key | TPM P-256 | TEE/StrongBox P-256 | TPM2 optional | Secure Enclave P-256 |
| Deep/trusted links | URI/App protocol | Verified App Links | desktop MIME/URI handler | Universal/custom URL handling |
| Transport sidecar | Excellent | Avoid as primary | Excellent | Possible |
| Mobile transport AAR | N/A | Excellent fit | N/A | N/A |
| Per-app OS VPN fallback | platform-specific | Excellent (`VpnService`) | possible but unnecessary | Network Extension requires separate design |
| Background lifecycle complexity | Low | High | Low | Medium |
| Distribution complexity | Medium | Medium | High fragmentation | High signing/notarization |

### Tier recommendation

```text
Tier 1: Windows, Android
Tier 2: Linux, macOS
Tier 3: OpenHarmony experiment
Deferred: iOS
```

---

# 4. Canonical cross-platform module boundaries

The repository should evolve toward:

```text
crates/
├── webgate-core/              state orchestration, no UI APIs
├── webgate-browser/           engine-neutral protected browser contract
├── webgate-browser-servo/     canonical Servo implementation
├── webgate-policy/            local + signed remote policy
├── webgate-crypto/            signatures, fingerprints, challenge formats
├── webgate-device/            device proof abstraction
├── webgate-auth/              SecureAcces client/session integration
├── webgate-transport/         provider/runtime-neutral contract
├── webgate-health/            readiness + failure state machines
├── webgate-deeplink/          strict shared parser
├── webgate-config/            signed bootstrap/policy formats
├── webgate-observability/     redacted events/metrics
└── webgate-platform/          narrow platform traits

platform/
├── windows/
├── android/
├── linux/
└── macos/
```

Avoid:

```text
webgate-core -> Win32
webgate-core -> Android JNI
webgate-browser -> Outline SDK
webgate-transport -> UI
```

The shared Rust core should contain the product's state machine and security policy; platform shells should contain lifecycle and OS integration only.

---

# 5. Android target architecture

Recommended Android composition:

```text
┌──────────────────────────────────────┐
│          WebGate Android APK         │
│                                      │
│ Minimal Kotlin/Java platform shell   │
│  ├─ Activity lifecycle               │
│  ├─ App Links                        │
│  ├─ permissions                      │
│  ├─ Android Keystore                 │
│  └─ notifications/service broker     │
│                 │                    │
│                 ▼ JNI/AAR boundary   │
│        Shared WebGate Rust Core      │
│                 │                    │
│       ┌─────────┴──────────┐         │
│       ▼                    ▼         │
│    Servo AAR         Transport bridge│
│       │                    │         │
│       └──HTTP(S)──► localhost proxy  │
│                            │         │
│                    Outline MobileProxy│
└────────────────────────────┼─────────┘
                             │
                        protected path
                             │
                           Relay
```

### Why a minimal Kotlin shell is preferable

Android lifecycle, App Links, Keystore, foreground services and VpnService are OS-native concepts. Hiding them behind an oversized cross-platform UI framework would create more lifecycle bugs than it removes.

Use Kotlin only as a thin platform adapter. Keep business/security logic in Rust.

---

# 6. Android lifecycle and resilience

Android is the platform with the highest lifecycle risk.

WebGate must explicitly handle:

```text
Activity created
Activity paused
Activity stopped
process backgrounded
process killed
screen locked
network changed
Wi-Fi -> LTE/5G
LTE/5G -> Wi-Fi
Doze
low-memory trim
app upgraded
device reboot
```

## Session continuity policy

Do not make browser RAM state authoritative.

Persist only:

- signed policy cache;
- encrypted/OS-protected session material as policy permits;
- device key handle;
- opaque last navigation target if safe;
- non-sensitive health hints.

After process death:

```text
restart
  ↓
load + verify signed policy
  ↓
device proof
  ↓
validate/refresh SecureAcces session
  ↓
start fail-closed proxy
  ↓
re-establish transport
  ↓
restore safe opaque navigation target
```

Never restore a stale WebView state as proof of authorization.

## Foreground service rule

If the secure transport must remain alive after the Activity is no longer foregrounded, Android requires a foreground-service-compatible design. For the initial document-reader UX, prefer **connect on demand while WebGate is actively used**, which reduces battery drain and system-service complexity.

---

# 7. Android App Links / Telegram flow

Use verified HTTPS App Links, not only `webgate://` custom schemes.

Canonical link:

```text
https://go.example.net/d/<opaque-id>
```

Android verifies ownership using Digital Asset Links / `assetlinks.json` and routes matching URLs directly to WebGate when installed.

Flow:

```text
Telegram
   ↓
verified HTTPS App Link
   ↓
WebGate
   ↓
strict shared parser
   ↓
opaque resource ID
   ↓
SecureAcces authorization after connection
```

Keep custom `webgate://` only as a secondary internal/desktop mechanism.

---

# 8. Network resilience model

WebGate should stop thinking in terms of a binary `VPN connected` state.

Use layered readiness:

```text
PolicyValid
DeviceTrusted
SessionUsable
ProxyListening
ProviderAlive
RelayReachable
OriginReachable
AuthorizationHealthy
BrowserReady
```

Protected content can be requested only if the required readiness predicate is true.

Suggested gate:

```text
CAN_NAVIGATE_PROTECTED =
    PolicyValid
 && DeviceTrusted
 && ProxyListening
 && TransportPathReady
 && BrowserReady
```

`SessionUsable` can be established through the protected auth/control path during startup.

This is more robust than a single mutable `connected: bool` flag.

---

# 9. Transport failover

Canonical independent path matrix remains:

```text
1 Relay A / Primary transport
2 Relay B / Primary transport
3 Relay A / Independent fallback
4 Relay B / Independent fallback
```

But transport selection should be a scored state machine rather than fixed sequential retry.

Inputs:

- connection success history;
- handshake time;
- origin health latency;
- packet/stream failure rate;
- recent circuit-breaker state;
- current network type;
- UDP capability;
- captive portal suspicion;
- IPv4/IPv6 availability.

Do not use raw latency alone; a 40 ms relay that intermittently drops traffic is worse than a stable 90 ms relay.

### Hysteresis

Once a backup path is healthy, do not immediately jump back to primary after one successful probe. Require a stability window to avoid flapping.

---

# 10. Local proxy resilience

The local proxy is the central kill switch.

It should be available before remote connectivity, but begin in `DENY_ALL_PROTECTED` state.

```text
PROCESS START
    ↓
LOCAL PROXY LISTENING
    ↓
DENY / synthetic offline response
    ↓
transport ready
    ↓
ALLOW signed destination set
```

This model is better than starting/stopping the proxy during reconnections because the browser's networking configuration never changes.

Required protections:

- loopback-only bind;
- random port;
- destination allowlist;
- optional per-process capability/token if provider supports it;
- bounded queues;
- connection/time limits;
- explicit DNS policy;
- IPv6 loopback testing;
- no `NO_PROXY` escape;
- no direct bypass on proxy error.

---

# 11. Origin and relay resilience

The origin should keep two independent outbound reverse paths alive.

```text
Origin RU
  ├── outbound path -> Relay A
  └── outbound path -> Relay B
```

The user client must never need to know the origin's public IP.

Recommended failure domains:

- Relay A and B on different providers;
- different ASN/prefix where practical;
- separate credentials;
- separate automation state;
- DNS served by independent authoritative infrastructure if possible;
- configuration able to carry literal bootstrap endpoints as signed fallback.

Origin watchdog should distinguish:

```text
local site failed
local database failed
reverse path A failed
reverse path B failed
ISP failed
DNS failed
```

and report each separately.

---

# 12. SecureAcces resilience

SecureAcces remains the authorization authority. WebGate must assume cached UI state can be stale.

Every protected resource request remains server-authorized.

Critical failure behavior:

| Failure | Required result |
|---|---|
| session revoked | deny immediately |
| account suspended | deny |
| membership revoked | deny |
| device revoked | refuse new/refresh sessions |
| control API unavailable | bounded grace only if explicitly designed; never invent access |
| stale browser cookie | server rejects |
| cross-tenant resource ID | authoritative resource mapping + deny |

The network tunnel is not an authorization credential.

---

# 13. Platform secret storage

## Windows

Preferred order:

1. TPM-backed CNG P-256 device key when available;
2. CNG software KSP / DPAPI-protected fallback;
3. never raw private-key JSON.

## Android

Preferred order:

1. StrongBox P-256 when available and compatible;
2. TEE-backed Android Keystore P-256;
3. Android Keystore software-backed fallback with server-visible security level.

## macOS

Preferred order:

1. Secure Enclave P-256;
2. Keychain-protected software key fallback.

## Linux

Linux hardware and desktop environments vary.

Preferred order:

1. TPM2-backed key provider where supported by the target distribution;
2. Secret Service / desktop keyring protected key;
3. encrypted file fallback only with explicit warning/policy and strict permissions.

Server policy may decide whether a software-only device is acceptable.

---

# 14. Updates and rollback resilience

An updater failure can be as damaging as a network failure.

Use two independent concepts:

```text
package authenticity
minimum acceptable security version
```

Every release manifest must be signed and include hashes for:

- WebGate executable/library;
- Servo dependency/build identity;
- transport sidecars/AAR/native libs;
- schema versions;
- platform/architecture;
- minimum compatible control-plane version.

Android additionally relies on package signing, but server-side WebGate update policy should still enforce a minimum supported version for emergency revocation of vulnerable builds.

Do not auto-update Servo independently from WebGate. Servo is part of the qualified application build.

---

# 15. Observability without leaking secrets

Recommended event model:

```text
browser.engine_started
browser.navigation_blocked
proxy.ready
proxy.policy_rejected
transport.path_selected
transport.path_failed
transport.failover
origin.health_failed
auth.session_refreshed
auth.session_rejected
device.proof_failed
policy.signature_failed
update.rollback_blocked
```

Never log:

- session token;
- enrollment token;
- private key;
- full sensitive query strings;
- document bytes;
- Telegram auth credential;
- transport private material.

Every event should have a stable event code so diagnostics work consistently across platforms.

---

# 16. Cross-platform test matrix

## Shared core tests

Run on every commit:

- policy parser/property tests;
- signed bundle fuzz tests;
- deep-link fuzz tests;
- state-machine model tests;
- authorization client tests;
- transport selection simulation;
- key-format/algorithm interoperability;
- monotonic rollback tests.

## Desktop integration

Windows/Linux/macOS:

- clean install;
- upgrade;
- uninstall/reinstall;
- user profile changes;
- suspend/resume;
- network transition;
- DNS loss;
- IPv4 only;
- IPv6 only/dual stack;
- proxy/provider crash;
- Relay A/B failure;
- corrupted local policy cache;
- clock skew.

## Android integration

Add:

- API-level matrix;
- arm64 physical devices;
- emulator where useful;
- process kill from recents/system;
- low-memory kill;
- background/foreground loops;
- screen lock/unlock;
- Wi-Fi <-> mobile network;
- airplane mode;
- Doze/battery saver;
- permission denial;
- Keystore key invalidation;
- App Link verification;
- APK upgrade preserving device identity;
- application data clear causing expected re-enrollment;
- concurrent external VPN in normal proxy mode;
- VpnService fallback with an existing VPN conflict;
- Android Network Security Config cleartext denial.

---

# 17. Resilience SLOs proposed for design validation

These are engineering targets, not current measured performance:

```text
No unauthorized direct fallback             100%
Transport crash -> safe offline state        < 1 s
Failover decision after confirmed outage     target < 5 s
Warm reconnect after network change          target < 5 s
Policy signature/tamper detection            100%
Revoked session/resource enforcement         next server request
Client crash must not expose direct origin   100%
```

Availability should not be achieved by weakening authorization or fail-closed rules.

---

# 18. Risk register

| Risk | Severity | Mitigation |
|---|---:|---|
| Servo pre-1.0 API/web compatibility | High | LTS pinning + site-specific compatibility suite |
| transport provider coupled to desktop sidecars | High | runtime-neutral TransportProvider contract |
| Ed25519-only device identity prevents best hardware storage | High | ES256/P-256 device profile |
| Android process/lifecycle kills transport | High | explicit lifecycle state machine; foreground service only where needed |
| hidden proxy/direct fallback | Critical | immutable proxy configuration + negative network tests |
| one VPS/provider outage | High | two independent relays/providers |
| one protocol blocked | High | independent fallback transport |
| stale policy | Medium/High | signed TTL + LKG cache + anti-rollback |
| Android VpnService conflict with existing VPN | Medium | app-local proxy is default; VpnService fallback only |
| Android Servo JNI limitations | Medium | narrow embedding contract; no arbitrary JS-native bridge |
| Linux distribution fragmentation | Medium | initial certified package targets, not every distro |
| macOS signing/notarization mistakes | Medium | CI signing/notarization pipeline and clean-machine tests |
| origin loses power/network | High | UPS + dual WAN optional + watchdog + relay health alerts |

---

# 19. Recommended implementation sequence

## R0 — Cross-platform contracts

Before significant UI code:

- create platform trait boundaries;
- define `TransportRuntime` separately from provider;
- define ES256-capable device identity wire format;
- freeze signed bootstrap/policy schema;
- define shared readiness state model.

## R1 — Windows reference implementation

Use Windows to prove:

- Servo embedding;
- immutable local proxy;
- real transport;
- SecureAcces integration;
- failover.

## R2 — Android vertical slice immediately after Windows browser/proxy proof

Do not postpone Android until the end.

Build:

```text
Telegram App Link
 -> Android WebGate
 -> Servo AAR
 -> Outline MobileProxy AAR
 -> test relay/origin
 -> SecureAcces protected page
```

This early vertical slice validates that desktop assumptions did not leak into the core design.

## R3 — Linux/macOS adapters

Once Windows + Android use the same shared core successfully, add desktop platform packaging and secure storage adapters.

## R4 — resilience certification

Chaos/network/lifecycle matrix on all supported Tier 1 platforms before production.

---

# 20. Final architectural verdict

The most robust WebGate is not four separate applications and not one monolithic desktop application forced onto mobile.

It is:

```text
                  SHARED SECURITY CORE
                         Rust
                          |
         +----------------+----------------+
         |                |                |
       Servo          Transport SPI     SecureAcces
         |                |                |
         +----------------+----------------+
                          |
                 PLATFORM ADAPTERS
            Windows / Android / Linux / macOS
```

The **browser, policy, authorization semantics, deep-link parser, transport state machine and signed configuration formats remain shared**.

Only these should differ materially by platform:

- window/activity lifecycle;
- secure hardware/key store;
- packaging/updating;
- OS deep-link registration;
- transport execution model;
- optional per-app VPN fallback.

With those boundaries, Android is not an afterthought: it becomes a first-class target using the same protected-browser model as Windows while leaving the rest of the phone's network traffic untouched.
