# WebGate Android Client — Tier-1 Implementation Program

- Status: **ACTIVE DESIGN / EXECUTION INPUT**
- Date: **2026-09-08**
- Architecture authority: `../architecture/ADR-0004-ANDROID-CLIENT-SECURITY-ARCHITECTURE.md`
- Planning authority: `../../MASTER_PLAN.md` remains the canonical task/status owner.
- Target: Android arm64 production client, API 36 target.

## 1. Goal

Deliver a production Android client whose protected-content security does not rely on renderer cooperation.

Canonical end state:

```text
Compose Host / Android platform shell
        |
        +-- Android Keystore / identity
        +-- verified App Links
        +-- Policy + CanonicalDestination Broker
        +-- SecureAcces authorization
        +-- canonical Transport Supervisor
        +-- Resource Governor
        +-- Trusted Compositor
        |
        +==== Binder/AIDL + bounded SharedMemory ====
        |
        `-- isolated WGBR renderer
              no own Android permissions
              no direct network capability
              no durable authority
```

The program is ordered to establish security boundaries before feature breadth.

## 2. Non-negotiable release gates

Android production qualification is forbidden unless all of the following hold:

```text
RendererNetworkCapability == 0        in STRICT
DevicePrivateKeyExportable == false   in production
SavedUiStateGrantsAuthority == false
UnknownRedirectImplicitlyAllowed == false
NetworkEpochFreshnessRequired == true
TransportStateMachineCount == 1
ResourceBoundsConfigured == true
SystemBrowserProtectedFallback == false
```

All Android tasks inherit the global cybernetic-stability production gates for release trust, relay identity, explicit routing, resource bounds and unified failover.

## 3. Target repository structure

```text
android/
|- settings.gradle.kts
|- build.gradle.kts
|- gradle.properties
|- app/
|  |- build.gradle.kts
|  |- src/main/AndroidManifest.xml
|  |- src/main/java/.../WebGateApplication.kt
|  |- src/main/java/.../MainActivity.kt
|  |- src/main/java/.../ui/
|  |- src/main/java/.../navigation/
|  `- src/androidTest/
|
|- core-bridge/
|  |- build.gradle.kts
|  |- generated bindings
|  `- src/main/java/.../WebGateCore.kt
|
|- platform/
|  |- AndroidDeviceKeyStore.kt
|  |- AndroidAttestation.kt
|  |- ConnectivityObserver.kt
|  |- AppLinkRouter.kt
|  |- SecureStorage.kt
|  `- RuntimeObservationAdapter.kt
|
|- renderer/
|  |- src/main/aidl/.../IRenderer.aidl
|  |- src/main/aidl/.../IRendererHost.aidl
|  |- RendererService.kt
|  |- RendererProcessController.kt
|  `- SharedRegionController.kt
|
`- transport/
   |- AndroidTransportRuntime.kt
   `- WebGateVpnService.kt           # compatibility-only

crates/
`- webgate-android-api/
   |- Cargo.toml
   |- src/lib.rs
   |- src/session.rs
   |- src/platform_contract.rs
   `- src/uniffi.udl or equivalent interface definition
```

Do not place authoritative security policy in `MainActivity`, Compose state, Intent parsing code or ViewModels.

## 4. Milestone model

```text
M0  buildable shell
 |
M1  identity + Rust bridge
 |
M2  host security/runtime supervisor
 |
M3  real mobile transport
 |
M4  isolated STRICT renderer + broker IPC
 |
M5  compositor/resource isolation
 |
M6  lifecycle/network/background hardening
 |
M7  physical-device + release qualification
```

Each milestone may expose demos, but only M7 can produce Tier-1 production qualification.

---

# M0 — Android build and packaging foundation

## AND-001 — Gradle/Compose application shell

**Priority:** P0 foundation

Deliver:

- Gradle Kotlin DSL project under `android/`;
- application package identifier reserved and stable;
- Jetpack Compose host UI;
- `compileSdk >= 36`;
- `targetSdk = 36`;
- initial `minSdk = 30` unless a later compatibility study changes it;
- `arm64-v8a` production ABI;
- `x86_64` emulator/CI lane where useful;
- debug/release build types;
- deterministic version metadata contract.

Acceptance:

- clean checkout builds APK in CI;
- physical arm64 install launches the shell;
- release configuration does not accidentally enable debug flags;
- manifest permissions are enumerated and architecture-reviewed.

## AND-002 — Android CI matrix

Required lanes:

```text
Kotlin lint/static analysis
Gradle unit tests
Rust Android cross-build
arm64 JNI/AAR packaging
x86_64 emulator smoke test
manifest/security assertions
release artifact metadata test
```

CI must fail if STRICT renderer manifest accidentally gains `INTERNET` or another prohibited permission path.

---

# M1 — Rust bridge and hardware identity

## AND-003 — `webgate-android-api`

Create a dedicated Rust Android-facing crate rather than exporting arbitrary internals from existing crates.

Initial coarse API:

```text
AndroidClient::bootstrap()
AndroidClient::get_status()
AndroidClient::open_resource(opaque_id)
AndroidClient::close_resource(session_id)
AndroidClient::network_observation(...)
AndroidClient::resume()
AndroidClient::suspend()
AndroidClient::enrollment_status()
```

Rules:

- Android API returns typed status/evidence rather than many independent booleans;
- Kotlin cannot mutate Rust authoritative state directly;
- interface is versioned;
- panics do not cross FFI;
- strings/byte arrays are bounded before crossing FFI;
- secret material is never exposed through diagnostic DTOs.

Preferred binding lane: UniFFI for coarse control-plane calls. JNI is permitted only for missing platform capabilities and must remain narrow.

## AND-004 — Android Keystore ES256 backend

Implement a real Android-backed `DeviceKeyStore` adapter.

Contract:

```text
Generate -> Keystore alias
PublicKey -> exportable
PrivateKey -> non-exportable
Sign(challenge) -> Keystore operation
Delete -> invalidates identity locally
```

Never serialize the private key or seed into Rust-owned files.

Required tests:

- generate/sign/verify;
- app restart preserves the same key;
- APK upgrade preserves the same key;
- clear app data removes effective local enrollment;
- alias missing/corrupted state fails closed;
- signing requires only the narrow key operation;
- backup/restore onto another device cannot clone effective device identity.

## AND-005 — Hardware attestation evidence

Enrollment evidence model:

```text
AttestationEvidence {
  challenge
  certificate_chain
  key_algorithm
  security_level
  app_package_binding where available/required
}
```

Server owns attestation verification.

Client must never convert a local property into authoritative `TrustedEnvironment`/`StrongBox` status without server verification.

---

# M2 — Host security/runtime supervisor

## AND-006 — Android Runtime Supervisor

Replace the conceptual coupling of UI lifecycle and security state with one Android runtime supervisor consuming observations.

Suggested states:

```text
EMPTY
IDENTITY_READY
POLICY_READY
CONNECTING
ROUTE_READY
AUTHORIZING
AUTHORIZED
RENDERER_STARTING
OPEN
SUSPENDING
SUSPENDED
RECOVERING
DENIED
OFFLINE
```

Required evidence fields include:

```text
state_epoch
policy_epoch
network_epoch
route_evidence_epoch
authorization_expiry
renderer_generation
resource_budget_generation
```

State transitions must be centralized. Compose/ViewModel may observe state but cannot invent transitions.

## AND-007 — `network_epoch`

`ConnectivityManager.NetworkCallback` is converted into typed observations:

```text
NetworkAvailable
NetworkLost
CapabilitiesChanged
DefaultPathChanged
ValidationChanged
CaptivePortalSuspected
```

A material path change increments monotonic `network_epoch` and invalidates route evidence from older epochs.

Mandatory property:

```text
route_evidence.network_epoch != current.network_epoch
=> state != OPEN unless fresh route evidence is obtained
```

## AND-008 — Canonical destination and redirect broker

Android renderer, transport and App Link flows use the same canonical destination semantics as the rest of WebGate.

Every redirect is a fresh broker decision.

Unknown/invalid URL forms fail closed.

Differential tests must prove that Android and core policy decisions agree for:

- Unicode/IDNA hosts;
- wildcard boundaries;
- IPv4/IPv6 literals;
- percent encoding;
- dot segments;
- scheme/port combinations;
- redirects;
- intentionally malformed input.

---

# M3 — Native mobile transport

## AND-009 — In-process Android transport runtime

Do not port the desktop sidecar topology literally.

The Android host uses the canonical Transport Supervisor and qualified providers in-process where practical.

```text
Android Host
   |
Transport Supervisor
   |-- Primary relay/provider
   |-- Fallback relay/provider
   `-- optional compatibility provider
```

Hard requirement: Android does not gain its own independent failover state machine.

## AND-010 — Mobile path-recovery qualification

Test:

- Wi-Fi -> LTE/5G;
- LTE/5G -> Wi-Fi;
- temporary no-network;
- captive portal;
- DNS change;
- external system VPN appears/disappears;
- relay A failure while mobile path changes;
- app background/foreground during reconnect.

Measure:

```text
T_detect
T_invalidate
T_reconnect
T_reauthorize if required
T_render_resume
```

No stale `READY`/`OPEN` state is allowed during the gap.

## AND-011 — Optional `VpnService` compatibility adapter

Only after normal in-process mode works.

Rules:

- explicit user-visible mode;
- restricted to WebGate package when Android API permits the intended routing;
- no claim that VPN mode is the canonical product architecture;
- no automatic transition into VPN mode if that changes the user's device-wide network semantics;
- must coexist with global fail-closed policy.

---

# M4 — Isolated STRICT renderer

## AND-012 — Renderer Service manifest boundary

Create a non-exported renderer service:

```xml
<service
    android:name="...RendererService"
    android:exported="false"
    android:isolatedProcess="true" />
```

Qualification must demonstrate at runtime that the renderer process has an isolated UID and no direct network authority.

A negative test from code executing inside the renderer must attempt direct socket/network access and fail independently of WebGate policy.

## AND-013 — Versioned AIDL capability interface

Define two narrow interfaces:

```text
IRenderer
  createDocument(...)
  deliverResource(...)
  deliverInput(...)
  trimMemory(...)
  closeDocument(...)

IRendererHost
  requestResource(...)
  requestSharedRegion(...)
  publishFrame(...)
  reportRendererEvidence(...)
  reportFault(...)
```

No generic `execute(command: String)` or untyped arbitrary JSON command channel.

Every message carries:

```text
protocol_version
session_id
document_id
generation
sequence
```

Stale generation messages are rejected.

## AND-014 — Resource request capability protocol

Implement bounded request/response flow from ADR-0004.

Resource bodies larger than a small control-plane threshold use SharedMemory/FD capabilities rather than large Binder payloads.

The host owns:

- canonicalization;
- authorization;
- redirect handling;
- network;
- cookies/storage policy;
- response size limits;
- MIME/type policy;
- cancellation.

## AND-015 — Renderer death semantics

Binder death/renderer crash must transition the document away from `OPEN` immediately.

Recovery creates a new `renderer_generation`; old SharedMemory regions and messages are invalid.

Renderer crash loops are bounded with backoff and a maximum restart budget.

---

# M5 — Display and resource isolation

## AND-016 — Bounded DisplayList protocol

Define a versioned typed DisplayList schema with explicit limits.

At minimum validate:

```text
frame_bytes <= FRAME_MAX
commands <= COMMAND_MAX
resource_refs <= RESOURCE_REF_MAX
clip_depth <= CLIP_DEPTH_MAX
text_run_count <= TEXT_RUN_MAX
image_decode_bytes <= IMAGE_BUDGET
```

Malformed DisplayList input must never reach unchecked native GPU operations.

## AND-017 — Trusted compositor

Host process validates DisplayList and owns actual Android surface/GPU composition.

Renderer cannot submit arbitrary Vulkan/OpenGL command streams in STRICT.

Measure per-frame:

```text
layout_time
serialize_time
IPC_wait
validate_time
compose_time
frame_bytes
```

## AND-018 — Android Resource Governor

Budgets must exist at three levels:

```text
per-document
per-session
per-app/global
```

Tracked resources:

- renderer processes;
- outstanding resource requests;
- Binder transactions;
- SharedMemory bytes;
- decoded image bytes;
- JS/Wasm memory;
- DOM size;
- timers/tasks;
- frame queue depth;
- file descriptors;
- network streams.

The app must shed lower-value work before global exhaustion.

Mandatory adversarial test: one malicious/slow document cannot prevent another healthy protected document/session from remaining responsive within defined degradation bounds.

---

# M6 — Lifecycle, storage and background hardening

## AND-019 — Saved-state minimization

Only opaque navigation/UI hints are restored from Android saved state.

Test process death at every security state and prove that restart cannot resurrect:

- authorization;
- route evidence;
- renderer readiness;
- active capability handles;
- `OPEN`.

## AND-020 — Memory-pressure policy

Map Android memory pressure into deterministic actions:

```text
moderate -> trim caches
low      -> suspend background document work
critical -> kill/recreate least valuable renderer(s), preserve only revalidatable host state
```

No memory-pressure path may bypass broker authorization.

## AND-021 — Background-session policy

Do not depend on a forever-running `dataSync` foreground service.

Define explicit product states:

```text
VISIBLE_ACTIVE
BACKGROUND_GRACE
USER_VISIBLE_FOREGROUND_SESSION  # only if justified and compliant
SUSPENDED
```

Background work starts only under platform-permitted conditions. Timeouts and inability to start a foreground service become normal recoverable observations, not crashes or fail-open paths.

## AND-022 — Backup/restore policy

Explicitly classify every persisted item as:

```text
backup_allowed
backup_forbidden
restore_requires_revalidation
```

Device-bound identity is never considered restored unless the original non-exportable Keystore key is present and valid.

---

# M7 — UX, ecosystem and production qualification

## AND-023 — Verified App Links

Canonical link:

```text
https://<trusted-domain>/r/<opaque-id>
```

Use `android:autoVerify="true"` and Digital Asset Links.

Negative tests:

- unsigned/unassociated domain;
- forged path;
- expired invitation;
- malformed opaque ID;
- internal URL injection;
- relay/Origin selection fields in link;
- duplicate link delivery/replay where invitation is one-time.

## AND-024 — Compose secure workspace UX

Primary UX is a secure workspace launcher, not a generic browser chrome.

Required states are visible and distinguishable:

```text
Secure
Connecting
Reconnecting
Authorization required
Access denied
Offline
Renderer recovering
Policy update required
```

UI must not call a state `Secure` solely because TLS exists. The label derives from the combined security evidence contract.

## AND-025 — Physical-device qualification matrix

Minimum representative matrix before Tier-1:

```text
Google Pixel / AOSP-like current Android
Samsung current supported Android
Xiaomi/HyperOS current supported Android
at least one lower-memory arm64 device
```

Where practical include Android versions spanning the chosen `minSdk` to current API 36 behavior.

Scenarios:

- cold start;
- warm start;
- rotation/config recreation;
- background/process kill;
- screen lock/unlock;
- battery saver;
- low memory;
- Wi-Fi/cellular switching;
- external VPN coexistence;
- OS update/app update;
- clear data;
- reinstall;
- revoked device;
- relay failure;
- authority failure;
- malicious document/resource flood.

## AND-026 — Signed APK/AAB release qualification

The final evidence uses the same signed APK/AAB intended for distribution.

Verify:

- package signing identity;
- release manifest/digest against the global Release Trust Kernel;
- upgrade continuity;
- downgrade/rollback behavior;
- ABI contents;
- no debug signing/config;
- no prohibited renderer permissions;
- reproducible build metadata where achievable;
- crash/ANR observability contains no secrets.

---

# 5. Mathematical acceptance model

Android client state:

```text
X(t) = [A, I, P, N, T, Z, R, Q, M]
```

Control actions:

```text
U(t) = [admit, route, connect, authorize, spawn, revoke, shed, suspend, recover]
```

Disturbances:

```text
W(t) = [process_kill, memory_pressure, path_change, packet_loss,
        relay_failure, authority_failure, malicious_document, user_background]
```

Hard safety set `S` includes:

```text
RendererNetworkCapability = 0
Q <= Qmax
Memory <= Mmax
FD <= FDmax
Open => FreshIdentityEvidence
Open => FreshPolicyEvidence
Open => FreshRouteEvidence(current_network_epoch)
Open => AuthorizationValid
Open => RendererProof(current_generation)
```

Liveness target:

```text
if a qualified path, valid identity/policy and authorization remain available,
and resource pressure returns below the recovery threshold,
then the runtime eventually leaves RECOVERING/OFFLINE and can reach OPEN
without user reinstall/restart.
```

This property prevents Android network/lifecycle events from creating an absorbing failure state.

# 6. Quantitative targets to establish during M7

Initial measurement gates must produce real numbers rather than unverified fixed promises:

```text
cold-start-to-shell p50/p95
open-resource-to-first-frame p50/p95
Wi-Fi->cellular recovery p50/p95
cellular->Wi-Fi recovery p50/p95
renderer restart recovery p50/p95
steady-state host RSS
renderer RSS per document
SharedMemory high-water mark
battery drain active session
battery drain background grace
ANR rate under stress
crash-free session rate
```

Thresholds become release gates only after baseline data on physical devices exists.

# 7. Security test families

## IPC fuzzing

Fuzz:

- unknown protocol versions;
- oversized lengths;
- negative/overflow lengths;
- stale generations;
- duplicate sequence numbers;
- invalid capability IDs;
- use-after-close region references;
- malformed DisplayLists;
- binder death during transaction.

## Property tests

Examples:

```text
processDeath(state) => !restored(Open)
networkEpoch++ => !oldRouteEvidence.valid
rendererDeath => !Open
policyEpoch++ => oldPolicyCapability.invalid
closeDocument => allDocumentCapabilities.revoked
unknownRedirect => Deny
```

## Chaos tests

Inject:

- relay loss;
- packet delay/loss;
- authority timeout;
- network switch;
- renderer crash;
- host background/foreground;
- memory pressure;
- shared-region exhaustion;
- repeated link opens.

# 8. Planning dependencies

Critical dependency chain:

```text
Global Release Trust Kernel
        |
Relay per-node identity + explicit routing
        |
Canonical Transport Supervisor + Resource Governor
        |
Android M0/M1 foundation
        |
Android M2/M3 host + transport
        |
Android M4/M5 isolation
        |
Android M6 resilience
        |
Android M7 production qualification
```

Android implementation can begin before every global P0 is complete, but Android cannot be marked production-qualified until the global P0 gates that its release/data path depends on are closed.

# 9. Definition of Android Tier-1 DONE

Android Tier-1 is DONE only when:

1. exact signed release artifacts pass the global release trust path;
2. host/renderer process boundary is real on physical devices;
3. direct network attempts from STRICT renderer fail;
4. Keystore private identity key is non-exportable;
5. attestation policy is server-verified;
6. App Links are verified and opaque;
7. mobile network transitions invalidate stale evidence and recover safely;
8. process death does not resurrect security state;
9. bounded resources survive hostile document/load tests;
10. no system browser/WebView protected fallback exists;
11. SecureAcces deny/revocation remains authoritative;
12. optional VPN mode remains explicit and separate;
13. compatibility renderer claims are supported by measured target-site evidence;
14. Android physical-device matrix passes repeatably;
15. documentation, manifests, runtime observations and release status all agree.

Anything less is a milestone/demo/release candidate, not Tier-1 production qualification.
