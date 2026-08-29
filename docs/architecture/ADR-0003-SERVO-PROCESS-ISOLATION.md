# ADR-0003 — Servo Compromise-Containment Boundary

- **Status:** ACCEPTED
- **Date:** 2026-08-29
- **Scope:** Windows / Android first; Linux/macOS compatible
- **Related:** ADR-0001 browser engine, ADR-0002 cross-platform runtime

## Context

WebGate intentionally fixes Servo as its primary protected browser engine. During T-004 pre-flight research, current Servo source exposed a security-relevant platform limitation: Servo's internal multiprocess sandbox path is not supported on Windows or Android/ARM. Servo's own browser-engine documentation has also historically described multiprocess/sandboxing as incomplete outside Linux/macOS.

This changes the trust model. Rust memory safety substantially reduces some classes of browser-engine vulnerability, but it is not equivalent to a mature renderer sandbox. WebGate must therefore **not use the Servo process as the trust boundary for long-lived secrets or transport authority**.

## Decision

Treat the browser runtime as a potentially compromised component.

```text
┌─────────────────────────────────────────────────────────────┐
│ TRUSTED BROKER / CONTROL PLANE                              │
│                                                             │
│ device-key operations                                       │
│ bootstrap/policy verification                               │
│ SecureAcces login/refresh authority                         │
│ transport control                                           │
│ update trust roots                                          │
│ audit/security decisions                                    │
└───────────────────┬─────────────────────────────────────────┘
                    │ narrow authenticated capability IPC
                    │ no raw private keys
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ BROWSER CAPSULE — ASSUME COMPROMISE                         │
│                                                             │
│ Servo + page state + rendering/input                        │
│ short-lived resource/session capabilities only              │
│ no device private key                                       │
│ no bootstrap secret                                         │
│ no transport private/control credentials                    │
└───────────────────┬─────────────────────────────────────────┘
                    │ protected application-local network path
                    ▼
             destination-restricted proxy
                    │
                    ▼
                 private origin
```

The security architecture therefore has **two distinct containment layers**:

1. **Capability separation** — mandatory and cross-platform. Long-lived secrets and privileged control functions do not live in the browser-owned boundary.
2. **OS process sandbox/restriction** — defense in depth, implemented per platform when practical.

Capability separation is the portable invariant. OS sandbox details may differ by platform.

## Browser compromise assumptions

Security review must assume a malicious document or browser-engine defect could obtain arbitrary execution in the browser capsule.

Under that assumption, the browser capsule must not be able to:

- export a device private key;
- mint a new long-lived device identity;
- modify trusted policy roots;
- disable fail-closed mode;
- reconfigure arbitrary transport endpoints;
- obtain reusable bootstrap/enrollment secrets;
- authorize a workspace/resource not granted by SecureAcces;
- turn the WebGate proxy into a general Internet proxy;
- silently downgrade to direct protected-origin access.

The browser may necessarily possess an active, bounded web session capability sufficient to render resources the user is currently authorized to access. Compromise containment therefore focuses on limiting duration, scope, escalation and persistence.

## Trusted broker responsibilities

The trusted broker is a product security boundary, not merely a convenience service.

It owns or brokers:

- device signing through a platform `DeviceSigner`;
- access to hardware/OS-backed secret stores;
- verification of signed bootstrap and remote policy;
- SecureAcces session issuance/refresh/revocation orchestration;
- transport provider lifecycle and endpoint policy;
- security-sensitive update verification;
- production audit events that must not be forgeable by page JavaScript.

The broker API must use semantic capabilities, not generic commands.

Bad:

```text
execute(command, args)
read_secret(name)
write_config(json)
```

Good:

```text
prove_device(challenge)
request_scoped_web_session(resource_context)
get_effective_browser_policy()
report_browser_health(event)
```

## IPC principles

The eventual browser↔broker protocol must be:

- versioned;
- authenticated/bound to the current application instance;
- narrow and deny-by-default;
- length/budget bounded;
- replay resistant where messages carry security meaning;
- free from generic file/process/secret APIs;
- safe when the browser sends malformed, reordered or high-rate messages;
- fuzz/property tested.

The browser must never receive a raw device private key. Signing occurs inside the broker/platform signer.

## Platform strategy

### Windows

Target defense-in-depth architecture:

- browser capsule can become a separate process once rendering integration is proven;
- restricted token / AppContainer-style isolation should be investigated;
- Job Objects control lifetime and child-process escape;
- broker remains outside the browser sandbox;
- named-pipe/other local IPC is ACL-restricted and instance-bound;
- network authority should be limited to the WebGate protected proxy path where the chosen Windows sandbox mechanism permits it.

The exact AppContainer/network/rendering design requires a prototype before production commitment.

### Android

Android already isolates applications from other apps, but a second process under the same application UID is **not** a sufficient secret boundary against a fully compromised browser process.

The architecture therefore requires:

- Android Keystore key operations to remain behind a narrow broker/platform interface;
- no raw hardware-backed private-key extraction into Servo-owned memory;
- explicit research of isolated-service/Binder/remote-surface options for stronger process separation;
- lifecycle recovery without duplicating credentials into the browser process;
- browser compromise tests that verify privileged broker operations reject unauthorized calls.

An Android isolated process may have different permission/network characteristics; WebGate must verify these empirically rather than assume desktop process semantics.

### Linux

Prefer process isolation using the available desktop sandbox primitives (namespaces/seccomp/portal-compatible design) while retaining the same broker capability boundary.

### macOS

Prefer App Sandbox/XPC-style privilege separation where practical, again keeping the same portable broker contract.

## Network isolation vs browser-compromise isolation

These are separate properties and tests must not conflate them.

**Network fail-closed test:** Servo's normal network stack cannot reach a protected origin when the WebGate proxy/transport is unavailable.

**Compromise-containment test:** even arbitrary code in the browser capsule cannot obtain long-lived secrets or invoke privileged broker operations outside its granted capability set.

Passing the first does not prove the second.

## Effect on implementation order

T-004 may still pin Servo and compile its adapter because no production secrets exist at that stage.

Before device keys, reusable authenticated sessions or production transport credentials are introduced, WebGate must establish the broker boundary. The plan therefore adds a foundational task after the basic Servo/proxy proof and before privileged identity/control-plane integration.

## Consequences

### Positive

- Servo remains the primary engine without pretending Rust memory safety equals a renderer sandbox;
- browser compromise has a defined blast-radius target;
- device identity and transport authority can remain outside web-content reach;
- the same trust model works across Windows, Android, Linux and macOS;
- future Servo sandbox improvements become defense in depth rather than a prerequisite for correctness.

### Negative

- more IPC/lifecycle complexity;
- full process separation may add rendering/startup overhead;
- Android strong isolation may require platform-specific UI/surface work;
- browser session capabilities still require careful TTL/scope design.

## Verification requirements

Before production:

1. browser process/capsule cannot access raw device-key material;
2. malformed/unauthorized broker requests fail closed;
3. browser crash/restart cannot expand authorization;
4. broker crash cannot produce direct-network fallback;
5. transport credentials are absent from browser logs/memory-facing APIs;
6. platform isolation configuration is audited independently from application capability checks;
7. tests distinguish network escape from browser-compromise escalation.

## Evidence / upstream references

- Servo's current source contains target-specific sandbox handling where Windows, Android and ARM do not use the supported sandbox path.
- Servo project article “Building a browser using Servo as a web engine!” describes multiprocess/sandbox support as partial and historically limited to Linux/macOS, with Windows AppContainer work identified as needed.
- Servo remains cross-platform and embeddable; this ADR is a WebGate containment decision, not a rejection of Servo.

Primary references:

- https://docs.rs/crate/servo/0.5.0
- https://docs.rs/crate/servo/0.5.0/source/servo.rs
- https://servo.org/blog/2024/09/11/building-browser/
- https://servo.org/
