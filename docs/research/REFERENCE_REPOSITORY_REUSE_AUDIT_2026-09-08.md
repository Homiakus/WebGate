# WebGate Reference Repository Reuse Audit

**Date:** 2026-09-08  
**Scope:** transport, relay, zero-trust service access, Android FFI, browser/runtime isolation, update trust, cryptography, control-plane patterns  
**Status:** architecture/reuse evidence; task state remains owned by `MASTER_PLAN.md`

---

## 1. Purpose

WebGate must remain intentionally opinionated about its security contract while avoiding unnecessary reinvention of mature infrastructure.

The goal of this audit is not to find one project that should replace WebGate. No examined project has the same complete contract:

```text
Application-scoped protected runtime
+ destination-restricted transport boundary
+ no OS default-route takeover
+ fail-closed protected navigation
+ per-device identity
+ SecureAcces authority separation
+ explicit relay/origin routing
+ bounded resources
+ release trust + anti-rollback
```

The useful strategy is therefore **selective reuse**:

1. keep WebGate-owned invariants and policy semantics;
2. adopt mature libraries where they eliminate commodity protocol/crypto/FFI work;
3. use strong projects as architectural references for state machines, supervision and failure handling;
4. reject any imported behavior that creates ambient network authority, generic proxying, implicit fallback, shared-secret trust concentration or readiness claims without evidence.

---

## 2. Decision vocabulary

Every external project is classified into one of three categories.

### ADOPT

A component may be introduced as a dependency after qualification if it materially reduces custom code without weakening WebGate invariants.

### REFERENCE

Study architecture, tests, state machines and operational patterns, but keep a WebGate-owned implementation or abstraction boundary.

### DO NOT IMPORT AS POLICY

Protocol mechanics or implementation ideas may be useful, but the external project's authorization/routing/fallback semantics must not become WebGate policy by accident.

---

## 3. Highest-value reference set

| Priority | Repository | Role for WebGate | Classification |
|---|---|---|---|
| S+ | `tailscale/tailcat` + selected `tailscale/tailscale` data-plane packages | userspace direct/relay data path, DERP bootstrap/relay-of-last-resort, NAT traversal reference | REFERENCE, possible bounded library evaluation |
| S+ | `cloudflare/cloudflared` | transport supervisor, reconnect/backoff, edge discovery, tunnel state, fallback evidence | REFERENCE |
| S+ | `quinn-rs/quinn` | QUIC streams/datagrams/multiplexing in Rust | ADOPT candidate |
| S | `openziti/ziti` | zero-trust identities, dark services, controller/data-plane separation, service policy | REFERENCE |
| S | `mozilla/uniffi-rs` | Rust↔Kotlin bindings for Android shell around shared Rust core | ADOPT candidate |
| A+ | `erebe/wstunnel` | Rust tunnel mechanics over WebSocket/HTTP2/WebTransport, SOCKS/HTTP proxy behavior, proxy traversal | REFERENCE |
| A+ | `rathole-org/rathole` | compact Rust reverse-tunnel client/server structure | REFERENCE |
| A+ | `servo/servo` | WebGate-owned renderer integration and process isolation evidence | existing strategic dependency/reference |
| A | `pomerium/pomerium` | identity-aware Go gateway/control-plane patterns | REFERENCE |
| S | `theupdateframework/rust-tuf`, `awslabs/tough` | TUF trust model, signed metadata, rollback/freeze protection | ADOPT/reference candidate after maturity review |
| S | `rustls/rustls` + vetted Ed25519/hash crates | TLS and cryptographic primitives | ADOPT candidate |

Repository names above are architectural references, not automatic dependency approvals. Any source dependency entering production must pass the supply-chain, licensing, maintenance, platform and security gates defined by this document and the implementation program.

---

## 4. `tailscale/tailcat` / Tailscale data plane

### Why it matters

Tailcat demonstrates a very close networking philosophy to WebGate:

- userspace networking;
- no requirement to modify machine routing tables or DNS;
- no root/admin requirement for its normal model;
- WireGuard-encrypted peer connectivity;
- DERP as bootstrap/relay path;
- NAT traversal with upgrade to direct UDP when possible;
- local forwarding/SOCKS style integration for applications that do not speak the transport natively.

This aligns with WebGate invariant **I-002 Application-scoped routing** and the target of a direct path as an optimization rather than a system-wide VPN.

### What to study

Priority reference areas:

```text
tailscale/tailcat
  ├── minimal userspace connection lifecycle
  ├── ephemeral connection addressing
  ├── direct-vs-relay evidence
  └── local forwarding boundaries

tailscale/tailscale
  ├── wgengine/magicsock
  ├── derp/
  ├── net/netcheck/
  └── netstack/
```

### What WebGate must keep

Do not delegate these semantics to Tailscale code:

- destination allowlist;
- SecureAcces authorization;
- relay/origin route authorization;
- device enrollment/revocation;
- policy epoch and release epoch;
- fail-closed browser state;
- admission/resource governor;
- readiness/qualification truth.

### Integration conclusion

Use Tailcat primarily as the reference for a future **authenticated rendezvous → relay bootstrap → direct upgrade → relay recovery** state machine. Evaluate code reuse only after the WebGate transport interface and threat model are stable.

---

## 5. `cloudflare/cloudflared`

### Why it matters

`cloudflared` contains mature operational decomposition for a long-lived outbound tunnel that must survive edge/path failure.

Useful concepts include:

- connection supervisor;
- explicit reconnect state;
- edge discovery;
- bounded retry/backoff;
- QUIC/HTTP2 transport selection;
- tunnel-state reporting;
- validation, metrics and tracing separation.

This maps directly to WebGate's open requirement for one authoritative failover/recovery supervisor.

### Recommended structural lesson

WebGate should converge toward a decomposition similar to:

```text
TransportSupervisor
  ├── PathInventory
  ├── HealthEvidence
  ├── RetryBudget
  ├── CircuitBreaker
  ├── SelectionPolicy
  ├── ConnectionLifecycle
  └── StateProjection
```

UI state, browser readiness and metrics must consume the supervisor projection; they must not independently infer transport truth.

### Do not copy blindly

WebGate transport selection is constrained by stronger security semantics than a generic outbound tunnel. A fallback is eligible only when it preserves:

```text
destination restriction
AND authenticated node identity
AND explicit routing
AND bounded resources
AND policy compatibility
AND fail-closed recovery
```

---

## 6. `quinn-rs/quinn`

### Why it matters

WebGate currently owns significant transport framing and multiplexing logic. The extended architecture also has an explicit requirement to eliminate connection-wide head-of-line failure semantics.

QUIC provides a mature substrate for:

- independent streams;
- connection-level congestion control;
- loss recovery;
- stream lifecycle;
- datagrams where needed;
- TLS-backed connection establishment;
- async Rust integration.

### Target layering

```text
WebGate capability/policy layer
        ↓
WebGate route + admission layer
        ↓
WebGate authenticated session/envelope
        ↓
Quinn / QUIC
        ↓
UDP path
```

Quinn must remain **below** WebGate policy. It does not decide who may access which service, which Origin a stream belongs to, or when a path is production-qualified.

### Expected benefit

If qualification succeeds, Quinn can delete or prevent large amounts of custom implementation in:

- multiplexing;
- stream isolation;
- retransmission/loss handling;
- congestion control;
- keepalive/transport liveness mechanics;
- parts of connection lifecycle.

### Required evaluation

Before adoption, benchmark and test:

- Android arm64 and Windows x86_64;
- UDP-blocked environments;
- memory per connection/stream;
- cancellation behavior;
- stream quota enforcement;
- idle/keepalive behavior;
- relay fan-in/fan-out;
- reconnect and migration semantics;
- chaos under packet loss/reordering;
- release binary size and startup cost.

---

## 7. `openziti/ziti`

### Why it matters

OpenZiti is the closest architectural relative in service-level zero-trust design:

- identity for users/devices/services/workloads;
- policy-driven service access rather than ambient network access;
- dark/private services;
- outbound-only connectivity from private networks;
- controller/control-plane separated from the forwarding plane;
- embedded SDK model in addition to tunnelers.

### What to use as reference

Study and compare WebGate against OpenZiti for:

- service identity model;
- enrollment lifecycle;
- revocation propagation;
- service registry;
- route authorization;
- dark origin patterns;
- controller/data-plane separation;
- explicit policy ownership.

### What not to replace

`SecureAcces` remains WebGate's authoritative application authorization boundary. OpenZiti is a design reference, not a replacement authority.

---

## 8. `mozilla/uniffi-rs`

### Why it matters

Android is a Tier-1 target, while WebGate intentionally centralizes security-sensitive policy and transport logic in Rust.

A generated FFI boundary allows the platform shell to remain small:

```text
Kotlin Android shell
  ├── Activity/UI
  ├── lifecycle hooks
  ├── Android Keystore adapter
  ├── notifications
  └── renderer surface
          ↕ generated typed bindings
Rust WebGate core
  ├── policy
  ├── identity orchestration
  ├── transport supervisor
  ├── session state
  └── qualification projection
```

### Integration objective

Prefer UniFFI over a growing handwritten JNI surface unless a measured incompatibility forces a narrower custom bridge.

### Boundary rule

Kotlin may own platform capability adapters, but it must not duplicate the authoritative WebGate state machines or authorization decisions.

---

## 9. `wstunnel` and `rathole`

These projects are especially useful references next to the existing WebGate transport files that implement SOCKS5, HTTP CONNECT, relay and failover behavior.

### `wstunnel`

Study:

- tunnel abstraction over multiple HTTP-class transports;
- WebSocket/HTTP2/WebTransport mechanics;
- SOCKS/HTTP proxy plumbing;
- TLS/mTLS integration;
- behavior behind existing proxies;
- keepalive and reconnect handling.

Do **not** import its generic tunneling semantics as WebGate policy. WebGate must never become an unrestricted forwarding tool.

### `rathole`

Study:

- compact Rust reverse-tunnel architecture;
- client/server control connection separation;
- configuration boundaries;
- minimal moving parts.

Use it as a complexity benchmark: if a WebGate transport component becomes much larger, document which additional security/resilience invariant justifies the complexity.

---

## 10. TUF / secure update references

WebGate already requires:

```text
local digest verification
+ real cryptographic signature verification
+ platform compatibility
+ version/epoch checks
+ anti-rollback
```

This problem belongs to the mature secure-update domain and must not be designed from first principles without comparison to TUF.

### Required TUF concepts to map

- root trust metadata;
- threshold/role separation where appropriate;
- signed versioned metadata;
- expiry/freeze protection;
- rollback protection;
- target hashes and lengths;
- key rotation;
- recovery authorization.

### Candidate implementations

- `theupdateframework/rust-tuf` — reference and candidate subject to maturity review;
- `awslabs/tough` — Rust TUF implementation/tooling candidate.

No dependency is approved merely by this audit. The implementation program requires an explicit adoption decision after compatibility and maintenance review.

---

## 11. `rustls` and removal of home-grown cryptographic primitives

### Finding

The current repository contains a pure safe-Rust SHA-512 implementation inside the Ed25519 path. Safe Rust removes memory-unsafety classes, but it does not remove cryptographic implementation risk.

For WebGate, cryptography is not a differentiating product capability. It is a trust foundation.

### Architectural rule

Prefer well-maintained, independently reviewed ecosystem cryptographic implementations over WebGate-owned primitive implementations unless a documented platform or assurance requirement makes external use impossible.

Target split:

```text
WebGate owns
  ├── what is signed
  ├── canonicalization
  ├── key roles
  ├── trust roots
  ├── epochs
  ├── rotation/revocation policy
  └── fail-closed verification semantics

Vetted crypto libraries own
  ├── SHA-2
  ├── Ed25519 arithmetic/verification
  ├── TLS 1.3 machinery
  ├── AEAD primitives
  └── constant-time low-level operations
```

`rustls` is the preferred TLS reference/candidate. Exact Ed25519/hash providers must be selected in a separate dependency decision with license, maintenance and platform evidence.

---

## 12. `Servo`

Servo remains relevant because WebGate requires an owned protected renderer path rather than a silent system-browser fallback.

The important architectural boundary is not simply "embed Servo". The renderer must be network-incapable except through explicitly granted WebGate capabilities.

Target model:

```text
WebGate policy
     ↓
WGBR broker
     ↓ typed capability IPC
qualified renderer process
     ↓
NO ambient/raw network capability
```

Servo upgrades remain subject to compatibility, network-escape, crash/recovery and performance qualification.

---

## 13. `Pomerium`

Use Pomerium as a Go control-plane/gateway reference for:

- route policy separation;
- identity-aware request handling;
- observability around policy decisions;
- configuration lifecycle;
- control/data-plane boundaries.

Do not copy an HTTP reverse-proxy-centric trust model into the relay layer where WebGate intends payload opacity and end-to-end sessions.

---

## 14. Recommended target dependency shape

The desired result is not dependency maximalism. It is a narrow stack where WebGate owns the differentiating logic and mature components own commodity protocol machinery.

```text
┌──────────────────────────────────────────┐
│          Android / Windows shell         │
│ Kotlin / native platform integration     │
└──────────────────┬───────────────────────┘
                   │ UniFFI candidate
┌──────────────────▼───────────────────────┐
│               WebGate Rust              │
│                                          │
│ WebGate-owned:                           │
│   capability policy                      │
│   destination restrictions               │
│   device/session orchestration            │
│   route authorization                    │
│   admission/resource governor            │
│   failover supervisor semantics           │
│   readiness/qualification truth           │
│                                          │
│ Candidate commodity substrate:           │
│   Quinn → QUIC streams                    │
│   rustls → TLS                            │
│   vetted Ed25519/SHA-2 crates             │
│   TUF-compatible update verification      │
└──────────────────┬───────────────────────┘
                   │
          direct / relay paths
                   │
┌──────────────────▼───────────────────────┐
│              WebGate Go plane           │
│                                          │
│ WebGate-owned:                           │
│   SecureAcces authority boundary         │
│   service/device/origin registries        │
│   explicit routing                       │
│   admission/resource control             │
│   audit/durable state                     │
│                                          │
│ References: OpenZiti / Pomerium /         │
│             cloudflared                   │
└──────────────────┬───────────────────────┘
                   │
              Origin Agent
                   │
             private service
```

---

## 15. Highest-leverage simplifications

### P0-equivalent architectural leverage

1. **Stop expanding home-grown crypto primitives.** Replace them behind compatibility vectors and negative tests.
2. **Run a Quinn spike before extending WGRL multiplexing.** If Quinn satisfies constraints, move stream mechanics below the WebGate session/policy layer.
3. **Converge transport truth into one supervisor.** Use cloudflared as the decomposition reference and Tailcat/Tailscale for direct/relay path semantics.
4. **Keep direct path as a verified optimization.** Authenticated rendezvous first; relay remains a safe recovery path.
5. **Use UniFFI for Android unless evidence rejects it.** Avoid duplicating Rust state machines in Kotlin/JNI.
6. **Map release trust to TUF concepts before adding more custom updater protocol.**
7. **Use OpenZiti as a standing architecture benchmark for identity/service isolation, not as a replacement authority.**

---

## 16. Dependency admission gate

Any new runtime dependency must have recorded evidence for:

- maintained upstream and release cadence;
- supported license compatible with WebGate distribution;
- known security/advisory process;
- acceptable transitive dependency graph;
- Android arm64 + Windows x86_64 compatibility for Tier-1 paths;
- deterministic/reproducible build impact where relevant;
- binary size/startup/runtime memory budget;
- ability to enforce WebGate resource bounds;
- fuzz/negative-test surface for protocol parsers;
- failure behavior under malformed/untrusted input;
- no hidden telemetry/control-plane dependency;
- no ambient system routing/DNS mutation;
- no forced vendor service dependency;
- update/rollback implications;
- SBOM and license inventory inclusion.

A dependency that passes functional tests but cannot satisfy the security boundary is rejected.

---

## 17. Anti-goals

This reuse program must **not** turn WebGate into:

- a generic VPN;
- an open SOCKS/HTTP proxy;
- a Tailscale/OpenZiti clone;
- a cloudflared wrapper;
- a generic QUIC tunnel;
- a renderer with ambient networking;
- a platform where third-party libraries own authorization policy;
- a collection of dependencies without a single authoritative state model.

---

## 18. Success criterion

The reuse program succeeds when WebGate has **less custom infrastructure code and fewer independent state machines while preserving or strengthening all security invariants**.

The preferred direction is:

```text
less custom protocol machinery
+ fewer duplicate state machines
+ more mature crypto/transport substrate
+ stronger qualification
+ unchanged WebGate policy ownership
= lower implementation risk and faster convergence
```
