# ADR-0005 — Reference Reuse and Dependency Boundaries

**Status:** Accepted for architecture direction  
**Date:** 2026-09-08  
**Decision owner:** WebGate architecture  
**Execution status owner:** `MASTER_PLAN.md`

---

## Context

WebGate has reached a stage where several open production blockers sit in domains that already have mature external implementations or reference architectures:

- QUIC/multiplexed transport;
- TLS and cryptographic primitives;
- secure update metadata and anti-rollback;
- Rust↔Kotlin FFI;
- long-lived tunnel supervision;
- direct/relay path management;
- zero-trust service identity and dark-service architecture.

Continuing to implement every mechanism in-house increases verification burden, implementation surface and the number of independent state machines.

At the same time, WebGate has non-negotiable product-specific invariants that external projects do not own:

```text
application-scoped routing
fail-closed protected navigation
destination restriction
SecureAcces authority separation
explicit origin/service routing
per-device/node trust semantics
bounded resource governance
truthful readiness/qualification
```

The project therefore needs a formal boundary between **what WebGate must own** and **what mature dependencies/reference implementations may own**.

---

## Decision

WebGate adopts a **policy-owned / mechanism-reused** architecture.

### WebGate MUST own

- capability model;
- destination restrictions;
- authoritative transport/failover state machine semantics;
- route authorization and origin/service binding;
- SecureAcces application-authorization boundary;
- device/node enrollment, revocation and trust-role semantics;
- admission/resource budgets;
- release acceptance policy and anti-rollback semantics;
- renderer capability boundary;
- readiness and production qualification state.

### Mature external components MAY own

After qualification:

- QUIC stream mechanics and congestion/loss recovery;
- TLS protocol implementation;
- SHA/Ed25519/AEAD low-level primitives;
- generated Rust↔Kotlin FFI bindings;
- TUF-compatible metadata parsing/verification;
- other commodity protocol machinery that does not decide WebGate policy.

### Reference implementations MAY influence

- supervisor decomposition;
- retry/backoff and circuit-breaker patterns;
- direct/relay path lifecycle;
- control/data-plane separation;
- dark-service/origin architecture;
- observability and operational state projection;
- reverse-tunnel process decomposition.

They MUST NOT become implicit policy owners.

---

## Approved reference set

The initial standing reference set is:

- `tailscale/tailcat` and selected `tailscale/tailscale` data-plane code;
- `cloudflare/cloudflared`;
- `quinn-rs/quinn`;
- `openziti/ziti`;
- `mozilla/uniffi-rs`;
- `erebe/wstunnel`;
- `rathole-org/rathole`;
- `servo/servo`;
- `pomerium/pomerium`;
- `theupdateframework/rust-tuf`;
- `awslabs/tough`;
- `rustls/rustls`;
- a separately selected vetted Ed25519/SHA-2 provider set.

The detailed reasoning is recorded in `docs/research/REFERENCE_REPOSITORY_REUSE_AUDIT_2026-09-08.md`.

---

## Dependency boundary

The canonical layering is:

```text
WebGate policy/capabilities
        ↓
WebGate route + admission + supervisor
        ↓
WebGate session/trust protocol
        ↓
qualified commodity mechanism
        ↓
OS/network
```

A dependency is invalid if application code must bypass WebGate policy to use it.

Examples:

### Valid

```text
WebGate decides permitted Origin/Service
        ↓
WebGate reserves stream budget
        ↓
Quinn opens an independent QUIC stream
```

### Invalid

```text
Quinn/Tailscale/OpenZiti chooses an arbitrary reachable service
        ↓
WebGate treats reachability as authorization
```

---

## Transport decision

Before extending custom WGRL multiplexing, WebGate must run a bounded Quinn/QUIC feasibility tranche.

If the candidate satisfies Tier-1 compatibility, resource governance, fail-closed recovery and relay requirements, the preferred architecture becomes:

```text
WebGate authenticated session/routing envelope
        ↓
QUIC streams via Quinn
```

rather than implementing general stream reliability/congestion/multiplexing machinery from scratch.

WGRL/1 remains a compatibility/evidence path until an explicit migration decision is qualified.

---

## Cryptography decision

WebGate will not expand home-grown cryptographic primitives as the default direction.

The project retains ownership of:

- canonical signed bytes;
- role/key semantics;
- trust roots;
- epochs and anti-rollback policy;
- rotation/revocation;
- fail-closed verification behavior.

Low-level algorithms should migrate to maintained reviewed crates when compatible.

Migration must be test-vector compatible and negative-test qualified before removing legacy code.

---

## Android decision

The preferred mobile architecture is:

```text
Kotlin platform shell
        ↕
UniFFI-generated typed boundary
        ↕
shared Rust WebGate core
```

Kotlin owns Android-specific lifecycle and capability adapters; Rust remains authoritative for shared security/policy/transport state.

A handwritten JNI layer is permitted only where UniFFI cannot satisfy an evidenced platform requirement.

---

## Update trust decision

WebGate release/update design must be mapped against TUF threat classes before extending custom metadata formats.

The project must explicitly account for:

- rollback;
- freeze/stale metadata;
- key compromise/rotation;
- target substitution;
- threshold/role separation where appropriate;
- recovery authorization.

Adopting `rust-tuf` or `tough` is a separate implementation decision; using TUF as the threat/model benchmark is accepted immediately.

---

## Supervisor decision

There must be one authoritative transport supervisor.

Other components may observe/project state but may not independently select transport or infer readiness.

Standing decomposition reference: `cloudflared`.

Direct/relay lifecycle reference: `tailcat` / Tailscale data plane.

The WebGate supervisor remains responsible for enforcing:

```text
hysteresis
retry budgets
circuit breaking
qualified path eligibility
failure-domain constraints
resource budgets
recovery windows
truthful readiness
```

---

## Zero-trust architecture decision

OpenZiti is accepted as a standing benchmark for:

- service identity;
- enrollment/revocation;
- dark services;
- outbound-only private origins;
- controller/data-plane separation;
- policy-driven service reachability.

It does not replace SecureAcces and does not own WebGate application authorization.

---

## Dependency admission rule

No new runtime dependency is approved solely because it reduces code.

It must pass a recorded gate covering at least:

- license;
- maintenance and upstream health;
- security/advisory process;
- transitive dependency risk;
- Tier-1 platform support;
- deterministic/reproducible build impact;
- resource usage;
- malformed-input behavior;
- fuzz/negative-testability;
- absence of hidden telemetry/control dependency;
- absence of ambient route/DNS mutation;
- SBOM/license inventory impact;
- rollback/update implications.

---

## Consequences

### Positive

- less custom protocol and cryptographic machinery;
- lower verification surface;
- faster Android integration;
- fewer duplicate state machines;
- better use of mature operational lessons;
- clearer ownership of WebGate's differentiating guarantees.

### Negative

- dependency supply-chain surface grows;
- upstream changes can affect qualification;
- abstractions must prevent library semantics from leaking into policy;
- migration work is required to replace already-written low-level code;
- some candidates may fail Tier-1 or resource constraints and must be rejected.

---

## Rejected alternatives

### Build everything in-house

Rejected as the default because it increases implementation and assurance burden in non-differentiating domains.

### Replace WebGate transport with Tailscale/OpenZiti wholesale

Rejected because WebGate has different policy, renderer, authorization and fail-closed boundaries.

### Use external libraries without a stable WebGate abstraction

Rejected because dependency semantics could become architectural policy and make future replacement unsafe.

### Keep duplicate Rust and Kotlin state machines

Rejected because split authority increases race, drift and false-readiness risk.

---

## Verification

This ADR is considered correctly implemented only when:

1. candidate integrations are behind WebGate-owned interfaces;
2. no dependency can bypass destination/route/admission policy;
3. migration tests prove security-equivalent or stronger behavior;
4. Tier-1 release binaries are qualified;
5. `MASTER_PLAN.md` tracks the implementation tranche and owns final task status.
