# WebGate — Cybernetic Stability Execution Program

**Status:** ACTIVE EXECUTION INPUT  
**Accepted:** 2026-09-08  
**Source audit:** `docs/research/CYBERNETIC_STABILITY_AUDIT_2026-09-08.md`  
**Planning authority:** `MASTER_PLAN.md` remains the canonical task-state owner; this document defines the detailed execution contract for the new stability/security tranche.

---

# 1. Purpose

This program converts the 2026-09-08 mathematical/cybernetic audit into implementation gates. It must not be treated as a parallel roadmap. All tasks below are intended to be mirrored into the living `MASTER_PLAN.md` task/finding/invariant registries and tracked by GitHub issues until closed.

The program is intentionally ordered to break positive feedback loops and reduce the trusted state space before adding more transports or renderer complexity.

Execution principle:

```text
truthful status
  ↓
trust boundary correctness
  ↓
resource bounds
  ↓
explicit routing
  ↓
state-machine convergence
  ↓
performance / resilience optimization
```

---

# 2. New findings to mirror into MASTER_PLAN

## F-CYB-001 — Release verification can produce false authenticity evidence

**Severity:** Critical / P0  
**Owner:** T-CYB-001

`ReleaseManifest::verify_artifact()` does not currently perform a complete cryptographic verification boundary over artifact bytes + canonical signed manifest.

Exit requires real digest calculation inside the verifier, real Ed25519 verification against a trusted key, rollback enforcement and negative mutation tests.

## F-CYB-002 — Relay production identity is still shared-secret centered

**Severity:** Critical / P0  
**Owner:** T-CYB-002

Production Relay/Origin identity must move to per-node cryptographic identity and an authenticated encrypted transport envelope.

## F-CYB-003 — Relay routing is ambiguous in multi-origin operation

**Severity:** Critical / P0  
**Owner:** T-CYB-003

`first active origin` semantics are forbidden. Every admitted stream must resolve an explicit route and route epoch before allocation.

## F-CYB-004 — One stream can stall unrelated streams

**Severity:** Critical/High / P0-P1  
**Owner:** T-CYB-004

Single connection frame dispatch can block on a saturated per-stream channel. Overflow must be isolated to the offending stream.

## F-CYB-005 — Concurrency/resource consumption is not globally bounded

**Severity:** High / P0  
**Owner:** T-CYB-005

Thread/task/stream/queue/FD/memory admission budgets are required before public relay exposure.

## F-CYB-006 — Failover semantics are duplicated

**Severity:** High / P0  
**Owner:** T-CYB-006

`failover.rs` and `dual_failover.rs` must converge on one authoritative transition model.

## F-CYB-007 — Canonical destination semantics differ by layer

**Severity:** High / P1  
**Owner:** T-CYB-007

Browser, SOCKS, CONNECT and gateway policy must consume one canonical destination representation.

## F-CYB-008 — Unknown HTTP method falls back to view permission

**Severity:** High / P1  
**Owner:** T-CYB-008

Unknown methods must be denied unless explicitly mapped by the protected service contract.

## F-CYB-009 — Child environment secret containment is blacklist-based

**Severity:** High / P1  
**Owner:** T-CYB-009

Child process environment must become allowlist-based.

## F-CYB-010 — Health evidence is shallower than the claimed readiness level

**Severity:** High / P1  
**Owner:** T-CYB-010

SOCKS/TCP readiness cannot by itself qualify protected-service readiness.

## F-CYB-011 — HA calculations ignore correlated/common-mode failures

**Severity:** High / P1  
**Owner:** T-CYB-011

Every candidate path needs a material failure-domain vector and redundancy qualification.

## F-CYB-012 — Request-time authority is an availability series dependency

**Severity:** Medium/High / P1  
**Owner:** T-CYB-012

Default remains fail-closed. Optional bounded signed authorization grants may reduce synchronous authority coupling, but never create fail-open behavior.

---

# 3. New invariants to mirror into MASTER_PLAN

```text
I-CYB-001 Verified release requires cryptographic proof over artifact bytes.
I-CYB-002 Production relay/origin identity is per-node and cryptographically bound.
I-CYB-003 Every client transit stream has an explicit authorized route.
I-CYB-004 Queue/task/stream/resource growth is bounded by admission policy.
I-CYB-005 One slow stream cannot indefinitely block unrelated streams.
I-CYB-006 Transport failover transitions have one authoritative implementation.
I-CYB-007 OFFLINE is recoverable when a qualified path returns.
I-CYB-008 All network policy layers consume one canonical destination model.
I-CYB-009 Unknown request methods are denied by default.
I-CYB-010 Child services inherit only an explicit environment allowlist.
I-CYB-011 Ready implies evidence at the level of the capability being claimed.
I-CYB-012 Redundancy requires measured material failure-domain independence.
I-CYB-013 ProductionReady implies zero open P0 blockers.
```

---

# 4. Execution order

## Wave 0 — Truth and release safety

### T-CYB-000 — Planning/readiness truth convergence

**Priority:** P0  
**Status:** READY

Actions:

- README must stop claiming that all phases are production-ready while P0 blockers remain;
- document the cybernetic audit and execution program;
- expose open P0/P1 work visibly;
- ensure project manager/release output cannot label a build production-qualified solely because compilation/tests passed.

Exit:

```text
README truthful
planning blockers visible
no contradictory production-ready statement
```

### T-CYB-001 — Cryptographic release trust kernel

**Priority:** P0 release blocker  
**Status:** READY

Implement:

```text
artifact bytes
   ↓ SHA-256 inside verifier
computed digest
   + canonical manifest fields
   ↓
canonical signed message
   ↓ Ed25519 verify(trusted release key)
VerifiedRelease
```

Required negative tests:

- artifact bit flip;
- manifest digest flip;
- signature bit flip;
- wrong key;
- unknown/revoked key ID;
- platform mismatch;
- rollback;
- malformed semver;
- changed source commit;
- signature present but invalid.

No caller-supplied digest may substitute for hashing the bytes being installed.

---

# 5. Wave 1 — Relay trust, routing and admission

### T-CYB-002 — Per-node relay/origin identity + secure envelope

**Priority:** P0  
**Status:** READY

Prefer standard TLS 1.3/mTLS or QUIC/TLS.

Required:

- unique identity per Relay and Origin;
- stable `key_id`;
- rotation;
- revocation;
- certificate/public-key pin policy as appropriate;
- cluster token downgraded to bootstrap/recovery only;
- route identity must match authenticated connection identity.

### T-CYB-003 — Explicit route table and reservations

**Priority:** P0  
**Status:** BLOCKED by T-CYB-002 for production trust binding

Introduce:

```text
RouteKey(tenant, cluster, origin, service)
RouteEpoch
Reservation(id, route, expiry, quotas)
```

Requirements:

- no map iteration for authoritative selection;
- deterministic lookup;
- atomic route-table replacement;
- no cross-tenant route collision;
- expired reservations cannot open streams;
- route changes do not silently retarget an existing stream.

### T-CYB-005 — Resource governor / admission controller

**Priority:** P0  
**Status:** READY

Introduce hard limits for:

```text
global connections
connections per tenant/device/origin
streams per connection
queued frames per stream
queued bytes per stream
queued bytes per origin
open file descriptors budget
worker/task budget
bandwidth budget
```

Overload behavior:

```text
reject new work early
preserve accepted work where possible
never allocate unbounded queues
never convert saturation directly into path-failure evidence
```

---

# 6. Wave 2 — Multiplexing and failover convergence

### T-CYB-004 — Stream isolation / no connection-wide HOL

**Priority:** P0/P1  
**Status:** BLOCKED by resource governor design

Preferred experiment lane:

- QUIC stream-per-application-stream.

Compatibility lane:

- bounded dispatcher;
- per-stream queues;
- DRR/WFQ scheduling;
- stream-local reset on overflow;
- separate control traffic priority.

Exit property:

```text
slow stream A + healthy streams B..N
→ B..N maintain bounded latency degradation
→ A may be throttled/reset
→ connection does not globally stall
```

### T-CYB-006 — Unified Transport Supervisor

**Priority:** P0  
**Status:** READY

Delete duplicated state-transition authority.

Target states:

```text
PRIMARY
PRIMARY_DEGRADED
FALLBACK
FALLBACK_DEGRADED
RECOVERING
OFFLINE
```

Inputs:

```text
EWMA latency
EWMA errors
liveness suspicion
saturation
real traffic outcome
synthetic probe outcome
```

Required:

- hysteresis;
- cooldown;
- jittered recovery probes;
- recovery from OFFLINE;
- no single request error changing global state without configured evidence;
- saturation classified separately from path failure;
- monotonic `state_epoch`.

---

# 7. Wave 3 — Policy and process boundaries

### T-CYB-007 — CanonicalDestination kernel

**Priority:** P1  
**Status:** READY

Create one canonical parser and a stable internal representation consumed by browser, CONNECT, SOCKS and gateway layers.

Required differential tests ensure all layers make the same decision for equivalent input.

### T-CYB-008 — Method/permission contract

**Priority:** P1  
**Status:** READY

Change default behavior:

```text
unknown method -> deny
```

Optionally allow each service to declare explicit method→permission mappings.

### T-CYB-009 — Child environment capability allowlist

**Priority:** P1  
**Status:** READY

Replace secret blacklist with environment allowlist. Add test proving newly introduced `WEBGATE_*` control secrets are not inherited unless explicitly approved.

---

# 8. Wave 4 — Readiness, correlated resilience and authority continuity

### T-CYB-010 — Multi-level health model

**Priority:** P1  
**Status:** READY

Levels:

```text
L1 process
L2 transport
L3 route
L4 authorized service
```

Readiness endpoint/reporting must identify which level is currently proven and freshness/age of evidence.

### T-CYB-011 — Failure-domain vector

**Priority:** P1  
**Status:** READY

Each path records:

```text
provider
ASN/network domain
region
relay implementation/version
transport family
DNS/discovery dependency
authority dependency
identity root
```

A primary/fallback pair only satisfies HA if configured independence policy passes.

### T-CYB-012 — Bounded signed authorization grants

**Priority:** P1 optional resilience  
**Status:** EXPERIMENTAL

Default fail-closed synchronous authority remains valid.

Evaluate signed short-lived grants:

```text
device
session
resource
permissions
expiry
policy_epoch
key_id
signature
```

No indefinite cache or fail-open semantics.

---

# 9. Formal verification tranche

### T-CYB-013 — TLA+/PlusCal safety/liveness model

**Priority:** P1 verification  
**Status:** READY after state-machine interfaces freeze

Model at minimum:

- route reservation;
- failover and recovery;
- revocation while streams exist;
- route epoch replacement;
- resource admission;
- offline recovery.

Safety properties:

```text
NoUnauthorizedRoute
NoCrossTenantStream
NoDirectFallback
NoVerifiedReleaseWithoutCryptoProof
NoUnboundedAdmission
```

Liveness:

```text
HealthyPathEventuallyUsable
DeadReservationEventuallyFreed
OfflineEventuallyRecoversWhenPathReturns
```

---

# 10. Qualification matrix

A P0 task is not DONE until its negative qualification exists.

| Area | Unit | Property/Fuzz | Race/Concurrency | Chaos | Load/Soak | Release binary |
|---|---:|---:|---:|---:|---:|---:|
| Release trust | yes | yes | n/a | n/a | n/a | yes |
| Relay identity | yes | yes | yes | yes | yes | yes |
| Routing | yes | yes | yes | yes | yes | yes |
| Admission | yes | yes | yes | yes | yes | yes |
| Stream isolation | yes | yes | yes | yes | yes | yes |
| Failover supervisor | yes | yes | yes | yes | yes | yes |
| Canonical policy | yes | yes | n/a | n/a | n/a | yes |

---

# 11. Global production gate

Until all P0 tasks above are DONE and qualified:

```text
ReleaseCandidate = allowed
ProductionQualified = false
```

Formal gate:

\[
ProductionQualified =
(P0_{open}=0)
\land ReleaseTrustQualified
\land RelayTrustQualified
\land RoutingQualified
\land AdmissionQualified
\land FailoverQualified
\land ReleaseBinaryE2E
\land SoakPass
\land ChaosPass
\]

The project manager and README should use this definition consistently.

---

# 12. Metrics required before tuning

Before changing thresholds, collect:

```text
wg_active_connections
wg_active_streams
wg_stream_queue_bytes
wg_rejected_connections_total
wg_rejected_streams_total
wg_relay_rtt_ms
wg_relay_error_ewma
wg_relay_suspicion
wg_failover_total
wg_switchback_total
wg_route_epoch
wg_route_reservations
wg_authority_latency_ms
wg_release_verification_failures_total
process_cpu
process_rss
open_fds
```

Tuning without these measurements is forbidden for production qualification.

---

# 13. Definition of Done for the stability program

The program is complete only when:

1. all P0 findings are resolved;
2. production status is truthful and machine-checkable;
3. resource use is bounded under hostile load;
4. relay identity/routing are explicit and cryptographically bound;
5. one slow stream cannot cause connection-wide failure;
6. failover/recovery is represented by one authoritative state machine;
7. release verification is a genuine cryptographic trust boundary;
8. formal safety/liveness model passes for the frozen transition contracts;
9. release-binary chaos/load/soak qualification passes;
10. supporting documentation and `MASTER_PLAN.md` are reconciled without parallel sources of truth.
