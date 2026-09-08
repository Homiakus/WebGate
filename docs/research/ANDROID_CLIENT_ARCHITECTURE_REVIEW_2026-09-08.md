# WebGate Android Client — Architecture Review

**Date:** 2026-09-08  
**Status:** accepted research input  
**Normative result:** `../architecture/ADR-0004-ANDROID-CLIENT-SECURITY-ARCHITECTURE.md`  
**Execution:** `../implementation/ANDROID_CLIENT_IMPLEMENTATION_PLAN.md`

## Executive conclusion

Android should not be treated as a reduced desktop WebGate client. The platform offers a stronger security primitive for the renderer than the original `browser -> localhost proxy` model: a protected renderer can run as an Android isolated service with no permissions of its own and communicate with a trusted host only through explicit process APIs.

The preferred Android architecture is therefore:

```text
trusted Android host
  identity / Keystore
  policy / authorization
  canonical destination broker
  canonical Transport Supervisor
  resource governor
  trusted compositor
        |
        | typed Binder/AIDL + bounded SharedMemory
        v
isolated WGBR renderer
  zero direct network capability
  no durable authority
```

This makes `RendererNetworkCapability = 0` an operating-system-enforced property rather than only a browser configuration convention.

## Current repository gap

The repository already contains useful Android-oriented abstractions:

- `crates/webgate-platform/src/android_lifecycle.rs` models Android lifecycle observations;
- `crates/webgate-platform/src/keystore.rs` defines a generic `DeviceKeyStore` boundary;
- `ADR-0002` already states that Android should use a thin Kotlin/Java shell and shared Rust security logic.

However, these are not yet a complete Android client. Missing production components include:

- Gradle/Compose application;
- signed APK/AAB packaging path;
- `webgate-android-api` Rust bridge;
- UniFFI/JNI bindings;
- real Android Keystore ES256 backend;
- server-verified Android key attestation flow;
- Binder/AIDL renderer protocol;
- isolated renderer service;
- SharedMemory bulk data protocol;
- Android-native transport runtime using the canonical Transport Supervisor;
- `network_epoch` integration;
- physical-device lifecycle/OEM qualification.

The existing file-backed Ed25519 keystore must not be used as the Android production device private-key store because Android provides a stronger non-exportable hardware-backed key boundary.

## Platform facts that drive the design

### Isolated process

Android documents `android:isolatedProcess="true"` for a service as a process isolated from the rest of the system with no permissions of its own; communication is through Service binding/starting APIs.

Reference: https://developer.android.com/guide/topics/manifest/service-element

### Shared memory

`android.os.SharedMemory` is a `Parcelable` anonymous shared-memory primitive and can be mapped read-only/read-write with protection controls. It is appropriate for explicitly bounded bulk cross-process buffers while Binder/AIDL remains the control plane.

Reference: https://developer.android.com/reference/android/os/SharedMemory

### Hardware-backed identity

Android key attestation can provide evidence that a key is hardware-backed and whether its attestation security level is `TrustedEnvironment` or `StrongBox`. Production verification must validate the certificate chain and revocation state on the server.

Reference: https://developer.android.com/privacy-and-security/security-key-attestation

### App Links

Android App Links use verified HTTPS associations via Digital Asset Links and the app signing certificate. They are preferable to an unverified custom URL scheme for the main trusted entry path.

Reference: https://developer.android.com/training/app-links/about

### Background constraints

Applications targeting modern Android cannot assume that they may start arbitrary foreground services while already in the background. Android 15+ also limits `dataSync` and `mediaProcessing` foreground-service background runtime to a shared six-hour budget per service type in a 24-hour period.

References:

- https://developer.android.com/develop/background-work/services/fgs/restrictions-bg-start
- https://developer.android.com/develop/background-work/services/fgs/timeout

Therefore the WebGate product model should be a bounded secure application session with fast recovery, not a mandatory forever-running mobile VPN daemon.

### API baseline

As of 2026-09-08, Google Play requires new applications and app updates submitted from 2026-08-31 to target Android 16 / API 36 or higher.

Reference: https://support.google.com/googleplay/android-developer/answer/11926878

The implementation should start directly at `targetSdk=36`.

## Threat model improvement

### Original proxy-dependent model

```text
renderer/browser
     |
     | expected to use proxy
     v
localhost proxy
     |
     v
protected transport
```

Failure class: a renderer/network-stack bug, alternate API, engine behavior or future integration error may create a path that does not traverse the intended proxy unless all network capability is comprehensively controlled.

### Capability model

```text
isolated renderer
     |
     | resource request only
     v
trusted broker
     |
     | policy + authorization + admission
     v
transport
```

The renderer receives bytes/capabilities, not a general network primitive.

This does not eliminate all renderer security work, but it narrows the consequence of renderer compromise substantially.

## Kotlin vs Rust

The correct split is not one-language purity.

Use Kotlin for Android framework ownership:

- Compose/UI;
- Activity/Service lifecycle;
- Binder/AIDL;
- Keystore;
- App Links;
- notifications;
- ConnectivityManager;
- optional VpnService.

Use Rust for portable security semantics:

- policy/canonicalization;
- authorization contracts;
- Transport Supervisor;
- session state;
- resource policy;
- release/config verification;
- renderer orchestration contract.

The bridge should be deliberately coarse. High-frequency renderer/frame transfer belongs to Binder/SharedMemory, not repeated Kotlin/Rust object conversion.

## Security-state model

Android lifecycle is not security state.

Use:

```text
X_android(t) = [A, I, P, N, T, Z, R, Q, M]
```

with:

- Android observation `A`;
- identity `I`;
- policy/epoch `P`;
- network/epoch `N`;
- transport `T`;
- authorization `Z`;
- renderer proof `R`;
- resource state `Q`;
- memory pressure `M`.

Protected `OPEN` is a conjunction of fresh evidence. Saved Android UI state cannot recreate that conjunction.

## Mobile network semantics

A phone changes physical path frequently. Introduce `network_epoch` and bind route evidence to it.

```text
network path material change
        => network_epoch++
        => old route evidence invalid
        => transport revalidation
        => authorization recheck where policy requires
        => renderer remains blocked from new protected fetches until fresh evidence
```

This prevents stale readiness after Wi-Fi/cellular/VPN changes.

## Browser/runtime recommendation

Use explicit modes:

- `STRICT`: isolated WGBR, zero direct network capability;
- `COMPAT`: Servo only after real Android target-site and security qualification;
- `SYSTEM_PUBLIC`: WebView/system surface for non-protected public/help content only.

The system browser cannot be an automatic fallback for protected content.

## Initial Android version/ABI recommendation

```text
compileSdk >= 36
targetSdk = 36
minSdk = 30 initially
production ABI = arm64-v8a
CI/emulator ABI = x86_64 where useful
```

`minSdk=30` is chosen to reduce compatibility branching for the first security-sensitive release. It can be lowered later only with an explicit compatibility/security qualification.

## Most important implementation risks

1. **IPC parser becomes a new attack surface.** Mitigation: typed/versioned AIDL, hard size limits, fuzzing, stale-generation rejection.
2. **SharedMemory exhaustion.** Mitigation: per-document/session/global byte budgets and generation-scoped capability revocation.
3. **Renderer crash/restart loops.** Mitigation: Binder death handling, bounded restart budget, backoff, no stale capability reuse.
4. **OEM lifecycle/background differences.** Mitigation: Pixel + Samsung + Xiaomi/HyperOS physical-device qualification.
5. **False readiness after network change.** Mitigation: `network_epoch` and evidence freshness.
6. **Keystore/backup ghost identity.** Mitigation: non-exportable key as identity anchor, backup exclusions, re-enrollment if key absent.
7. **Duplicated mobile failover semantics.** Mitigation: Android consumes the canonical Transport Supervisor; no Android-specific failover FSM.
8. **WGBR compatibility gap.** Mitigation: constrained WGWeb STRICT profile plus separately measured Servo COMPAT lane.
9. **Background-daemon assumption.** Mitigation: session-oriented runtime, suspend/recover model, explicit foreground operation only where Android permits and product behavior justifies it.
10. **Status inflation.** Mitigation: Android is Tier-1 only after signed-release physical-device qualification; build/demo states are not production states.

## Recommendation

Proceed with Android as a first-class security target after the global P0 release/relay/resource/failover work is converged enough to expose stable contracts. Begin M0/M1 in parallel because build shell, Rust bridge and Keystore work do not require all transport P0s to be complete.

The main architectural insight is that Android should strengthen WebGate rather than merely host it: use the OS process model to remove renderer capabilities structurally.
