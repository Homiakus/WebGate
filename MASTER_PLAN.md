# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Execution baseline:** `2b7c5a59ae456c207ad7fd992210767e68893a24`  
**Reconciled:** 2026-08-31  

This file is the **only execution source of truth**. Architecture/research documents under `docs/` are supporting evidence and design references only; they do not own task state, priority, acceptance, or release readiness.

A task is `DONE` only when its observable production contract is implemented, relevant negative tests exist, required verification passes, and the verified state reaches `main` without force push. A model, interface, mock, state-machine simulation, compile-only probe, or documentation is not production qualification by itself.

---

# 1. Mission

Build WebGate as a secure application-scoped access system for a small trusted-user set:

```text
trusted link
  ↓
WebGate-owned browser capsule
  ↓
destination-restricted loopback proxy
  ↓
real protected transport
  ↓
independent Relay A / Relay B
  ↓
persistent outbound reverse connectivity from private Origin
  ↓
WebGate data gateway
  ↓
SecureAcces authoritative authentication/authorization
  ↓
registered private service
```

The Origin must work behind dynamic IP / CGNAT with no inbound port forwarding. Protected traffic must never silently escape through normal OS Internet or an unproxied system browser.

---

# 2. Truth hierarchy

When evidence conflicts, use this order:

1. observed runtime behavior;
2. reproducible tests/experiments;
3. security/correctness invariants;
4. code;
5. this plan;
6. older design documents;
7. original assumptions.

Unexpected material evidence becomes `F-XXX` before task scope/order changes.

---

# 3. Current verified state

## 3.1 Verified foundations

The repository currently has:

- Rust workspace boundaries with `unsafe_code = forbid`;
- architecture/dependency checks and locked Rust CI;
- Go server format/vet/test CI gate;
- pure navigation-policy and browser-capsule state models;
- transport SPI and deterministic failover model;
- Go `ProtectedService` registry and gateway baseline;
- server-side Ed25519 device proof-of-possession with single-use challenge;
- session↔device and user↔device binding checks;
- split loopback Data/Admin listeners;
- temporary fail-closed Admin bearer-token middleware;
- process spawn failure no longer reported as `RUNNING`;
- admin/dashboard/release/config/Telegram prototype surfaces.

Current `main` CI at the execution baseline is green for Rust, dependency policy, Go formatting of touched files, `go vet`, and `go test`.

## 3.2 Not production-qualified

The following are **not** production-ready at the current baseline:

- real Servo embedding/runtime;
- real browser network enforcement;
- real destination-restricted local proxy;
- real primary transport provider;
- real independent fallback transport;
- real Relay A/B connectivity;
- Origin reverse-connectivity agent;
- server operation behind CGNAT as an end-to-end proven path;
- platform-backed production device key storage;
- authoritative SecureAcces runtime adapter;
- durable server state;
- SecureAcces-backed administrator authorization;
- true end-to-end transport/browser/origin qualification;
- production release/distribution qualification against the real runtime path.

Evidence:

- `crates/webgate-browser/Cargo.toml` has no Servo dependency; `BrowserCapsule` is a state/policy model.
- `DynamicRelayTransport::state()` returns `Ready` unconditionally and only synthesizes a loopback endpoint.
- `SecureRelayTransport::start_tunnel()` sets `Ready` without opening a socket or connecting to a relay; its latency probe performs no network I/O.
- `TransportFailoverController::start()` selects primary without proving provider readiness.
- `webgate-app` creates `InMemoryDeviceKeyStore`, whose keys/signatures are synthetic.
- default desktop launch opens Edge/Chrome/system browser without a WebGate proxy binding.
- `SecureAccessAuthorizer` stores sessions/memberships in process memory and is not an authoritative SecureAcces adapter.

---

# 4. Critical invariants

- **I-001 Browser ownership:** protected content is rendered only by a WebGate-qualified browser path.
- **I-002 Application-scoped routing:** normal WebGate mode does not change the OS default route.
- **I-003 Fail closed:** transport loss yields explicit protected-navigation failure; never direct fallback.
- **I-004 No silent engine fallback:** browser/runtime failure never opens protected content in an unproxied system browser.
- **I-005 Real readiness:** `Ready`/`Running` means required external side effects and health checks actually succeeded.
- **I-006 Destination restriction:** browser-facing proxy is loopback-only and can reach only policy-authorized protected destinations.
- **I-007 Network access ≠ authorization:** transport credentials alone never grant application access.
- **I-008 SecureAcces authority:** user/session/workspace/permission decisions come from SecureAcces, not a parallel production ACL/session database.
- **I-009 Device binding:** protected session is bound to the exact active device and owning user.
- **I-010 Real PoP:** device activation requires valid cryptographic proof over a short-lived single-use server challenge.
- **I-011 Production keys:** production device private keys are generated/stored by a platform security facility; synthetic/in-memory stores are test-only.
- **I-012 Origin no-public-IP:** Origin requires no public/static IP or inbound NAT mapping; it maintains outbound persistent relay connections.
- **I-013 Failure-domain diversity:** production has at least two materially independent relay failure domains.
- **I-014 Transport diversity:** at least one fallback differs materially in implementation/protocol/failure mode.
- **I-015 Admin isolation:** Admin and user data-plane surfaces are isolated; privileged operations require explicit management authorization and audit.
- **I-016 Server-owned routing:** client input cannot choose authoritative upstream, tenant, workspace, permissions, executable, or working directory.
- **I-017 No generic proxy:** gateway/proxy cannot become an arbitrary SSRF/open-proxy pivot.
- **I-018 Durable security state:** restart/crash cannot silently reset authoritative device/session/service/release/audit security state.
- **I-019 Signed policy/release:** production policy/config/update/release artifacts are signed, versioned, expiry/rollback-aware where applicable.
- **I-020 No false qualification:** no task/capability may be marked production `DONE` from mocks/simulations alone.
- **I-021 Trusted CI:** every production language/runtime path has build/static/test gates; security-critical concurrency gets race checking.
- **I-022 Mutation resistance:** critical allow/deny/state-transition logic has meaningful mutation testing where tooling is technically applicable.

---

# 5. Findings registry

Historical F-001..F-028 remain in Git history. Findings below supersede stale completion claims where evidence conflicts.

## F-029 — Execution plan reports simulated capabilities as DONE

**Status:** RESOLVED by T-034  
**Category:** Engineering Process / Qualification  
**Severity:** Critical  
**Confidence:** High  

**Evidence:** prior `MASTER_PLAN.md` marked T-004/T-010/T-012/T-013/T-014/T-016/T-027 and related phases `DONE`, while runtime files show no Servo dependency, synthetic keystore, unconditional transport readiness, no relay connection, and system-browser launch.

**Root cause:** completion criteria measured existence of abstractions/tests instead of observable production side effects.

**Impact:** false convergence, unsafe release claims, wrong task priority, and green CI that cannot prove the intended system path.

**Affected invariants:** I-001, I-003–I-005, I-008, I-011–I-014, I-020–I-022.

**Resolution direction:** T-034 restores truthful statuses and qualification semantics.

## F-030 — Client transport readiness is synthetic

**Status:** OPEN  
**Category:** Network / Correctness / Security  
**Severity:** Critical  
**Confidence:** High  

`DynamicRelayTransport` always reports `Ready`; `SecureRelayTransport::start_tunnel()` performs no network operation; failover startup does not validate readiness.

**Impact:** UI/capsule can believe a protected path exists when no listener/tunnel exists.

**Resolution:** T-035 then T-036/T-042.

## F-031 — Protected browser path is not a real Servo/proxied runtime

**Status:** OPEN  
**Category:** Browser / Security Boundary  
**Severity:** Critical  
**Confidence:** High  

`webgate-browser` currently models state/policy only. Desktop launcher opens Edge/Chrome/default browser without binding protected traffic to the WebGate proxy.

**Impact:** I-001/I-003/I-004 cannot be claimed.

**Resolution:** T-041.

## F-032 — Production entrypoint uses synthetic device keys

**Status:** OPEN  
**Category:** Identity / Key Management  
**Severity:** Critical  
**Confidence:** High  

`webgate-app` instantiates `InMemoryDeviceKeyStore`; implementation generates deterministic synthetic material and synthetic signatures.

**Resolution:** T-040.

## F-033 — SecureAcces integration is an in-memory surrogate

**Status:** OPEN  
**Category:** Authorization  
**Severity:** Critical  
**Confidence:** High  

`SecureAccessAuthorizer` owns in-memory session and membership maps. It is not an authoritative SecureAcces runtime/control adapter.

**Resolution:** T-038.

## F-034 — Origin reverse connectivity is absent

**Status:** OPEN  
**Category:** Network / Product Core  
**Severity:** Critical  
**Confidence:** High  

No production Origin agent currently establishes persistent outbound connections to independent relays. Therefore the core “server behind CGNAT without public IP” path is not implemented end-to-end.

**Resolution:** T-037.

## F-035 — Security/operations state is largely ephemeral

**Status:** OPEN  
**Category:** Persistence / Reliability  
**Severity:** High  
**Confidence:** High  

Registries, sessions/memberships, audit/process/release state are primarily memory-backed.

**Resolution:** T-039.

## F-036 — Admin authentication is only an interim shared token

**Status:** OPEN / contained  
**Category:** Admin Security  
**Severity:** High  
**Confidence:** High  

P0 hardening moved Admin to a separate loopback listener and requires a strong `WEBGATE_ADMIN_TOKEN`. This contains remote exposure but is not the target SecureAcces-backed administrator/session/device authorization model.

**Resolution:** T-038. The shared token remains an interim bootstrap/control boundary only.

## F-037 — Client configuration load can fail open to defaults

**Status:** OPEN  
**Category:** Configuration / Fail-Closed  
**Severity:** High  
**Confidence:** High  

`ClientConfigProfile::load_from_file(...).unwrap_or_default()` silently substitutes defaults after an explicitly requested config fails to load/parse.

**Resolution:** T-035.

## F-038 — CI does not yet enforce race/mutation/security depth promised by the plan

**Status:** OPEN  
**Category:** Test System / CI  
**Severity:** High  
**Confidence:** High  

Go `vet/test` is now gated, but `go test -race`, meaningful mutation testing and targeted fuzz/property gates are not yet required by CI.

**Resolution:** T-044.

---

# 6. Reconciled task state

Status meanings: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `DEFERRED`.

## 6.1 Trusted completed foundations

- **T-001 — Living execution-plan foundation:** DONE.
- **T-002 — Portable Rust workspace/executable baseline:** DONE.
- **T-003 — Rust dependency/architecture CI baseline:** DONE.
- **T-006 — Android lifecycle state-model probe:** DONE as a probe; not production Android qualification.
- **T-007 — Strict navigation/deep-link policy model:** DONE.
- **T-009 — Algorithm-agile device identity model:** DONE.
- **T-021 — ProtectedService registry baseline:** DONE as in-memory domain baseline.
- **T-022 — Multi-service gateway baseline:** DONE as local server baseline; additional SSRF/persistence qualification remains.
- **T-024 — Admin UI prototype:** DONE as UI capability, not production admin-security qualification.
- **T-025 — Server device registry + real Ed25519 PoP:** DONE for current in-memory server registry.
- **T-030 — Process spawn/lifecycle baseline:** DONE after P0 false-Running fix; deeper supervision remains later reliability scope.
- **T-032 — Editorial UI transformation:** DONE.

## 6.2 Reopened / requalification-required historical tasks

- **T-004 — Real Servo embedding adapter:** REOPENED; current browser crate has no Servo dependency/runtime.
- **T-005 — Real fail-closed browser networking:** REOPENED; current capsule proves policy state, not real browser network behavior.
- **T-008 — Failover controller:** REOPENED for readiness/startup semantics and real provider observations.
- **T-010 — Platform device-key adapters:** REOPENED; production entrypoint still uses synthetic in-memory keystore.
- **T-011 — SecureAcces integration:** REOPENED; current authorizer is local memory state.
- **T-012 — Primary production transport:** REOPENED.
- **T-013 — Independent fallback/dual-relay:** REOPENED.
- **T-014 — Servo/site/security/performance qualification:** REOPENED.
- **T-015 — Production release authority:** NEEDS_REQUALIFICATION after real runtime/security path exists.
- **T-016 — Final adversarial re-audit:** REOPENED by F-029..F-038.
- **T-019 — Trusted broker boundary:** NEEDS_REQUALIFICATION against real browser/process boundary.
- **T-023 — Admin Control API:** NEEDS_REQUALIFICATION; loopback + token is interim auth only.
- **T-026 — Audit/health operations:** NEEDS_REQUALIFICATION with durable state and end-to-end health.
- **T-027 — Full adversarial E2E qualification:** REOPENED; current tests do not traverse real browser→transport→relay→origin path.
- **T-028 — Telegram/release distribution:** NEEDS_REQUALIFICATION against authoritative users/devices/releases and real client runtime.
- **T-029 — Config profile binding:** REOPENED for fail-closed explicit-config behavior and signed policy trust.
- **T-031 — Telegram Admin Bot lifecycle:** NEEDS_REQUALIFICATION under target admin authorization and persistence model.
- **T-033 — Integrity audit:** historical audit complete, but its production-completeness conclusion is superseded by F-029..F-038.

## 6.3 New execution tasks

### T-034 — Restore execution truth and qualification semantics

**Status:** DONE  
**Priority:** P0  
**Type:** PROCESS / PLAN / SAFETY

Replace stale false-DONE state with evidence-based capability states; establish that supporting docs are non-authoritative; record current Critical/High findings and new dependency order.

Acceptance:

- `MASTER_PLAN.md` reflects current code evidence;
- simulated/model capabilities cannot be interpreted as production-qualified;
- open Critical/High findings are explicit;
- next task is selected by risk/dependency leverage.

### T-035 — Eliminate false readiness and fail-open client bootstrap

**Status:** READY  
**Priority:** P0  
**Type:** CORRECTNESS / SECURITY / FOUNDATION

Root causes addressed: F-030 startup semantics and F-037 explicit config fallback.

Scope:

- failover startup must not select a provider that is not actually `Ready` with a loopback endpoint;
- prefer a ready fallback only when the primary is unavailable and fallback is actually ready;
- if neither is ready, aggregate state is `Offline` and proxy is `None`;
- high-latency policy must not be dead configuration;
- explicit client `--config` parse/load failure is fatal and never silently replaced by defaults;
- production app must stop claiming `ready` solely from configured relay port.

Characterization/negative tests precede production changes.

### T-036 — Implement real destination-restricted loopback proxy + primary provider

**Status:** TODO  
**Priority:** P0  
**Type:** NETWORK / SECURITY

Implement a real listener, destination allowlist, bounded CONNECT/SOCKS semantics as selected, provider lifecycle, authenticated control boundary, cancellation and health. `Ready` requires a bound listener plus working protected upstream path.

### T-037 — Implement Origin agent and reverse Relay A/B connectivity

**Status:** TODO  
**Priority:** P0  
**Type:** NETWORK / CGNAT / RELIABILITY

Implement persistent outbound Origin connections, authentication, multiplexed streams, reconnect/backoff, relay registration, graceful rotation and local gateway forwarding. Prove operation with no inbound port forwarding.

### T-038 — Integrate authoritative SecureAcces + administrator authorization

**Status:** TODO  
**Priority:** P0  
**Type:** AUTHORIZATION / ADMIN SECURITY

Replace production in-memory session/membership authority with a narrow SecureAcces adapter; preserve local fake only as test fixture. Bind admin identity/session/device/management permission. Unknown/unavailable authorization fails closed.

### T-039 — Durable transactional server state

**Status:** TODO  
**Priority:** P0/P1  
**Type:** PERSISTENCE / RELIABILITY

Persist WebGate-owned service/device/release/audit/config metadata using a transactional store (SQLite is preferred for the small deployment unless evidence requires otherwise). SecureAcces-owned identity/permission data remains SecureAcces-owned.

### T-040 — Production platform key stores

**Status:** TODO  
**Priority:** P0  
**Type:** IDENTITY / PLATFORM SECURITY

Windows CNG/DPAPI/TPM-backed implementation, Android Keystore, and explicit assurance-tier fallbacks for other platforms. `InMemoryDeviceKeyStore` becomes test/dev-only and production build/runtime must reject it.

### T-041 — Real Servo runtime and enforced protected proxy

**Status:** TODO  
**Priority:** P0  
**Type:** BROWSER / SECURITY BOUNDARY

Integrate actual Servo runtime, configure protected networking before navigation, remove unproxied protected system-browser fallback, and prove direct-egress negative cases.

### T-042 — Real dual-transport / dual-relay failover

**Status:** TODO  
**Priority:** P1  
**Type:** RELIABILITY / NETWORK

At least four logical route candidates across two independent relay failure domains and materially different transport families. End-to-end health, jittered backoff, circuit breaking and stable switchback.

### T-043 — Harden upstream routing and SSRF containment

**Status:** TODO  
**Priority:** P0  
**Type:** SERVER SECURITY

Canonicalize/validate server-owned upstreams, prohibit metadata/link-local/arbitrary LAN/Internet pivots unless explicitly policy-owned, prevent redirect/DNS-rebinding escapes, and add adversarial tests.

### T-044 — Make CI a trustworthy security feedback loop

**Status:** TODO  
**Priority:** P1  
**Type:** CI / TEST-OF-TESTS

Add relevant `go test -race`, pinned mutation tooling, meaningful mutation targets for auth/transport/policy, targeted fuzz/property tests, failure classification and no-growth formatting debt.

### T-045 — Real end-to-end system qualification

**Status:** TODO  
**Priority:** P0 before release  
**Type:** E2E / SECURITY / CHAOS

Qualify real client→proxy→transport→relay→reverse-Origin→gateway→SecureAcces→service flow, including network transitions, revocation, relay/provider failure, restart, CGNAT/no-port-forward deployment and 24h soak.

### T-046 — Requalify release/distribution against production runtime

**Status:** TODO  
**Priority:** P1 before release  
**Type:** SUPPLY CHAIN / PRODUCT

Re-run packaging/signing/Telegram/update claims only after T-038/T-040/T-041/T-045 are qualified.

### T-047 — Final re-audit and convergence

**Status:** TODO  
**Priority:** P0 final gate  
**Type:** ADVERSARIAL AUDIT / DEBT DELETION

Full architecture/security/reliability/persistence/API/tests/mutation/CI/performance re-audit. Delete obsolete prototype paths. Convergence requires zero unresolved Critical findings and no unaccepted High findings.

### T-017 — Enforce verified-main repository rule

**Status:** BLOCKED  
**Priority:** P2  

Repository settings mutation is not currently available through the connected GitHub action surface. Continue independent work; never force push.

---

# 7. Dependency DAG and priority

```text
T-034 DONE
   ↓
T-035 false-readiness/fail-open removal
   ├──→ T-036 real local proxy + primary transport
   │       ├──→ T-037 Origin reverse connectivity
   │       └──→ T-042 multi-provider/multi-relay failover
   ├──→ T-040 platform keystore
   └──→ T-041 real Servo/proxy enforcement

T-038 SecureAcces authority ─┐
T-039 durable state ─────────┼→ T-045 real system qualification
T-043 SSRF hardening ────────┤
T-037/T-040/T-041/T-042 ─────┘

T-044 trustworthy CI supports every track and must land before T-045 final qualification.
T-045 → T-046 → T-047 convergence.
```

Current ordering by risk/dependency leverage:

1. **T-035** — prevents false security state and gives later tests trustworthy semantics.
2. **T-043 / T-038** — server trust boundaries.
3. **T-039** — durable state required before meaningful restart/revocation qualification.
4. **T-036 / T-037** — actual protected path and no-public-IP product core.
5. **T-040 / T-041** — real identity and browser boundaries.
6. **T-044 / T-042** — stronger feedback and resilient multi-path operation.
7. **T-045 / T-046 / T-047** — real qualification, release requalification, convergence.

Priority is recalculated after every successful push or material finding.

---

# 8. Verification strategy

Cheap-to-expensive ladder:

```text
formatter
→ targeted unit/characterization tests
→ property/fuzz/race where relevant
→ package tests
→ static analysis
→ integration/contract tests
→ security negatives
→ full Rust/Go/Python suite
→ mutation
→ benchmark
→ real system/chaos/soak qualification
```

Current baseline commands:

```text
python3 -m unittest discover -s scripts/tests -p 'test_*.py' -v
python3 scripts/project_manager.py verify --dry-run
python3 scripts/check_architecture.py
cargo metadata --locked --no-deps --format-version 1
cargo fmt --all -- --check
cargo check --workspace --all-targets --locked
cargo test --workspace --locked
cargo clippy --workspace --all-targets --locked -- -D warnings
cargo deny check --all-features
cd server && go vet ./...
cd server && go test ./...
```

Required additions tracked by T-044:

```text
cd server && go test -race ./...
cargo-mutants / selected Rust targets
selected Go mutation tool / critical Go packages
fuzz/property targets for parsers/policies/auth/state machines
```

A retry does not convert a flaky failure into PASS.

---

# 9. Multidimensional edge-space model

For critical paths project at least:

```text
INPUT × STATE × CONCURRENCY × TIMING × FAILURE × PERMISSIONS ×
CONFIGURATION × EXTERNAL STATE × VERSION × PLATFORM × RESOURCE PRESSURE
```

Baseline technique: equivalence partitions + boundary values + pairwise, then high-risk N-wise combinations, property/fuzz exploration and known production failure vectors.

Critical network examples:

- malformed/empty/stale signed config;
- configured relay with no listener;
- listener exists but relay/origin is unavailable;
- primary failure during active stream;
- both relays unavailable;
- UDP unavailable / TCP fallback only;
- DNS resolution failure/rebinding;
- Wi-Fi→Ethernet/hotspot transition;
- suspend/resume;
- Origin IP/router change;
- revocation during active request;
- process crash during failover;
- resource exhaustion and bounded queues.

---

# 10. Mutation strategy

Mutation testing is mandatory where technically applicable for:

- authorization allow/deny;
- device challenge verification;
- navigation/origin policy;
- fail-closed transport state transitions;
- signed config/release verification;
- service/upstream validators;
- release promotion/revocation;
- critical retry/circuit-breaker decisions.

Track semantic survivors, not raw score alone. Equivalent mutants are classified explicitly. A survived security mutant creates or updates a Finding before test changes.

---

# 11. Performance and reliability budgets

Do not optimize before measurement. T-045 establishes hard budgets from a real topology. Initial measurements must include:

- client start→protected proxy ready;
- protected navigation cold/warm latency;
- Client↔Relay RTT and Relay↔Origin RTT;
- proxy/gateway overhead;
- failover interruption duration;
- Origin reconnect after IP/router change;
- concurrent request throughput;
- CPU/RSS/allocations;
- lock contention;
- authorization/revocation convergence;
- 24h connection stability.

Security/correctness cannot be traded for benchmark wins.

---

# 12. Process rules

Before each substantial change:

1. synchronize remote `main` and determine HEAD;
2. read this file and relevant project instructions;
3. inspect CI and unresolved Critical/High findings;
4. select exactly one atomic task by risk/dependency leverage;
5. define root cause, invariants, change/protected surfaces, observable contract, failure modes, rollback and verification;
6. characterize current wrong behavior where possible;
7. implement minimum root-cause fix;
8. verify cheaply first, then attack solution;
9. record material unexpected evidence as a Finding before changing scope;
10. reconcile this plan before commit;
11. verify remote HEAD again; never overwrite concurrent work;
12. push only a verified state to `main`, never force;
13. append an iteration log and context checkpoint;
14. immediately recalculate priorities.

Every recurring defect class should gain a prevention/detection mechanism, not only a local fix.

---

# 13. Iteration log

Historical Iterations 1–9 are preserved in Git history before T-034. Their implementation artifacts remain useful, but production-completeness claims superseded by F-029..F-038 are no longer authoritative.

## Iteration 10

**Task:** T-034 — Restore execution truth and qualification semantics  
**Findings addressed:** F-029  
**Unexpected findings recorded:** F-030..F-038  
**Changes:** replaced stale false-converged plan with evidence-based current state, reopened unqualified capabilities, established supporting-doc non-authority, rebuilt task DAG and acceptance semantics.  
**Tests:** evidence reconciliation against current source and green baseline CI; no production code changed.  
**Mutation:** N/A — documentation/process task.  
**Plan changes:** major truth reconciliation; next task T-035.  
**Process improvements:** a capability cannot be `DONE` unless observable production side effects and qualification evidence exist; mocks/state models are explicitly distinguished from runtime capability.  
**Commit:** SELF — see the Git commit containing this log.  
**Push:** `main`  
**Result:** PASS when remote HEAD contains this plan.

---

# 14. Context compression checkpoint

```text
CURRENT HEAD: resolve from remote main before next iteration
CURRENT QUALIFIED MILESTONE: P0 server hardening baseline; production protected network path NOT qualified

ARCHITECTURE:
- Rust client contracts + Go server gateway
- target browser = Servo owned by WebGate
- target network = restricted local proxy → diverse transports/relays → outbound Origin reverse links
- target auth = SecureAcces authoritative

CRITICAL INVARIANTS:
- no false Ready/Running
- no direct protected egress/system-browser fallback
- session bound to device/user
- real device PoP; production keys platform-backed
- origin works behind CGNAT without inbound ports
- admin/data planes isolated and authorized
- routing/upstreams server-owned and non-generic

COMPLETED THIS ITERATION:
- T-034

RESOLVED FINDINGS:
- F-029

OPEN CRITICAL/HIGH FINDINGS:
- F-030 synthetic transport readiness
- F-031 no real Servo/proxied browser runtime
- F-032 synthetic production keystore
- F-033 SecureAcces surrogate
- F-034 no Origin reverse connectivity
- F-035 ephemeral state
- F-036 interim shared-token admin auth
- F-037 explicit client config can fail open
- F-038 race/mutation CI gap

BLOCKERS:
- T-017 repository-setting write capability unavailable
- real external relay/VPS/hardware validation may require external environment later; local deterministic implementation/tests can proceed

NEXT TASK:
- T-035 Eliminate false readiness and fail-open client bootstrap

WHY NEXT:
- highest dependency leverage: prevents security-state lies before real transport/browser work and makes subsequent tests trustworthy

CRITICAL FILES:
- MASTER_PLAN.md
- crates/webgate-app/src/main.rs
- crates/webgate-transport/src/failover.rs
- crates/webgate-transport/src/relay.rs
- crates/webgate-platform/src/keystore.rs
- server/pkg/auth/secureaccess.go
- server/cmd/webgate-server/main.go

VERIFICATION COMMANDS:
- Rust/Python/architecture suite from section 8
- Go vet/test from section 8
- race/mutation tracked by T-044

IMPORTANT DECISIONS:
- supporting implementation/research docs are non-authoritative for execution state
- production readiness requires real side effects + end-to-end evidence
- no force push

REJECTED OPTIONS:
- treating mocks/models as production qualification
- hiding stale task state behind historical DONE labels
- parallel roadmap outside MASTER_PLAN.md

NEW PROCESS LEARNING:
- task completion criteria must name observable production behavior, not only code artifacts/tests
```

---

# 15. Convergence criterion

WebGate converges only when:

- Critical findings = 0;
- High findings = 0 or explicitly accepted risk;
- P0 tasks are DONE;
- P1 tasks are DONE or evidence-based deferred;
- real browser/proxy/transport/relay/Origin/SecureAcces path is qualified;
- Origin no-public-IP/CGNAT operation is proven;
- key invariants are mechanically enforced;
- clean builds and trustworthy CI are reproducible;
- no unexplained flaky tests/silent error paths;
- relevant race/security/static/mutation gates pass;
- persistence crash/restart recovery is verified;
- compatibility and performance budgets pass;
- obsolete prototype/compatibility paths are removed;
- documentation matches implementation;
- final adversarial re-audit finds no fundamental blocker;
- final verified state is in `main`.
