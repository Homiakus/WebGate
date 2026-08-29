# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Research baseline:** 2026-08-29  
**Canonical browser decision:** Servo is the primary protected browser engine.

This file is the single source of truth for implementation order, findings, invariants, verification and iteration history. Code must not materially outrun this plan.

# 1. Mission

Build WebGate as a secure, resilient, cross-platform protected-browser client for a small set of trusted users. WebGate opens trusted links in an embedded Servo browser and routes only protected-browser traffic through an application-local fail-closed transport. The private origin may have dynamic addressing or CGNAT. Authorization remains server-side in `SecureAcces`.

Primary targets:

- Windows;
- Android as an early first-class architecture gate;
- Linux;
- macOS.

Product UX:

```text
Telegram / trusted HTTPS link
        ↓
      WebGate
        ↓
       Servo
        ↓
application-local protected proxy
        ↓
resilient transport / relay set
        ↓
private origin
        ↓
SecureAcces authorization
```

# 2. Current State

The repository is currently documentation-first. `main` contains architecture, research and integration documents, but no executable Rust workspace, no application code, no CI workflow and no runnable tests.

Existing decisions already documented:

- Servo is the primary browser engine;
- WebView2 is compatibility-only and never a silent fallback;
- protected traffic must remain application-scoped;
- transport failure must fail closed;
- multiple independent transport/relay failure domains are required for production;
- SecureAcces remains the authoritative authorization layer;
- Android must be considered early enough to prevent desktop-only architecture from hardening into the core.

# 3. Architecture Map

```text
UNTRUSTED INPUT
Telegram / HTTPS / deep link
        │
        ▼
┌───────────────────────────────────────────────┐
│ WebGate application                          │
│                                               │
│ webgate-core                                  │
│ ├─ policy / state machines                    │
│ ├─ deep-link parsing                          │
│ ├─ transport contracts                        │
│ ├─ device/session abstractions                │
│ └─ browser contracts                          │
│                                               │
│ browser adapter                               │
│ └─ Servo primary                              │
│                                               │
│ platform adapters                             │
│ ├─ Windows                                    │
│ ├─ Android                                    │
│ ├─ Linux                                      │
│ └─ macOS                                      │
└───────────────────────┬───────────────────────┘
                        │
                 fail-closed proxy
                        │
              TransportProvider SPI
                │                 │
             primary           fallback
                │                 │
             Relay A           Relay B
                 \               /
                  \             /
                    private origin
                         │
                    SecureAcces
```

Hard boundary rule: browser, transport, authorization and platform code must remain separate capabilities. Servo must not know concrete VPN technology. Transport must not decide resource authorization. Web content must not receive arbitrary native capability access.

# 4. Baseline

Baseline captured on 2026-08-29 from `main`:

| Check | Result | Classification |
|---|---|---|
| repository topology | documentation only | pre-existing |
| Rust build | N/A — no `Cargo.toml` | pre-existing |
| unit tests | N/A | pre-existing |
| integration tests | N/A | pre-existing |
| e2e | N/A | pre-existing |
| formatting/lint | N/A | pre-existing |
| static analysis | N/A | pre-existing |
| race/concurrency checks | N/A | pre-existing |
| dependency checks | N/A | pre-existing |
| security scan | N/A | pre-existing |
| coverage | N/A | pre-existing |
| mutation score | N/A | pre-existing |
| benchmarks | N/A | pre-existing |
| CI/CD | absent | pre-existing |

The first code iteration must create a baseline that can actually fail.

# 5. System Invariants

I-001. Servo is the default protected browser engine.

I-002. Protected traffic is application-scoped; WebGate does not change the OS default route in normal mode.

I-003. Transport loss fails closed. Protected traffic never silently falls back to direct Internet.

I-004. Browser-engine failure never triggers a silent system-browser/WebView2 fallback.

I-005. Links identify resources; they are not long-lived bearer credentials.

I-006. Device private keys are generated on-device and never distributed in reusable configuration files.

I-007. Bootstrap bundles, remote policy and updates are signed and rollback-aware.

I-008. Remote policy may tighten but may not weaken compiled hard security invariants.

I-009. Transport providers are replaceable behind a stable application-level interface.

I-010. SecureAcces is authoritative for account/session/workspace/resource authorization.

I-011. Android is an architecture target, not a late rewrite. Shared core code must not depend directly on DPAPI, Win32 process supervision or desktop-only sidecars.

I-012. Platform secret stores are capability adapters. Device identity must support hardware-backed implementations where available.

I-013. Production requires at least two materially independent network failure domains.

# 6. Findings Registry

## F-001 — Repository has no executable baseline

**Status:** Planned  
**Category:** Delivery / Architecture  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** `main` contains documentation only; no Cargo workspace, source tree or CI workflow.  
**Files / symbols:** repository root.  
**Current behavior:** build/test/security checks cannot run.  
**Expected behavior:** every implementation iteration has executable verification.  
**Root cause:** project intentionally began documentation-first and has not crossed the implementation boundary.  
**Impact:** no code-level invariants can yet be enforced.  
**Blast radius:** all implementation phases.  
**Affected invariants:** all.  
**Affected tasks:** T-002, T-003.  
**Recommended direction:** create the smallest portable Rust workspace and CI before Servo/transport complexity.

## F-002 — Previous plan was roadmap-shaped, not execution-shaped

**Status:** Resolved  
**Category:** Process / Architecture  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** previous `MASTER_PLAN.md` had phases and milestones but no formal Findings Registry, atomic task records, dependency DAG, baseline table or iteration log.  
**Root cause:** research plan preceded implementation protocol.  
**Impact:** unexpected findings and task completion could not be traced rigorously.  
**Affected tasks:** T-001.  
**Recommended direction:** this plan supersedes the old format while preserving its architectural decisions.

## F-003 — Desktop-only assumptions remain in device/runtime design

**Status:** Planned  
**Category:** Cross-platform / Architecture  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** previous plan named DPAPI directly in core milestones and assumed supervised desktop sidecars for transport.  
**Root cause:** Windows-first research occurred before Android became a first-class target.  
**Impact:** late Android port could force core rewrites.  
**Blast radius:** platform, identity, transport lifecycle, updates.  
**Affected invariants:** I-011, I-012.  
**Affected tasks:** T-002, T-006, T-010.  
**Recommended direction:** capability traits in shared core; platform-specific implementations outside core.

## F-004 — A single Ed25519 device-key assumption is not portable enough for hardware-backed identity

**Status:** Planned  
**Category:** Security / Cross-platform  
**Severity:** High  
**Confidence:** Strong

**Evidence:** previous plan fixed Ed25519 for the device key; Android Keystore, Apple Secure Enclave and Windows hardware-backed key paths are more consistently aligned around platform-supported ECDSA P-256/ES256 profiles.  
**Root cause:** policy-signing and device-identity algorithms were conflated.  
**Impact:** non-exportable device identity may be unavailable or require software key fallback on some platforms.  
**Affected invariants:** I-006, I-012.  
**Affected tasks:** T-009.  
**Recommended direction:** algorithm-agile `DeviceSigner`; prefer platform hardware-backed P-256 when available; keep Ed25519 suitable for project-controlled policy/update signatures.

## F-005 — Servo compatibility and security are version-sensitive

**Status:** Planned  
**Category:** Browser / Security  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** Servo is pre-1.0, releases monthly, has an LTS track and active compatibility/security changes. Servo 0.4.0 includes important JS-runtime security updates while upstream still documents feature gaps and active embedding work.  
**Root cause:** deliberate selection of a rapidly evolving embedded engine.  
**Impact:** unreviewed upgrades or stale pins can create security/compatibility regressions.  
**Affected invariants:** I-001, I-003, I-004.  
**Affected tasks:** T-004, T-005, T-014.  
**Recommended direction:** explicit version pin, compatibility suite and upgrade gate.

## F-006 — No automated architecture enforcement exists

**Status:** Planned  
**Category:** Architecture / Testing  
**Severity:** Medium  
**Confidence:** Confirmed

**Evidence:** no source workspace exists.  
**Impact:** platform-specific types, direct-network APIs or authorization logic can leak into wrong layers.  
**Affected invariants:** I-002, I-009, I-010, I-011.  
**Affected tasks:** T-002, T-003.

# 7. Risk Register

| Risk | Probability | Impact | Mitigation |
|---|---:|---:|---|
| Servo cannot render a REQUIRED site feature | Medium | High | capability inventory + compatibility tests + explicit fallback policy |
| direct-network escape on proxy failure | Medium | Critical | negative tests first; browser network config immutable during session |
| Android lifecycle kills/restarts transport | High | High | lifecycle state machine; reconnect/idempotency tests |
| provider/UDP path blocked | Medium | High | independent fallback transport + two relays |
| stale browser dependency contains known security issue | Medium | High | pinned/LTS policy + security upgrade gate |
| hardware-backed identity differs by OS | High | Medium/High | algorithm-agile DeviceSigner capability |
| scope expands into a general VPN/browser | Medium | High | destination allowlist + explicit non-goals |
| SecureAcces contract duplicated client-side | Low | High | server remains authoritative; client only carries session/device proofs |

# 8. Pareto Improvements

Highest leverage work now:

1. create a compiling cross-platform core with explicit boundaries;
2. encode fail-closed/navigation rules as pure, mutation-testable policy before networking;
3. prove Servo can only reach protected origins through the local proxy;
4. run an Android architecture probe early;
5. add CI/security gates before large dependency growth;
6. add transport diversity only after browser isolation is proven.

# 9. Dependency DAG

```text
T-001 plan/baseline
   ↓
T-002 portable Rust workspace ─────┐
   ↓                              │
T-003 CI + quality gates           │
   ↓                              │
T-004 Servo pin + embedding spike │
   ↓                              │
T-005 fail-closed browser proxy   │
   ├───────────────┐              │
   ↓               ↓              │
T-006 Android probe T-007 policy/deeplink core
   │               │
   └───────┬───────┘
           ↓
T-008 transport SPI/state machine
           ↓
T-009 device identity abstraction
           ↓
T-010 platform secret/device adapters
           ↓
T-011 SecureAcces control API integration
           ↓
T-012 primary transport provider
           ↓
T-013 independent fallback + relay failover
           ↓
T-014 compatibility/performance/security qualification
           ↓
T-015 packaging/update/deep-link UX
           ↓
T-016 final adversarial re-audit
```

# 10. Implementation Phases

**Phase A — Executable foundation:** T-001..T-003.  
**Phase B — Protected Servo capsule:** T-004..T-007.  
**Phase C — Portable transport/device boundaries:** T-008..T-010.  
**Phase D — Server authorization and real transport:** T-011..T-013.  
**Phase E — Qualification and release:** T-014..T-016.

# 11. Atomic Tasks

## T-001 — Reconcile repository into an execution-grade living plan

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

### Problem
The existing documentation is strong research but cannot drive the requested audit→implementation loop reliably.

### Evidence
F-001, F-002, F-003, F-004, F-005, F-006.

### Goal
Create the canonical plan, baseline, findings, DAG, atomic backlog, verification strategy and iteration log.

### Scope
`MASTER_PLAN.md` only.

### Non-goals
No production code in this iteration.

### Tests
Repository/tree inspection; verify all existing architecture decisions remain represented.

### Acceptance criteria
Required 21 plan sections exist; first findings/tasks are registered; next task is dependency-ready.

### Verification commands
N/A until T-002 creates executable tooling.

### Dependencies
None.

### Risk
Low.

### Rollback
Restore previous plan from Git history.

### Estimated effort
XS.

## T-002 — Scaffold the portable Rust workspace and enforce layer ownership

**Status:** READY  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

### Problem
There is no executable baseline, and desktop-only assumptions could enter shared code immediately.

### Evidence
F-001, F-003, F-006.

### Goal
Create a minimal compiling workspace with dependency direction explicit before Servo or transport implementations are added.

### Scope
Root Cargo files and minimal crates: `webgate-core`, `webgate-browser`, `webgate-transport`, `webgate-platform`, `webgate-app`.

### Non-goals
No Servo runtime, no real VPN, no OS secret-store implementation.

### Implementation
Use stable Rust with workspace lint policy. Shared core exposes capability traits and value types only. Platform crate depends on core, never the reverse. App composes interfaces.

### Invariants
I-002, I-009, I-010, I-011, I-012.

### Edge cases
Unsupported platform cfg, object-safety/API leakage, accidental cyclic dependencies.

### Tests
Unit compile smoke tests and architecture-oriented tests for portable types.

### Mutation tests
Not yet meaningful; record N/A.

### Acceptance criteria
`cargo fmt --check`, `cargo check --workspace`, `cargo test --workspace`, `cargo clippy --workspace --all-targets -- -D warnings` pass.

### Verification commands
As above.

### Dependencies
T-001.

### Blocks
T-003..T-010.

### Risk
Low/Medium.

### Rollback
Delete scaffold commit.

### Estimated effort
S.

## T-003 — Add CI and baseline quality/security gates

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Goal
Run format/check/test/clippy on every main change; add dependency/security gates that are practical before Servo enters the graph.

### Scope
GitHub Actions, dependency policy, documented local commands.

### Tests
Workflow syntax plus local commands.

### Dependencies
T-002.

## T-004 — Pin Servo and build the minimal embedding adapter

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

### Goal
Introduce a reviewed Servo version behind `webgate-browser` without coupling the rest of core to Servo types.

### Compatibility constraints
Servo is pre-1.0; pin exactly and document upgrade policy. Start from a release carrying current security fixes, subject to build evidence.

### Dependencies
T-003.

## T-005 — Prove fail-closed Servo networking through a restricted local proxy

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Goal
The protected origin is reachable only through the WebGate proxy; proxy/transport loss cannot trigger direct access.

### Tests
Positive proxy path plus negative direct-IP, DNS alias, redirect, IPv4/IPv6, subresource and browser-restart cases.

### Mutation tests
Mutate allow/deny and fallback branches; surviving mutants block completion.

### Dependencies
T-004.

## T-006 — Build an early Android lifecycle/embedding architecture probe

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

### Goal
Prove shared core has no desktop-only assumptions before transport complexity grows.

### Scope
Android Servo launch/proxy/lifecycle probe, not production UX.

### Dependencies
T-005.

## T-007 — Implement strict navigation and deep-link policy as pure core logic

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

### Goal
Make link parsing/origin authorization deterministic, fuzzable and browser-independent.

### Tests
Property tests, IDN/Unicode, scheme confusion, encoded separators, redirect targets, opaque resource IDs.

### Mutation tests
Required.

### Dependencies
T-002; integrates with T-005.

## T-008 — Implement transport SPI and deterministic health/failover state machine

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

### Goal
Model provider lifecycle independently from process/JNI implementation.

### Tests
State/property tests across timing, failure, retry, suspend/resume and network-change dimensions.

### Mutation tests
Required for transitions/circuit-breaker policy.

### Dependencies
T-005, T-006, T-007.

## T-009 — Introduce algorithm-agile device identity

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

### Goal
Define `DeviceSigner`/public-key proof contracts without assuming one software-only algorithm.

### Direction
Prefer hardware-backed P-256/ES256 when platform support allows; retain algorithm identifiers and permit project-controlled Ed25519 for policy/update signing.

### Dependencies
T-002.

## T-010 — Implement platform secret/device adapters

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Windows: DPAPI/CNG. Android: Keystore. macOS: Keychain/Secure Enclave where applicable. Linux: selected keyring/secure-storage strategy with explicit fallback policy.

### Dependencies
T-009, T-006.

## T-011 — Integrate the control plane with SecureAcces

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Use SecureAcces sessions, resource resolution, authorization and revocation. Do not reimplement tenant authorization in WebGate.

### Dependencies
T-009, T-010.

## T-012 — Implement and qualify the primary resilient transport

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Candidate path: Outline SDK + AmneziaWG, exposed to WebGate as a restricted local proxy/dialer. Desktop may use a supervised process; Android must use an in-process/mobile-compatible adapter rather than assuming child-process semantics.

### Dependencies
T-008.

## T-013 — Add independent fallback transport and dual-relay failover

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Fallback must differ materially from primary protocol/implementation/failure mode. Validate relay/provider diversity and automatic recovery.

### Dependencies
T-012.

## T-014 — Qualify Servo/site compatibility, security and performance

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Machine-readable REQUIRED/OPTIONAL/NOT_USED web capability inventory, visual regressions, actual workload performance, pinned Servo upgrade gate and known-security review.

### Dependencies
T-004, T-005.

## T-015 — Implement signed packaging, updates and one-click link UX

**Status:** TODO  
**Priority:** P2  
**Type:** HARDEN  
**Leverage:** MEDIUM

Cross-platform signed artifacts, rollback-aware update manifest, deep-link registration and Telegram launcher flow.

### Dependencies
T-010, T-011, T-013, T-014.

## T-016 — Perform full adversarial re-audit and debt deletion pass

**Status:** TODO  
**Priority:** P0 before final release  
**Type:** HARDEN  
**Leverage:** HIGH

Repeat architecture/correctness/security/concurrency/reliability/API/testing/mutation/performance/CI/dependency audit. Delete obsolete adapters/flags and feed all new findings back into this plan.

### Dependencies
All release-scope tasks.

# 12. Testing Strategy

Testing layers:

- pure core unit/property tests first;
- compile-time/module-boundary checks;
- browser adapter integration tests;
- local proxy network-escape negative tests;
- platform adapter tests;
- Android lifecycle tests;
- transport chaos tests;
- SecureAcces integration tests;
- end-to-end deep-link → document tests.

Critical edge-case model:

`input × state × concurrency × timing × failure × permissions × configuration × external state`.

Use boundary partitions, pairwise coverage, high-risk N-wise combinations, fuzzing, property and metamorphic tests.

# 13. Mutation Testing Strategy

Mandatory for:

- URL/origin policy;
- fail-closed decisions;
- transport state machine;
- signed policy validation;
- device challenge verification;
- authorization adapters.

Initial Rust candidate: `cargo-mutants`, introduced only after the relevant pure logic exists. A survived mutant must be explained as an observable contract gap or explicitly equivalent mutant.

# 14. Performance Baselines

No code baseline exists yet.

After T-004/T-005 measure at minimum:

- process start → shell visible;
- Servo ready;
- transport/proxy ready;
- trusted-link click → first protected paint;
- warm navigation;
- idle/active RSS;
- CPU at idle;
- long-document scroll frame stability;
- reconnect recovery time;
- Android cold/warm start and battery-sensitive reconnect behavior.

Performance work cannot weaken fail-closed semantics.

# 15. Security Hardening

Required properties:

- no reusable private key in bootstrap config;
- signed and versioned policy/update formats;
- explicit trust roots and key rotation;
- destination-restricted local proxy;
- no direct protected-origin fallback;
- no arbitrary web→native bridge;
- secrets redacted structurally from logs;
- per-device revocation;
- hardware-backed device keys where available;
- pinned dependency review and SBOM;
- resource authorization on server for every protected object.

# 16. Migration Strategy

For large changes use:

`characterize → introduce boundary → dual compatibility if required → migrate callers → verify → delete legacy`.

Servo compatibility fallback is allowed only as an explicit adapter behind the same protected-network policy; never as automatic browser escape.

# 17. Deferred Work

- iOS: architecturally possible but not currently a first release target; app-store/browser-engine policy and Servo maturity must be reevaluated before commitment.
- general-purpose full-device VPN mode;
- arbitrary multi-tab general browsing;
- enterprise fleet/MDM orchestration;
- distributed multi-region authorization stores beyond actual scale needs.

# 18. Rejected Decisions

- **System-wide VPN as default:** rejected because unrelated traffic must remain untouched.
- **Secret bearer links as authorization:** rejected because forwarding/history/previews create unacceptable credential semantics.
- **Silent WebView2/system-browser fallback:** rejected because browser failure must not create a network/security downgrade.
- **Core dependency on DPAPI/Win32:** rejected because Android/Linux/macOS are architectural targets.
- **One shared user VPN key/config:** rejected because revocation and attribution must remain per user/device.
- **Authorization inside relay/VPN layer:** rejected; SecureAcces remains authoritative.

# 19. Completed Tasks

- T-001 — execution-grade plan/baseline reconciliation.
- Research: browser-engine audit and Servo decision.
- Research: SecureAcces compatibility analysis.
- Research: resilience/cross-platform/Android audit.
- Architecture: target topology and cross-platform runtime ADRs.

# 20. Iteration Log

## Iteration 1

**Task:** T-001  
**Findings addressed:** F-002  
**Unexpected findings:** F-001, F-003, F-004, F-005, F-006  
**Changes:** Replaced roadmap-style plan with execution-grade living plan; established baseline, findings, risk register, dependency DAG, atomic tasks and convergence rules.  
**Tests:** repository/tree and document consistency inspection; no executable checks exist before T-002.  
**Plan changes:** Android moved into the early dependency path; device identity made algorithm-agile; CI and portable core promoted ahead of Servo networking.  
**Commit:** `docs(plan): establish execution-grade living master plan`  
**Push:** main  
**Result:** PASS

# 21. Definition of Final Done

WebGate converges only when:

- no known Critical/High findings remain unresolved for release scope;
- all P0/P1 release tasks are DONE or explicitly REJECTED with evidence;
- protected traffic cannot escape to direct networking under tested failure modes;
- Servo REQUIRED site capabilities pass on supported platforms;
- Windows and Android architecture/runtime paths are proven; Linux/macOS meet declared support gates;
- device/session revocation works end to end with SecureAcces;
- primary and independent fallback transports survive defined chaos tests;
- critical policy/state/parser logic has mutation-resistant tests;
- formatting, build, tests, lint/static/security checks pass;
- performance meets recorded targets without weakening security;
- signed packaging/update flow is verified;
- documentation matches code;
- final re-audit finds no new fundamental blocker;
- the final verified state is pushed to `main`.
