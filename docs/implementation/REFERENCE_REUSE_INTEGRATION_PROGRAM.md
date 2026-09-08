# WebGate Reference Reuse Integration Program

**Date:** 2026-09-08  
**Purpose:** convert the reference-repository audit into bounded implementation tranches  
**Architecture decision:** `docs/architecture/ADR-0005-REFERENCE-REUSE-AND-DEPENDENCY-BOUNDARIES.md`  
**Research basis:** `docs/research/REFERENCE_REPOSITORY_REUSE_AUDIT_2026-09-08.md`  
**Canonical task/status owner:** `MASTER_PLAN.md`

> This document defines execution contracts and acceptance gates. It does not override task status in `MASTER_PLAN.md`. Any tranche promoted from evaluation to production work must be mirrored into the canonical master plan before it can claim DONE/qualified status.

---

# 1. Program objective

Reduce implementation risk and custom infrastructure code while preserving or strengthening all WebGate invariants.

Target equation:

```text
WebGate-owned policy
+ mature protocol/crypto/FFI mechanisms
+ fewer state machines
+ explicit qualification gates
= simpler and more reliable WebGate
```

The program is intentionally migration-first, not rewrite-first.

---

# 2. Non-negotiable ownership boundaries

The following remain WebGate-owned regardless of dependency choices:

- destination restriction;
- fail-closed browser/runtime behavior;
- service/origin route authorization;
- SecureAcces authority separation;
- device and node trust roles;
- admission/resource governance;
- transport eligibility and failover semantics;
- release acceptance and anti-rollback policy;
- renderer capability boundary;
- readiness/qualification truth.

No external library may become an implicit authority for these decisions.

---

# 3. Execution order

Recommended order by leverage and dependency:

```text
RRP-001 Dependency admission framework
   ├──► RRP-002 Crypto primitive migration
   ├──► RRP-003 Quinn/QUIC transport spike
   ├──► RRP-004 Supervisor convergence
   ├──► RRP-005 Tailcat/Tailscale direct-relay study
   ├──► RRP-006 Android UniFFI spike
   └──► RRP-007 TUF mapping

RRP-003 + RRP-004 + RRP-005
   └──► RRP-008 Next-gen transport decision

RRP-006
   └──► RRP-009 Android binding migration

RRP-007
   └──► RRP-010 Release Trust Kernel convergence

all adopted dependencies
   └──► RRP-011 Supply-chain/SBOM/release qualification

RRP-008 + RRP-009 + RRP-010 + RRP-011
   └──► RRP-012 Cross-stack release-binary requalification
```

---

# 4. RRP-001 — Dependency Admission Framework

**Priority:** P0-enabler  
**Mode:** implementation/process  
**Reference:** ADR-0005

## Goal

Create one reusable gate for every production runtime dependency.

## Required evidence fields

For each candidate dependency record:

```text
name/version
upstream repository
license
maintainer/release health
security policy/advisory channel
transitive dependency count/risk
Tier-1 platform support
build/reproducibility impact
binary size impact
startup impact
steady-state memory impact
resource-control hooks
malformed-input behavior
fuzzability/testability
network/telemetry side effects
routing/DNS side effects
update/rollback impact
SBOM status
rejection/acceptance decision
```

## Acceptance

- machine-readable or consistently structured dependency decision records exist;
- CI can detect unreviewed runtime dependency additions or lockfile drift where practical;
- license/security inventory includes adopted dependencies;
- dependency adoption cannot silently bypass WebGate architecture review.

---

# 5. RRP-002 — Replace Home-Grown Cryptographic Primitives

**Priority:** P0-equivalent security leverage  
**Mode:** migrate behind existing interfaces  
**Candidates:** `rustls`, vetted Ed25519/SHA-2 crates

## Goal

Stop expanding custom low-level crypto and migrate existing primitive implementation to reviewed ecosystem crates without changing WebGate trust semantics.

## Keep WebGate-owned

- canonical message construction;
- domain separation;
- key role semantics;
- key IDs/epochs;
- rotation/revocation;
- anti-rollback;
- verification failure behavior.

## Migration sequence

1. freeze behavior with known-answer vectors;
2. add adversarial negative vectors;
3. introduce provider abstraction only if needed;
4. implement vetted provider;
5. run old/new differential verification;
6. switch default provider;
7. remove obsolete primitive implementation only after release-binary qualification.

## Required tests

- RFC/standard vectors where applicable;
- invalid signature rejection;
- malformed key rejection;
- modified message rejection;
- wrong-key rejection;
- canonicalization mismatch rejection;
- release signature round trip;
- device PoP round trip;
- fuzz parser/verification boundaries;
- cross-platform deterministic vectors.

## Acceptance

- no WebGate-owned SHA/Ed25519 arithmetic remains on the production path unless explicitly justified;
- all trust semantics remain unchanged or stronger;
- failures remain fail closed;
- dependency gate passes.

---

# 6. RRP-003 — Quinn/QUIC Feasibility Tranche

**Priority:** P0/P1 architecture decision  
**Mode:** spike before extending WGRL stream machinery  
**Candidate:** `quinn-rs/quinn`

## Goal

Determine whether Quinn can become the production stream substrate for client↔relay and/or origin↔relay links while preserving WebGate policy and resource controls.

## Prototype topology

```text
Client
  ↓ WebGate authenticated/authorized session
Quinn endpoint
  ↓ QUIC
Relay
  ↓ explicit Origin routing
Quinn stream
  ↓
Origin
```

This prototype does not need full browser integration initially.

## Measurements

- connect/reconnect latency;
- stream open/close latency;
- memory per connection;
- memory per idle/active stream;
- CPU under 1/10/100/1000 streams;
- backpressure behavior;
- cancellation cleanup;
- packet loss 1/5/10/20%;
- reordering/jitter;
- relay fan-in;
- mobile network transition behavior where testable;
- idle/keepalive battery impact on Android;
- binary size impact;
- behavior when UDP is blocked.

## Security gates

- stream admission before expensive allocation;
- hard maximum concurrent streams;
- per-peer and global memory budgets;
- explicit route binding before protected forwarding;
- no arbitrary destination dialing;
- peer identity available to WebGate policy;
- malformed handshake/input cannot cause unbounded work;
- connection loss cannot produce direct fallback.

## Decision outcomes

### ADOPT

Quinn becomes transport substrate behind WebGate interfaces.

### PARTIAL

Use Quinn only on selected paths while preserving another qualified transport family.

### REJECT

Document measured blocker; keep WebGate transport abstraction and pursue another substrate.

## Acceptance

A signed-off decision record contains measured evidence and migration implications. No custom multiplexing expansion proceeds without referencing this result.

---

# 7. RRP-004 — Authoritative Transport Supervisor Convergence

**Priority:** P0  
**Reference:** `cloudflare/cloudflared`

## Goal

Collapse duplicated/competing transport state into one authoritative supervisor.

## Canonical state ownership

The supervisor owns:

```text
candidate paths
current active path
health evidence
retry timers
failure counters
circuit state
recovery window
resource eligibility
path qualification state
readiness projection
```

UI/browser/runtime consume a projection; they do not infer transport state independently.

## Suggested state model

```text
STOPPED
  ↓
BOOTSTRAPPING
  ↓
PROBING
  ├──► CONNECTED_RELAY
  ├──► CONNECTED_DIRECT
  └──► DEGRADED
          ↓
      FAILING_OVER
          ├──► CONNECTED_RELAY
          ├──► CONNECTED_DIRECT
          └──► OFFLINE

any connected state
  ├── evidence loss → DEGRADED
  ├── policy revoke → FAIL_CLOSED
  └── stop → STOPPED
```

Exact state names may differ; authority must not.

## Required controls

- exponential/jittered bounded retry;
- retry budget;
- per-path circuit breaker;
- hysteresis;
- minimum stability window before switchback;
- monotonic evidence timestamps;
- independent health dimensions;
- failure-domain scoring;
- resource-budget eligibility;
- explicit reason codes for every state transition.

## Acceptance

- only one component selects the active path;
- state transitions are deterministic/testable;
- stale health cannot report ready;
- no fast oscillation under intermittent failure;
- browser remains fail closed during transition;
- chaos tests demonstrate bounded recovery.

---

# 8. RRP-005 — Tailcat/Tailscale Direct/Relay Path Study

**Priority:** P1 strategic  
**Reference:** `tailscale/tailcat`, selected `tailscale/tailscale`

## Goal

Design WebGate's direct-path optimization using proven userspace NAT-traversal/relay concepts without making the OS route table part of the trust model.

## Required conceptual flow

```text
known/authenticated peer identity
        ↓
authenticated rendezvous metadata
        ↓
qualified relay bootstrap
        ↓
direct-path probing
        ↓
cryptographic path proof
        ↓
optional direct upgrade
        ↓ failure/degradation
safe relay recovery
```

## Study outputs

- candidate-address lifecycle;
- NAT discovery model;
- relay bootstrap role;
- direct-path proof semantics;
- peer-key binding;
- path-change behavior;
- mobile roaming implications;
- relay-of-last-resort semantics;
- privacy impact of discovery metadata.

## Constraints

- no OS default-route takeover;
- no transport discovery metadata may authorize a service;
- direct path is never a bypass around WebGate policy;
- signed/authorized discovery remains authoritative;
- arbitrary LAN reachability does not become ambient authority.

## Acceptance

A WebGate-owned direct/relay state-machine design exists with explicit security invariants and failure semantics before direct path is implemented.

---

# 9. RRP-006 — Android UniFFI Spike

**Priority:** P0/P1 for Android Tier-1  
**Candidate:** `mozilla/uniffi-rs`

## Goal

Prove a small typed Rust↔Kotlin boundary before handwritten JNI grows.

## Spike API

Expose only a narrow vertical slice:

```text
initialize(runtime_config)
get_status()
start_session(target_capability)
stop_session()
subscribe_state_events()
```

Do not expose raw socket handles, secrets or generic command execution.

## Android-owned adapters

- lifecycle;
- foreground-service handling where required;
- Android Keystore;
- notifications;
- deep/app links;
- renderer/surface integration;
- network-change callbacks.

## Rust-owned authority

- session state machine;
- destination policy;
- transport supervisor;
- device identity orchestration;
- readiness projection;
- release/policy verification.

## Measurements

- APK/AAB size impact;
- binding generation reproducibility;
- exception/error mapping;
- async/callback behavior;
- process death/restart behavior;
- ABI compatibility;
- arm64 build/release flow;
- instrumentation-test ergonomics.

## Acceptance

UniFFI is adopted unless a measured blocker is documented. If rejected, the replacement JNI contract must remain narrow and generated/typed where possible.

---

# 10. RRP-007 — TUF Threat-Model Mapping

**Priority:** P0 release-trust leverage  
**References:** TUF specification, `theupdateframework/rust-tuf`, `awslabs/tough`

## Goal

Map WebGate's Release Trust Kernel to mature secure-update threat classes before implementing more custom metadata/protocol logic.

## Mapping matrix

WebGate must explicitly map:

```text
trust root
metadata roles
key rotation
version monotonicity
expiry
freeze/stale metadata
rollback
mix-and-match metadata
target hash/length
platform targeting
recovery release
offline/online signing roles
threshold signatures where justified
```

## Candidate decision

Evaluate whether:

1. a TUF implementation can be used directly;
2. TUF metadata can wrap WebGate release artifacts;
3. only selected TUF threat-model concepts should be implemented within the existing manifest format.

## Acceptance

No production updater extension is accepted without a documented TUF comparison and explicit residual-risk list.

---

# 11. RRP-008 — Next-Generation Transport Decision

**Priority:** P0 architecture convergence  
**Depends on:** RRP-003, RRP-004, RRP-005

## Goal

Choose the smallest transport architecture that satisfies:

- native authenticated confidentiality/integrity;
- per-node identity;
- explicit routing;
- independent streams/no global HOL;
- hard resource bounds;
- relay opacity where required;
- direct-path upgrade;
- heterogeneous fallback;
- Tier-1 support.

## Candidate target

Preferred if evidence supports it:

```text
WebGate policy/session protocol
        ↓
WebGate route + admission layer
        ↓
Quinn/QUIC primary stream substrate
        ↓
relay/direct path

+ independent HTTP/2/TLS-class fallback transport
```

`wstunnel`, `cloudflared` and related projects are references for HTTP-class fallback mechanics, not automatic dependencies.

## Acceptance

- architecture decision recorded;
- old/new compatibility strategy defined;
- migration does not create two authoritative supervisors;
- explicit fallback diversity remains measurable;
- release-binary qualification plan updated.

---

# 12. RRP-009 — Android Binding Migration

**Priority:** P1  
**Depends on:** RRP-006

## Goal

Make Kotlin a thin platform shell and eliminate duplicated shared state machines.

## Required boundaries

Kotlin must not independently own:

- transport selection;
- device authorization state;
- destination allowlist;
- release acceptance;
- policy epoch;
- ready/open decision.

## Acceptance

- one Rust source of truth for shared state;
- Android process/lifecycle restart is deterministic;
- Keystore failures invalidate readiness rather than silently falling back;
- instrumentation tests cover process death, background/foreground and network changes.

---

# 13. RRP-010 — Release Trust Kernel Convergence

**Priority:** P0  
**Depends on:** RRP-002, RRP-007

## Goal

Replace placeholder/partial release verification with a complete qualified acceptance kernel.

## Canonical acceptance predicate

```text
AcceptRelease =
    canonical_manifest_parse
    AND target_digest_matches
    AND target_length_matches
    AND signature_policy_valid
    AND trusted_role_valid
    AND platform_matches
    AND version_epoch_monotonic
    AND metadata_not_expired
    AND rollback_policy_satisfied
    AND recovery_rules_satisfied
```

Unknown/malformed states deny.

## Acceptance

- negative tests for every conjunct;
- expired/stale/rollback artifacts rejected;
- recovery does not disable verification;
- trust-root rotation is explicit and auditable;
- final release binaries run the verifier actually shipped.

---

# 14. RRP-011 — Supply Chain, SBOM and Upstream Qualification

**Priority:** P1 release gate  
**Depends on:** all adopted dependencies

## Goal

Ensure simplification does not trade implementation risk for opaque supply-chain risk.

## Required outputs

- dependency inventory;
- license inventory;
- SBOM;
- pinned/reproducible versions;
- vulnerability/advisory scanning process;
- upgrade policy;
- emergency pin/rollback policy;
- upstream abandonment response;
- provenance/signing evidence where feasible.

## Acceptance

Release qualification can answer exactly which version of each security-critical dependency is shipped and why it is trusted.

---

# 15. RRP-012 — Cross-Stack Release-Binary Requalification

**Priority:** final P0 gate for adopted architecture  
**Depends on:** RRP-008, RRP-009, RRP-010, RRP-011

## Goal

Prove that reuse works in the final binaries, not just unit tests and library spikes.

## Required matrix

### Platforms

- Windows x86_64;
- Android arm64;
- Linux x86_64/aarch64 as Tier-2 evidence where applicable.

### Network conditions

- healthy primary relay;
- primary relay failure;
- UDP unavailable;
- high loss/jitter/reordering;
- Origin restart;
- relay restart;
- client network transition;
- authority unavailable/revoked;
- stale policy/release metadata.

### Resource conditions

- connection flood within lab boundaries;
- stream limit reached;
- memory pressure;
- slow consumer;
- stalled stream;
- reconnect storm.

### Security conditions

- wrong peer identity;
- wrong Origin route;
- destination escape attempt;
- malformed frames/metadata;
- replayed/expired release metadata;
- revoked device/session;
- renderer direct-network attempt.

## Acceptance

No production qualification until actual release binaries demonstrate the complete fail-closed path and resource bounds.

---

# 16. Reference-specific rules

## Tailcat/Tailscale

Use for userspace direct/relay mechanics and state-machine reference. Do not delegate service authorization or WebGate route policy.

## Cloudflared

Use for supervisor/retry/tunnel-state decomposition. Do not copy fallback semantics without WebGate eligibility gates.

## Quinn

Candidate mechanism below WebGate route/admission/session policy.

## OpenZiti

Use as zero-trust service architecture benchmark. Do not replace SecureAcces.

## UniFFI

Preferred binding generator for shared Rust core ↔ Kotlin Android shell.

## wstunnel/rathole

Use as implementation/complexity references. Do not turn WebGate into a generic reverse proxy.

## TUF implementations

Use as secure-update benchmark/candidate. Adoption requires maturity and dependency review.

## rustls/vetted crypto crates

Preferred low-level cryptographic machinery; WebGate retains trust policy.

---

# 17. Code-deletion targets

The program should explicitly measure code removed/avoided, not just code added.

Candidate deletion/avoidance areas:

- custom SHA/Ed25519 primitive internals;
- future custom congestion/loss-recovery code;
- future custom generic multiplexing code;
- handwritten JNI boilerplate;
- duplicate Kotlin/Rust state machines;
- bespoke secure-update threat handling already covered by adopted TUF semantics;
- duplicate failover/retry controllers.

For each adopted dependency, record:

```text
LOC added
LOC deleted
state machines deleted/merged
tests added
new dependency surface
residual custom complexity
```

A dependency that only adds complexity without deleting or preventing meaningful custom machinery should be challenged.

---

# 18. Stop conditions

Stop or reject an integration if it causes any of:

- system-wide VPN/routing requirement in the normal WebGate path;
- generic proxy capability;
- inability to enforce per-peer/stream resource bounds;
- hidden vendor/cloud dependency;
- unowned telemetry/control channel;
- duplicate authority/state machine;
- inability to build/qualify Android arm64 or Windows x86_64;
- unverifiable cryptographic behavior;
- unacceptable supply-chain risk;
- readiness state that cannot be evidenced from actual end-to-end health.

---

# 19. Recommended first implementation tranche

The highest-leverage immediate sequence is:

```text
1. RRP-001 dependency gate
2. RRP-002 crypto migration design + differential tests
3. RRP-003 minimal Quinn spike
4. RRP-004 authoritative supervisor model
5. RRP-006 UniFFI Android vertical slice
6. RRP-007 TUF mapping
```

Run these as bounded experiments before broad refactors.

The expected architectural payoff is that WebGate remains custom only where it should be custom:

```text
capability isolation
+ destination restriction
+ SecureAcces authority separation
+ explicit routing
+ resource governance
+ fail-closed runtime
+ qualification truth
```

while mature projects absorb commodity protocol machinery.
