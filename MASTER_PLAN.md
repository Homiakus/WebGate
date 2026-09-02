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
- **T-036:** real destination-restricted browser-facing loopback SOCKS5 proxy with local domain/port policy enforcement, live handshake verification, loopback-only plaintext sidecar upstream, and live endpoint revocation on transport/sidecar failure.
- **T-037:** Origin agent maintaining persistent authenticated outbound reverse sessions to independent Relay A and Relay B without public inbound ports / CGNAT traversal, multiplexed stream dispatching, loopback-only data gateway forwarding, and automatic reconnect resilience.
- **T-040:** Safe pure-Rust RFC 8032 Ed25519 & FIPS 180-4 SHA-512 cryptographic engine, atomic disk-persisted `PersistentFileDeviceKeyStore` with memory zeroing, corruption fail-closed verification, and Proof-of-Possession signature contract compatible with Go server.
- **T-041:** Real Servo embedding adapter and BrowserCapsule with strict fail-closed loopback proxy enforcement, subresource policy verification, lifecycle preservation, and document rendering qualification (SPA, CSR, SSR).
- **T-042:** High-availability dual-relay failover transport (`DualRelayFailoverTransport`) managing independent Relay A and Relay B upstreams, live health observation, immediate relay failover on primary crash, cooldown-aware standby probe and switchback, and seamless BrowserCapsule integration.
- **T-048:** Transactional fail-closed runtime client configuration binding (`transactional_bind_config`) preventing false success reporting, validating scheme/port/destinations/domains/syntax fail-closed, leaving runtime state unchanged on errors, and returning HTTP 400 with explicit structured error details.
- **T-044:** Automated trustworthy security feedback loop including full `go test -race` concurrency validation, pinned multi-stack mutation testing engine (`scripts/run_mutation_tests.py` with 8/8 security/durability mutants killed fail-closed), and native Go fuzzing matrix for Relay framing and persistence deserialization.

## Still not production-qualified

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
- **F-031 — No real Servo/proxied runtime:** RESOLVED by T-041.
- **F-032 — Synthetic production keystore:** RESOLVED by T-040.
- **F-033 — Real SecureAcces production authority absent:** OPEN / Critical → T-052. Public bridge is qualified; private provider is not yet qualified/promoted.
- **F-034 — Origin reverse connectivity absent:** OPEN / Critical → T-037.
- **F-035 — Security/operations state ephemeral:** PARTIALLY RESOLVED / High. WebGate-owned state is now qualified by T-039; SecureAcces-owned durability remains T-052.
- **F-036 — Admin auth is interim shared token:** OPEN/CONTAINED / High → T-051.
- **F-037 — Explicit config failed open:** RESOLVED by T-035.
- **F-038 — Race/mutation/fuzz CI depth missing:** RESOLVED by T-044.
- **F-039 — Runtime client config can report false success:** RESOLVED by T-048.
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

T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(in-memory baseline), T-030, T-032, T-034, T-035, T-043, T-049, T-050, T-039A, T-039B1, T-039B2, T-039, T-036, T-037, T-040, T-041, T-042, T-048 and **T-044** are DONE under their recorded scopes.

### T-044 — Trustworthy security feedback loop
**Status:** DONE · **Priority:** P1

Evidence chain:
- Multi-package Go race detector validation (`go test -race ./pkg/persistence ./pkg/registry ./pkg/origin ./pkg/relay`) passing with zero data races (`I-021`).
- Automated mutation testing framework (`scripts/run_mutation_tests.py`) covering 8 critical security and durability invariants (persist-before-memory, non-loopback rejection in Origin, unauthenticated relay rejection, client config fail-closed, keystore header validation, browser loopback proxy enforcement, dual-relay live failover, and restricted SOCKS5 loopback restriction).
- 8/8 mutants confirmed KILLED fail-closed by targeted test gates (`I-022`).
- Native Go fuzz testing suite in `server/pkg/relay/fuzz_test.go` and `server/pkg/persistence/fuzz_test.go` verifying binary frame decoding, durable config unmarshaling, and audit event deserialization without panics or memory corruption.
- Full quality gate integration in `scripts/project_manager.py` (`verify`, `mutate`, `fuzz`) and unit tests in `scripts/tests/test_mutation_runner.py` (3/3 PASS).

Qualified contract:
- Concurrency-critical Go modules pass race detection on every verification pass.
- Invariant-breaking mutations across core security/durability boundaries are automatically detected and killed by test gates.
- Binary protocol and persistent state parsers are hardened against malformed input via continuous fuzzing.

### T-048 — Transactional fail-closed runtime client config binding
**Status:** DONE · **Priority:** P1/HIGH

Evidence chain:
- `transactional_bind_config` in `crates/webgate-app/src/main.rs` executing atomic parsing, validation, and in-memory swap under `RwLock`.
- Robust JSON unescaping and extraction for `{"content": "..."}` payloads, rejecting malformed JSON strings fail-closed.
- Comprehensive TOML validation in `crates/webgate-core/src/config.rs`: non-empty `profile_id`, non-zero `primary_relay.port`, non-empty relay addresses, non-empty `destinations`, strict URI scheme restriction (`webgate://` or `https://`), non-empty `allowed_domains`.
- HTTP server handling in `handle_client_stream`: returns `HTTP 400 Bad Request` with structured JSON error `{"status":"error","message":"..."}` upon any syntax, parse, or validation failure.
- Active runtime profile is guaranteed untouched whenever a binding error occurs.
- Success responses return `HTTP 200 OK` with `{"status":"ok","profile_id":"...","version":"..."}` and atomically updated configuration.
- Unit and integration tests in `crates/webgate-app/src/main.rs` (7/7 PASS) and `crates/webgate-core/src/config.rs` (7/7 PASS).

Qualified contract:
- Runtime configuration upload can never falsely report `200 OK` when parsing or validation fails (`I-003`, `I-005`).
- Any configuration corruption or invalid value fails closed and preserves prior active configuration untouched.
- All destination targets are restricted to legitimate secure schemes (`webgate://` or `https://`).

### T-042 — Real dual-transport / dual-relay failover
**Status:** DONE · **Priority:** P1

Evidence chain:
- `DualRelayFailoverTransport` managing independent Primary (Relay A) and Fallback (Relay B) upstreams over a unified, destination-restricted loopback SOCKS5 proxy listener.
- Live probe validation at startup: starts on Primary if available (`Ready`), degrades to Fallback if Primary is down (`Degraded`), or fails closed if both are down (`Offline`, `DualRelayError::AllUpstreamsUnavailable`).
- Automatic live failover: detects primary connection drop / timeout / error during client traffic and immediately fails over to Fallback upstream without interrupting local proxy listener.
- Cooldown-aware standby primary probing and switchback: `probe_and_maybe_switchback()` validates primary recovery after `switchback_cooldown_sec` and returns active routing to Primary.
- Full destination policy enforcement: domain and port whitelisting enforced locally before upstream connect; non-loopback plaintext upstreams rejected fail-closed.
- `BrowserCapsule` end-to-end integration: Servo-powered capsule navigates and performs subresource fetches across live relay failover without browser engine recreation.
- Client main entrypoint (`crates/webgate-app/src/main.rs`) wired to `DualRelayFailoverTransport`.
- Integration test suites in `crates/webgate-transport/tests/dual_failover.rs` (7/7 PASS) and `crates/webgate-browser/tests/proxy_enforcement.rs` (6/6 PASS).

Qualified contract:
- Protected corporate web content routes across independent multi-relay infrastructure with zero single points of network failure (`I-013`, `I-014`).
- Browser capsule never recreates or alters security boundary during transport failover (`I-001`, `I-004`, `I-006`).
- Total transport outage fails closed immediately without silent fallback (`I-003`).

### T-041 — Real Servo runtime + enforced protected proxy
**Status:** DONE · **Priority:** P0

Evidence chain:
- Servo embedding adapter (`ServoEmbeddingAdapter`) and configuration requiring strict loopback proxy attachment.
- Fail-closed startup: missing or non-loopback proxy address immediately halts initialization and transitions engine to `BrowserState::Failed` (`I-003`, `I-004`, `I-006`).
- `BrowserCapsule` integration executing and validating subresource network fetches (`execute_proxied_fetch`) against `NavigationPolicy`.
- Disallowed scheme rejection (e.g. `file://`, `data:`, `javascript:`) for both primary navigation and subresources.
- Lifecycle preservation (Android pause, resume, low-memory cache clear, activity recreation with URL state rehydration).
- Qualification runner (`QualificationRunner`) verifying SPA, CSR, and SSR rendering workflows over the secure loopback proxy.
- Integration test suite in `crates/webgate-browser/tests/proxy_enforcement.rs` (5/5 PASS).

Qualified contract:
- Protected corporate web content executes strictly inside `BrowserCapsule` powered by Servo (`I-001`).
- No silent fallback to unproxied system browsers under any circumstance (`I-004`).
- All network fetches (document and subresources) are bounded by loopback proxy and navigation policy (`I-006`).
- Platform lifecycle transitions fail closed if proxy connectivity is lost upon resume.

### T-040 — Production platform key stores
**Status:** DONE · **Priority:** P0

Evidence chain:
- Safe pure-Rust RFC 8032 Ed25519 & FIPS 180-4 SHA-512 implementation under `#![forbid(unsafe_code)]` with zero external crate dependencies.
- Standard RFC 8032 test vectors 1, 2, and 3 validated in unit test suite (`crates/webgate-core/src/ed25519.rs`).
- `PersistentFileDeviceKeyStore` with atomic file write (`.tmp` → rename) and memory-zeroing key deletion.
- Corruption fail-closed validation: corrupt storage header, invalid keys, or mismatched public key fail closed (`PersistentFileDeviceKeyStore::open`).
- Integration tests in `crates/webgate-platform/tests/keystore_crypto.rs` validating signature generation, tamper rejection, disk recovery, and Go-server PoP challenge contract compatibility.
- Client main entrypoint (`crates/webgate-app/src/main.rs`) wired to `PersistentFileDeviceKeyStore` with configurable storage path.

Qualified contract:
- Production client device identity is platform-backed and persisted across process restarts (I-011).
- Device identity uses standard RFC 8032 Ed25519 matching server-side `crypto/ed25519` verification.
- Synthetic `InMemoryDeviceKeyStore` restricted to ephemeral sandbox/test execution.
- Key generation, signing, and storage fail closed on corrupted disk state.

### T-037 — Origin agent + reverse Relay A/B connectivity
**Status:** DONE · **Priority:** P0

Evidence chain:
- Unit & integration tests in `pkg/origin` and `pkg/relay`.
- Multi-relay end-to-end streaming (`TestOriginAgentDualRelayEndToEnd`).
- Auto-reconnect on relay failure/restart (`TestOriginAgentAutoReconnectOnRelayRestart`).
- Rejection of non-loopback target forwarding (`TestOriginAgentRejectsNonLoopbackTarget`).
- Fail-closed client rejection when no Origin is connected (`TestRelayFailsClosedWhenNoOriginConnected`).
- Rejection of unauthenticated Origin (`TestRelayRejectsUnauthenticatedOrigin`).
- Concurrent multi-client multiplexing without cross-talk (`TestRelayConcurrentStreamsNoCrosstalk`).
- Standalone Relay CLI binary (`cmd/webgate-relay`).

Qualified contract:
- Origin gateway operates strictly behind CGNAT/firewalls with zero inbound port-forwarding requirements (I-012).
- Maintains concurrent persistent outbound reverse sessions to at least two independent relay domains (I-013).
- Strict mutual authentication with cluster token before establishing reverse session.
- Bridges incoming reverse streams strictly to loopback data gateway (`127.0.0.1:DataPort`).
- Independent Relay server holding minimal state and failing closed on disconnected Origin.

### T-036 — Real destination-restricted loopback proxy + primary provider
**Status:** DONE · **Priority:** P0

Evidence chain:
- Characterization / RED: `2be41065...`, `5ac5ea91...`.
- Green & hardened: `d9a6ce6b...`, `ea716374...`, `690ea51...`.
- Negative tests: empty destination policy rejected, plaintext non-loopback upstream rejected, non-SOCKS5 sidecar rejected, destination port / domain policy enforced locally before connect, UDP/BIND rejected, domain preservation for upstream DNS, live status revokes endpoint on sidecar failure.

Qualified contract:
- local loopback-only SOCKS5 proxy listener;
- literal loopback upstream restriction for unencrypted sidecar;
- destination policy enforcement (domains, ports, CONNECT-only);
- live handshake and readiness probe before advertising endpoint;
- endpoint revocation immediately upon sidecar failure or connection drop;
- no OS default route modification.

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
T-035 DONE → T-036 DONE → T-037 DONE → T-040 DONE → T-041 DONE → T-042 DONE ─────────┤
T-048 DONE ──────────────────────────────────────────────────────────────────────────┤
T-044 DONE ──────────────────────────────────────────────────────────────────────────┘
T-045 → T-046 → T-047.
```

Priority now:
1. check private SecureAcces executable CI once;
2. if available: finish T-052 → T-051;
3. T-045 (Real end-to-end qualification) → T-046 → T-047.

---

# 8. Permanent verification matrix

Baseline:

```text
python3 -m unittest discover -s scripts/tests -p 'test_*.py' -v
python3 scripts/project_manager.py verify --dry-run
python3 scripts/run_mutation_tests.py
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
cd server && go test -race ./pkg/persistence ./pkg/registry ./pkg/origin ./pkg/relay
cd server && CGO_ENABLED=0 go test ./pkg/persistence ./pkg/registry ./pkg/origin ./pkg/relay
cd server && CGO_ENABLED=0 go build ./cmd/webgate-server ./cmd/webgate-relay
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

Reverse connectivity negatives:

```text
no origin connected → relay rejects client connection immediately (fail-closed)
unauthenticated origin / wrong cluster token → relay rejects connection
non-loopback data gateway target → origin agent rejects configuration
relay restart / connection drop → origin agent auto-reconnects with backoff
malformed protocol magic → relay rejects connection immediately
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
- **Iteration 17 / T-036:** real destination-restricted loopback SOCKS5 proxy with local domain/port policy enforcement, live handshake verification, loopback-only plaintext sidecar upstream, and live endpoint revocation on transport/sidecar failure. RED `2be41065...`; green/hardened `690ea51...`; integration & quality gates PASS.
- **Iteration 18 / T-037:** Origin reverse agent with persistent authenticated reverse Relay A/B connectivity and standalone Relay transit node. Integration tests in `pkg/origin` and `pkg/relay`; multi-relay end-to-end, stream multiplexing, auto-reconnect on restart, non-loopback rejection, and fail-closed gates PASS.
- **Iteration 19 / T-040:** Safe pure-Rust RFC 8032 Ed25519 and FIPS 180-4 SHA-512 engine with `#![forbid(unsafe_code)]` and `PersistentFileDeviceKeyStore` with atomic file write, memory zeroing, corruption fail-closed verification, and Proof-of-Possession contract compatibility with Go server. Test vectors and integration tests PASS (`3f8fa7e`).
- **Iteration 20 / T-041:** Real Servo embedding adapter and BrowserCapsule with strict fail-closed loopback proxy enforcement, subresource policy verification, lifecycle preservation, and document rendering qualification (SPA, CSR, SSR). (`28fb8d5`).
- **Iteration 21 / T-042:** High-availability dual-relay failover transport (`DualRelayFailoverTransport`) managing independent Primary (Relay A) and Fallback (Relay B) upstreams over unified loopback proxy, live health observation, immediate relay failover on primary crash, cooldown-aware standby probe and switchback, and seamless BrowserCapsule integration. Integration suites in `webgate-transport` and `webgate-browser` PASS (`ee7cd21`).
- **Iteration 22 / T-048:** Transactional fail-closed runtime client configuration binding (`transactional_bind_config`) resolving F-039. Atomic parsing/validation/swap under `RwLock`, strict scheme/port/destinations/domains/syntax validation, fail-closed HTTP 400 with structured JSON error details on invalid payload, and full isolation of active runtime profile on any error (`46a2013`).
- **Iteration 23 / T-044:** Trustworthy security feedback loop resolving F-038. Multi-package Go race detector validation (`go test -race ./pkg/...`), automated mutation testing framework (`scripts/run_mutation_tests.py`) with 8/8 security/durability mutants killed fail-closed, native Go fuzzing matrix in `pkg/relay` and `pkg/persistence`, and full quality gate integration in `scripts/project_manager.py` (`0e17abe`).

---

# 11. Context checkpoint

```text
WEBGATE QUALIFIED MAIN: 0e17abe4f5e0fa0c13bb1eaeb7feeeebdaee59dc
SECUREACCES LAST KNOWN MAIN: 827abb1add11a9fcbd0a9944e65efbd20c675739

T-044 DONE:
- Multi-package Go race detector validation (go test -race ./pkg/persistence ./pkg/registry ./pkg/origin ./pkg/relay) (I-021)
- Automated mutation testing framework (scripts/run_mutation_tests.py) covering 8 critical security/durability invariants (I-022)
- 8/8 mutants confirmed KILLED fail-closed across Go and Rust verification gates
- Native Go fuzz testing suite in server/pkg/relay/fuzz_test.go and server/pkg/persistence/fuzz_test.go
- Full quality gate integration in scripts/project_manager.py (verify, mutate, fuzz) and scripts/tests/test_mutation_runner.py (3/3 PASS)

T-048 DONE:
- Transactional fail-closed runtime client config binding (transactional_bind_config) in crates/webgate-app/src/main.rs
- F-039 fully resolved: no false success responses upon syntax, parse, or validation error
- Active client profile remains strictly untouched upon any invalid binding payload
- Enhanced validation in crates/webgate-core/src/config.rs (profile_id, ports, URI schemes, allowed_domains)
- Unit and integration test suites in webgate-app and webgate-core PASS

T-042 DONE:
- DualRelayFailoverTransport with independent Primary (Relay A) & Fallback (Relay B) upstreams
- Loopback-only unified proxy listener with destination policy enforcement (I-006)
- Live health observation, consecutive failure tracking, and instant failover on primary drop
- Cooldown-aware standby primary probing and automatic switchback
- BrowserCapsule integration with seamless failover across live relay failure
- Integration test suites in crates/webgate-transport/tests/dual_failover.rs (7/7 PASS) and crates/webgate-browser/tests/proxy_enforcement.rs (6/6 PASS)

T-041 DONE:
- ServoEmbeddingAdapter & BrowserCapsule with fail-closed loopback proxy enforcement
- Subresource network fetch validation against NavigationPolicy
- SPA, CSR, and SSR rendering qualification
- Platform lifecycle transitions (pause, resume, low-memory trim, recreation)

T-040 DONE:
- Safe pure-Rust RFC 8032 Ed25519 & FIPS 180-4 SHA-512 under #![forbid(unsafe_code)]
- PersistentFileDeviceKeyStore with atomic file persistence & memory zeroing
- Corruption fail-closed validation on invalid disk state
- Go-server PoP challenge contract compatibility verified in integration suite
- Wired into webgate-app client main entrypoint

T-037 DONE:
- Origin agent maintaining persistent authenticated outbound reverse sessions to independent Relay A and Relay B
- No public inbound ports / CGNAT traversal (I-012)
- Multiplexed stream dispatching to local loopback data gateway (127.0.0.1:DataPort)
- Auto-reconnect with exponential backoff on relay failure
- Standalone Relay CLI node (cmd/webgate-relay)

T-036 DONE:
- real destination-restricted SOCKS5 proxy listener (loopback)
- domain, port, and CONNECT-only policy enforcement
- literal loopback upstream restriction for unencrypted sidecar
- live handshake and readiness probe before advertising endpoint
- live endpoint revocation on transport/sidecar failure

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
2) T-045 Real end-to-end qualification
3) T-046 Requalify release/distribution
4) T-047 Final re-audit/convergence

NO FORCE PUSH.
```

---

# 12. Convergence criterion

Converged only when Critical findings are zero; High findings are zero or explicitly accepted; the real browser/proxy/transport/relay/Origin/SecureAcces path works behind CGNAT; private/public supply-chain boundaries are reproducibly qualified; WebGate and SecureAcces recovery are proven; management authorization/audit is fail-closed; race/security/static/mutation gates pass; performance/compatibility budgets pass; obsolete prototype paths are removed; docs match behavior; final adversarial re-audit finds no blocker; and the exact final state is verified in `main`.

