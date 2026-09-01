# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Reconciled:** 2026-09-01  
**Last qualified main:** `aad9be2ff541f12b2281e76dfae384175bdcefd8`

This file is the only execution source of truth. Supporting documents under `docs/` are design/evidence references; they do not own task state, release readiness, or acceptance.

A task is `DONE` only when its observable production contract exists, relevant negative tests exist, qualification passes, the exact state reaches `main` without force push, and repeated `main` CI passes.

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

# 3. Qualified foundations

- Rust workspace/architecture/lock/format/check/test/clippy gates.
- Go 1.26.6 gate with `go mod tidy -diff`, gofmt, vet, tests and scoped `CGO_ENABLED=0` portability.
- Split loopback-only Data/Admin listeners and bootstrap Admin-token containment.
- Explicit client/server config failure is fail-closed.
- Ed25519 device proof-of-possession with short-lived single-use challenges.
- Process spawn failure cannot report false `RUNNING`.
- **T-043:** protected service upstreams are loopback-only; environment-proxy/redirect SSRF escape is contained.
- **T-049:** reproducible SecureAcces v0.4.0 dependency/toolchain anchor.
- **T-050:** data-plane depends on context-aware `ServiceAuthorizer`; unavailable authority → `503`, policy deny → `403`.
- **T-052 public half:** loopback-only remote authority bridge, exact AccountID/DeviceID binding, strict response validation, bounded bodies/timeouts, no redirects/proxy escape, control-secret scrubbing for child services.
- **T-039:** WebGate-owned service/device/release/control/audit state is durable and recovery-qualified in `main` at `aad9be2ff541f12b2281e76dfae384175bdcefd8`; repeated main CI run `33473723354` passed all jobs.

## Still not production-qualified

- Real destination-restricted browser-facing proxy/provider.
- Real primary/fallback protected transports and independent Relay A/B.
- Origin reverse-connectivity/no-public-IP path.
- Real Servo/browser runtime forced through protected proxy.
- Platform-backed production device keys.
- Private SecureAcces provider in its own qualified `main`.
- SecureAcces-owned durable state + recovery qualification.
- SecureAcces-backed administrator management authorization.
- Atomic legacy privileged action + audit transaction across all Admin routes; T-051 owns this convergence.
- Real end-to-end/chaos/release qualification.

---

# 4. Critical invariants

- **I-001 Browser ownership:** protected content only through a WebGate-qualified browser path.
- **I-002 Application-scoped routing:** normal mode never changes OS default route.
- **I-003 Fail closed:** transport/authority/browser/config/state loss never creates direct or surrogate fallback.
- **I-004 No silent engine fallback:** protected content never opens in an unproxied system browser.
- **I-005 Real readiness:** `Ready`/`Running` means required side effects and health proof succeeded.
- **I-006 Destination restriction:** browser-facing proxy is loopback-only and policy bounded.
- **I-007 Network access ≠ authorization:** transport credentials never grant application access.
- **I-008 SecureAcces authority:** production authorization comes from real SecureAcces state, never WebGate maps.
- **I-009 Device binding:** SecureAcces session DeviceID and account exactly match the active WebGate device; tenant-local UserID is not global AccountID.
- **I-010 Real PoP:** activation requires cryptographic proof over a short-lived single-use challenge.
- **I-011 Production keys:** platform-backed; synthetic/in-memory stores are test-only.
- **I-012 Origin no-public-IP:** no inbound NAT requirement; Origin maintains outbound persistent links.
- **I-013 Failure-domain diversity:** at least two materially independent relay domains.
- **I-014 Transport diversity:** fallback differs materially in protocol/implementation/failure mode.
- **I-015 Admin isolation:** privileged operations require management authorization + durable audit on isolated plane.
- **I-016 Server-owned routing:** clients cannot choose authoritative upstream/tenant/workspace/permission/process fields.
- **I-017 No generic proxy:** no SSRF/open-proxy pivot.
- **I-018 Durable security state:** restart/crash cannot silently reset authoritative state.
- **I-019 Signed policy/release:** signed/versioned/rollback-aware artifacts where applicable.
- **I-020 No false qualification:** mock/interface/work-branch state cannot mark production capability complete.
- **I-021 Trusted CI:** runtime/provider paths have build/static/test gates; critical concurrency gets race checks before release.
- **I-022 Mutation resistance:** critical allow/deny/state logic has meaningful test-of-tests evidence.
- **I-023 Private-source boundary:** public WebGate must not implicitly publish private SecureAcces implementation.
- **I-024 Control-secret containment:** authority/admin/Telegram credentials are not persisted in WebGate SQLite and are not inherited by launched protected services.
- **I-025 Registry ownership:** registries own mutable state; inputs/results are detached and mutations pass through explicit locked APIs.
- **I-026 Persist-before-memory:** durable mutation commits before authoritative memory changes; failure leaves memory unchanged.
- **I-027 Runtime is ephemeral:** process PID/state/StartedAt never restore as `RUNNING` after restart.
- **I-028 Durable identity:** persisted devices require explicit global AccountID; no UserID→AccountID inference.
- **I-029 Append-only audit:** durable WebGate audit cannot be UPDATEd/DELETEd; identical replay is idempotent and conflicting event-ID reuse is corruption.
- **I-030 Secret-free control snapshot:** durable control metadata excludes runtime secrets and service-registry records.
- **I-031 No resurrection:** an existing state DB, including an empty service registry, is authoritative and cannot be silently reseeded.
- **I-032 Qualified recovery:** backup is a consistent SQLite snapshot; restore validates schema/checksums before create-only atomic install.

---

# 5. Findings registry

Historical F-001..F-028 remain in Git history.

- **F-029 — False convergence:** RESOLVED by T-034.
- **F-030 — Synthetic client transport readiness:** CONTAINED by T-035; real provider → T-036.
- **F-031 — No real Servo/proxied runtime:** OPEN / Critical → T-041.
- **F-032 — Synthetic production keystore:** OPEN / Critical → T-040.
- **F-033 — Real SecureAcces production authority absent:** OPEN / Critical → T-052. Public bridge is qualified; private provider is not yet qualified/promoted.
- **F-034 — Origin reverse connectivity absent:** OPEN / Critical → T-037.
- **F-035 — Security/operations state ephemeral:** PARTIALLY RESOLVED / High. WebGate-owned state is now qualified by T-039; SecureAcces-owned durability remains T-052.
- **F-036 — Admin auth is interim shared token:** OPEN/CONTAINED / High → T-051.
- **F-037 — Explicit config failed open:** RESOLVED by T-035.
- **F-038 — Race/mutation/fuzz CI depth missing:** OPEN / High → T-044.
- **F-039 — Runtime client config can report false success:** OPEN / High → T-048.
- **F-040 — SecureAcces dependency/auth boundaries under-modeled:** PARTIALLY RESOLVED by T-049/T-050; T-052/T-051 remain.
- **F-041 — Private/durable SecureAcces deployment channel absent:** OPEN / Critical → T-052; previous private Actions terminated before executable steps.
- **F-042 — Device UserID/AccountID conflation:** CONTAINED / High. Public authority and WebGate durable device state require explicit AccountID; private legacy cleanup remains T-052.
- **F-043 — Child services inherited WebGate control secrets:** RESOLVED by T-052 public half.
- **F-044 — Registry state escaped through live pointers:** RESOLVED by T-039A.
- **F-045 — Existing empty durable service registry could be silently re-seeded:** RESOLVED by T-039B2; regression test protects new-state-only bootstrap.

---

# 6. Task state

Status vocabulary: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`.

## DONE foundations

T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(in-memory baseline), T-030, T-032, T-034, T-035, T-043, T-049, T-050, T-039A, T-039B1, T-039B2 and **T-039** are DONE under their recorded scopes.

### T-039 — Durable transactional WebGate-owned state
**Status:** DONE · **Priority:** P0/P1

Evidence chain:
- T-039A RED `fb84310a...`; green `1b06e56c...`; pointer mutant `7afc1e4c...` killed; final/main `26d05d53...` repeated main PASS.
- T-039B1 RED `70133ebd...`; lock probe `86e8b7b1...`; green `8e7a61b2...`; persist-order mutant `d404e823...` killed; hardened green `6d637207...`; final/main `30481a7bc7164239b0a1128b5ae4e397b927c514`; repeated run `33471683325` PASS.
- T-039B2 RED `72d510a0...`; green1 `2fc491c2...`; secret-persistence mutant `b1dda3ef...` killed after module-lock/gofmt/vet PASS; hardened greens `d7ada7aa...` and `06eaf4ae...`; exact final tree `9491e3826502b3ec49291aed9b036c6e058ad618` reached `main` at `aad9be2ff541f12b2281e76dfae384175bdcefd8`; repeated main run `33473723354` PASS.

Qualified contract:
- pure-Go pinned SQLite state with WAL/FULL/checksums/versioning;
- persist-before-memory service/device/release mutations;
- process runtime and PoP challenges remain ephemeral;
- device durability requires explicit AccountID;
- singleton non-secret control metadata with checksum validation;
- append-only durable audit with UPDATE/DELETE DB triggers and replay-conflict detection;
- config + audit transaction before memory update;
- runtime-only `WEBGATE_TELEGRAM_BOT_TOKEN` precedence;
- no service resurrection from defaults for an existing state DB;
- validated online backup (`VACUUM INTO`) and offline/create-only validated restore;
- corrupt registry/control/backup state fails closed;
- runbook: `docs/operations/STATE_BACKUP_RESTORE.md`.

Known boundary deliberately outside T-039: legacy service/device/release handlers can commit registry state before wrapper audit sync fails. Success response is suppressed, but action+audit is not one transaction. T-051 owns this management transaction.

## Active P0 work

### T-038 — Authoritative SecureAcces + administrator authorization
**Status:** IN_PROGRESS · **Priority:** P0

Converges when T-049 + T-050 + T-052 + T-051 are complete.

### T-052 — Private durable SecureAcces production provider/deployment
**Status:** IN_PROGRESS / PRIVATE-CI-BLOCKED · **Priority:** P0

Public bridge is qualified in WebGate. Private RED `c0a0f82c...` and candidate `b1d41238...` exist in `Homiakus/SecureAcces`; previous private workflows failed before executable steps. Complete only after executable private CI passes, normal promotion occurs, public/private protocol compatibility is exercised, and SecureAcces backup/restore evidence exists.

### T-051 — SecureAcces administrator management authorization
**Status:** TODO · **Priority:** P0

Replace shared-token-only management with request-scoped SecureAcces principal/actor authorization. Shared token may remain only as explicitly scoped bootstrap/recovery factor. Converge privileged action + durable audit into a fail-closed management transaction where feasible.

### T-036 — Real destination-restricted loopback proxy + primary provider
**Status:** READY · **Priority:** P0

Implement the first real browser-facing loopback proxy/provider. It must own a real listener, expose a proxy endpoint only after bind/readiness proof, restrict destinations by policy/session rather than becoming a generic proxy, and fail closed on provider/listener loss. No OS default-route changes.

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

Add `go test -race`, pinned mutation tooling, fuzz/property gates and failure classification. Scoped manual mutants do not complete T-044.

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

Historical T-004/T-005/T-008/T-010/T-011/T-012/T-013/T-014/T-015/T-016/T-019/T-023/T-026/T-027/T-028/T-029/T-031/T-033 remain reopened, superseded, or requalification-required under the active tasks above.

---

# 7. Dependency order / current priority

```text
T-049 DONE → T-050 DONE → T-052(private) → T-051 → T-038 convergence ───────────────┐
T-039 DONE ──────────────────────────────────────────────────────────────────────────┼→ T-045
T-035 DONE → T-036 → T-037 ─────────────────────────────────────────────────────────┤
             ├→ T-040 ───────────────────────────────────────────────────────────────┤
             ├→ T-041 ───────────────────────────────────────────────────────────────┤
             └→ T-042 ───────────────────────────────────────────────────────────────┘
T-044 must land before T-045 final qualification.
T-048 is independent High/P1 work.
T-045 → T-046 → T-047.
```

Priority now:
1. check private SecureAcces executable CI once;
2. if available: finish T-052 → T-051;
3. if still externally blocked: execute T-036 → T-037;
4. T-040 → T-041;
5. T-044/T-042/T-048;
6. T-045 → T-046 → T-047.

---

# 8. Permanent verification matrix

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

Durability negatives:

```text
caller mutates input/read snapshot → registry unchanged
durable write failure → memory unchanged
closed SQLite store → mutation fails without version/status advance
corrupt registry/control payload/checksum → load/startup fails closed
corrupt backup → restore fails; target remains absent
audit UPDATE/DELETE → trigger rejects
same audit ID + different payload → corruption
runtime PID/state/StartedAt → never resurrected
PoP challenge → never restored
device without AccountID → durable write rejected
runtime control secret → absent from durable JSON and backup bytes
existing empty state DB → defaults are not re-seeded
persist-before-memory mutant → killed
secret-persistence mutant → killed
Go module drift → `go mod tidy -diff` fails
CGO-only regression → portability gate fails
```

Authorization negatives:

```text
nil/unavailable authority → 503 before upstream
policy deny → 403
network failure/redirect/oversized/malformed authority response → unavailable
200 allow with wrong AccountID/DeviceID or empty SessionID → unavailable
device without AccountID → deny before authority I/O
child process environment → no WEBGATE_AUTHORITY_TOKEN / WEBGATE_ADMIN_TOKEN
```

T-044 later adds race + automated mutation/fuzz gates.

---

# 9. Process rules

1. Synchronize remote `main` and CI.
2. Select one atomic task by risk/dependency leverage.
3. State root cause, invariants, protected surfaces, rollback and verification.
4. Characterize wrong behavior first where feasible.
5. Implement the smallest root-cause fix.
6. Verify cheap → expensive and attack with negative/mutant evidence.
7. Reconcile this plan before final promotion.
8. Recheck remote HEAD before promotion.
9. Fast-forward only verified state; never force push.
10. Repeat CI on `main` before marking DONE.
11. Immediately select the next task after qualification.

Green CI never overrules a stronger invariant. A workflow that never starts executable steps is not code evidence.

---

# 10. Recent iteration log

- **Iteration 10 / T-034:** execution truth restored; `9e31ea07...` PASS.
- **Iteration 11 / T-035:** false readiness/config fail-open removed; `a4780a37...` PASS.
- **Iteration 12 / T-043:** upstream/SSRF containment; `82635a87...` PASS.
- **Iteration 13A / T-049:** dependency/toolchain foundation; `e6e87c3b...` repeated main PASS.
- **Iteration 13B / T-050:** fail-closed data-plane authority; `2805999a...` repeated main PASS.
- **Iteration 13C / T-052 public:** `e7dc9607...` repeated main PASS.
- **Iteration 13D / T-052 private:** candidate `b1d41238...`; qualification externally blocked pre-step.
- **Iteration 14 / T-039A:** ownership boundary; pointer mutant killed; `26d05d53...` repeated main PASS.
- **Iteration 15 / T-039B1:** transactional durable registries; persist-order mutant killed; `30481a7b...`; run `33471683325` PASS.
- **Iteration 16 / T-039B2:** audit/control/recovery. RED `72d510a0...`; secret mutant `b1dda3ef...` killed; qualified tree `9491e382...`; final/main `aad9be2ff541f12b2281e76dfae384175bdcefd8`; run `33473723354` PASS. During promotion two accidental empty-placeholder commits were created by contents-write; they were forward-corrected without force/rewrite, and the final qualified tree contains no placeholder file.

---

# 11. Context checkpoint

```text
WEBGATE QUALIFIED MAIN: aad9be2ff541f12b2281e76dfae384175bdcefd8
SECUREACCES LAST KNOWN MAIN: 827abb1add11a9fcbd0a9944e65efbd20c675739

T-039 DONE:
- durable service/device/release state
- durable non-secret control metadata
- append-only WebGate admin audit
- consistent backup
- validated offline restore
- no existing-DB service resurrection
- runtime-only Telegram secret override

INTENTIONALLY EPHEMERAL / OUT-OF-BAND:
- process PID / ProcessState / StartedAt
- device PoP challenges
- WEBGATE_ADMIN_TOKEN
- WEBGATE_AUTHORITY_TOKEN
- Telegram bot token in WebGate SQLite

OPEN MANAGEMENT BOUNDARY:
legacy privileged mutation may commit before audit-sync failure;
response fails closed, but action+audit are not one transaction → T-051.

NEXT:
1) one controlled SecureAcces private-CI recheck
2) if still blocked: T-036 real loopback protected proxy/provider
3) T-037 Origin reverse connectivity

NO FORCE PUSH.
```

---

# 12. Convergence criterion

Converged only when Critical findings are zero; High findings are zero or explicitly accepted; the real browser/proxy/transport/relay/Origin/SecureAcces path works behind CGNAT; private/public supply-chain boundaries are reproducibly qualified; WebGate and SecureAcces recovery are proven; management authorization/audit is fail-closed; race/security/static/mutation gates pass; performance/compatibility budgets pass; obsolete prototype paths are removed; docs match behavior; final adversarial re-audit finds no blocker; and the exact final state is verified in `main`.
