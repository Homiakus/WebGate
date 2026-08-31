# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Last qualified main before Iteration 11:** `9e31ea07ccd722d8beb14e38d819085b2fa6f4d9`  
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
- transport SPI and a deterministic failover controller;
- failover startup that selects only a `Ready` provider exposing a proxy endpoint;
- repeated configured high latency participates in failover health;
- configured-only relay providers no longer claim connectivity;
- `SecureRelayTransport` fails closed until a real backend exists;
- explicit client `--config` load/parse failure is fatal instead of silently selecting defaults;
- Go `ProtectedService` registry and gateway baseline;
- server-side Ed25519 device proof-of-possession with single-use challenge;
- session↔device and user↔device binding checks;
- split loopback Data/Admin listeners;
- temporary fail-closed Admin bearer-token middleware;
- process spawn failure no longer reported as `RUNNING`;
- admin/dashboard/release/config/Telegram prototype surfaces.

## 3.2 Not production-qualified

The following are **not** production-ready:

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

Important current evidence:

- `crates/webgate-browser/Cargo.toml` has no Servo dependency; `BrowserCapsule` is a state/policy model.
- configured relay placeholders are deliberately `Offline` until T-036 provides real side effects.
- `webgate-app` still uses `InMemoryDeviceKeyStore`; this is explicitly tracked by F-032/T-040.
- desktop system browser is used only for the local control UI; protected navigation remains unavailable while transport is Offline.
- `SecureAccessAuthorizer` still stores sessions/memberships in process memory and is not authoritative SecureAcces.

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

Earlier completion criteria measured existence of abstractions/tests instead of observable production side effects, causing false convergence. T-034 restored evidence-based task state and qualification semantics.

## F-030 — Client transport readiness was synthetic

**Status:** PARTIALLY RESOLVED / CONTAINED by T-035  
**Category:** Network / Correctness / Security  
**Severity:** Critical  
**Confidence:** High  

Characterization on commit `c7ecb7759a16d6ec53334ce0f04428c70fa0548a` proved old startup selected Offline providers, accepted `Ready` without an endpoint, ignored the high-latency threshold and selected an unready fallback. T-035 removes those false-success states. The finding remains open until T-036 supplies a real bound/probed transport provider.

## F-031 — Protected browser path is not a real Servo/proxied runtime

**Status:** OPEN  
**Category:** Browser / Security Boundary  
**Severity:** Critical  
**Confidence:** High  

`webgate-browser` currently models state/policy only. The local desktop control UI may open in Edge/Chrome/system browser, but protected resources must not do so. Real protected browser ownership is T-041.

## F-032 — Production entrypoint uses synthetic device keys

**Status:** OPEN  
**Category:** Identity / Key Management  
**Severity:** Critical  
**Confidence:** High  

`webgate-app` still instantiates `InMemoryDeviceKeyStore`; implementation generates synthetic key material/signatures. T-035 now labels this limitation explicitly instead of treating it as production qualification.

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

No production Origin agent currently establishes persistent outbound connections to independent relays. The core “server behind CGNAT without public IP” path is not implemented end-to-end.

**Resolution:** T-037.

## F-035 — Security/operations state is largely ephemeral

**Status:** OPEN  
**Category:** Persistence / Reliability  
**Severity:** High  
**Confidence:** High  

Registries, local sessions/memberships, audit/process/release state are primarily memory-backed.

**Resolution:** T-039.

## F-036 — Admin authentication is only an interim shared token

**Status:** OPEN / CONTAINED  
**Category:** Admin Security  
**Severity:** High  
**Confidence:** High  

P0 hardening moved Admin to a separate loopback listener and requires a strong `WEBGATE_ADMIN_TOKEN`. This is containment, not the target SecureAcces-backed administrator/session/device model.

**Resolution:** T-038.

## F-037 — Explicit client config failed open to defaults

**Status:** RESOLVED by T-035  
**Category:** Configuration / Fail-Closed  
**Severity:** High  
**Confidence:** High  

Old `ClientConfigProfile::load_from_file(...).unwrap_or_default()` silently substituted defaults after an explicitly requested file failed. T-035 separates “no config requested” from “requested config invalid/unavailable” and fails the latter path.

## F-038 — CI lacks race/mutation/security depth promised by the plan

**Status:** OPEN  
**Category:** Test System / CI  
**Severity:** High  
**Confidence:** High  

Go `vet/test` is gated, but `go test -race`, pinned mutation tooling and targeted fuzz/property gates are not yet required by CI.

**Resolution:** T-044.

## F-039 — Runtime config binding reports success after parse/apply failure

**Status:** OPEN  
**Category:** Configuration / API Correctness / Observability  
**Severity:** High  
**Confidence:** High  

During T-035 review, `POST /api/bind_config` in the local client control server was observed to attempt parse/write opportunistically but always return HTTP 200 `{"status":"ok"}`. Invalid content or poisoned lock therefore cannot be distinguished by the caller.

**Root cause:** parse/apply errors are discarded inside an untyped ad-hoc HTTP handler.

**Affected invariants:** I-003, I-005, I-019, I-020.

**Decision:** do not expand T-035's CLI/bootstrap contract into a runtime HTTP protocol migration. Split T-048.

---

# 6. Reconciled task state

Status meanings: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`.

## 6.1 Trusted completed foundations

- **T-001 — Living execution-plan foundation:** DONE.
- **T-002 — Portable Rust workspace/executable baseline:** DONE.
- **T-003 — Rust dependency/architecture CI baseline:** DONE.
- **T-006 — Android lifecycle state-model probe:** DONE as a probe; not production Android qualification.
- **T-007 — Strict navigation/deep-link policy model:** DONE.
- **T-009 — Algorithm-agile device identity model:** DONE.
- **T-021 — ProtectedService registry baseline:** DONE as in-memory domain baseline.
- **T-022 — Multi-service gateway baseline:** DONE as local server baseline; SSRF/persistence qualification remains.
- **T-024 — Admin UI prototype:** DONE as UI capability, not production admin-security qualification.
- **T-025 — Server device registry + real Ed25519 PoP:** DONE for current in-memory server registry.
- **T-030 — Process spawn/lifecycle baseline:** DONE after false-Running fix; deeper supervision remains reliability scope.
- **T-032 — Editorial UI transformation:** DONE.
- **T-034 — Restore execution truth and qualification semantics:** DONE.
- **T-035 — Eliminate false readiness and fail-open client bootstrap:** DONE after green work-branch qualification and fast-forward to `main`.

## 6.2 Reopened / requalification-required historical tasks

- **T-004 — Real Servo embedding adapter:** REOPENED; current browser crate has no Servo runtime dependency.
- **T-005 — Real fail-closed browser networking:** REOPENED; current capsule proves policy state, not real browser network behavior.
- **T-008 — Failover controller:** core false-readiness semantics repaired by T-035; real-provider qualification remains T-036/T-042.
- **T-010 — Platform device-key adapters:** REOPENED; production entrypoint still uses synthetic in-memory keystore.
- **T-011 — SecureAcces integration:** REOPENED; current authorizer is local memory state.
- **T-012 — Primary production transport:** REOPENED.
- **T-013 — Independent fallback/dual-relay:** REOPENED.
- **T-014 — Servo/site/security/performance qualification:** REOPENED.
- **T-015 — Production release authority:** NEEDS_REQUALIFICATION after real runtime/security path exists.
- **T-016 — Final adversarial re-audit:** REOPENED by F-029..F-039.
- **T-019 — Trusted broker boundary:** NEEDS_REQUALIFICATION against real browser/process boundary.
- **T-023 — Admin Control API:** NEEDS_REQUALIFICATION; loopback + token is interim auth only.
- **T-026 — Audit/health operations:** NEEDS_REQUALIFICATION with durable state and end-to-end health.
- **T-027 — Full adversarial E2E qualification:** REOPENED; current tests do not traverse real browser→transport→relay→origin path.
- **T-028 — Telegram/release distribution:** NEEDS_REQUALIFICATION against authoritative users/devices/releases and real client runtime.
- **T-029 — Config profile binding:** CLI explicit-load fail-open repaired by T-035; signed policy/runtime binding remain open.
- **T-031 — Telegram Admin Bot lifecycle:** NEEDS_REQUALIFICATION under target admin authorization and persistence model.
- **T-033 — Integrity audit:** historical audit complete, but production-completeness conclusions are superseded by F-029..F-039.

## 6.3 Active execution tasks

### T-035 — Eliminate false readiness and fail-open client bootstrap

**Status:** DONE  
**Priority:** P0  
**Type:** CORRECTNESS / SECURITY / FOUNDATION

Acceptance satisfied:

- startup never selects provider solely because it is configured;
- `Ready` without proxy endpoint is unusable;
- ready fallback can be selected if primary is unavailable;
- unready fallback causes fail-closed Offline state;
- repeated latency above configured threshold counts toward failover;
- `SecureRelayTransport` no longer simulates tunnel start/probe success;
- explicit invalid/missing `--config` does not silently become defaults;
- control UI status/navigate responses do not claim a protected proxy while transport is Offline;
- regression/negative tests exist.

### T-036 — Implement real destination-restricted loopback proxy + primary provider

**Status:** TODO  
**Priority:** P0  
**Type:** NETWORK / SECURITY

Implement a real listener, destination allowlist, bounded proxy semantics, provider lifecycle, authenticated control boundary, cancellation and health. `Ready` requires real side effects and protected-path evidence.

### T-037 — Implement Origin agent and reverse Relay A/B connectivity

**Status:** TODO  
**Priority:** P0  
**Type:** NETWORK / CGNAT / RELIABILITY

Persistent outbound Origin connections, authentication, multiplexed streams, reconnect/backoff, relay registration, graceful rotation and local gateway forwarding. Prove no inbound port forwarding is needed.

### T-038 — Integrate authoritative SecureAcces + administrator authorization

**Status:** TODO  
**Priority:** P0  
**Type:** AUTHORIZATION / ADMIN SECURITY

Replace production in-memory session/membership authority with a narrow SecureAcces adapter; local fake remains test-only. Bind admin identity/session/device/management permission. Unknown/unavailable authorization fails closed.

### T-039 — Durable transactional server state

**Status:** TODO  
**Priority:** P0/P1  
**Type:** PERSISTENCE / RELIABILITY

Persist WebGate-owned service/device/release/audit/config metadata using a transactional store. SQLite is preferred for the small deployment unless measurements require otherwise. SecureAcces-owned identity/permission data remains SecureAcces-owned.

### T-040 — Production platform key stores

**Status:** TODO  
**Priority:** P0  
**Type:** IDENTITY / PLATFORM SECURITY

Windows CNG/DPAPI/TPM-backed implementation, Android Keystore, explicit assurance-tier fallbacks elsewhere. `InMemoryDeviceKeyStore` becomes test/dev-only and production build/runtime rejects it.

### T-041 — Real Servo runtime and enforced protected proxy

**Status:** TODO  
**Priority:** P0  
**Type:** BROWSER / SECURITY BOUNDARY

Integrate actual Servo runtime, configure protected networking before navigation, remove any protected system-browser fallback, and prove direct-egress negatives.

### T-042 — Real dual-transport / dual-relay failover

**Status:** TODO  
**Priority:** P1  
**Type:** RELIABILITY / NETWORK

At least four logical route candidates across two independent relay failure domains and materially different transport families. End-to-end health, jittered backoff, circuit breaking and stable switchback.

### T-043 — Harden upstream routing and SSRF containment

**Status:** READY  
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

Qualify real client→proxy→transport→relay→reverse-Origin→gateway→SecureAcces→service flow, including network transitions, revocation, relay/provider failure, restart, CGNAT/no-port-forward deployment and soak.

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

### T-048 — Make runtime client config binding transactional and fail closed

**Status:** TODO  
**Priority:** P1/HIGH  
**Type:** CONFIGURATION / API CORRECTNESS

Characterize `POST /api/bind_config`; replace ad-hoc success response with typed parse/apply result, bounded body parsing and atomic profile swap. Invalid content, poisoned lock or apply failure must return explicit error and leave the last valid profile unchanged.

### T-017 — Enforce verified-main repository rule

**Status:** BLOCKED  
**Priority:** P2

Repository settings mutation is not currently available through the connected GitHub action surface. Continue independent work; never force push.

---

# 7. Dependency DAG and priority

```text
T-034 DONE → T-035 DONE

T-035
   ├──→ T-036 real local proxy + primary transport
   │       ├──→ T-037 Origin reverse connectivity
   │       └──→ T-042 multi-provider/multi-relay failover
   ├──→ T-040 platform keystore
   ├──→ T-041 real Servo/proxy enforcement
   └──→ T-048 runtime config transaction

T-043 SSRF hardening ─────────┐
T-038 SecureAcces authority ──┤
T-039 durable state ──────────┼→ T-045 real system qualification
T-037/T-040/T-041/T-042 ─────┘

T-044 trustworthy CI supports every track and must land before T-045 final qualification.
T-045 → T-046 → T-047 convergence.
```

Current ordering by risk/dependency leverage:

1. **T-043** — server gateway is already runnable; close its SSRF/pivot boundary before adding real remote connectivity.
2. **T-038** — authoritative authorization/admin trust boundary.
3. **T-039** — durable state before restart/revocation qualification.
4. **T-036 / T-037** — actual protected path and no-public-IP product core.
5. **T-040 / T-041** — real identity and browser boundaries.
6. **T-044 / T-042 / T-048** — stronger feedback, resilient multi-path, runtime config correctness.
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

T-035 covered representative combinations:

- provider state: Ready / Offline;
- endpoint: present / absent;
- primary vs fallback readiness combinations;
- success with normal vs pathological latency;
- repeated failure threshold boundary;
- zero latency threshold semantics;
- explicit config absent vs explicitly missing;
- UI reporting with no protected endpoint.

Future network work additionally covers listener exists but relay/origin unavailable, active-stream failure, both relays unavailable, UDP restricted, DNS failure/rebinding, network transition, suspend/resume, Origin IP change, revocation during request, process crash and resource pressure.

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

For T-035, tests are specifically expected to kill semantic mutations such as:

- `Ready && endpoint` → `Ready || endpoint`;
- primary readiness branch inverted;
- fallback readiness check removed;
- `latency > threshold` ignored or boundary changed incorrectly;
- explicit config error converted to default success.

Pinned automated mutation infrastructure remains T-044. Manual semantic mutation evidence may be used before T-044 but does not replace it.

---

# 11. Performance and reliability discipline

Do not optimize before measurement. T-045 establishes hard budgets from a real topology. Measurements include startup→protected proxy ready, protected navigation cold/warm latency, Client↔Relay RTT, Relay↔Origin RTT, proxy/gateway overhead, failover interruption, Origin reconnect, throughput, CPU/RSS/allocations, lock contention, authorization/revocation convergence and long-session stability.

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

Historical Iterations 1–9 are preserved in Git history before T-034. Their implementation artifacts remain useful, but production-completeness claims superseded by F-029..F-039 are no longer authoritative.

## Iteration 10

**Task:** T-034 — Restore execution truth and qualification semantics  
**Findings addressed:** F-029  
**Unexpected findings recorded:** F-030..F-038  
**Changes:** replaced stale false-converged plan with evidence-based current state, reopened unqualified capabilities, established supporting-doc non-authority, rebuilt task DAG and acceptance semantics.  
**Tests:** evidence reconciliation against current source; `main` CI run 37 passed Rust/project/architecture/dependency and Go gates.  
**Mutation:** N/A — documentation/process task.  
**Plan changes:** major truth reconciliation; selected T-035.  
**Process improvements:** a capability cannot be `DONE` unless observable production side effects and qualification evidence exist; mocks/state models are explicitly distinguished from runtime capability.  
**Commit:** `9e31ea07ccd722d8beb14e38d819085b2fa6f4d9`  
**Push:** `main`  
**Result:** PASS.

## Iteration 11

**Task:** T-035 — Eliminate false readiness and fail-open client bootstrap  
**Findings addressed:** F-030 (contained/partial), F-037 (resolved)  
**Unexpected findings:** F-039 split to T-048  
**Characterization:** test-only branch commit `c7ecb7759a16d6ec53334ce0f04428c70fa0548a` produced expected RED after formatter/check passed: five failover contract tests failed on old logic (Offline primary selection, missing endpoint acceptance, ignored latency threshold, unready fallback). This red branch is not merged to main.  
**Changes:** failover selects only actually Ready providers with endpoints; fallback is readiness-checked; repeated high latency participates in health; configured relay placeholders and incomplete `SecureRelayTransport` no longer advertise Ready/proxy success; explicit requested config load failure is fatal; local UI status/navigation no longer claim protected connectivity while Offline.  
**Tests:** startup primary/fallback matrix, endpoint absence, dual-offline, latency threshold/boundary, unready fallback, explicit missing config, configured-only provider, offline proxy serialization, existing failover regressions.  
**Mutation:** pinned automatic tooling is still F-038/T-044; test contracts target condition-inversion/removal mutants for provider readiness, endpoint presence, fallback readiness, latency threshold and explicit-config error handling. Manual semantic mutation check is performed on the green candidate before main promotion.  
**Race:** no new shared mutable transport state; controller remains single-owner. Full Go race gate is tracked by T-044.  
**Security:** I-003/I-005/I-006/I-020 strengthened; incomplete provider state is deny/offline.  
**Performance:** no optimization; state checks are O(1), no I/O added.  
**Compatibility:** explicit invalid `--config` changes from silent default to intentional fail-closed error; this is a security contract correction.  
**Plan reconciliation:** T-035 DONE after green branch qualification; F-037 resolved; F-030 contained until T-036; F-039/T-048 added; next task T-043.  
**Process learning:** use isolated red characterization branches to prove pre-fix behavior, then build one clean atomic candidate from qualified `main`; never merge intentionally red commits.  
**Commit:** SELF — the Git commit containing this log.  
**Push:** `main` after work-branch CI and semantic mutation evidence pass.  
**Result:** PASS when remote `main` contains the verified candidate.

---

# 14. Context compression checkpoint

```text
CURRENT HEAD: resolve from remote main before next iteration
CURRENT QUALIFIED MILESTONE: false-readiness containment complete; production protected network path NOT qualified

ARCHITECTURE:
- Rust client contracts + Go server gateway
- target browser = Servo owned by WebGate
- target network = restricted local proxy → diverse transports/relays → outbound Origin reverse links
- target auth = SecureAcces authoritative

CRITICAL INVARIANTS:
- no false Ready/Running
- no direct protected egress/system-browser protected fallback
- session bound to device/user
- real device PoP; production keys platform-backed
- origin works behind CGNAT without inbound ports
- admin/data planes isolated and authorized
- routing/upstreams server-owned and non-generic

COMPLETED RECENTLY:
- T-034 execution truth reconciliation
- T-035 false-readiness + explicit-config fail-closed

RESOLVED/CONTAINED FINDINGS:
- F-029 resolved
- F-030 contained/partial until real provider
- F-037 resolved

OPEN CRITICAL/HIGH FINDINGS:
- F-030 real provider still absent
- F-031 no real Servo/proxied browser runtime
- F-032 synthetic production keystore
- F-033 SecureAcces surrogate
- F-034 no Origin reverse connectivity
- F-035 ephemeral state
- F-036 interim shared-token admin auth
- F-038 race/mutation CI gap
- F-039 runtime config bind false-success

BLOCKERS:
- T-017 repository-setting write capability unavailable
- external relay/VPS/hardware qualification may require environment later; deterministic implementation/tests can proceed

NEXT TASK:
- T-043 Harden upstream routing and SSRF containment

WHY NEXT:
- server gateway is already runnable and will become remotely reachable through future reverse transport; close SSRF/pivot boundary before adding that connectivity

CRITICAL FILES:
- MASTER_PLAN.md
- crates/webgate-app/src/main.rs
- crates/webgate-transport/src/failover.rs
- crates/webgate-transport/src/relay.rs
- server/pkg/gateway/gateway.go
- server/pkg/domain/service.go
- server/pkg/registry/service_registry.go

VERIFICATION:
- Rust/Python/architecture/dependency suite
- Go format/vet/test
- T-044 adds race/mutation/fuzz gates

IMPORTANT DECISIONS:
- configured endpoints are not readiness
- incomplete provider = Offline, no synthetic proxy
- explicit requested config failure = deny/fatal
- local control UI browser is not protected browser runtime
- no force push

REJECTED OPTIONS:
- preserve fake Ready for demo UX
- silently default after explicit config failure
- merge red characterization commit
- widen T-035 into runtime bind protocol or real VPN implementation

NEW PROCESS LEARNING:
- RED characterization can be preserved off-main and cited as evidence while main receives one green logical implementation commit
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
