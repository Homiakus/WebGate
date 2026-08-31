# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Reconciled:** 2026-08-31  
**Last qualified main before Iteration 14:** `e7dc960722cffc68b778bb35cab449af096f8273`

This file is the **only execution source of truth**. Supporting documents under `docs/` are evidence/design references; they do not own task state, priority, acceptance, or release readiness.

A task is `DONE` only when its observable production contract is implemented, relevant negative tests exist, required verification passes, and the verified state reaches `main` without force push. Models, mocks, interfaces, compile probes, work-branch candidates, and documentation are not production qualification by themselves.

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
- **T-052 public half:** final/main `e7dc960722cffc68b778bb35cab449af096f8273` provides the loopback-only remote authority client, explicit SecureAcces `AccountID` device binding, strict response validation, redirect/proxy refusal, bounded responses, and child-process control-secret scrubbing; pre-promotion and repeated `main` CI passed.

## Not production-qualified

- Real Servo runtime and enforced browser proxy.
- Real destination-restricted client proxy.
- Real primary/fallback transports and Relay A/B.
- Origin reverse-connectivity / no-public-IP path.
- Platform-backed production device keys.
- **Private SecureAcces authority process in its own `main`:** candidate exists only on a private work branch; executable CI is currently blocked before runner steps.
- Durable SecureAcces Accounts/Users/Memberships/Sessions/Audit deployment + backup/restore evidence.
- SecureAcces-backed administrator management authorization.
- Durable WebGate-owned service/device/release/audit/config state. T-039A closes the mutable ownership prerequisite only; on-disk transactional persistence remains open.
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
- **I-025 Registry ownership:** WebGate registries own their mutable state; registration inputs and read results are detached snapshots, and mutations pass through explicit locked APIs before durable persistence is introduced.

---

# 5. Findings registry

Historical F-001..F-028 remain in Git history.

- **F-029 — False convergence:** RESOLVED by T-034.
- **F-030 — Synthetic client transport readiness:** CONTAINED by T-035; real provider T-036.
- **F-031 — No real Servo/proxied runtime:** OPEN / Critical → T-041.
- **F-032 — Synthetic production keystore:** OPEN / Critical → T-040.
- **F-033 — Real SecureAcces production authority absent:** OPEN / Critical → T-052. T-050 removed the surrogate and the public bridge is now qualified in `main`, but private provider is not yet qualified/promoted.
- **F-034 — Origin reverse connectivity absent:** OPEN / Critical → T-037.
- **F-035 — Security/operations state ephemeral:** OPEN / High → T-039 plus SecureAcces persistence in T-052.
- **F-036 — Admin auth is interim shared token:** OPEN/CONTAINED / High → T-051.
- **F-037 — Explicit config failed open:** RESOLVED by T-035.
- **F-038 — Race/mutation/fuzz CI depth missing:** OPEN / High → T-044.
- **F-039 — Runtime client config can report false success:** OPEN / High → T-048.
- **F-040 — SecureAcces dependency/toolchain/auth boundaries under-modeled:** PARTIALLY RESOLVED by T-049/T-050; T-052/T-051 remain.
- **F-041 — Private/durable SecureAcces deployment channel absent:** OPEN / Critical → T-052. `Homiakus/SecureAcces` is private; canonical `github.com/Homiakus/secureaccess` is not published; full source vendoring into public WebGate is forbidden. A private sidecar candidate exists, but private GitHub Actions still fail before the first executable step and cannot qualify it.
- **F-042 — Device identity model conflated tenant UserID with global SecureAcces AccountID:** CONTAINED / High → T-052 + T-039. Public model has explicit `AccountID`; remote authority refuses a device without it and never derives it from legacy `UserID`. Durable enrollment migration/cleanup remains with durable device state.
- **F-043 — WebGate-launched service processes inherited control-plane environment secrets:** RESOLVED by T-052 public half. Child environment explicitly removes `WEBGATE_AUTHORITY_TOKEN` and `WEBGATE_ADMIN_TOKEN`; focused tests protect the invariant.
- **F-044 — Registry state escaped through live mutable pointers:** IN PROGRESS / High → T-039. `ServiceRegistry`, `DeviceRegistry`, and `ReleaseRegistry` stored caller pointers and returned internal pointers/slices; callers could mutate state outside locks, validation/versioning and any future persistence transaction. ProcessManager and Telegram `/bind` also depended on this write-through behavior. T-039A removes those aliases and routes runtime/bind writes through explicit registry APIs; durable backend remains T-039B.

T-038 is a convergence umbrella. Inspection of real upstream progressively decomposed it into T-049 → T-050 → T-052 → T-051. This remains one living plan, not a parallel roadmap.

---

# 6. Task state

Status vocabulary: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`.

## Completed / trusted foundations

T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(current in-memory registry), T-030, T-032, T-034, T-035, T-043, T-049, T-050 are DONE under their recorded scopes.

### T-050 — Establish fail-closed data-plane authority boundary
**Status:** DONE · **Priority:** P0 · **Type:** AUTHORIZATION BOUNDARY

Evidence: correct RED `df3e1defa3689c68538ae199f9a76d8930aff87b`; corrected green `732e9aab8114d0c74edb1916ce5bdb9e90653749`; fail-open mutant `e3ba593ca509d9f3d4077f371751d7f12de8159c` killed; final/main commit `2805999a3b59472cbbb02156aefee99689b3cf60` passed repeated main CI. Missing authority stops before upstream with `503`; policy deny is `403`.

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
**Status:** IN_PROGRESS / PRIVATE-CI-BLOCKED · **Priority:** P0 · **Type:** AUTHORIZATION / PERSISTENCE / SUPPLY CHAIN

Public half is qualified in WebGate `main` at `e7dc960722cffc68b778bb35cab449af096f8273` with repeated main CI PASS:
- `RemoteServiceAuthorizer` accepts only literal-loopback HTTP endpoint + ≥32-byte bridge token; no URL credentials/path/query/fragment.
- HTTP environment proxy disabled; redirects are not followed; timeout and response body are bounded; unknown/malformed responses fail unavailable.
- Request carries only opaque session token plus WebGate server-owned service tenant/workspace/id/permission and explicit DeviceID/AccountID.
- `403` is policy deny; bridge/auth/contract/network/5xx/invalid allow response is authority unavailable.
- 200 `allow` is accepted only when returned AccountID and DeviceID exactly match the presented active device and SessionID is non-empty.
- device without AccountID is denied before bridge I/O; no `UserID → AccountID` fallback exists.
- production bootstrap uses remote provider only for an explicit valid configuration; unconfigured remains `UnavailableServiceAuthorizer`.
- WebGate child processes do not inherit authority/admin control secrets.
- RED `4113b68efca0a91335feb86b05b5bc574c5bb5e6`; rejected first green `37139e07b1bb3d001504042cd8650fa41e037488`; green2 `160520950d3c9a8cd7dd5802e7870c64a8866631` PASS; adversarial extension `cd856403d849109500a6db1928b1bdb8726c2d18` PASS; final/main `e7dc960722cffc68b778bb35cab449af096f8273` PASS before and after promotion.

Private half candidate in `Homiakus/SecureAcces`:
- RED `c0a0f82cc378142724a42e7178687919ce5ccdb9` characterizes durable session/device/account authorization, Pebble restart/revocation durability, store outage and bridge-secret rules.
- green candidate `b1d412387087bcaed9da5abf5148acd558c8708c` adds private loopback `webgate-authority` backed by `secureaccess.Service + axiomstore.OpenPebble`, store health probe, audit-failure fail-closed, bridge authentication, exact session-device/account checks and hardened HTTP server.
- Candidate is **not qualified and not in private main**. SecureAcces root/Axiom workflows fail before any executable job step (`steps=[]`, no log blob). A later controlled diagnostic Axiom Go 1.26 rerun again reproduced the same pre-step infrastructure failure. No retry-until-green is allowed.

T-052 remains incomplete until private executable CI runs real steps and passes, private main is promoted normally, public/private protocol compatibility is exercised, and backup/restore evidence is attached.

### T-051 — SecureAcces administrator management authorization
**Status:** TODO · **Priority:** P0  
After T-052, replace shared-token-only authority with request-scoped SecureAcces principal/actor management authorization. Shared token may remain only as explicitly scoped bootstrap/recovery factor. Remove remaining AdminAPI legacy-authorizer dependency and audit privileged decisions.

### T-039 — Durable transactional WebGate-owned state
**Status:** IN_PROGRESS · **Priority:** P0/P1  
Persist WebGate-owned service/device/release/audit/config metadata. Durable device schema must make `AccountID` authoritative and remove/deprecate legacy `UserID` identity semantics without migration-by-guessing.

**T-039A — Registry ownership boundary before persistence:** branch qualification PASS. Corrected RED `fb84310a8473522dd932219bb8e0b312914f2015` passed gofmt/vet and failed `go test` because caller mutations changed registered service/device state. Green `1b06e56c3808ca85c7866d379dcbafa6d14f1b66` makes service/device/release inputs and reads detached, deep-copies `ExecArgs`, release artifacts and pointer timestamps, isolates returned device challenges, adds `UpdateProcessRuntime`, moves ProcessManager writes through the registry, and moves Telegram `/bind` through `UpdateExecutable`. Full branch CI PASS. Mutant `7afc1e4cceb5e4d9ce7a1960d84ed4d88470a7ab` returned an internal service pointer; gofmt/vet passed and `TestServiceRegistryOwnsStoredState` killed it. Final atomic commit is rebuilt from qualified `main` with this reconciled plan; T-039A is complete only after that commit reaches `main` and repeated main CI passes.

**T-039B — Transactional durable backend:** next. Introduce a schema/versioned transactional store for WebGate-owned service/device/release/audit/config metadata, with atomic migrations, restart/crash recovery, corruption/failure handling, backup/restore and tests proving runtime process PID/state is not resurrected as `RUNNING`. Do not duplicate SecureAcces-owned identity/session state. A simple JSON file is not sufficient evidence for DONE.

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
T-049 DONE → T-050 DONE → T-052(private provider + public bridge) → T-051 → T-038 convergence ─┐
T-039A ownership → T-039B durable backend ──────────────────────────────────────────────────────┼→ T-045
T-035 → T-036 → T-037 ───────────────────────────────────────────────────────────────────────────┤
      ├→ T-040 ───────────────────────────────────────────────────────────────────────────────────┤
      ├→ T-041 ───────────────────────────────────────────────────────────────────────────────────┤
      └→ T-042 ───────────────────────────────────────────────────────────────────────────────────┘
T-044 must land before T-045 final qualification.
T-048 is independent High/P1 work.
T-045 → T-046 → T-047.
```

Priority now:
1. **T-039B** while SecureAcces private CI is externally blocked — transactional WebGate-owned persistence/recovery.
2. **T-052 private half** immediately when executable SecureAcces CI is available — qualify/promote durable sidecar, protocol integration, backup/restore.
3. **T-051** admin authority using the same real provider.
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

T-044 adds race + pinned automated mutation/fuzz gates. Retries never turn flakiness into PASS. An infrastructure retry is allowed only after explicit classification and cannot count as code evidence unless real steps execute.

Permanent authorization negatives:

```text
nil/unavailable authority → 503 before upstream
policy deny → 403
request context → provider
network failure/redirect/oversized/malformed authority response → unavailable
200 allow with wrong AccountID/DeviceID or empty SessionID → unavailable
Device without AccountID → deny before authority I/O
child process environment → no WEBGATE_AUTHORITY_TOKEN / WEBGATE_ADMIN_TOKEN
```

Permanent T-039 ownership negatives:

```text
mutate object after Register/Enroll/AddDraft → registry unchanged
mutate Get/Resolve/List/GetLatestPromoted result → registry unchanged
mutate returned DeviceChallenge payload → stored PoP challenge unchanged
mutate nested ExecArgs/artifact slices → registry unchanged
process start/stop/restart → explicit registry runtime mutation, config Version unchanged
Telegram /bind → authoritative registry mutation, not snapshot mutation
internal-pointer mutant → killed
```

T-052/T-051 matrix:

```text
session × account × tenant-user × membership × permission
× service tenant/workspace × SecureAcces DeviceID × WebGate DeviceID/AccountID/state
× bridge authentication × provider availability × persistence/audit state
× restart/restore/revocation × admin action
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

Green CI cannot overrule a stronger invariant. A provider branch whose CI never started executable steps is not green.

---

# 10. Recent iteration log

Historical Iterations 1–9 remain in Git history.

- **Iteration 10 / T-034:** execution truth restored; commit `9e31ea07ccd722d8beb14e38d819085b2fa6f4d9` — PASS.
- **Iteration 11 / T-035:** false readiness/config fail-open removed; commit `a4780a370f0720512552a16b45241e84c4252f73` — PASS.
- **Iteration 12 / T-043:** SSRF/upstream containment; final `82635a87693c5c34921e5d2be48b92fc7e15ec29` — PASS.
- **Iteration 13A / T-049:** dependency/toolchain foundation; final `e6e87c3bb3dd54a5d6fde429d71d2d33dc187809` — repeated main CI PASS.
- **Iteration 13B / T-050:** fail-closed data-plane authority boundary; final `2805999a3b59472cbbb02156aefee99689b3cf60` — repeated main CI PASS.
- **Iteration 13C / T-052 public half:** F-042/F-043 discovered. RED `4113b68e...`; rejected green `37139e07...`; green2 `16052095...` PASS; adversarial `cd856403...` PASS; final/main `e7dc960722cffc68b778bb35cab449af096f8273` — repeated main CI PASS.
- **Iteration 13D / T-052 private half:** SecureAcces RED `c0a0f82c...`; private sidecar green candidate `b1d41238...`; qualification BLOCKED because GitHub Actions jobs terminate pre-step with no logs, including controlled diagnostic reruns. Private main remains `827abb1add11a9fcbd0a9944e65efbd20c675739`.
- **Iteration 14 / T-039A:** F-044 discovered. Invalid first RED `26a2aab6...` rejected because its fixture did not compile; corrected RED `fb84310a8473522dd932219bb8e0b312914f2015` failed only on observed service/device aliasing. Green `1b06e56c3808ca85c7866d379dcbafa6d14f1b66` full CI PASS. Internal-pointer mutant `7afc1e4cceb5e4d9ce7a1960d84ed4d88470a7ab` killed by ownership test after gofmt/vet PASS. Final atomic SELF pending branch/main qualification.

---

# 11. Context checkpoint

```text
WEBGATE QUALIFIED MAIN: e7dc960722cffc68b778bb35cab449af096f8273
SECUREACCES QUALIFIED/UNCHANGED MAIN: 827abb1add11a9fcbd0a9944e65efbd20c675739

CURRENT MILESTONE:
- T-052 public remote-authority bridge complete in WebGate main
- T-052 private durable provider implemented on work branch but CI-infrastructure blocked
- T-039A ownership-boundary green + mutant evidence complete; final atomic promotion pending

TARGET DURABILITY BOUNDARY:
callers
  → detached registry snapshots
  → explicit locked mutation APIs
  → T-039B transactional durable store
  → restart/crash/recovery/backup evidence

NEW IDENTITY RULE:
Device.AccountID = global SecureAcces AccountID
Device.UserID = legacy tenant-local metadata only
never infer one from the other

OPEN CRITICAL/HIGH:
F-030 real transport provider
F-031 real Servo/proxied runtime
F-032 platform key store
F-033 real SecureAcces authority → T-052
F-034 Origin reverse connectivity
F-035 durability → T-039/T-052
F-036 admin authority → T-051
F-038 race/mutation CI depth
F-039 runtime config false-success
F-041 private/durable SecureAcces deployment → T-052
F-042 durable AccountID migration → T-052/T-039
F-044 durable registry transaction boundary → T-039B

NEXT:
1) promote final atomic T-039A only after branch + main CI
2) T-039B transactional durable backend + migration/recovery/backup tests
3) recheck SecureAcces private CI when external runner state changes; no retry-until-green
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
- process runtime state does not increment durable config Version
- do not resurrect persisted PID/state as RUNNING after restart
- no force push
```

---

# 12. Convergence criterion

Converged only when Critical findings are zero; High findings are zero or explicitly accepted; real browser/proxy/transport/relay/Origin/SecureAcces path works behind CGNAT; public/private supply-chain boundaries are reproducibly qualified; both WebGate-owned and SecureAcces-owned persistence recovery are proven; race/security/static/mutation gates pass; performance/compatibility budgets pass; obsolete prototype paths are removed; docs match behavior; final adversarial re-audit finds no blocker; and the final verified state is in `main`.
