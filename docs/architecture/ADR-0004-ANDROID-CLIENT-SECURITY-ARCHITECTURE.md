# ADR-0004 — Android Client Security Architecture

- Status: **ACCEPTED**
- Date: **2026-09-08**
- Scope: Android arm64 Tier-1 client
- Supersedes: Android-specific renderer/transport details in `ADR-0002-CROSS-PLATFORM-RUNTIME.md`
- Related: `ADR-0001-BROWSER-ENGINE.md`, `ADR-0002-CROSS-PLATFORM-RUNTIME.md`, `ADR-0003-SERVO-PROCESS-ISOLATION.md`, `TARGET_ARCHITECTURE.md`, `../implementation/ANDROID_CLIENT_IMPLEMENTATION_PLAN.md`, `../../MASTER_PLAN.md`

## 1. Decision summary

Android is not a thin port of the desktop process model. It is a first-class WebGate runtime that uses Android process isolation, Keystore, Binder/AIDL, SharedMemory and lifecycle/network observations as security primitives.

The canonical Android architecture is:

```text
verified App Link / explicit user action
              |
              v
+--------------------------------------------------+
| Trusted Android Host Process                     |
|                                                  |
| Jetpack Compose UI                               |
| Android lifecycle / App Links                    |
| Android Keystore / device proof                  |
| policy + authorization broker                    |
| canonical destination broker                     |
| Transport Supervisor                             |
| relay / QUIC / TLS transport                     |
| persistence broker                               |
| resource governor                                |
| trusted compositor                               |
|                                                  |
| INTERNET permission may exist here               |
+----------------------+---------------------------+
                       |
                       | typed Binder/AIDL capability IPC
                       | bounded SharedMemory regions
                       v
+--------------------------------------------------+
| WGBR STRICT Renderer Service                     |
| android:isolatedProcess="true"                   |
|                                                  |
| DocumentActor                                    |
| DOM / style / layout                             |
| QuickJS-in-Wasm                                  |
| bounded DisplayList generation                   |
|                                                  |
| NO network permission                            |
| NO application permissions                       |
| NO direct persistent security state              |
| NO direct authorization authority                |
+--------------------------------------------------+
```

The principal Android security invariant is therefore a capability invariant rather than a proxy convention:

```text
RendererNetworkCapability = 0
```

A protected document is never allowed to depend on the renderer voluntarily choosing the correct proxy. The renderer has no direct network authority in STRICT mode.

## 2. Why Android is architecturally different

Android provides useful kernel/framework-enforced primitives that are stronger than emulating the desktop model:

- `android:isolatedProcess="true"` gives a Service a special isolated process with no permissions of its own; communication is through the Service API;
- Binder/AIDL provides an explicit typed process boundary;
- `android.os.SharedMemory` is `Parcelable` and supports bounded anonymous shared-memory regions;
- Android Keystore can hold non-exportable hardware-backed device keys and expose attestation evidence;
- `ConnectivityManager.NetworkCallback` exposes path changes that must invalidate stale transport evidence;
- verified Android App Links bind HTTPS links to the installed signed application;
- Android background-execution limits make a forever-running desktop-style daemon a poor default execution model.

Normative external references:

- Android `<service>` manifest element / `isolatedProcess`: https://developer.android.com/guide/topics/manifest/service-element
- Android `SharedMemory`: https://developer.android.com/reference/android/os/SharedMemory
- Android key attestation: https://developer.android.com/privacy-and-security/security-key-attestation
- Android App Links: https://developer.android.com/training/app-links/about
- Android foreground-service restrictions: https://developer.android.com/develop/background-work/services/fgs/restrictions-bg-start
- Android foreground-service timeouts: https://developer.android.com/develop/background-work/services/fgs/timeout
- Google Play target API policy: https://support.google.com/googleplay/android-developer/answer/11926878

As of 2026-09-08, Google Play requires new Android mobile apps and updates submitted after 2026-08-31 to target Android 16 / API 36 or higher. The WebGate Android baseline therefore targets API 36 from the first production build rather than designing around an obsolete target SDK.

## 3. Process and authority model

### 3.1 Trusted Host Process

The host owns authority-bearing functions:

```text
AndroidHost
  |- UI/navigation
  |- App Link verification boundary
  |- device identity operations
  |- policy verification
  |- authorization session
  |- CanonicalDestination resolution
  |- redirect re-authorization
  |- transport state
  |- resource admission
  |- downloads/uploads
  |- cookies/storage broker
  |- renderer lifecycle
  `- trusted composition
```

The host may have `INTERNET`; the STRICT renderer must not.

### 3.2 Isolated Renderer Process

The renderer is treated as attacker-controlled after parsing arbitrary remote content.

It may receive only explicit capabilities needed to render the current document. It must not receive:

- relay credentials;
- SecureAcces credentials;
- Android Keystore handles that permit arbitrary signing;
- raw policy signing keys;
- unrestricted filesystem paths;
- arbitrary sockets;
- route selection authority;
- direct Android content-provider authority;
- unrestricted GPU/native command authority.

Renderer compromise must not imply direct network compromise or durable identity compromise.

### 3.3 Compatibility renderers

Renderer modes are separated explicitly:

| Mode | Engine | Intended role | Network authority |
|---|---|---|---|
| `STRICT` | WGBR isolated renderer | canonical protected WebGate content | none |
| `COMPAT` | Servo after measured qualification | complex target sites not yet supported by WGBR | brokered only; no silent direct fallback |
| `SYSTEM_PUBLIC` | Android WebView/system browser surface | public/help content only | never eligible for protected-session qualification |

There is no automatic downgrade from `STRICT` or `COMPAT` protected content into `SYSTEM_PUBLIC`.

## 4. Renderer request broker

The renderer never performs arbitrary URL fetches. It asks the trusted host for a resource capability.

Conceptual request:

```text
ResourceRequest {
  document_id
  request_id
  canonical_or_raw_url
  method_class
  resource_type
  redirect_depth
  initiator_origin
  policy_epoch_seen
  network_epoch_seen
}
```

Trusted path:

```text
Renderer
   |
   | ResourceRequest
   v
CanonicalDestination Broker
   |
   v
Policy / Redirect Authorization
   |
   v
Admission + Resource Governor
   |
   v
Transport Supervisor
   |
   v
Relay -> Origin -> Gateway -> SecureAcces -> Service
```

The response is capability-scoped:

```text
ResourceResponse {
  request_id
  status
  sanitized_headers
  mime_class
  body_capability
  body_length_limit
  policy_epoch
  network_epoch
}
```

Redirects are not blindly followed by the renderer or host HTTP stack. Every redirect becomes a new canonical destination and authorization decision.

## 5. Binder/AIDL control plane and SharedMemory data plane

Binder/AIDL is the control plane. Large bodies and display data must not be copied as arbitrarily large Binder parcels.

Use bounded `SharedMemory` or file-descriptor-backed capabilities for:

- response bodies;
- decoded image surfaces where justified;
- DisplayList buffers;
- other explicitly sized bulk payloads.

Every shared region has:

```text
RegionCapability {
  capability_id
  owner_document
  purpose
  max_size
  actual_size
  generation
  read_write_mode
  expiry
}
```

Host validates size and generation before mapping/consuming the region. Closing a document revokes all document-scoped regions.

## 6. DisplayList / GPU boundary

STRICT renderer does not issue arbitrary native/GPU drawing commands.

```text
DOM + JS
   |
   v
Style/Layout
   |
   v
Typed DisplayList
   |
   v
SharedMemory
   |
   v
Host Validator
   |
   v
Trusted Compositor
   |
   v
Android Surface
```

Hard limits must exist for at least:

- DisplayList bytes per frame;
- DOM nodes;
- images/fonts/resources per document;
- decoded image bytes;
- renderer heap;
- Wasm memory;
- JS execution budget;
- task/timer count;
- outstanding resource requests;
- frame production rate.

A renderer that exceeds a limit is throttled, fails the document deterministically, or is killed. It must not transfer unbounded work to the trusted host.

## 7. Kotlin/Rust boundary

The architecture deliberately uses both languages.

### Kotlin owns Android framework boundaries

- `Activity` / Compose UI;
- `Service` binding;
- Binder/AIDL stubs;
- App Links;
- Android Keystore calls;
- notifications;
- foreground-service policy where required;
- optional `VpnService` compatibility adapter;
- framework connectivity callbacks.

### Rust owns shared security semantics

- canonical policy model;
- canonical destinations;
- security/session state machine;
- authorization contracts;
- relay protocol;
- Transport Supervisor;
- resource-governor policy;
- release/config verification;
- renderer orchestration contracts.

A dedicated `webgate-android-api` `cdylib`/AAR-facing boundary should expose a deliberately small API. UniFFI is preferred for coarse control-plane calls; JNI can be used only where UniFFI cannot express the required Android boundary cleanly.

Do not use UniFFI/JNI as a high-frequency frame-copy transport. Binder + SharedMemory owns cross-process renderer traffic.

## 8. Device identity and Android Keystore

The production Android device key is generated inside Android Keystore and is non-exportable.

Preferred canonical algorithm:

```text
ECDSA P-256 / ES256
```

Security level is surfaced as evidence:

```text
SOFTWARE < TRUSTED_ENVIRONMENT < STRONGBOX
```

Server policy may require a minimum level for sensitive resources.

Example policy concept:

```text
public/internal docs   -> software or better
engineering services  -> TEE or better
high-privilege admin   -> StrongBox where available + explicit user authentication policy
```

Hardware attestation is verified server-side. Verification must include certificate-chain signature validation, challenge binding, security level and revocation checks. Device enrollment cannot trust a client-declared string such as `strongbox=true`.

Key purposes remain separate:

- device authentication key;
- transport/node identity where applicable;
- release verification trust roots;
- authorization/session material.

The existing file-backed Ed25519 keystore is not a production Android identity backend.

## 9. Android runtime state model

Android UI lifecycle is an observation source, not the security state machine.

The existing `AndroidLifecycleProbe` remains useful as a testable adapter/probe, but it must not become authoritative for protected-session readiness.

Canonical Android security state vector:

```text
X_android(t) = [A, I, P, N, T, Z, R, Q, M]
```

where:

- `A` = Android lifecycle observation;
- `I` = identity/key evidence;
- `P` = policy state + `policy_epoch`;
- `N` = network path + `network_epoch`;
- `T` = Transport Supervisor state;
- `Z` = SecureAcces authorization state;
- `R` = renderer proof/state;
- `Q` = resource/admission state;
- `M` = memory-pressure state.

Protected `OPEN` requires conjunction, never a fallback OR:

```text
OPEN = IdentityValid
    && PolicyValid
    && RouteEvidenceFresh
    && AuthorizationValid
    && RendererProofValid
    && ResourceBudgetAvailable
```

Saved Android UI state cannot restore these booleans directly.

## 10. Process death / recreation

Android may kill the app process without a security cleanup callback. `onDestroy()` is therefore not an authoritative revocation or persistence event.

Saved UI state may contain only non-authoritative navigation hints, for example:

- opaque resource ID;
- selected service;
- scroll/UI hints;
- tab identity.

It must not persist/restore:

- `authorized=true`;
- `transport_ready=true`;
- `renderer_ready=true`;
- route proof;
- active capability handles;
- access tokens in plain saved state.

Recovery sequence:

```text
restore opaque navigation intent
        |
        v
reload device identity evidence
        |
        v
verify policy + policy_epoch
        |
        v
recreate transport / route evidence
        |
        v
re-authorize
        |
        v
spawn/rebind isolated renderer
        |
        v
obtain renderer proof
        |
        v
OPEN
```

## 11. Network epoch and mobile path changes

Android frequently transitions between Wi-Fi, cellular, VPN and captive-portal states. Stale path readiness must never survive a material route change.

Introduce monotonic:

```text
network_epoch
```

A material connectivity/path change increments the epoch. All route/transport evidence is tagged with the epoch that produced it.

```text
RouteReady(network_epoch = n)
```

is not valid after:

```text
network_epoch = n + 1
```

The Transport Supervisor then performs bounded reconnection/revalidation. UI must distinguish `RECONNECTING` from `READY`.

## 12. Transport execution model

Android normal mode uses in-process WebGate transport rather than desktop sidecars.

```text
Host Process
   |
Transport Supervisor
   |-- relay A (QUIC/TLS where qualified)
   |-- relay B (independent failure domain)
   `-- H2/TLS or other explicitly qualified compatibility provider
```

This transport consumes the same explicit-routing, per-node identity, admission-control and failover contracts defined by the cybernetic stability program. Android must not fork a second independent failover implementation.

### Optional `VpnService` compatibility mode

`VpnService` is not the default WebGate architecture. It is allowed only where a provider requires TUN/IP semantics and must be restricted to the WebGate package with Android per-app VPN controls where possible.

Normal mode remains application-scoped and should coexist with unrelated applications and, where Android permits, another system VPN better than a default WebGate VPN design.

## 13. Background execution policy

WebGate is a secure application-session client, not a forever-running mobile VPN daemon.

Canonical lifecycle:

```text
user opens/activates WebGate
        |
        v
establish bounded protected session
        |
        v
foreground/visible use
        |
        v
background transition
        |
        +--> bounded grace / explicit user-visible foreground mode if justified
        |
        `--> suspend transport and recover quickly on resume
```

Design must not depend on unlimited `dataSync` foreground-service runtime. Android 15+ limits `dataSync` and `mediaProcessing` foreground-service background execution to a shared six-hour budget per type in a 24-hour window, and Android restricts starting foreground services from the background.

## 14. Verified App Links

Trusted mobile entry uses verified HTTPS Android App Links:

```text
https://<trusted-domain>/r/<opaque-resource-id>
```

The link may carry only opaque/bounded identifiers or short-lived invitation material. It must not carry authoritative:

- internal service URL;
- relay address;
- OriginID selected by the client;
- bearer credential;
- permission set.

The host resolves the opaque identifier against signed policy/server-owned routing.

## 15. Persistence and backup

Persist only durable, revalidatable state:

- device public metadata;
- signed policy/config + epoch;
- resource catalogue metadata;
- UI preferences;
- explicitly encrypted refresh material where the protocol requires it.

Do not persist as authoritative truth:

- active `OPEN` state;
- transport readiness;
- renderer readiness;
- route evidence;
- bearer access tokens in plaintext;
- renderer capabilities.

Device identity/session material must be excluded from Android backup/restore if restoration to another device could create a ghost-device identity. Restoring app data onto a different physical device must require re-enrollment whenever the original non-exportable key is absent.

## 16. Build baseline

Canonical first production baseline:

```text
compileSdk >= 36
targetSdk = 36
production ABI = arm64-v8a
CI/emulator ABI = x86_64 where useful
recommended initial minSdk = 30
```

`minSdk=30` is an architectural recommendation, not an eternal product constraint. Lower versions may be added later behind an explicit compatibility study; lowering `minSdk` must not weaken security invariants.

## 17. Android-specific invariants

- **A-I-001 Renderer has zero direct network capability in STRICT mode.**
- **A-I-002 Protected navigation cannot occur before fresh policy, route, authorization and renderer evidence exist.**
- **A-I-003 Android UI/lifecycle state cannot grant security state.**
- **A-I-004 Process death causes revalidation, not state resurrection.**
- **A-I-005 Material network changes invalidate route readiness through `network_epoch`.**
- **A-I-006 Device private keys are non-exportable in production Android builds.**
- **A-I-007 Hardware security level is evidence, not a client claim.**
- **A-I-008 Every redirect is canonicalized and re-authorized by the trusted host.**
- **A-I-009 Renderer/host IPC is typed, bounded and document-capability scoped.**
- **A-I-010 Renderer cannot directly persist protected cookies/storage or durable authority.**
- **A-I-011 DisplayList and bulk buffers are explicitly size/resource bounded.**
- **A-I-012 `VpnService` is compatibility-only unless a future ADR explicitly changes the product model.**
- **A-I-013 No system-browser/WebView fallback is permitted for protected content.**
- **A-I-014 Android uses the canonical Transport Supervisor; it cannot fork failover semantics.**
- **A-I-015 App Links contain opaque resource identity, not long-lived authorization authority.**

## 18. Tier-1 acceptance

Android may be called Tier-1 production-qualified only after physical-device evidence proves:

1. signed APK/AAB installs and upgrades without changing the enrolled device identity;
2. clearing app data removes local enrollment and requires re-enrollment;
3. hardware-backed ES256 proof works on supported hardware and server-side attestation policy behaves correctly;
4. verified App Links open only the intended opaque resource flow;
5. STRICT renderer runs as an isolated process with no direct network authority;
6. direct renderer network attempts fail independently of WebGate policy code;
7. resource broker rejects unauthorized destinations and redirect pivots;
8. one slow/malicious document cannot exhaust global IPC/shared-memory/resource budgets;
9. Wi-Fi <-> cellular transitions increment `network_epoch`, invalidate stale route proof and recover safely;
10. process kill/restart cannot resurrect `OPEN`, authorization or route readiness;
11. low-memory pressure sheds renderer/resource state without bypassing policy;
12. transport loss fails closed;
13. SecureAcces revocation is enforced;
14. unrelated Android apps remain outside normal WebGate routing;
15. optional `VpnService` mode is explicitly entered and package-scoped;
16. background execution remains correct under Android foreground-service restrictions;
17. release-binary qualification uses the exact signed APK/AAB artifacts intended for distribution.

## 19. Consequences

### Positive

- renderer RCE is strongly separated from direct network authority;
- Android's OS process model becomes part of WebGate's security proof;
- Kotlin remains small and platform-specific while shared security semantics remain in Rust;
- lifecycle/process death becomes an explicit evidence-recovery problem instead of implicit UI state restoration;
- mobile path changes become mathematically visible through epochs;
- Android can reach a cleaner capability architecture than a desktop browser wrapped around a localhost proxy.

### Cost / risk

- Binder/AIDL and SharedMemory contracts become security-critical and require fuzz/property/size-bound tests;
- WGBR renderer integration is more work than embedding a WebView;
- process separation creates explicit serialization/versioning requirements;
- physical-device testing across OEMs is mandatory;
- Samsung/Xiaomi/other OEM background and memory behavior must be included in qualification;
- Servo compatibility remains a measured fallback lane until both security and target-site coverage are demonstrated.

## 20. Supersession rule

Where older Android documentation states or implies that the canonical protected Android path is simply:

```text
Servo AAR -> localhost proxy -> transport
```

that statement is superseded by this ADR.

The canonical STRICT path is:

```text
Trusted Android Host/Broker
        <-> capability IPC
Isolated WGBR Renderer (zero network capability)
```

Servo remains an explicitly qualified COMPAT lane, not the sole Android security architecture.
