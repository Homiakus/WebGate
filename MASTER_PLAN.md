# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Research baseline:** 2026-08-29  
**Canonical browser decision:** Servo is the primary protected browser engine.

This file is the single source of truth for implementation order, findings, invariants, verification and iteration history. Code must not materially outrun this plan.

# 1. Mission

Build WebGate as a secure, resilient, cross-platform protected-browser client for a small set of trusted users. WebGate opens trusted links in an embedded Servo browser and routes only protected-browser traffic through an application-local fail-closed transport. The private origin may have dynamic addressing or CGNAT. Authorization remains server-side in `SecureAcces`.

Primary targets: Windows, Android as an early first-class architecture gate, Linux and macOS.

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

The repository has crossed from documentation-only research into the first executable foundation. The portable Rust workspace contains separate core, browser, transport, platform and app crates. No Servo runtime or real VPN implementation is present yet.

Existing decisions:

- Servo is primary;
- WebView2 is compatibility-only and never a silent fallback;
- protected traffic is application-scoped and fail-closed;
- SecureAcces remains authoritative for authorization;
- Android is an early architecture gate;
- production requires independent transport/relay failure domains.

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
│ └─ portable values/invariants                 │
│                                               │
│ webgate-browser ── ProtectedBrowser           │
│ webgate-transport ─ TransportProvider         │
│ webgate-platform ─ PlatformRuntime            │
│ webgate-app ─ composition only                │
└───────────────────────┬───────────────────────┘
                        │
                 fail-closed proxy
                        │
              transport implementations
                        │
                    relays/origin
                        │
                    SecureAcces
```

Dependency rule: app/adapters may depend inward on portable contracts; `webgate-core` must not depend on OS/browser/VPN implementations.

# 4. Baseline

Initial pre-code baseline was N/A for build/tests because no Cargo workspace existed.

T-002 establishes the first executable baseline:

| Check | Baseline after T-002 |
|---|---|
| Cargo workspace | present |
| external runtime dependencies | zero |
| unit tests | core/platform/browser/transport smoke + endpoint invariant tests |
| `cargo fmt --check` | required by CI |
| `cargo check --workspace --all-targets` | required by CI |
| `cargo test --workspace` | required by CI |
| `cargo clippy --workspace --all-targets -- -D warnings` | required by CI |
| security scan | planned T-003 |
| mutation score | N/A until critical pure policy exists |
| coverage | not yet established |
| benchmarks | N/A before Servo/proxy |

Environment note: the assistant execution container has neither a Rust toolchain nor outbound GitHub DNS. Exact-commit verification for T-002 is therefore performed in GitHub Actions on an isolated work branch before `main` is advanced.

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

I-014. A `LocalProxyEndpoint` exposed to the browser must be loopback-only and refer to an already-bound non-zero port.

# 6. Findings Registry

## F-001 — Repository had no executable baseline

**Status:** Resolved  
**Category:** Delivery / Architecture  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** initial `main` had documentation only.  
**Root cause:** project intentionally began documentation-first.  
**Impact:** no code-level invariant could be verified.  
**Resolution:** T-002 creates the portable workspace and executable quality baseline.  
**Affected tasks:** T-002, T-003.

## F-002 — Previous plan was roadmap-shaped, not execution-shaped

**Status:** Resolved  
**Category:** Process / Architecture  
**Severity:** High  
**Confidence:** Confirmed

**Resolution:** T-001 replaced it with this Findings/Tasks/DAG/Iteration model.

## F-003 — Desktop-only assumptions remained in device/runtime design

**Status:** Planned  
**Category:** Cross-platform / Architecture  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** earlier design named DPAPI and supervised desktop sidecars directly in shared milestones.  
**Root cause:** Windows-first research preceded Android-first-class requirement.  
**Impact:** Android could require core rewrites.  
**Affected invariants:** I-011, I-012.  
**Affected tasks:** T-006, T-009, T-010.  
**Direction:** shared capabilities only; OS implementations stay in adapters.

## F-004 — Single Ed25519 device-key assumption is not portable enough for hardware-backed identity

**Status:** Planned  
**Category:** Security / Cross-platform  
**Severity:** High  
**Confidence:** Strong

**Impact:** non-exportable device identity may be unavailable on some OS/hardware combinations.  
**Affected tasks:** T-009, T-010.  
**Direction:** algorithm-agile `DeviceSigner`; prefer platform hardware-backed P-256/ES256 where appropriate; retain project-controlled Ed25519 for policy/update signatures.

## F-005 — Servo compatibility and security are version-sensitive

**Status:** Planned  
**Category:** Browser / Security  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** Servo is pre-1.0 with monthly releases and an LTS path; upstream embedding and web compatibility continue to evolve.  
**Affected tasks:** T-004, T-005, T-014.  
**Direction:** exact pin, site compatibility suite, security review and explicit upgrade gate.

## F-006 — No automated architecture enforcement existed

**Status:** Planned  
**Category:** Architecture / Testing  
**Severity:** Medium  
**Confidence:** Confirmed

**Resolution progress:** T-002 creates crate boundaries and forbids unsafe code workspace-wide. T-003 will add dependency/security policy and stronger architecture checks.

## F-007 — Local verification environment lacks Rust and outbound GitHub access

**Status:** Resolved  
**Category:** Tooling / Environment  
**Severity:** Medium  
**Confidence:** Confirmed

**Evidence:** `rustc`/`cargo` are absent and direct clone cannot resolve `github.com` in the execution container.  
**Impact:** local build cannot be the proof gate for repository changes.  
**Resolution:** candidate changes are assembled as one Git commit on an isolated work branch, GitHub Actions verifies that exact commit, and `main` advances only after green verification. No force push is used.

# 7. Risk Register

| Risk | Probability | Impact | Mitigation |
|---|---:|---:|---|
| Servo cannot render a REQUIRED site feature | Medium | High | capability inventory + tests + explicit fallback policy |
| direct-network escape on proxy failure | Medium | Critical | negative tests first; immutable protected proxy binding |
| Android lifecycle interrupts transport | High | High | early Android probe + idempotent lifecycle state machine |
| provider/UDP path blocked | Medium | High | independent fallback transport + two relays |
| stale Servo pin contains known issue | Medium | High | pin/LTS policy + security upgrade gate |
| hardware-backed identity differs by OS | High | Medium/High | algorithm-agile DeviceSigner |
| scope expands into general VPN/browser | Medium | High | destination allowlist + explicit non-goals |
| authorization is duplicated client-side | Low | High | SecureAcces remains authoritative |

# 8. Pareto Improvements

1. Compiling portable boundaries before large dependencies.
2. Fail-closed/navigation policy as pure mutation-testable logic.
3. Servo proxy escape proof before real VPN work.
4. Android probe before desktop lifecycle patterns harden.
5. CI/security gates before dependency growth.
6. Transport diversity only after browser isolation is proven.

# 9. Dependency DAG

```text
T-001 plan/baseline
   ↓
T-002 portable Rust workspace
   ↓
T-003 CI/security/architecture gates
   ↓
T-004 Servo pin + embedding spike
   ↓
T-005 fail-closed Servo proxy ────┐
   ↓                              │
T-006 Android probe               │
   ↓                              │
T-007 policy/deeplink core ◄──────┘
   ↓
T-008 transport SPI/state machine
   ↓
T-009 device identity abstraction
   ↓
T-010 platform secret/device adapters
   ↓
T-011 SecureAcces integration
   ↓
T-012 primary transport
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

**A — Executable foundation:** T-001..T-003.  
**B — Protected Servo capsule:** T-004..T-007.  
**C — Portable transport/device boundaries:** T-008..T-010.  
**D — Authorization and real transport:** T-011..T-013.  
**E — Qualification/release:** T-014..T-016.

# 11. Atomic Tasks

## T-001 — Reconcile repository into an execution-grade living plan

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Acceptance:** required plan structure, first findings, DAG and iteration log established.

## T-002 — Scaffold the portable Rust workspace and enforce layer ownership

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

### Problem
No executable baseline existed and desktop assumptions could enter shared code immediately.

### Evidence
F-001, F-003, F-006, F-007.

### Goal
Create a minimal compiling workspace with explicit dependency direction before Servo or real transports.

### Scope
Root Cargo/toolchain files; `webgate-core`, `webgate-browser`, `webgate-transport`, `webgate-platform`, `webgate-app`; minimal verification workflow needed to prove the exact candidate commit in the available environment.

### Non-goals
No Servo runtime, real VPN, secret-store implementation, deep-link policy or failover algorithm.

### Implementation
- stable Rust, edition 2024, MSRV floor compatible with current Servo requirements;
- workspace-wide `unsafe_code = forbid`;
- portable core has no OS/browser/VPN dependencies;
- browser/transport/platform are contracts around the core;
- app only composes boundaries;
- local proxy endpoint type accepts only loopback and non-zero bound ports.

### Invariants
I-002, I-009, I-010, I-011, I-012, I-014.

### Edge cases
IPv4/IPv6 loopback, non-loopback rejection, zero-port rejection, target-platform selection.

### Tests
Unit tests for platform identity and proxy endpoint invariants; workspace build/lint/test in CI.

### Mutation tests
N/A: mutation tooling is deferred until nontrivial policy/state logic exists.

### Acceptance criteria
`cargo fmt --all -- --check`, `cargo check --workspace --all-targets`, `cargo test --workspace`, `cargo clippy --workspace --all-targets -- -D warnings` pass on the exact commit before main advances.

### Dependencies
T-001.

### Blocks
T-003 onward.

### Risk
Low.

### Rollback
Revert this single scaffold commit.

## T-003 — Harden CI, dependency policy and architecture checks

**Status:** READY  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

Add locked dependency workflow, `cargo audit`/advisory strategy, `cargo deny` policy, action pinning, cross-target compile strategy where practical and architecture-boundary checks. Establish committed `Cargo.lock` policy for application builds.

**Dependencies:** T-002.

## T-004 — Pin Servo and build the minimal embedding adapter

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

Introduce a reviewed exact Servo release behind `webgate-browser`; no Servo type leaks into core. Record build/platform prerequisites and upgrade policy.

**Dependencies:** T-003.

## T-005 — Prove fail-closed Servo networking through a restricted local proxy

**Status:** TODO  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

Positive proxy path plus negative direct-IP, DNS alias, redirect, IPv4/IPv6, subresource and restart cases. Mutation testing required for allow/deny/fallback decisions.

**Dependencies:** T-004.

## T-006 — Build an early Android lifecycle/embedding architecture probe

**Status:** TODO  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

Prove shared core has no desktop-only assumptions; validate Servo Android embedding, proxy path, pause/resume/recreate and transport lifecycle.

**Dependencies:** T-005.

## T-007 — Implement strict navigation and deep-link policy as pure core logic

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Property/fuzz/mutation tests for URL schemes, IDN/Unicode, origin matching, redirects, opaque resource IDs and external-browser policy.

**Dependencies:** T-002; integrates with T-005.

## T-008 — Implement transport SPI and deterministic health/failover state machine

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

State/property/mutation tests across timing, failure, retry, suspend/resume and network changes.

**Dependencies:** T-005, T-006, T-007.

## T-009 — Introduce algorithm-agile device identity

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Define DeviceSigner/public-key proof contracts. Prefer hardware-backed P-256/ES256 where platform support allows; algorithm identifiers are explicit.

**Dependencies:** T-002.

## T-010 — Implement platform secret/device adapters

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Windows DPAPI/CNG; Android Keystore; macOS Keychain/Secure Enclave where applicable; Linux secure-storage strategy with explicit fallback policy.

**Dependencies:** T-006, T-009.

## T-011 — Integrate the control plane with SecureAcces

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Use SecureAcces sessions/resource resolution/authorization/revocation; never reimplement tenant authorization in WebGate.

**Dependencies:** T-009, T-010.

## T-012 — Implement and qualify the primary resilient transport

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Candidate Outline SDK + AmneziaWG behind restricted proxy/dialer. Desktop may use supervised process; Android must use a mobile-compatible/in-process adapter rather than assuming child-process semantics.

**Dependencies:** T-008.

## T-013 — Add independent fallback transport and dual-relay failover

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Fallback must differ materially in protocol/implementation/failure mode from primary. Validate relay/provider diversity and recovery hysteresis.

**Dependencies:** T-012.

## T-014 — Qualify Servo/site compatibility, security and performance

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

REQUIRED/OPTIONAL/NOT_USED capability inventory, visual regressions, actual workload performance, pinned Servo upgrade gate and known-security review.

**Dependencies:** T-004, T-005.

## T-015 — Implement signed packaging, updates and one-click link UX

**Status:** TODO  
**Priority:** P2  
**Type:** HARDEN  
**Leverage:** MEDIUM

Cross-platform signed artifacts, rollback-aware update manifest, deep-link registration and Telegram launcher flow.

**Dependencies:** T-010, T-011, T-013, T-014.

## T-016 — Perform full adversarial re-audit and debt deletion pass

**Status:** TODO  
**Priority:** P0 before final release  
**Type:** HARDEN  
**Leverage:** HIGH

Repeat architecture/correctness/security/concurrency/reliability/API/testing/mutation/performance/CI/dependency audit; delete obsolete paths and feed all findings back into this plan.

# 12. Testing Strategy

- pure core unit/property tests first;
- compile/module-boundary checks;
- browser adapter integration tests;
- local-proxy network-escape negative tests;
- platform and Android lifecycle tests;
- transport chaos tests;
- SecureAcces integration tests;
- end-to-end deep-link → document tests.

Critical model: `input × state × concurrency × timing × failure × permissions × configuration × external state` using boundary partitions, pairwise/high-risk N-wise, fuzzing, property and metamorphic tests.

# 13. Mutation Testing Strategy

Mandatory for URL/origin policy, fail-closed decisions, transport state machine, signed policy validation, device challenge verification and authorization adapters. Initial Rust candidate: `cargo-mutants`. Surviving mutants require observable-contract analysis, not superficial coverage inflation.

# 14. Performance Baselines

After Servo/proxy exists measure process→shell, Servo ready, proxy/transport ready, trusted-link→first-paint, warm navigation, idle/active RSS, CPU idle, scroll stability, reconnect time, Android cold/warm start and battery-sensitive reconnect behavior.

# 15. Security Hardening

No reusable private key in bootstrap; signed/versioned policy/update formats; explicit roots/rotation; destination-restricted local proxy; no direct protected-origin fallback; no arbitrary web→native bridge; structured secret redaction; per-device revocation; hardware-backed identity where possible; pinned dependency review/SBOM; server authorization for every protected resource.

# 16. Migration Strategy

`characterize → introduce boundary → dual compatibility if required → migrate callers → verify → remove legacy`.

Servo compatibility fallback is explicit only and must use the same protected network policy.

# 17. Deferred Work

- iOS until app-store/browser-engine policy and Servo maturity are reevaluated;
- general-purpose full-device VPN mode;
- arbitrary general web browsing;
- enterprise fleet/MDM orchestration;
- distributed authorization infrastructure beyond actual scale needs.

# 18. Rejected Decisions

- system-wide VPN as default;
- bearer-secret document links;
- silent WebView2/system-browser fallback;
- core dependency on DPAPI/Win32;
- one shared user VPN key/config;
- authorization inside relay/VPN layer.

# 19. Completed Tasks

- T-001 — execution-grade plan/baseline reconciliation.
- T-002 — portable executable Rust workspace and first build/test/lint baseline.
- Research — browser/Servo, SecureAcces integration, resilience/cross-platform/Android.
- Architecture — browser and cross-platform ADRs plus target topology.

# 20. Iteration Log

## Iteration 1

**Task:** T-001  
**Findings addressed:** F-002  
**Unexpected findings:** F-001, F-003, F-004, F-005, F-006  
**Changes:** Established baseline, findings, risks, DAG, atomic tasks and convergence rules.  
**Tests:** repository/tree/document inspection.  
**Plan changes:** Android moved early; device identity algorithm made a future abstraction; CI/portable core promoted.  
**Commit:** `docs(plan): establish execution-grade living master plan`  
**Push:** main  
**Result:** PASS

## Iteration 2

**Task:** T-002  
**Findings addressed:** F-001; partial F-006  
**Unexpected findings:** F-007  
**Changes:** Added portable Rust workspace, core/browser/transport/platform/app boundaries, loopback-only proxy endpoint invariant, unit tests, workspace lint policy and minimal exact-commit CI verification.  
**Tests:** `cargo fmt --all -- --check`; `cargo check --workspace --all-targets`; `cargo test --workspace`; `cargo clippy --workspace --all-targets -- -D warnings`. Verification is performed by GitHub Actions on the candidate commit because the execution container lacks Rust.  
**Plan changes:** CI bootstrap admitted into T-002 as the minimum mechanism required to verify the implementation; deeper CI/security hardening remains T-003.  
**Commit:** `refactor(core): scaffold portable Rust boundaries`  
**Push:** main after exact-commit CI PASS  
**Result:** PASS

# 21. Definition of Final Done

- no unresolved Critical/High release findings;
- all P0/P1 release tasks DONE or evidence-based REJECTED;
- protected traffic cannot escape to direct networking under tested failures;
- Servo REQUIRED site capabilities pass supported platforms;
- Windows and Android runtime paths are proven; Linux/macOS meet declared support gates;
- device/session revocation works end-to-end with SecureAcces;
- primary and independent fallback transports survive chaos tests;
- critical policy/state/parser logic is mutation-resistant;
- format/build/test/lint/static/security checks pass;
- performance meets recorded targets without security regression;
- signed packaging/update flow is verified;
- documentation matches code;
- final re-audit finds no fundamental blocker;
- final verified state is pushed to `main`.
