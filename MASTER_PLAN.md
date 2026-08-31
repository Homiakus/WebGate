# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Reconciled:** 2026-08-31  
**Last qualified main before Iteration 13B:** `e6e87c3bb3dd54a5d6fde429d71d2d33dc187809`

This file is the **only execution source of truth**. Supporting documents under `docs/` are evidence/design references; they do not own task state, priority, acceptance, or release readiness.

A task is `DONE` only when its observable contract is implemented, relevant negative tests exist, required verification passes, and the verified state reaches `main` without force push. Models, mocks, interfaces, compile probes, and documentation are not production qualification by themselves.

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
private + durable SecureAcces authority
  ↓
registered private service
```

Origin must work behind dynamic IP / CGNAT with no inbound port forwarding. Protected traffic must never silently escape through normal OS Internet, an unproxied browser, an unowned upstream, or a local authorization surrogate. Missing authoritative dependencies fail closed.

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
- Go server format/vet/test gate on Go 1.26.6.
- Navigation/browser-capsule state models.
- Transport SPI/failover state model; configured-only providers remain `Offline`.
- Explicit client config failure is fatal.
- ProtectedService registry/gateway baseline.
- Ed25519 device PoP with single-use challenge.
- Split loopback Data/Admin listeners and interim Admin bearer containment.
- Process spawn failure cannot report `RUNNING`.
- **T-043:** loopback-only server upstream invariant at construction/mutation/use; environment-proxy and redirect escape contained.
- **T-049:** reproducible SecureAcces v0.4.0 dependency anchor + Go 1.26.6.
- **T-050:** gateway depends on context-aware `ServiceAuthorizer`; missing/unavailable authority is explicit fail-closed `503`, ordinary policy deny is `403`, and production `/svc/*` no longer uses the WebGate-owned map surrogate.

## Not production-qualified

- Real Servo runtime and enforced browser proxy.
- Real destination-restricted client proxy.
- Real primary/fallback transports and Relay A/B.
- Origin reverse-connectivity / no-public-IP path.
- Platform-backed production device keys.
- **Real SecureAcces provider/deployment:** T-050 is only the safe boundary.
- Durable SecureAcces Accounts/Users/Memberships/Sessions/Audit state + recovery evidence.
- SecureAcces-backed administrator management authorization.
- Durable WebGate-owned service/device/release/audit/config state.
- Real end-to-end/chaos/release qualification.

---

# 4. Critical invariants

- **I-001 Browser ownership:** protected content only through a WebGate-qualified browser path.
- **I-002 Application-scoped routing:** normal mode does not change OS default route.
- **I-003 Fail closed:** transport/authority/browser/config loss never produces direct or surrogate fallback.
- **I-004 No silent engine fallback:** protected content never opens in an unproxied system browser.
- **I-005 Real readiness:** `Ready`/`Running` means required side effects and health proof succeeded.
- **I-006 Destination restriction:** browser-facing proxy is loopback-only and policy bounded.
- **I-007 Network access ≠ authorization:** transport credentials never grant app access.
- **I-008 SecureAcces authority:** production session/workspace/permission decisions come from real SecureAcces state, never WebGate maps.
- **I-009 Device binding:** SecureAcces session identity binds to the exact active WebGate device/account.
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
- **I-020 No false qualification:** mock/interface/demo cannot mark production capability complete.
- **I-021 Trusted CI:** every production runtime/provider path has build/static/test gates; critical concurrency gets race checks.
- **I-022 Mutation resistance:** critical allow/deny/state logic has meaningful mutation tests.
- **I-023 Private-source boundary:** public WebGate must not implicitly publish private SecureAcces implementation.

---

# 5. Findings registry

Historical F-001..F-028 remain in Git history.

- **F-029 — False convergence:** RESOLVED by T-034.
- **F-030 — Synthetic client transport readiness:** CONTAINED by T-035; real provider T-036.
- **F-031 — No real Servo/proxied runtime:** OPEN / Critical → T-041.
- **F-032 — Synthetic production keystore:** OPEN / Critical → T-040.
- **F-033 — Real SecureAcces production authority absent:** OPEN / Critical → T-052. T-050 removed the surrogate from production data-plane wiring but does not create real authority.
- **F-034 — Origin reverse connectivity absent:** OPEN / Critical → T-037.
- **F-035 — Security/operations state ephemeral:** OPEN / High → T-039 plus SecureAcces persistence in T-052.
- **F-036 — Admin auth is interim shared token:** OPEN/CONTAINED / High → T-051.
- **F-037 — Explicit config failed open:** RESOLVED by T-035.
- **F-038 — Race/mutation/fuzz CI depth missing:** OPEN / High → T-044.
- **F-039 — Runtime client config can report false success:** OPEN / High → T-048.
- **F-040 — SecureAcces dependency/toolchain/auth boundaries under-modeled:** PARTIALLY RESOLVED by T-049/T-050; T-052/T-051 remain.
- **F-041 — Private/durable SecureAcces deployment channel absent:** OPEN / Critical → T-052. `Homiakus/SecureAcces` is private; public canonical module repo/tag `github.com/Homiakus/secureaccess` is not published; clean public WebGate CI has no declared cross-repo credential; SecureAcces production runbook requires durable Axiom/Pebble and backup/restore evidence. Full source vendoring into public WebGate would declassify private source and is not an implicit option.

T-038 is a convergence umbrella. Inspection of real upstream progressively decomposed it into T-049 → T-050 → T-052 → T-051. This remains one living plan, not a parallel roadmap.

---

# 6. Task state

Status vocabulary: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`.

## Completed / trusted foundations

T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(current in-memory registry), T-030, T-032, T-034, T-035, T-043, T-049 are DONE under their recorded scopes.

### T-050 — Establish fail-closed data-plane authority boundary
**Status:** DONE · **Priority:** P0 · **Type:** AUTHORIZATION BOUNDARY

Acceptance/evidence:
- Rejected overscoped RED `202f67a313625bf80608796c2907196d63ba7894` reached into Admin/T-051.
- Correct RED `df3e1defa3689c68538ae199f9a76d8930aff87b` failed exactly because gateway used concrete surrogate, production main constructed it, and no fail-closed SPI existed; existing Go and dependency gates remained green.
- Gateway uses `auth.ServiceAuthorizer` and passes request `context.Context`, opaque session token, active WebGate device, server-resolved service and required permission.
- `nil` provider normalizes to `UnavailableServiceAuthorizer`.
- Authority unavailable → `503` + `Cache-Control: no-store` before upstream I/O; policy deny → `403`.
- Production `main` deliberately wires unavailable authority until T-052; protected availability is reduced rather than authorization strength.
- Legacy map authorizer remains only as compatibility/test and unrequalified Admin-prototype code; it is not `/svc/*` production wiring.
- First green `18a6893154ebfe0054c8d78ceff5f420897f33d5` rejected for gofmt-only failure.
- Corrected green `732e9aab8114d0c74edb1916ce5bdb9e90653749` passed full baseline before plan reconciliation.
- Fail-open mutant `e3ba593ca509d9f3d4077f371751d7f12de8159c` returned nil from unavailable authority; gofmt/vet passed, tests killed it and observed forbidden upstream reach (`502` instead of authorization `503`).
- Final atomic commit: SELF from last qualified `main`; DONE only after reconciled branch + repeated main CI.

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
Umbrella only. Complete when T-049 + T-050 + T-052 + T-051 are complete.

### T-052 — Private durable SecureAcces production provider/deployment
**Status:** READY · **Priority:** P0 · **Type:** AUTHORIZATION / PERSISTENCE / SUPPLY CHAIN

Provide a real `ServiceAuthorizer` backed by SecureAcces v0.4.0 without publishing private source into public WebGate. It must authenticate every opaque session against authoritative state, authorize server-owned tenant/workspace/resource identity, expose the SecureAcces session/account identity needed for WebGate device binding, and fail closed on authority/persistence/audit failure. Production state must use upstream-qualified durable Axiom/Pebble (or another upstream-qualified durable Store); memory Store is test/dev only. Define explicit private packaging/deployment and trusted CI. Prove persistence, restart, revocation, backup/restore, device binding and authority outage. Do not create/publish `Homiakus/secureaccess` or change source visibility implicitly.

### T-051 — SecureAcces administrator management authorization
**Status:** TODO · **Priority:** P0  
After T-052, replace shared-token-only authority with request-scoped SecureAcces principal/actor management authorization. Shared token may remain only as explicitly scoped bootstrap/recovery factor. Remove remaining AdminAPI legacy-authorizer dependency and audit privileged decisions.

### T-039 — Durable transactional WebGate-owned state
**Status:** TODO · **Priority:** P0/P1  
Persist WebGate-owned service/device/release/audit/config metadata. Do not duplicate SecureAcces-owned identity state.

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
**Status:** TODO · **Priority:** P1. Add `go test -race`, pinned mutation tooling, fuzz/property gates, failure classification.

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
T-049 DONE → T-050 DONE → T-052 → T-051 → T-038 convergence ─┐
T-039 WebGate-owned durability ─────────────────────────────────┼→ T-045
T-035 → T-036 → T-037 ─────────────────────────────────────────┤
      ├→ T-040 ─────────────────────────────────────────────────┤
      ├→ T-041 ─────────────────────────────────────────────────┤
      └→ T-042 ─────────────────────────────────────────────────┘
T-044 must land before T-045 final qualification.
T-048 is independent High/P1 work.
T-045 → T-046 → T-047.
```

Priority now:
1. **T-052** real durable/private SecureAcces provider — closes Critical F-033/F-041 and restores protected availability safely.
2. **T-051** admin authority using same provider.
3. **T-039** WebGate-owned durability.
4. **T-036/T-037** transport + no-public-IP core.
5. **T-040/T-041** key/browser boundaries.
6. **T-044/T-042/T-048** feedback/resilience/config.
7. **T-045/T-046/T-047** qualification/release/convergence.

---

# 8. Verification / test-of-tests

```text
formatter → targeted tests → property/fuzz/race → package tests → static analysis
→ integration/contracts → security negatives → full suite → mutation → benchmark
→ real system/chaos/soak
```

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
cd server && go vet ./...
cd server && go test ./...
```

T-044 adds race + pinned automated mutation/fuzz gates. Retries never turn flakiness into PASS.

T-050 negatives that must remain:

```text
nil authority → 503 before upstream
unavailable authority → 503 before upstream
policy deny → 403
request context → propagated to provider
unavailable-return-nil mutant → killed
```

T-052/T-051 matrix must cover:

```text
session × account × tenant-user × membership × permission
× service tenant/workspace × SecureAcces DeviceID × WebGate device/state/owner
× provider availability × persistence/audit state × restart/restore × admin action
```

---

# 9. Process rules

1. Synchronize remote `main` and CI.
2. Select one atomic task by risk/dependency leverage.
3. State root cause, invariants, change/protected surfaces, failures, rollback, verification.
4. Characterize wrong behavior first where feasible.
5. Implement minimum root-cause fix.
6. Verify cheap→expensive; attack the solution.
7. Record material evidence before changing scope/order.
8. Reconcile this plan before final commit.
9. Recheck remote HEAD; never overwrite concurrent work.
10. Push only verified state; never force.
11. Record checkpoint and immediately select next task.

Green CI cannot overrule a stronger invariant.

---

# 10. Recent iteration log

Historical Iterations 1–9 remain in Git history.

- **Iteration 10 / T-034:** execution truth restored; commit `9e31ea07ccd722d8beb14e38d819085b2fa6f4d9` — PASS.
- **Iteration 11 / T-035:** false readiness/config fail-open removed; commit `a4780a370f0720512552a16b45241e84c4252f73` — PASS.
- **Iteration 12 / T-043:** SSRF/upstream containment; RED `610561f7...` + `5c8c44da...`; over-broad green rejected; mutant killed; commit `82635a87693c5c34921e5d2be48b92fc7e15ec29` — PASS.
- **Iteration 13A / T-049:** SecureAcces dependency/toolchain foundation; RED `95be5b0e...`; exact provenance + Go 1.26.6; commit `e6e87c3bb3dd54a5d6fde429d71d2d33dc187809`; repeated main CI PASS.
- **Iteration 13B / T-050:** data-plane authority SPI/fail-closed boundary. F-041 recorded. Overscoped RED `202f67a3...` rejected; correct RED `df3e1def...`; first green `18a68931...` rejected for formatting; corrected green `732e9aab...` baseline PASS; fail-open mutant `e3ba593c...` killed. Final atomic commit SELF pending reconciled CI/main fast-forward.

---

# 11. Context checkpoint

```text
CURRENT HEAD: resolve from remote main before next iteration
QUALIFIED MILESTONE: T-050 boundary complete; /svc/* intentionally fails closed until T-052

TARGET AUTH:
public WebGate gateway → ServiceAuthorizer → private/durable SecureAcces provider

OPEN CRITICAL/HIGH:
F-030 real transport provider
F-031 real Servo/proxied runtime
F-032 platform key store
F-033 real SecureAcces authority → T-052
F-034 Origin reverse connectivity
F-035 durability
F-036 admin authority → T-051
F-038 race/mutation CI depth
F-039 runtime config false-success
F-041 private/durable SecureAcces deployment → T-052

NEXT: T-052
WHY: gateway now refuses surrogate authority; T-052 is shortest safe path to restore protected service availability

IMPORTANT DECISIONS:
- incomplete provider = unavailable/offline
- no local auth fallback
- protected upstream = loopback-only unless explicit typed policy owns another route
- missing authority = 503 before upstream
- private SecureAcces source is not silently published
- memory SecureAcces Store is test/dev only
- no force push
```

---

# 12. Convergence criterion

Converged only when Critical findings are zero; High findings are zero or explicitly accepted; real browser/proxy/transport/relay/Origin/SecureAcces path works behind CGNAT; public/private supply-chain boundaries are reproducibly qualified; both WebGate-owned and SecureAcces-owned persistence recovery are proven; race/security/static/mutation gates pass; performance/compatibility budgets pass; obsolete prototype paths are removed; docs match behavior; final adversarial re-audit finds no blocker; and the final verified state is in `main`.
