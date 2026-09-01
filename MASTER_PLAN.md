# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Reconciled:** 2026-09-01  
**Last qualified main before Iteration 16:** `30481a7bc7164239b0a1128b5ae4e397b927c514`

This is the only execution source of truth. Supporting documents under `docs/` are design/evidence references; they do not own task state, release readiness, or acceptance.

A task is `DONE` only when its observable production contract is implemented, relevant negative tests exist, required CI/experiments pass, the exact clean state reaches `main` without force push, and repeated `main` CI passes. Work branches, mocks, probes, documentation, or interfaces alone are not qualification.

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
loopback SecureAcces authority bridge
  ↓
private + durable SecureAcces authority
  ↓
registered private service
```

Origin must work behind dynamic IP / CGNAT with no inbound port forwarding. Protected traffic must never silently escape through normal OS Internet, an unproxied system browser, an unowned upstream, a local authorization surrogate, or an unavailable authority.

---

# 2. Truth hierarchy

1. Observed runtime behavior.
2. Reproducible tests/experiments.
3. Security/correctness invariants.
4. Code.
5. This plan.
6. Older design documents.
7. Initial assumptions.

Material unexpected evidence becomes an `F-XXX` finding before task scope/order changes.

---

# 3. Current verified state

## Qualified foundations

- Rust workspace and architecture gates.
- Go 1.26.6 gate with `go mod tidy -diff`, gofmt, vet, tests and scoped `CGO_ENABLED=0` portability.
- Split loopback-only Data/Admin listeners and bootstrap Admin token containment.
- Explicit config failure is fatal.
- Ed25519 device proof-of-possession with single-use challenges.
- Process spawn failure cannot report false `RUNNING`.
- **T-043:** loopback-only protected upstream invariant; environment-proxy and redirect escape contained.
- **T-049:** pinned SecureAcces v0.4.0 dependency anchor + Go 1.26.6.
- **T-050:** fail-closed data-plane `ServiceAuthorizer`; unavailable authority → 503, policy deny → 403.
- **T-052 public half:** loopback-only remote authority bridge, explicit global AccountID/DeviceID matching, strict response validation, no redirects/proxy escape, bounded responses, child-process control-secret scrubbing.
- **T-039A:** registries own mutable state; inputs/results/challenges are detached and runtime/bind writes use explicit registry mutation APIs.
- **T-039B1:** final/main `30481a7bc7164239b0a1128b5ae4e397b927c514` provides transactional SQLite persistence for WebGate service/device/release state; repeated main CI run `33471683325` passed all jobs.

## Not production-qualified

- Real destination-restricted browser-facing proxy.
- Real primary/fallback transport providers and independent Relay A/B.
- Origin reverse-connectivity/no-public-IP path.
- Real Servo/browser runtime forced through the protected proxy.
- Platform-backed production device keys.
- Private SecureAcces authority sidecar in its own qualified `main`.
- SecureAcces persistence backup/restore qualification.
- SecureAcces-backed administrator management authorization.
- Atomic legacy management action + audit commit across all Admin routes; T-039B2 makes audit durable and fail-closed at the response boundary, but an already committed registry mutation can precede an audit-store failure. T-051 owns this management-transaction convergence.
- Real end-to-end/chaos/release qualification.

---

# 4. Critical invariants

- **I-001 Browser ownership:** protected content only through a WebGate-qualified browser path.
- **I-002 Application-scoped routing:** normal mode never changes the OS default route.
- **I-003 Fail closed:** transport/authority/browser/config/state loss never creates direct or surrogate fallback.
- **I-004 No silent engine fallback:** protected content never opens in an unproxied system browser.
- **I-005 Real readiness:** `Ready`/`Running` means required side effects and health proof succeeded.
- **I-006 Destination restriction:** browser-facing proxy is loopback-only and policy bounded.
- **I-007 Network access ≠ authorization:** transport credentials never grant app authorization.
- **I-008 SecureAcces authority:** production session/workspace/permission decisions come from real SecureAcces state, never WebGate maps.
- **I-009 Device binding:** SecureAcces session DeviceID and account exactly match the active WebGate device; legacy tenant-local `UserID` is not global `AccountID`.
- **I-010 Real PoP:** activation requires cryptographic proof over a short-lived single-use challenge.
- **I-011 Production keys:** platform-backed; synthetic/in-memory stores are test-only.
- **I-012 Origin no-public-IP:** no inbound NAT requirement; Origin maintains outbound persistent links.
- **I-013 Failure-domain diversity:** at least two materially independent relay domains.
- **I-014 Transport diversity:** fallback differs materially in protocol/implementation/failure mode.
- **I-015 Admin isolation:** privileged operations require management authorization + audit on isolated plane.
- **I-016 Server-owned routing:** clients cannot choose authoritative upstream/tenant/workspace/permission/process fields.
- **I-017 No generic proxy:** no SSRF/open-proxy pivot.
- **I-018 Durable security state:** restart/crash cannot silently reset authoritative state.
- **I-019 Signed policy/release:** signed/versioned/rollback-aware artifacts where applicable.
- **I-020 No false qualification:** mock/interface/work-branch state cannot mark production capability complete.
- **I-021 Trusted CI:** production runtime/provider paths have build/static/test gates; critical concurrency eventually gets race gates.
- **I-022 Mutation resistance:** critical allow/deny/state logic has meaningful test-of-tests evidence.
- **I-023 Private-source boundary:** public WebGate must not publish private SecureAcces implementation implicitly.
- **I-024 Control-secret containment:** authority/admin/Telegram credentials are not durable WebGate SQLite metadata and are not inherited by launched protected services.
- **I-025 Registry ownership:** registries own mutable state and mutate it only through explicit locked APIs.
- **I-026 Persist-before-memory:** durable mutation commits before authoritative memory changes; persistence failure leaves memory unchanged.
- **I-027 Runtime is ephemeral:** process PID/state/StartedAt never restore as `RUNNING` after restart.
- **I-028 Durable identity:** persisted devices require explicit global `AccountID`; no identity inference from `UserID`.
- **I-029 Append-only audit:** durable WebGate audit records cannot be UPDATEd/DELETEd; replay by identical event ID/payload is idempotent and conflicting reuse is corruption.
- **I-030 Secret-free control snapshot:** durable control configuration excludes runtime control secrets and service-registry records.
- **I-031 No resurrection:** once a state DB exists, even an empty service registry is authoritative and restart cannot reseed deleted services from defaults/config.
- **I-032 Qualified recovery:** backup uses a consistent SQLite snapshot and restore validates schema/checksums before installing a create-only target.

---

# 5. Findings registry

Historical F-001..F-028 remain in Git history.

- **F-029 — False convergence:** RESOLVED by T-034.
- **F-030 — Synthetic client transport readiness:** CONTAINED by T-035; real provider → T-036.
- **F-031 — No real Servo/proxied runtime:** OPEN / Critical → T-041.
- **F-032 — Synthetic production keystore:** OPEN / Critical → T-040.
- **F-033 — Real SecureAcces production authority absent:** OPEN / Critical → T-052. Public bridge is qualified; private provider is not qualified/promoted.
- **F-034 — Origin reverse connectivity absent:** OPEN / Critical → T-037.
- **F-035 — Security/operations state ephemeral:** PARTIALLY RESOLVED / High → T-039 + T-052. WebGate-owned service/device/release state is qualified in B1; control/audit/backup is branch-qualified in B2 pending final main. SecureAcces durability remains T-052.
- **F-036 — Admin auth is interim shared token:** OPEN/CONTAINED / High → T-051.
- **F-037 — Explicit config failed open:** RESOLVED by T-035.
- **F-038 — Race/mutation/fuzz CI depth missing:** OPEN / High → T-044.
- **F-039 — Runtime client config can report false success:** OPEN / High → T-048.
- **F-040 — SecureAcces dependency/auth boundaries under-modeled:** PARTIALLY RESOLVED by T-049/T-050; T-052/T-051 remain.
- **F-041 — Private/durable SecureAcces deployment channel absent:** OPEN / Critical → T-052; private Actions previously terminated before executable steps.
- **F-042 — Device UserID/AccountID conflation:** CONTAINED / High → T-052 + T-039. Public authority and durable device persistence require explicit AccountID; legacy cleanup remains.
- **F-043 — Child services inherited WebGate control secrets:** RESOLVED by T-052 public half.
- **F-044 — Registry state escaped through live pointers:** RESOLVED by T-039A.
- **F-045 — Existing empty durable service registry could be silently re-seeded from defaults/config after restart:** RESOLVED IN T-039B2 BRANCH / High. Bootstrap now occurs only when the state DB did not exist before opening and the new registry is empty; table-driven regression test protects the rule. Final status becomes RESOLVED after B2 reaches/repasses main.

---

# 6. Task state

Status vocabulary: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`.

## Completed foundations

T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(in-memory baseline), T-030, T-032, T-034, T-035, T-043, T-049, T-050, T-039A and T-039B1 are DONE under their recorded scopes.

### T-039A — Registry ownership boundary
**Status:** DONE · **Priority:** P0

RED `fb84310a8473522dd932219bb8e0b312914f2015`; green `1b06e56c3808ca85c7866d379dcbafa6d14f1b66`; internal-pointer mutant `7afc1e4cceb5e4d9ce7a1960d84ed4d88470a7ab` killed; final/main `26d05d53a24aa221370ce66b4deadee0b568a7d4` passed repeated main CI.

### T-039B1 — Durable service/device/release registries
**Status:** DONE · **Priority:** P0

Evidence:
- corrected RED `70133ebda74f1b5061689646a820126e519ecd9b` failed on missing persistence APIs after formatting;
- first green rejected for missing Go checksums/structural-test issue;
- Go 1.26.6 lock probe `86e8b7b105af6dcfba51960ef2dcf0fc1571af07` produced exact dependency graph;
- green `8e7a61b291dd8e1cb7d6db96b236301fe5ed270a` PASS;
- persist-before-memory mutant `d404e823ae1f1f1a91d3dff721f550cc8cc28e7d` killed by independent failure tests;
- hardened green `6d6372071403fcdf53751f745e13e19c1c58c987` PASS with module-lock and no-CGO gates;
- clean final/main `30481a7bc7164239b0a1128b5ae4e397b927c514`; repeated main run `33471683325` SUCCESS.

Contract:
- pure-Go `modernc.org/sqlite v1.57.0`, pinned via `go.sum`;
- WAL + FULL synchronous + busy timeout + restricted permissions;
- versioned/checksummed service/device/release records;
- persist-before-memory durable mutations;
- device persistence requires AccountID;
- PoP challenge stays ephemeral;
- process runtime state stays ephemeral and restores STOPPED/0/nil;
- startup fails closed on corrupt/incompatible durable registry state.

## Active P0 work

### T-039 — Durable transactional WebGate-owned state
**Status:** IN_PROGRESS · **Priority:** P0/P1

T-039 becomes DONE only when B2 clean final reaches `main` and repeated main CI passes.

#### T-039B2 — Durable audit/config + operational recovery
**Status:** BRANCH QUALIFIED / PENDING FINAL MAIN · **Priority:** P0/P1

Evidence:
- RED `72d510a01b9a1199b8d8bde83f809fe9a656775d`: module lock/gofmt passed; vet failed only on absent `DurableSnapshot` / `OpenSQLiteControlStore` / durable-admin APIs;
- green1 `2fc491c26ef631f568211a22efcebf8c0f069a2f`: full CI PASS;
- secret-persistence mutant `b1dda3ef58fb78451cd91036ce33aea05e8f8a3c`: module-lock/gofmt/vet passed, then tests killed it because the Telegram token appeared both in durable JSON and physical backup bytes;
- green2 `d7ada7aaa510520c3a898e2d2b9487349d156067`: added corrupt-checksum/failed-restore, durable-config rollback and no-reseed regression coverage; Go tests/no-CGO PASS;
- green3 `06eaf4ae323d735a1e1bf9963125ec7c0089dcc8`: adds `WEBGATE_TELEGRAM_BOT_TOKEN` runtime-only override; full verify/dependency-policy/Go/no-CGO CI PASS.

Implementation contract:
- separate versioned `SQLiteControlStore` on the same WebGate state DB;
- singleton non-secret control config with checksum validation;
- append-only audit table protected by SQLite UPDATE/DELETE triggers;
- audit replay with identical ID/payload is idempotent; conflicting payload for same ID is corruption;
- control config + its audit event commit in one SQL transaction before memory update;
- ordinary legacy Admin handlers are response-buffered until emitted audit records are durable; audit failure yields 503 instead of a false success response;
- config GET/update/bind responses redact Telegram credentials;
- durable config excludes Telegram token and service registry definitions;
- `WEBGATE_TELEGRAM_BOT_TOKEN` provides final runtime-only credential precedence;
- existing state DB never silently re-seeds an intentionally empty service registry;
- `--backup-state` produces validated `VACUUM INTO` snapshot with fsync;
- `--restore-state` is offline/create-only, stages and validates the backup before atomic install, and leaves no target on validation failure;
- corruption/checksum failure is fail-closed;
- operator runbook: `docs/operations/STATE_BACKUP_RESTORE.md`.

Known boundary intentionally left open: legacy service/device/release handlers commit their registry mutation before the wrapper durably syncs their audit event. The wrapper suppresses success if audit persistence fails, but this is not a single database transaction. T-051 must converge request-scoped management authorization and action+audit transactional semantics.

### T-038 — Authoritative SecureAcces + administrator authorization
**Status:** IN_PROGRESS · **Priority:** P0

Converges when T-049 + T-050 + T-052 + T-051 are complete.

### T-052 — Private durable SecureAcces production provider/deployment
**Status:** IN_PROGRESS / PRIVATE-CI-BLOCKED · **Priority:** P0

Public bridge is qualified in WebGate main. Private RED `c0a0f82cc378142724a42e7178687919ce5ccdb9` and candidate `b1d412387087bcaed9da5abf5148acd558c8708c` exist in `Homiakus/SecureAcces`; previous private workflows failed before executable steps. Private main remains `827abb1add11a9fcbd0a9944e65efbd20c675739`. Complete only after executable private CI passes, normal promotion occurs, protocol compatibility is exercised, and SecureAcces backup/restore evidence exists.

### T-051 — SecureAcces administrator management authorization
**Status:** TODO · **Priority:** P0

Replace shared-token-only management with request-scoped SecureAcces principal/actor authorization. Shared token may remain only as explicitly scoped bootstrap/recovery factor. Converge privileged action + durable audit into one fail-closed management transaction where feasible and remove the remaining legacy-authorizer shape.

### T-036 — Real destination-restricted loopback proxy + primary provider
**Status:** TODO · **Priority:** P0

### T-037 — Origin agent + reverse Relay A/B connectivity
**Status:** TODO · **Priority:** P0

### T-040 — Production platform key stores
**Status:** TODO · **Priority:** P0

### T-041 — Real Servo runtime + enforced protected proxy
**Status:** TODO · **Priority:** P0

### T-042 — Real dual-transport / dual-relay failover
**Status:** TODO · **Priority:** P1

### T-044 — Trustworthy security feedback loop
**Status:** TODO · **Priority:** P1

Add `go test -race`, pinned mutation tooling, fuzz/property gates and failure classification. B1/B2 scoped mutants and no-CGO checks are evidence but do not complete T-044.

### T-048 — Transactional fail-closed runtime client config binding
**Status:** TODO · **Priority:** P1/HIGH

### T-045 — Real end-to-end qualification
**Status:** TODO · **Priority:** P0 before release

### T-046 — Requalify release/distribution
**Status:** TODO · **Priority:** P1 before release

### T-047 — Final re-audit/convergence
**Status:** TODO · **Priority:** P0 final gate

### T-017 — Enforce verified-main repository rule
**Status:** BLOCKED · **Priority:** P2

Repository-setting write capability is unavailable; never force push.

## Historical work requiring later requalification

T-004/T-005 Servo/runtime networking, T-008 failover, T-010 keys, T-011 SecureAcces integration, T-012/T-013 transports/relays, T-014 browser/site qualification, T-015 release authority, T-016 final audit, T-019 broker boundary, T-023 Admin API, T-026 operations/audit, T-027 E2E, T-028 distribution, T-029 runtime config binding, T-031 Telegram lifecycle and T-033 completeness claims remain reopened or superseded by the active tasks above.

---

# 7. Current dependency order

```text
T-049 DONE → T-050 DONE → T-052(private provider) → T-051 → T-038 convergence ───────┐
T-039A DONE → T-039B1 DONE → T-039B2(final main) ───────────────────────────────────┼→ T-045
T-035 DONE → T-036 → T-037 ─────────────────────────────────────────────────────────┤
             ├→ T-040 ───────────────────────────────────────────────────────────────┤
             ├→ T-041 ───────────────────────────────────────────────────────────────┤
             └→ T-042 ───────────────────────────────────────────────────────────────┘
T-044 must land before T-045 final qualification.
T-048 is independent High/P1 work.
T-045 → T-046 → T-047.
```

Priority after B2 promotion:
1. check once whether private SecureAcces executable CI is available; do not retry-until-green;
2. if available, finish T-052 then T-051;
3. if still externally blocked, proceed with T-036/T-037 real protected transport + no-public-IP core;
4. T-040/T-041 key/browser enforcement;
5. T-044/T-042/T-048 resilience/config feedback;
6. T-045/T-046/T-047 qualification/release/convergence.

---

# 8. Verification / permanent negative matrix

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

Permanent durability negatives:

```text
caller mutates input/read snapshot → registry unchanged
durable write failure → authoritative memory unchanged
closed SQLite store → mutation fails; status/version unchanged
corrupt registry/control payload or checksum → load/startup fails closed
corrupt backup → restore fails and target remains absent
audit UPDATE/DELETE → database trigger rejects
same audit ID + different payload → corruption
runtime PID/state/StartedAt → never resurrected
PoP challenge → never restored
device without AccountID → durable write rejected
runtime control secret → absent from durable JSON and backup bytes
existing empty state DB → defaults are not re-seeded
persist-before-memory mutant → killed
secret-persistence mutant → killed
Go module drift → go mod tidy -diff fails
CGO-only regression → pure-Go portability gate fails
```

Permanent authorization negatives:

```text
nil/unavailable authority → 503 before upstream
policy deny → 403
network failure/redirect/oversized/malformed authority response → unavailable
200 allow with wrong AccountID/DeviceID or empty SessionID → unavailable
device without AccountID → denied before authority I/O
child process environment → no WEBGATE_AUTHORITY_TOKEN / WEBGATE_ADMIN_TOKEN
```

T-044 later adds race + automated mutation/fuzz gates. Infrastructure retries count only after explicit classification and only when real steps execute.

---

# 9. Process rules

1. Synchronize remote `main` and CI.
2. Select one atomic task by risk/dependency leverage.
3. State root cause, invariant, protected surfaces, rollback and verification.
4. Characterize wrong behavior first where feasible.
5. Implement the smallest root-cause fix.
6. Verify cheap → expensive and attack the solution with negative/mutant evidence.
7. Record material unexpected findings in this plan.
8. Reconcile this plan before the clean final commit.
9. Recheck remote HEAD before promotion.
10. Fast-forward only verified state; never force push.
11. Repeat CI on `main` before marking DONE.
12. Immediately select the next task after qualification.

Green CI never overrules a stronger invariant. A workflow that never starts executable steps is not code evidence.

---

# 10. Recent iteration log

Historical Iterations 1–9 remain in Git history.

- **Iteration 10 / T-034:** execution truth restored; `9e31ea07...` PASS.
- **Iteration 11 / T-035:** false readiness/config fail-open removed; `a4780a37...` PASS.
- **Iteration 12 / T-043:** upstream/SSRF containment; final `82635a87...` PASS.
- **Iteration 13A / T-049:** dependency/toolchain foundation; final `e6e87c3b...` repeated main PASS.
- **Iteration 13B / T-050:** fail-closed data-plane authority; final `2805999a...` repeated main PASS.
- **Iteration 13C / T-052 public:** final/main `e7dc9607...` repeated main PASS.
- **Iteration 13D / T-052 private:** private candidate `b1d41238...`; qualification externally blocked pre-step; private main unchanged.
- **Iteration 14 / T-039A:** corrected RED `fb84310a...`; green `1b06e56c...`; pointer mutant `7afc1e4c...` killed; final/main `26d05d53...` repeated main PASS.
- **Iteration 15 / T-039B1:** RED `70133ebd...`; lock probe `86e8b7b1...`; green `8e7a61b2...`; persist-order mutant `d404e823...` killed; hardened green `6d637207...`; final/main `30481a7bc7164239b0a1128b5ae4e397b927c514`; repeated main run `33471683325` SUCCESS.
- **Iteration 16 / T-039B2:** RED `72d510a0...`; green1 `2fc491c2...` PASS; secret mutant `b1dda3ef...` killed after vet PASS; green2 `d7ada7aa...` adds corruption/rollback/no-reseed negatives; green3 `06eaf4ae323d735a1e1bf9963125ec7c0089dcc8` full CI PASS. Clean final/main promotion pending.

---

# 11. Context checkpoint

```text
WEBGATE QUALIFIED MAIN: 30481a7bc7164239b0a1128b5ae4e397b927c514
SECUREACCES QUALIFIED/UNCHANGED MAIN: 827abb1add11a9fcbd0a9944e65efbd20c675739

WEBGATE DURABLE NOW QUALIFIED IN MAIN:
- service configuration/lifecycle status
- device identity/lifecycle with explicit AccountID
- release lifecycle/artifacts

T-039B2 BRANCH-QUALIFIED, FINAL MAIN PENDING:
- non-secret control metadata
- append-only WebGate admin audit
- validated online SQLite backup snapshot
- offline/create-only validated restore
- no service resurrection on existing empty DB
- WEBGATE_TELEGRAM_BOT_TOKEN runtime-only override

INTENTIONALLY EPHEMERAL / OUT-OF-BAND:
- process PID / ProcessState / StartedAt
- device PoP challenges
- WEBGATE_ADMIN_TOKEN
- WEBGATE_AUTHORITY_TOKEN
- Telegram bot token in WebGate durable SQLite

KNOWN OPEN MANAGEMENT BOUNDARY:
legacy service/device/release mutation may commit before audit sync fails;
response is fail-closed, but action+audit are not one transaction yet → T-051.

NEXT:
1) clean B2 final from qualified main, branch CI, fast-forward, repeated main CI
2) one controlled recheck of private SecureAcces CI
3) if available: T-052 → T-051
4) if still blocked: T-036 → T-037 real protected transport/no-public-IP core

NO FORCE PUSH.
```

---

# 12. Convergence criterion

Converged only when Critical findings are zero; High findings are zero or explicitly accepted; real browser/proxy/transport/relay/Origin/SecureAcces path works behind CGNAT; private/public supply-chain boundaries are reproducibly qualified; WebGate and SecureAcces recovery are both proven; management authorization/audit is fail-closed; race/security/static/mutation gates pass; performance/compatibility budgets pass; obsolete prototype paths are removed; docs match behavior; final adversarial re-audit finds no blocker; and the final exact state is verified in `main`.
