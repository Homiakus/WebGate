# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Reconciled:** 2026-09-01  
**Last qualified main before Iteration 15:** `26d05d53a24aa221370ce66b4deadee0b568a7d4`

This file is the **only execution source of truth**. Supporting documents under `docs/` are evidence/design references; they do not own task state, priority, acceptance, or release readiness.

A task is `DONE` only when its observable production contract is implemented, relevant negative tests exist, required verification passes, and the verified state reaches `main` without force push. Models, mocks, interfaces, probes, work-branch candidates, and documentation are not production qualification by themselves.

---

# 1. Mission

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
loopback authority bridge
  ↓
private + durable SecureAcces authority
  ↓
registered private service
```

Origin must work behind dynamic IP / CGNAT with no inbound port forwarding. Protected traffic must never silently escape through normal OS Internet, an unproxied browser, an unowned upstream, a local authorization surrogate, or an unavailable authority. Missing authoritative dependencies fail closed.

---

# 2. Truth hierarchy

1. Observed runtime behavior.
2. Reproducible tests/experiments.
3. Security/correctness invariants.
4. Code.
5. `MASTER_PLAN.md`.
6. Older design documents.
7. Initial assumptions.

Material unexpected evidence becomes `F-XXX` before task scope/order changes.

---

# 3. Current verified state

## Verified foundations

- Rust workspace/architecture boundaries and locked CI.
- Go server gate on Go 1.26.6.
- Navigation/browser-capsule state models.
- Transport SPI/failover state model; configured-only providers remain `Offline`.
- Explicit client config failure is fatal.
- Ed25519 device PoP with single-use challenge.
- Split loopback Data/Admin listeners and interim Admin bearer containment.
- Process spawn failure cannot report `RUNNING`.
- **T-043:** loopback-only server upstream invariant at construction/mutation/use; environment-proxy and redirect escape contained.
- **T-049:** reproducible SecureAcces v0.4.0 dependency anchor + Go 1.26.6.
- **T-050:** gateway depends on context-aware `ServiceAuthorizer`; missing/unavailable authority is fail-closed `503`, ordinary policy deny is `403`.
- **T-052 public half:** final/main `e7dc960722cffc68b778bb35cab449af096f8273` provides loopback-only remote authority, explicit SecureAcces `AccountID` device binding, strict response validation, redirect/proxy refusal, bounded responses and child-process control-secret scrubbing; repeated main CI passed.
- **T-039A:** final/main `26d05d53a24aa221370ce66b4deadee0b568a7d4` establishes registry ownership: mutable inputs/reads/challenges are detached and process/Telegram writes use explicit registry mutation APIs; branch mutant was killed and repeated main CI passed.

## Not production-qualified

- Real Servo runtime and enforced browser proxy.
- Real destination-restricted client proxy.
- Real primary/fallback transports and Relay A/B.
- Origin reverse-connectivity / no-public-IP path.
- Platform-backed production device keys.
- **Private SecureAcces authority process in its own `main`:** candidate exists only on a private work branch; executable CI remains externally blocked before runner steps.
- Durable SecureAcces Accounts/Users/Memberships/Sessions/Audit deployment + backup/restore evidence.
- SecureAcces-backed administrator management authorization.
- Full WebGate-owned durability. Service/device/release transactional SQLite persistence is branch-qualified in T-039B1 but must still reach/repass `main`; audit/config durability plus backup/restore remain T-039B2.
- Real end-to-end/chaos/release qualification.

---

# 4. Critical invariants

- **I-001 Browser ownership:** protected content only through a WebGate-qualified browser path.
- **I-002 Application-scoped routing:** normal mode does not change OS default route.
- **I-003 Fail closed:** transport/authority/browser/config/state loss never produces direct or surrogate fallback.
- **I-004 No silent engine fallback:** protected content never opens in an unproxied system browser.
- **I-005 Real readiness:** `Ready`/`Running` means required side effects and health proof succeeded.
- **I-006 Destination restriction:** browser-facing proxy is loopback-only and policy bounded.
- **I-007 Network access ≠ authorization:** transport credentials never grant app access.
- **I-008 SecureAcces authority:** production session/workspace/permission decisions come from real SecureAcces state, never WebGate maps.
- **I-009 Device binding:** `SecureAcces Session.DeviceID == WebGate Device.ID` and `SecureAcces Principal.Account().ID == WebGate Device.AccountID`; tenant-local `UserID` is not a substitute for global `AccountID`.
- **I-010 Real PoP:** activation requires cryptographic proof over short-lived single-use challenge.
- **I-011 Production keys:** platform-backed; synthetic/in-memory key stores are test-only.
- **I-012 Origin no-public-IP:** no inbound NAT; Origin maintains outbound persistent links.
- **I-013 Failure-domain diversity:** ≥2 materially independent relay domains.
- **I-014 Transport diversity:** fallback differs materially in protocol/implementation/failure mode.
- **I-015 Admin isolation:** privileged operations require management authorization + audit on isolated plane.
- **I-016 Server-owned routing:** client cannot choose authoritative upstream/tenant/workspace/permissions/process fields.
- **I-017 No generic proxy:** no SSRF/open-proxy pivot.
- **I-018 Durable security state:** restart/crash cannot silently reset authoritative state.
- **I-019 Signed policy/release:** signed/versioned/rollback-aware artifacts where applicable.
- **I-020 No false qualification:** mock/interface/demo/work-branch candidate cannot mark production capability complete.
- **I-021 Trusted CI:** every production runtime/provider path has build/static/test gates; critical concurrency gets race checks.
- **I-022 Mutation resistance:** critical allow/deny/state logic has meaningful mutation testing.
- **I-023 Private-source boundary:** public WebGate must not implicitly publish private SecureAcces implementation.
- **I-024 Control-secret containment:** authority/admin credentials are not forwarded to protected upstreams or inherited by WebGate-launched service processes.
- **I-025 Registry ownership:** WebGate registries own mutable state; inputs/results are detached and mutations pass through explicit locked APIs.
- **I-026 Persist-before-memory:** durable WebGate mutations commit to the transactional store before authoritative in-memory state changes; persistence failure leaves memory unchanged.
- **I-027 Runtime is ephemeral:** process PID/state/StartedAt are never resurrected from durable configuration as `RUNNING` after restart.
- **I-028 Durable identity:** persisted devices require explicit global `AccountID`; `UserID` is never guessed into `AccountID`.

---

# 5. Findings registry

Historical F-001..F-028 remain in Git history.

- **F-029 — False convergence:** RESOLVED by T-034.
- **F-030 — Synthetic client transport readiness:** CONTAINED by T-035; real provider T-036.
- **F-031 — No real Servo/proxied runtime:** OPEN / Critical → T-041.
- **F-032 — Synthetic production keystore:** OPEN / Critical → T-040.
- **F-033 — Real SecureAcces production authority absent:** OPEN / Critical → T-052. Public bridge is qualified in `main`; private provider is not qualified/promoted.
- **F-034 — Origin reverse connectivity absent:** OPEN / Critical → T-037.
- **F-035 — Security/operations state ephemeral:** PARTIALLY RESOLVED / High → T-039 + T-052. T-039B1 covers WebGate service/device/release records once promoted; audit/config/backup remain T-039B2 and SecureAcces durability remains T-052.
- **F-036 — Admin auth is interim shared token:** OPEN/CONTAINED / High → T-051.
- **F-037 — Explicit config failed open:** RESOLVED by T-035.
- **F-038 — Race/mutation/fuzz CI depth missing:** OPEN / High → T-044.
- **F-039 — Runtime client config can report false success:** OPEN / High → T-048.
- **F-040 — SecureAcces dependency/toolchain/auth boundaries under-modeled:** PARTIALLY RESOLVED by T-049/T-050; T-052/T-051 remain.
- **F-041 — Private/durable SecureAcces deployment channel absent:** OPEN / Critical → T-052. Private sidecar candidate exists; private GitHub Actions still fail before executable steps and cannot qualify it.
- **F-042 — Device identity conflated tenant UserID with global AccountID:** CONTAINED / High → T-052 + T-039. Public authority path requires `AccountID`; T-039B1 durable device store also rejects missing `AccountID`. Legacy migration/cleanup semantics remain before convergence.
- **F-043 — Child processes inherited control-plane secrets:** RESOLVED by T-052 public half.
- **F-044 — Registry state escaped through live mutable pointers:** RESOLVED by T-039A final/main `26d05d53a24aa221370ce66b4deadee0b568a7d4` with repeated main CI and killed pointer mutant.

T-038 remains a convergence umbrella: T-049 → T-050 → T-052 → T-051.

---

# 6. Task state

Status vocabulary: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`.

## Completed / trusted foundations

T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(in-memory baseline), T-030, T-032, T-034, T-035, T-043, T-049, T-050 and T-039A are DONE under their recorded scopes.

### T-050 — Establish fail-closed data-plane authority boundary
**Status:** DONE · **Priority:** P0 · **Type:** AUTHORIZATION BOUNDARY

Evidence: RED `df3e1defa3689c68538ae199f9a76d8930aff87b`; corrected green `732e9aab8114d0c74edb1916ce5bdb9e90653749`; fail-open mutant `e3ba593ca509d9f3d4077f371751d7f12de8159c` killed; final/main `2805999a3b59472cbbb02156aefee99689b3cf60` passed repeated main CI.

### T-039A — Registry ownership boundary
**Status:** DONE · **Priority:** P0 · **Type:** STATE OWNERSHIP

Corrected RED `fb84310a8473522dd932219bb8e0b312914f2015` failed on observable service/device aliasing after gofmt/vet passed. Green `1b06e56c3808ca85c7866d379dcbafa6d14f1b66` detached service/device/release/challenge state and moved ProcessManager/Telegram mutations through registry APIs. Mutant `7afc1e4cceb5e4d9ce7a1960d84ed4d88470a7ab` exposed an internal service pointer and was killed by `TestServiceRegistryOwnsStoredState`. Final/main `26d05d53a24aa221370ce66b4deadee0b568a7d4` passed pre-promotion and repeated main CI.

## Reopened / requalification-required historical work

- T-004/T-005 Servo/runtime networking — REOPENED.
- T-008 failover — model repaired; real qualification T-036/T-042.
- T-010 platform device keys — REOPENED.
- T-011 SecureAcces integration — REOPENED; execution T-049/T-050/T-052/T-051.
- T-012/T-013 production transports/relays — REOPENED.
- T-014 browser/site qualification — REOPENED.
- T-015 release authority — NEEDS_REQUALIFICATION.
- T-016 final audit — REOPENED.
- T-019 broker boundary — NEEDS_REQUALIFICATION.
- T-023 Admin API — NEEDS_REQUALIFICATION under target auth.
- T-026 operations/audit — NEEDS_REQUALIFICATION with durability.
- T-027 E2E — REOPENED.
- T-028 distribution — NEEDS_REQUALIFICATION.
- T-029 config binding — CLI failure fixed; signed/runtime binding open.
- T-031 Telegram Admin lifecycle — NEEDS_REQUALIFICATION.
- T-033 integrity audit — historical only; completeness claims superseded.

## Active execution tasks

### T-038 — Authoritative SecureAcces + administrator authorization
**Status:** IN_PROGRESS · **Priority:** P0  
Complete when T-049 + T-050 + T-052 + T-051 are complete.

### T-052 — Private durable SecureAcces production provider/deployment
**Status:** IN_PROGRESS / PRIVATE-CI-BLOCKED · **Priority:** P0

Public half is qualified in WebGate `main` at `e7dc960722cffc68b778bb35cab449af096f8273` with repeated main CI PASS. Private RED `c0a0f82cc378142724a42e7178687919ce5ccdb9` and green candidate `b1d412387087bcaed9da5abf5148acd558c8708c` exist in `Homiakus/SecureAcces`, but private workflows terminate before executable steps. Private main remains `827abb1add11a9fcbd0a9944e65efbd20c675739`. T-052 remains incomplete until real private CI passes, promotion is normal, public/private protocol is exercised and SecureAcces backup/restore evidence exists.

### T-051 — SecureAcces administrator management authorization
**Status:** TODO · **Priority:** P0  
After T-052, replace shared-token-only authority with request-scoped SecureAcces management authorization and audited privileged decisions.

### T-039 — Durable transactional WebGate-owned state
**Status:** IN_PROGRESS · **Priority:** P0/P1

#### T-039B1 — Durable service/device/release registries
**Status:** QUALIFIED ON WORK BRANCH / PENDING FINAL MAIN · **Priority:** P0

Scope/evidence:
- corrected RED `70133ebda74f1b5061689646a820126e519ecd9b`: gofmt passed; vet failed only because persistence APIs did not exist;
- first green `0a1e64bd34476825ce7634b801fc6e1f1378b033` was rejected before code qualification because Go checksums were not committed and a SecureAcces structural test assumed single-line `require` syntax;
- lock probe `86e8b7b105af6dcfba51960ef2dcf0fc1571af07` ran `go mod tidy` under Go 1.26.6 and produced the exact `modernc.org/sqlite v1.57.0` dependency graph/checksums; probe failure was intentional and is not production history;
- green2 `8e7a61b291dd8e1cb7d6db96b236301fe5ed270a` full CI PASS with real SQLite restart/corruption/failure tests;
- persist-order mutant `d404e823ae1f1f1a91d3dff721f550cc8cc28e7d` passed gofmt/vet then was killed by both injected-persistence and closed-store tests because memory advanced before disk;
- green3 `6d6372071403fcdf53751f745e13e19c1c58c987` full CI PASS after adding `go mod tidy -diff` and `CGO_ENABLED=0` portability gates.

Implementation contract:
- pure-Go `modernc.org/sqlite v1.57.0`, pinned with `go.sum`;
- schema/versioned SQLite state with `journal_mode=WAL`, `synchronous=FULL`, busy timeout and restricted file permissions;
- SHA-256 checksums bind record kind/key/payload and corrupted records fail startup/load;
- durable service/device/release mutations persist before memory changes;
- failed durable writes do not advance authoritative memory;
- process PID/state/StartedAt are excluded from durable service snapshots and restore as STOPPED/0/nil;
- durable devices require explicit `AccountID`; no `UserID → AccountID` inference;
- PoP challenges remain ephemeral and are never restored;
- production startup opens/restores durable registries before config application and fails closed on corrupt/incompatible state;
- default state path is `data/webgate-state.db`, overridable through `WEBGATE_STATE_DB` / `--state-db`.

B1 becomes complete only after a clean final commit built directly from qualified `main` reaches `main` and repeated main CI passes.

#### T-039B2 — Durable audit/config + operational recovery
**Status:** TODO · **Priority:** P0/P1

Persist append-only/auditable management events and WebGate-owned runtime configuration metadata without storing control secrets. Add schema migrations where required, consistent snapshot/backup procedure, restore validation, crash/interruption tests and operator documentation. T-039 is not DONE until B2 backup/restore is proven.

### T-036 — Real destination-restricted loopback proxy + primary provider
**Status:** TODO · **Priority:** P0.

### T-037 — Origin agent + reverse Relay A/B connectivity
**Status:** TODO · **Priority:** P0.

### T-040 — Production platform key stores
**Status:** TODO · **Priority:** P0.

### T-041 — Real Servo runtime + enforced protected proxy
**Status:** TODO · **Priority:** P0.

### T-042 — Real dual-transport / dual-relay failover
**Status:** TODO · **Priority:** P1.

### T-044 — Trustworthy security feedback loop
**Status:** TODO · **Priority:** P1. Add `go test -race`, pinned automated mutation tooling, fuzz/property gates and failure classification. T-039B1 already adds scoped Go module-lock and no-CGO portability gates; these do not complete T-044.

### T-045 — Real end-to-end qualification
**Status:** TODO · **Priority:** P0 before release.

### T-046 — Requalify release/distribution
**Status:** TODO · **Priority:** P1 before release.

### T-047 — Final re-audit/convergence
**Status:** TODO · **Priority:** P0 final gate.

### T-048 — Transactional fail-closed runtime client config binding
**Status:** TODO · **Priority:** P1/HIGH.

### T-017 — Enforce verified-main repository rule
**Status:** BLOCKED · **Priority:** P2. Repository-setting write capability unavailable; never force push.

---

# 7. Dependency DAG / current order

```text
T-034 DONE → T-035 DONE
T-043 DONE
T-049 DONE → T-050 DONE → T-052(private provider + public bridge) → T-051 → T-038 convergence ─┐
T-039A DONE → T-039B1 → T-039B2 ───────────────────────────────────────────────────────────────┼→ T-045
T-035 → T-036 → T-037 ─────────────────────────────────────────────────────────────────────────┤
      ├→ T-040 ─────────────────────────────────────────────────────────────────────────────────┤
      ├→ T-041 ─────────────────────────────────────────────────────────────────────────────────┤
      └→ T-042 ─────────────────────────────────────────────────────────────────────────────────┘
T-044 must land before T-045 final qualification.
T-048 is independent High/P1 work.
T-045 → T-046 → T-047.
```

Priority now:
1. finish **T-039B1 final/main qualification**;
2. **T-039B2** audit/config durability + backup/restore;
3. **T-052 private half** immediately when executable private CI is available;
4. **T-051** management authority;
5. **T-036/T-037** transport + no-public-IP core;
6. **T-040/T-041** key/browser boundaries;
7. **T-044/T-042/T-048** feedback/resilience/config;
8. **T-045/T-046/T-047** qualification/release/convergence.

---

# 8. Verification / test-of-tests

Baseline:

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
cd server && go mod tidy -diff
cd server && go vet ./...
cd server && go test ./...
cd server && CGO_ENABLED=0 go test ./pkg/persistence ./pkg/registry
cd server && CGO_ENABLED=0 go build ./cmd/webgate-server
```

T-044 later adds race + pinned automated mutation/fuzz gates. Retries never turn flakiness into PASS. An infrastructure retry counts only if real steps execute.

Permanent T-039 negatives:

```text
input/read/challenge mutation → registry unchanged
durable write failure → in-memory durable state unchanged
closed SQLite store → mutation fails; status/version unchanged
corrupted payload/checksum → load/startup fails closed
restart after persisted RUNNING-like payload → STOPPED, PID 0, StartedAt nil
durable device without AccountID → rejected
PoP challenge → not durable/restored
persist-before-memory mutant → killed
Go module graph drift → go mod tidy -diff fails
CGO-only persistence regression → pure-Go portability gate fails
```

Permanent authorization negatives remain:

```text
nil/unavailable authority → 503 before upstream
policy deny → 403
network failure/redirect/oversized/malformed authority response → unavailable
200 allow with wrong AccountID/DeviceID or empty SessionID → unavailable
Device without AccountID → deny before authority I/O
child process environment → no WEBGATE_AUTHORITY_TOKEN / WEBGATE_ADMIN_TOKEN
```

---

# 9. Process rules

1. Synchronize remote `main` and CI.
2. Select one atomic task by risk/dependency leverage.
3. State root cause, invariants, change/protected surfaces, failures, rollback and verification.
4. Characterize wrong behavior first where feasible.
5. Implement minimum root-cause fix.
6. Verify cheap→expensive; attack the solution.
7. Record material evidence before changing scope/order.
8. Reconcile this plan before final commit.
9. Recheck remote HEAD; never overwrite concurrent work.
10. Push only verified state; never force.
11. Record checkpoint and immediately select next task.

Green CI cannot overrule a stronger invariant. A workflow that never starts executable steps is not code evidence.

---

# 10. Recent iteration log

Historical Iterations 1–9 remain in Git history.

- **Iteration 10 / T-034:** execution truth restored; `9e31ea07ccd722d8beb14e38d819085b2fa6f4d9` — PASS.
- **Iteration 11 / T-035:** false readiness/config fail-open removed; `a4780a370f0720512552a16b45241e84c4252f73` — PASS.
- **Iteration 12 / T-043:** SSRF/upstream containment; final `82635a87693c5c34921e5d2be48b92fc7e15ec29` — PASS.
- **Iteration 13A / T-049:** dependency/toolchain foundation; final `e6e87c3bb3dd54a5d6fde429d71d2d33dc187809` — repeated main CI PASS.
- **Iteration 13B / T-050:** fail-closed data-plane authority; final `2805999a3b59472cbbb02156aefee99689b3cf60` — repeated main CI PASS.
- **Iteration 13C / T-052 public half:** final/main `e7dc960722cffc68b778bb35cab449af096f8273` — repeated main CI PASS.
- **Iteration 13D / T-052 private half:** private candidate `b1d412387087bcaed9da5abf5148acd558c8708c`; qualification externally BLOCKED pre-step; private main unchanged `827abb1add11a9fcbd0a9944e65efbd20c675739`.
- **Iteration 14 / T-039A:** corrected RED `fb84310a...`; green `1b06e56c...`; pointer mutant `7afc1e4c...` killed; final/main `26d05d53a24aa221370ce66b4deadee0b568a7d4` — repeated main CI PASS.
- **Iteration 15 / T-039B1:** corrected RED `70133ebd...`; rejected first green `0a1e64bd...`; lock probe `86e8b7b1...`; green2 `8e7a61b2...` PASS; persist-order mutant `d404e823...` killed; green3 `6d637207...` PASS including module-lock and no-CGO gates. Clean final/main promotion pending.

---

# 11. Context checkpoint

```text
WEBGATE QUALIFIED MAIN: 26d05d53a24aa221370ce66b4deadee0b568a7d4
SECUREACCES QUALIFIED/UNCHANGED MAIN: 827abb1add11a9fcbd0a9944e65efbd20c675739

CURRENT MILESTONE:
- T-052 public remote-authority bridge complete in WebGate main
- T-052 private durable provider implemented on private work branch but CI-infrastructure blocked
- T-039A complete in WebGate main
- T-039B1 branch-qualified; clean final/main promotion pending

TARGET DURABILITY BOUNDARY:
registry mutation
  → detached candidate
  → SQLite transaction (WAL + FULL)
  → checksum/versioned durable record
  → commit success
  → authoritative in-memory replacement

T-039B1 DURABLE:
- service configuration/lifecycle status
- device identity/lifecycle with explicit AccountID
- release lifecycle/artifacts

INTENTIONALLY EPHEMERAL:
- process PID / ProcessState / StartedAt
- device PoP challenges
- authority/admin control secrets

STILL T-039B2:
- management audit log
- WebGate-owned config metadata
- consistent backup/snapshot procedure
- restore validation/operator runbook

NEXT:
1) promote clean T-039B1 final commit after branch CI and repeat main CI
2) T-039B2 audit/config durability + backup/restore
3) recheck SecureAcces private CI only when external runner state meaningfully changes
4) qualify/promote private sidecar, then T-051

IMPORTANT DECISIONS:
- incomplete provider = unavailable/offline
- no local auth fallback
- authority endpoint = literal loopback only
- authority redirects/proxy escape forbidden
- protected upstream = loopback-only unless explicit typed policy owns another route
- missing authority = 503 before upstream
- private SecureAcces source is not silently published
- memory SecureAcces Store is test/dev only
- child services do not inherit WebGate authority/admin tokens
- registries never expose mutable internal state
- durable mutation = persist before memory
- runtime PID/state is never restored as RUNNING
- durable device requires explicit AccountID
- WebGate SQLite dependency is pinned pure-Go modernc.org/sqlite v1.57.0
- no force push
```

---

# 12. Convergence criterion

Converged only when Critical findings are zero; High findings are zero or explicitly accepted; real browser/proxy/transport/relay/Origin/SecureAcces path works behind CGNAT; public/private supply-chain boundaries are reproducibly qualified; both WebGate-owned and SecureAcces-owned persistence recovery are proven; race/security/static/mutation gates pass; performance/compatibility budgets pass; obsolete prototype paths are removed; docs match behavior; final adversarial re-audit finds no blocker; and final verified state is in `main`.
