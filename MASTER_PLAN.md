# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Reconciled:** 2026-08-31  
**Last qualified main before Iteration 13:** `82635a87693c5c34921e5d2be48b92fc7e15ec29`

This file is the **only execution source of truth**. Supporting documents under `docs/` are evidence/design references; they do not own task state, priority, acceptance, or release readiness.

A task is `DONE` only when its observable production contract is implemented, relevant negative tests exist, required verification passes, and the verified state reaches `main` without force push. Models, mocks, state-machine simulations, compile probes, and documentation are not production qualification by themselves.

---

# 1. Mission

Build WebGate as a secure application-scoped access system:

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

Origin must work behind dynamic IP / CGNAT with no inbound port forwarding. Protected traffic must never silently escape through normal OS Internet or an unproxied system browser.

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

- Rust workspace boundaries with `unsafe_code = forbid`.
- Architecture/dependency checks and locked Rust CI.
- Go server format/vet/test CI gate.
- Navigation-policy/browser-capsule state models.
- Transport SPI and deterministic failover controller.
- Failover startup accepts only a `Ready` provider with a real proxy endpoint.
- Configured-only relay providers remain `Offline`; incomplete `SecureRelayTransport` does not claim connectivity.
- Explicit client `--config` failure is fatal instead of silently selecting defaults.
- Go `ProtectedService` registry and gateway baseline.
- Server-side Ed25519 device PoP with single-use challenge.
- Session↔device and user↔device binding checks.
- Split loopback Data/Admin listeners and interim strong Admin bearer token.
- Process spawn failure is not reported as `RUNNING`.
- T-043: server upstream routing is loopback-only by default, validated at construction/mutation/use, DNS/public/private-LAN/link-local/metadata pivots are denied, gateway does not use an environment HTTP proxy, server-side redirects are not followed, same-origin redirects are rewritten through `/svc/{slug}`, and cross-origin redirect escapes fail closed.
- T-049: Go security build line is pinned to 1.26.6 and the private SecureAcces v0.4.0 dependency has a local provenance/integrity anchor whose vendored bytes are checked against upstream Git blob identities.

## Not production-qualified

- Real Servo embedding/runtime and browser network enforcement.
- Real destination-restricted client loopback proxy.
- Real primary and materially independent fallback transports.
- Real Relay A/B connectivity and Origin reverse-connectivity agent.
- No-public-IP/CGNAT end-to-end path.
- Platform-backed production device key storage.
- Authoritative SecureAcces runtime/data-plane adapter; T-049 is dependency foundation only.
- SecureAcces-backed administrator management authorization; interim shared token remains containment only.
- Durable WebGate-owned server state.
- End-to-end production qualification and release requalification.

---

# 4. Critical invariants

- **I-001 Browser ownership:** protected content is rendered only by a WebGate-qualified browser path.
- **I-002 Application-scoped routing:** normal WebGate mode does not change the OS default route.
- **I-003 Fail closed:** transport loss yields explicit protected-navigation failure; never direct fallback.
- **I-004 No silent engine fallback:** browser/runtime failure never opens protected content in an unproxied system browser.
- **I-005 Real readiness:** `Ready`/`Running` means required external side effects and health checks actually succeeded.
- **I-006 Destination restriction:** browser-facing proxy is loopback-only and reaches only policy-authorized protected destinations.
- **I-007 Network access ≠ authorization:** transport credentials alone never grant application access.
- **I-008 SecureAcces authority:** production user/session/workspace/permission decisions come from SecureAcces.
- **I-009 Device binding:** protected session is bound to the exact active device and owning user.
- **I-010 Real PoP:** device activation requires valid cryptographic proof over a short-lived single-use challenge.
- **I-011 Production keys:** production private keys are platform-backed; synthetic/in-memory stores are test-only.
- **I-012 Origin no-public-IP:** Origin requires no inbound NAT mapping and maintains outbound persistent relay connections.
- **I-013 Failure-domain diversity:** production has at least two materially independent relay failure domains.
- **I-014 Transport diversity:** at least one fallback differs materially in implementation/protocol/failure mode.
- **I-015 Admin isolation:** privileged operations require explicit management authorization and audit on an isolated plane.
- **I-016 Server-owned routing:** client input cannot choose authoritative upstream/tenant/workspace/permissions/process fields.
- **I-017 No generic proxy:** gateway/proxy cannot become an arbitrary SSRF/open-proxy pivot.
- **I-018 Durable security state:** restart/crash cannot silently reset authoritative security state.
- **I-019 Signed policy/release:** production policy/config/update/release artifacts are signed/versioned/rollback-aware as applicable.
- **I-020 No false qualification:** mocks/simulations cannot mark a production capability `DONE`.
- **I-021 Trusted CI:** every production language/runtime path has build/static/test gates; critical concurrency gets race checking.
- **I-022 Mutation resistance:** critical allow/deny/state-transition logic has meaningful mutation testing where technically applicable.

---

# 5. Findings registry

Historical F-001..F-028 remain preserved in Git history.

- **F-029 — False convergence in plan:** RESOLVED by T-034.
- **F-030 — Synthetic client transport readiness:** PARTIALLY RESOLVED/CONTAINED by T-035; real provider remains T-036.
- **F-031 — No real Servo/proxied protected browser runtime:** OPEN / Critical → T-041.
- **F-032 — Production entrypoint uses synthetic device keys:** OPEN / Critical → T-040.
- **F-033 — SecureAcces integration is an in-memory surrogate:** OPEN / Critical → T-050 under T-038.
- **F-034 — Origin reverse connectivity absent:** OPEN / Critical → T-037.
- **F-035 — Security/operations state largely ephemeral:** OPEN / High → T-039.
- **F-036 — Admin authentication is interim shared token:** OPEN/CONTAINED / High → T-051 under T-038.
- **F-037 — Explicit client config failed open to defaults:** RESOLVED by T-035.
- **F-038 — CI lacks promised race/mutation/fuzz depth:** OPEN / High → T-044.
- **F-039 — Runtime client config bind can report false success:** OPEN / High → T-048.
- **F-040 — SecureAcces dependency/toolchain/authentication boundaries were under-modeled:** PARTIALLY RESOLVED by T-049; runtime/data authority remains T-050 and administrator authority remains T-051.

T-043 did not create a new finding ID: metadata/public/LAN/DNS/redirect pivots were already inside the task's declared SSRF attack surface. Adversarial review did reject an intermediate solution that still allowed arbitrary RFC1918 destinations.

T-038 was decomposed after inspection of the real private `Homiakus/SecureAcces` v0.4.0 contract. The upstream security-supported production builds are Go 1.26.x/1.27.x while WebGate was on Go 1.23, and a direct private sibling-module fetch would make clean CI dependent on an undeclared cross-repository credential. Mixing dependency reproducibility, data-plane authority and administrator authority in one commit would violate the atomic-task rule. T-049/T-050/T-051 therefore remain the only implementation path for T-038; no parallel roadmap was created.

---

# 6. Task state

Status vocabulary: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`.

## Trusted completed foundations

- T-001 Living execution-plan foundation — DONE.
- T-002 Portable Rust workspace/executable baseline — DONE.
- T-003 Rust dependency/architecture CI baseline — DONE.
- T-006 Android lifecycle state-model probe — DONE as probe only.
- T-007 Strict navigation/deep-link policy model — DONE.
- T-009 Algorithm-agile device identity model — DONE.
- T-021 ProtectedService registry baseline — DONE as in-memory domain baseline.
- T-022 Multi-service gateway baseline — DONE; T-043 now closes the current upstream SSRF boundary.
- T-024 Admin UI prototype — DONE as UI capability only.
- T-025 Server device registry + Ed25519 PoP — DONE for current in-memory registry.
- T-030 Process spawn/lifecycle baseline — DONE.
- T-032 Editorial UI transformation — DONE.
- T-034 Restore execution truth and qualification semantics — DONE.
- T-035 Eliminate false readiness and fail-open client bootstrap — DONE.
- **T-043 Harden upstream routing and SSRF containment — DONE.**
- **T-049 Pin reproducible SecureAcces dependency foundation — DONE.**

### T-043 acceptance evidence

- `ProtectedService.Validate()` canonicalizes the server-owned upstream.
- `ServiceRegistry.UpdateRoute()` cannot bypass the invariant and leaves last valid route unchanged on failure.
- Gateway revalidates the route immediately before I/O, protecting against stale/corrupt state.
- Default policy permits explicit loopback only (`127/8`, `::1`, canonicalized `localhost`).
- Metadata/link-local, public Internet, unspecified, multicast, arbitrary DNS, userinfo, and unowned RFC1918/LAN destinations are rejected.
- No DNS resolution is performed for upstream selection; therefore DNS rebinding cannot widen egress under the current contract.
- Gateway transport disables environment HTTP proxy use for protected upstream I/O.
- Gateway HTTP client never follows redirects server-side.
- Same-origin redirects are rewritten into `/svc/{slug}/...`; cross-origin/LAN/public redirect targets are rejected before browser-visible headers are emitted.
- RED characterization commits: `610561f7ebcd3053077458d04c3d7a017bfcdc0f` and `5c8c44da31101a00cbd52aec86dfd98617b9b769`.
- First green candidate `ce4f2031a6034a81144ee9b9b322037a99a859d4` was **rejected before main** by adversarial review because it still allowed arbitrary RFC1918; green CI is not sufficient when the invariant is wrong.
- Semantic DNS-allow mutant `79e93555d40868713501a87a31e204fe418f086c` was killed by the `example.com` negative test after gofmt/vet passed.

### T-049 acceptance evidence

- RED characterization commit `95be5b0e0721529bff215f1ebf15003d26173aa7` failed exactly because WebGate still used Go 1.23 and had no pinned local SecureAcces dependency anchor; existing Go gates remained green.
- WebGate server build line is `go 1.26` with `toolchain go1.26.6`, matching the upstream v0.4.0 supported production line.
- `server/go.mod` requires `github.com/Homiakus/secureaccess v0.4.0` and replaces it with a repository-local path, so clean WebGate CI does not require a private cross-repository credential.
- `server/third_party/secureaccess/UPSTREAM.md` pins upstream commit `827abb1add11a9fcbd0a9944e65efbd20c675739`, version `v0.4.0`, scope and update rules.
- Vendored anchor files (`go.mod`, LICENSE, package docs, version) are byte-identical to upstream; the Python contract test recomputes Git blob object IDs and rejects drift.
- A Go package test imports the local SecureAcces module and verifies `Version == 0.4.0`; a textual `replace` alone is not treated as qualification.
- Full authorization source/Store/runtime wiring is intentionally **not** claimed by T-049; that behavior belongs to T-050. Admin management authority belongs to T-051.
- Work-branch qualification candidate `6a299ed7d62e024d830d36e74e37997f403c4495` passed project-manager/architecture/Rust/clippy, `cargo-deny`, Go 1.26.6 format/vet/test before plan reconciliation.
- Mutation is not semantically applicable to this metadata/provenance anchor; integrity is tested by byte identity. Security allow/deny mutation resumes in T-050/T-051 and the pinned mutation framework remains T-044.

## Reopened / requalification-required historical tasks

- T-004 Real Servo embedding adapter — REOPENED.
- T-005 Real fail-closed browser networking — REOPENED.
- T-008 Failover controller — state semantics repaired; real-provider qualification remains T-036/T-042.
- T-010 Platform device-key adapters — REOPENED.
- T-011 SecureAcces integration — REOPENED; execution is T-049/T-050/T-051 under T-038.
- T-012 Primary production transport — REOPENED.
- T-013 Independent fallback/dual-relay — REOPENED.
- T-014 Servo/site/security/performance qualification — REOPENED.
- T-015 Production release authority — NEEDS_REQUALIFICATION.
- T-016 Final adversarial re-audit — REOPENED.
- T-019 Trusted broker boundary — NEEDS_REQUALIFICATION against real browser/process boundary.
- T-023 Admin Control API — NEEDS_REQUALIFICATION under target auth.
- T-026 Audit/health operations — NEEDS_REQUALIFICATION with durable/end-to-end state.
- T-027 Full adversarial E2E qualification — REOPENED.
- T-028 Telegram/release distribution — NEEDS_REQUALIFICATION.
- T-029 Config profile binding — CLI fail-open fixed; signed/runtime binding remains open.
- T-031 Telegram Admin Bot lifecycle — NEEDS_REQUALIFICATION.
- T-033 Integrity audit — historical audit preserved; old production-completeness claims superseded.

## Active execution tasks

### T-036 — Real destination-restricted loopback proxy + primary provider
**Status:** TODO · **Priority:** P0 · **Type:** NETWORK / SECURITY  
Real listener, destination allowlist, bounded proxy semantics, provider lifecycle, authenticated control boundary, cancellation/health. `Ready` requires actual side effects.

### T-037 — Origin agent and reverse Relay A/B connectivity
**Status:** TODO · **Priority:** P0 · **Type:** NETWORK / CGNAT / RELIABILITY  
Persistent outbound Origin connections, authentication, multiplexing, reconnect/backoff, relay registration/rotation, local gateway forwarding; prove no inbound port forwarding.

### T-038 — Authoritative SecureAcces + administrator authorization
**Status:** IN_PROGRESS · **Priority:** P0 · **Type:** AUTHORIZATION / ADMIN SECURITY  
Umbrella convergence task only. It is complete only when T-049 + T-050 + T-051 are complete. Do not implement T-038 as a giant combined change.

### T-050 — SecureAcces authoritative data-plane session/resource authorization
**Status:** READY · **Priority:** P0 · **Type:** AUTHORIZATION / DATA PLANE  
Introduce the exact qualified SecureAcces implementation required by WebGate behind a narrow adapter. Every protected request must authenticate the authoritative session, bind its SecureAcces `SessionView.DeviceID` to the presented active WebGate device, resolve tenant/workspace from the server-owned service registry, and call SecureAcces `Authorize`. Unknown/unavailable/revoked/expired/mismatched state fails closed. Remove the production path through WebGate-owned session/membership maps without weakening testability.

### T-051 — SecureAcces administrator management authorization
**Status:** TODO · **Priority:** P0 · **Type:** ADMIN SECURITY  
Replace shared-token-only administrator authority with request-scoped SecureAcces principal/actor management authorization. `WEBGATE_ADMIN_TOKEN` may remain only as an explicitly scoped bootstrap/recovery factor if justified; it cannot by itself authorize process/config/device/release mutations. Preserve isolated Admin listener and audit every privileged decision.

### T-039 — Durable transactional server state
**Status:** TODO · **Priority:** P0/P1 · **Type:** PERSISTENCE / RELIABILITY  
Persist WebGate-owned service/device/release/audit/config metadata transactionally; SQLite preferred unless measurements disagree. SecureAcces-owned identity remains its own authority boundary.

### T-040 — Production platform key stores
**Status:** TODO · **Priority:** P0 · **Type:** IDENTITY / PLATFORM SECURITY  
Windows CNG/DPAPI/TPM, Android Keystore, explicit assurance-tier fallbacks; production rejects `InMemoryDeviceKeyStore`.

### T-041 — Real Servo runtime and enforced protected proxy
**Status:** TODO · **Priority:** P0 · **Type:** BROWSER / SECURITY  
Actual Servo runtime, protected networking before navigation, no protected system-browser fallback, direct-egress negative proof.

### T-042 — Real dual-transport / dual-relay failover
**Status:** TODO · **Priority:** P1 · **Type:** RELIABILITY / NETWORK  
At least four logical route candidates across independent relay failure domains and materially different transport families; health/backoff/circuit-break/switchback qualification.

### T-044 — Trustworthy security feedback loop
**Status:** TODO · **Priority:** P1 · **Type:** CI / TEST-OF-TESTS  
Add `go test -race`, pinned mutation tooling, targeted fuzz/property tests, failure classification, and no-growth formatting debt.

### T-045 — Real end-to-end system qualification
**Status:** TODO · **Priority:** P0 before release · **Type:** E2E / SECURITY / CHAOS  
Real client→proxy→transport→relay→reverse-Origin→gateway→SecureAcces→service path, network transitions, revocation, restart, CGNAT/no-port-forward and soak.

### T-046 — Requalify release/distribution
**Status:** TODO · **Priority:** P1 before release · **Type:** SUPPLY CHAIN / PRODUCT.

### T-047 — Final re-audit and convergence
**Status:** TODO · **Priority:** P0 final gate · **Type:** ADVERSARIAL AUDIT / DEBT DELETION.

### T-048 — Transactional fail-closed runtime client config binding
**Status:** TODO · **Priority:** P1/HIGH · **Type:** CONFIGURATION / API CORRECTNESS  
Typed parse/apply result, bounded body, atomic profile swap; failure leaves last valid profile unchanged.

### T-017 — Enforce verified-main repository rule
**Status:** BLOCKED · **Priority:** P2  
Repository-setting write capability unavailable. Never force push; continue independent work.

---

# 7. Dependency DAG and current priority

```text
T-034 DONE → T-035 DONE
T-043 DONE
T-049 DONE → T-050 → T-051 → T-038 convergence ─┐
T-039 durable state ──────────────────────────────┼→ T-045 real system qualification
T-035 → T-036 → T-037 ───────────────────────────┤
      ├→ T-040 platform keystore ────────────────┤
      ├→ T-041 real Servo ───────────────────────┤
      └→ T-042 diverse failover ─────────────────┘
T-044 trustworthy CI must land before T-045 final qualification.
T-048 runtime config correctness is independent High/P1 work.
T-045 → T-046 → T-047 convergence.
```

Current ordering by risk/dependency leverage:

1. **T-050** — replaces the Critical F-033 production authorization surrogate with real SecureAcces data-plane authority.
2. **T-051** — closes High F-036 by moving privileged Admin decisions to SecureAcces management authorization.
3. **T-039** — durable security/operations state before restart/revocation qualification.
4. **T-036 / T-037** — real protected transport and no-public-IP product core.
5. **T-040 / T-041** — real key/browser boundaries.
6. **T-044 / T-042 / T-048** — stronger feedback, resilience, runtime config correctness.
7. **T-045 / T-046 / T-047** — real qualification, release requalification, convergence.

Priority is recalculated after every successful push/material finding.

---

# 8. Verification and test-of-tests

Cheap-to-expensive ladder:

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

T-044 adds `go test -race`, pinned Rust/Go mutation tooling, and selected fuzz/property gates. A retry never converts flakiness into PASS.

For security routing, model at least:

```text
scheme × host-kind × IP-class × port × userinfo × redirect-kind × registry-state
× authorization-state × concurrent mutation × malformed URL
```

T-043 representative classes: loopback v4/v6/localhost; metadata/link-local; public; unspecified; multicast; RFC1918 LAN; arbitrary DNS; userinfo; unsafe route mutation; same-origin redirect; cross-origin redirect; environment proxy bypass.

For SecureAcces integration, T-050/T-051 must model at least:

```text
session state × account state × tenant-user state × membership state × permission bits
× service tenant/workspace × SecureAcces DeviceID × presented WebGate device
× WebGate device state/owner × authority availability × admin action
```

---

# 9. Process rules

1. Synchronize remote `main`, determine HEAD, read this plan/instructions/CI.
2. Select one atomic task by risk/dependency leverage.
3. Define root cause, invariants, change/protected surfaces, observable contract, failure modes, rollback, verification.
4. Characterize wrong behavior before production modification where feasible.
5. Implement minimum root-cause fix.
6. Verify cheaply first, then attack the solution.
7. Record material unexpected evidence before changing task scope/order.
8. Reconcile this plan before commit.
9. Recheck remote HEAD and integrate concurrent work; never overwrite it.
10. Push only verified state to `main`; never force.
11. Record iteration/context checkpoint and immediately select the next task.

A green test suite cannot overrule a stronger security invariant. Intermediate candidates may be rejected after adversarial review even when CI is green.

---

# 10. Iteration log

Historical Iterations 1–9 remain in Git history.

## Iteration 10 — T-034

Execution truth restored; false production-completeness claims reopened. F-029 resolved, F-030..F-038 recorded. Commit `9e31ea07ccd722d8beb14e38d819085b2fa6f4d9`. Result PASS.

## Iteration 11 — T-035

False transport readiness and explicit-config fail-open removed. RED characterization `c7ecb7759a16d6ec53334ce0f04428c70fa0548a`; semantic offline-proxy mutant killed; F-037 resolved, F-030 contained, F-039/T-048 split. Commit `a4780a370f0720512552a16b45241e84c4252f73`. Result PASS.

## Iteration 12 — T-043

**Task:** Harden upstream routing and SSRF containment.  
**Root cause:** upstream trust was represented as an unchecked string; construction, mutation, use, HTTP redirect handling and environment-proxy behavior did not share one fail-closed routing invariant.  
**Characterization:** `610561f7ebcd3053077458d04c3d7a017bfcdc0f` proved metadata/public/unspecified/multicast/userinfo/DNS and `UpdateRoute` bypass; `5c8c44da31101a00cbd52aec86dfd98617b9b769` separately proved arbitrary RFC1918 LAN pivots. In both runs gofmt/vet passed before expected Go test failures.  
**Implementation:** canonical loopback-only upstream invariant; validation at service construction and route mutation; gateway revalidation before I/O; no environment proxy; no server-side redirect following; same-origin redirects rewritten through WebGate; cross-origin redirects denied.  
**Adversarial review:** candidate `ce4f2031a6034a81144ee9b9b322037a99a859d4` had green CI but was rejected because generic RFC1918 allowance violated the stated arbitrary-LAN invariant.  
**Mutation:** semantic DNS-allow mutant `79e93555d40868713501a87a31e204fe418f086c` survived formatting/static checks but was killed by the `example.com` negative test. Automated pinned mutation remains T-044.  
**Race:** no new shared mutable state; routing policy is pure/deterministic. Registry mutation remains under its existing mutex; full Go race gate remains T-044.  
**Security:** I-016/I-017 strengthened; absent/unknown/unowned destination = deny.  
**Performance:** URL/IP validation is local O(length(url)); no DNS/network lookup added; redirects stop earlier.  
**Compatibility:** arbitrary DNS/private-LAN/public upstreams now intentionally fail closed. Existing repository production example routes are loopback and remain valid. A future LAN route requires an explicit policy-owned allowlist rather than generic RFC1918 trust.  
**Plan reconciliation:** T-043 DONE; next task T-038.  
**Process learning:** enforce a security invariant at construction + mutation + use; green CI is necessary but cannot justify an over-broad trust policy.  
**Commit:** `82635a87693c5c34921e5d2be48b92fc7e15ec29`.  
**Push:** `main`, normal fast-forward only.  
**Result:** PASS; work-branch and repeated main CI succeeded.

## Iteration 13A — T-049

**Task:** Pin reproducible SecureAcces dependency foundation after decomposing T-038.  
**Root cause:** WebGate production Go was 1.23 although SecureAcces v0.4.0 supports production builds on Go 1.26.x/1.27.x, and the private sibling repository could not be fetched reproducibly by clean WebGate CI without undeclared cross-repository credentials.  
**Characterization:** RED commit `95be5b0e0721529bff215f1ebf15003d26173aa7` added only dependency contracts; exactly the Go-line and pinned-local-dependency assertions failed while existing Go gates remained green.  
**Implementation:** server `go 1.26`, `toolchain go1.26.6`, require SecureAcces v0.4.0 with local replacement; immutable upstream dependency anchor with source/version/license provenance; Git blob identity verification; Go import/version smoke test.  
**Adversarial review:** an attempted broader manual core vendoring step produced an `admin.go` blob whose Git SHA did not equal upstream and was rejected before entering any candidate tree. T-049 was narrowed back to dependency foundation; full authorization implementation remains T-050.  
**Mutation:** not applicable to immutable dependency metadata; byte drift is covered by object-identity tests. Allow/deny mutation belongs to T-050/T-051; automated tooling remains T-044.  
**Race:** no runtime shared state introduced.  
**Security:** clean CI no longer needs a private sibling credential merely to resolve the pinned dependency anchor; vendored anchor bytes are tamper-evident against recorded upstream Git blob IDs.  
**Performance:** no runtime hot-path behavior changed.  
**Compatibility:** Go build hosts must support Go 1.26.6; this is intentional because the selected security dependency does not support WebGate's old production runtime line.  
**Plan reconciliation:** F-040 recorded/partially resolved; T-038 decomposed into T-049 DONE → T-050 READY → T-051 TODO.  
**Process learning:** dependency/toolchain qualification is a separate trust boundary from behavioral authorization; do not hide both inside one giant security commit.  
**Commit:** SELF — final atomic commit containing dependency foundation/tests/plan.  
**Push:** `main`, normal fast-forward only.  
**Result:** PASS only when reconciled work-branch CI and repeated remote-main verification succeed.

---

# 11. Context compression checkpoint

```text
CURRENT HEAD: resolve from remote main before next iteration
CURRENT QUALIFIED MILESTONE: T-049 SecureAcces dependency/toolchain foundation complete; SecureAcces runtime authority still NOT qualified

ARCHITECTURE:
- Rust client contracts + Go server gateway
- target browser = Servo owned by WebGate
- target network = restricted client proxy → diverse transports/relays → outbound Origin reverse links
- target auth = SecureAcces authoritative
- server protected-service upstream = loopback-only until an explicit policy-owned non-loopback route model exists
- SecureAcces dependency anchor = v0.4.0 / upstream commit 827abb1..., local replace, Go 1.26.6

CRITICAL INVARIANTS:
- no false Ready/Running
- no direct protected egress/system-browser protected fallback
- session bound to device/user
- production keys platform-backed
- Origin behind CGNAT without inbound ports
- admin/data planes isolated and authorized
- routing/upstreams server-owned, loopback-only by default, non-generic
- production session/workspace/permission authority must be SecureAcces, not WebGate surrogate maps

COMPLETED RECENTLY:
- T-034 execution truth reconciliation
- T-035 fail-closed readiness/bootstrap
- T-043 SSRF/upstream containment
- T-049 reproducible SecureAcces dependency/toolchain foundation

OPEN CRITICAL/HIGH FINDINGS:
- F-030 real provider absent
- F-031 no real Servo/proxied runtime
- F-032 synthetic production keystore
- F-033 SecureAcces surrogate → T-050
- F-034 no Origin reverse connectivity
- F-035 ephemeral state
- F-036 interim admin token → T-051
- F-038 race/mutation CI gap
- F-039 runtime config bind false-success
- F-040 dependency boundary partially resolved; behavior remains T-050/T-051

BLOCKERS:
- T-017 repository-setting write capability unavailable
- external relay/VPS/hardware qualification may require environment later

NEXT TASK: T-050 — SecureAcces authoritative data-plane session/resource authorization
WHY NEXT: closes Critical F-033 before adding more network reachability; prevents real transport from exposing a surrogate authorization boundary

CRITICAL FILES:
- MASTER_PLAN.md
- server/go.mod
- server/third_party/secureaccess/*
- server/pkg/auth/*
- server/pkg/gateway/*
- server/pkg/domain/service.go
- server/pkg/registry/service_registry.go

IMPORTANT DECISIONS:
- incomplete provider = Offline
- explicit config failure = deny/fatal
- protected upstream = loopback-only unless a typed policy explicitly owns another route
- arbitrary DNS/RFC1918/public destinations are not implicitly trusted
- redirects cannot expose direct upstream/cross-origin navigation
- SecureAcces private dependency is resolved reproducibly without CI PAT dependence
- dependency anchor is not authorization qualification
- no force push

REJECTED OPTIONS:
- fake Ready for demo UX
- silent config defaults after explicit failure
- merging red characterization commits
- generic RFC1918 trust merely because an address is private
- treating a green candidate as sufficient when adversarial review shows invariant mismatch
- claiming SecureAcces integration merely because module metadata/version compiles
- accepting manually reconstructed security source when its Git blob SHA differs from upstream
```

---

# 12. Convergence criterion

WebGate converges only when Critical findings are zero; High findings are zero or explicitly accepted; P0s are done; P1s are done or evidence-deferred; real browser/proxy/transport/relay/Origin/SecureAcces path and CGNAT operation are qualified; key invariants are mechanically enforced; CI is reproducible/trustworthy; no unexplained flaky/silent paths remain; race/security/static/mutation gates pass; persistence recovery is verified; compatibility/performance budgets pass; obsolete prototype paths are removed; documentation matches implementation; final adversarial re-audit finds no fundamental blocker; and the final verified state is in `main`.
