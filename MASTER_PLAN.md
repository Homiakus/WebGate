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

The repository now has a compiling portable Rust foundation with separate core, browser, transport, platform and app crates. CI verifies the exact commit before `main` advances. No Servo runtime, real VPN provider or platform secret-store implementation is present yet.

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

Dependency rule: app/adapters may depend inward on portable contracts; `webgate-core` must not depend on OS/browser/VPN implementations. `scripts/check_architecture.py` enforces the current internal dependency graph in CI.

# 4. Baseline

Initial pre-code baseline was N/A for build/tests because no Cargo workspace existed.

Current executable baseline:

| Check | Result / policy |
|---|---|
| Cargo workspace | present |
| committed `Cargo.lock` | required |
| external runtime dependencies | zero before Servo |
| architecture dependency check | required |
| `cargo fmt --check` | required |
| `cargo check --workspace --all-targets --locked` | required |
| `cargo test --workspace --locked` | required |
| `cargo clippy --workspace --all-targets --locked -- -D warnings` | required |
| cargo-deny advisories/licenses/bans/sources | required |
| GitHub Actions third-party refs | commit-SHA pinned |
| mutation score | N/A until critical policy/state logic exists |
| coverage | not yet established |
| benchmarks | N/A before Servo/proxy |

Environment note: the assistant execution container has neither a Rust toolchain nor outbound GitHub DNS. Candidate changes are therefore assembled as a single Git commit on an isolated work branch, GitHub Actions verifies that exact SHA, and `main` advances only after green verification.

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

I-015. Internal crate dependency direction is machine-enforced; adding or changing a crate boundary requires a deliberate architecture-policy change.

I-016. CI actions with code execution are pinned to immutable commit SHAs.

# 6. Findings Registry

## F-001 — Repository had no executable baseline

**Status:** Resolved  
**Category:** Delivery / Architecture  
**Severity:** High  
**Confidence:** Confirmed  
**Resolution:** T-002 created the portable workspace and build/test/lint baseline.

## F-002 — Previous plan was roadmap-shaped, not execution-shaped

**Status:** Resolved  
**Category:** Process / Architecture  
**Severity:** High  
**Confidence:** Confirmed  
**Resolution:** T-001 established this Findings/Tasks/DAG/Iteration model.

## F-003 — Desktop-only assumptions remained in device/runtime design

**Status:** Planned  
**Category:** Cross-platform / Architecture  
**Severity:** High  
**Confidence:** Confirmed  
**Impact:** Android could require core rewrites.  
**Affected tasks:** T-006, T-009, T-010.  
**Direction:** shared capabilities only; OS implementations stay in adapters.

## F-004 — Single Ed25519 device-key assumption is not portable enough for hardware-backed identity

**Status:** Planned  
**Category:** Security / Cross-platform  
**Severity:** High  
**Confidence:** Strong  
**Affected tasks:** T-009, T-010.  
**Direction:** algorithm-agile `DeviceSigner`; prefer platform hardware-backed P-256/ES256 where appropriate; retain project-controlled Ed25519 for policy/update signatures.

## F-005 — Servo compatibility and security are version-sensitive

**Status:** Planned  
**Category:** Browser / Security  
**Severity:** High  
**Confidence:** Confirmed  
**Affected tasks:** T-004, T-005, T-014.  
**Direction:** exact pin, site compatibility suite, security review and explicit upgrade gate.

## F-006 — No automated architecture enforcement existed

**Status:** Resolved  
**Category:** Architecture / Testing  
**Severity:** Medium  
**Confidence:** Confirmed  
**Resolution:** T-002 created crate boundaries; T-003 added machine-enforced dependency policy and immutable CI dependency policy.

## F-007 — Local verification environment lacks Rust and outbound GitHub access

**Status:** Resolved  
**Category:** Tooling / Environment  
**Severity:** Medium  
**Confidence:** Confirmed  
**Resolution:** isolated work-branch exact-commit GitHub Actions verification before a non-force fast-forward to main.

## F-008 — `main` is not repository-enforced as a protected branch

**Status:** Planned / external capability limited  
**Category:** CI/CD / Governance  
**Severity:** Medium  
**Confidence:** Confirmed

**Evidence:** GitHub reports `protected: false` with required status checks disabled.  
**Impact:** repository settings do not prevent an accidental direct broken push by another actor.  
**Current mitigation:** autonomous workflow always verifies the exact commit on an isolated branch before advancing main and never force-pushes.  
**Affected task:** T-017.  
**Direction:** configure a ruleset/branch protection compatible with the verified-fast-forward workflow when a write-capable repository-settings connector is available.

# 7. Risk Register

| Risk | Probability | Impact | Mitigation |
|---|---:|---:|---|
| Servo misses REQUIRED site feature | Medium | High | capability inventory + tests + explicit fallback policy |
| direct-network escape on proxy failure | Medium | Critical | negative tests first; immutable protected proxy binding |
| Android lifecycle interrupts transport | High | High | early Android probe + idempotent lifecycle state machine |
| provider/UDP path blocked | Medium | High | independent fallback + two relays |
| stale Servo pin contains known issue | Medium | High | pin/LTS policy + security upgrade gate |
| hardware-backed identity differs by OS | High | Medium/High | algorithm-agile DeviceSigner |
| dependency/supply-chain drift | Medium | High | lockfile + cargo-deny + immutable action refs |
| authorization duplicated client-side | Low | High | SecureAcces remains authoritative |

# 8. Pareto Improvements

1. Keep portable boundaries compiling before large dependencies.
2. Encode fail-closed/navigation rules as pure mutation-testable logic.
3. Prove Servo proxy isolation before real VPN work.
4. Run Android probe before desktop lifecycle assumptions harden.
5. Keep dependency/security gates green as Servo expands the graph.
6. Add transport diversity only after browser isolation is proven.

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

T-017 branch/ruleset enforcement is independent and remains BLOCKED on repository-settings write capability.
```

# 10. Implementation Phases

**A — Executable foundation:** T-001..T-003.  
**B — Protected Servo capsule:** T-004..T-007.  
**C — Portable transport/device boundaries:** T-008..T-010.  
**D — Authorization and real transport:** T-011..T-013.  
**E — Qualification/release:** T-014..T-016.  
**Governance parallel:** T-017.

# 11. Atomic Tasks

## T-001 — Reconcile repository into an execution-grade living plan

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

## T-002 — Scaffold the portable Rust workspace and enforce layer ownership

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH  
**Verified:** exact candidate SHA passed format/check/test/clippy before main fast-forward.

## T-003 — Harden CI, dependency policy and architecture checks

**Status:** DONE  
**Priority:** P0  
**Type:** HARDEN  
**Leverage:** HIGH

### Problem
The first CI proved compilation but did not lock the dependency graph, scan advisories/licenses/sources, pin executable Actions to immutable commits or machine-enforce the crate dependency DAG.

### Evidence
F-006 plus initial workflow/tooling review.

### Goal
Make dependency and architecture drift fail CI before Servo increases the graph dramatically.

### Scope
`Cargo.lock`, `deny.toml`, `scripts/check_architecture.py`, `.github/workflows/rust-ci.yml`, this plan.

### Non-goals
No Servo dependency, branch-settings mutation, coverage or mutation tooling yet.

### Implementation
- commit application workspace lockfile;
- require `--locked` for metadata/check/test/clippy;
- pin `actions/checkout` and `cargo-deny-action` to immutable commit SHAs;
- run cargo-deny advisory/license/ban/source checks;
- fail on unapproved internal `webgate-*` dependency edges or unreviewed workspace package additions.

### Invariants
I-009, I-011, I-015, I-016.

### Tests
Architecture script, locked Cargo verification, full Rust CI and cargo-deny on exact commit.

### Mutation tests
N/A; script is guardrail tooling, while mutation testing begins with security policy/state logic.

### Acceptance criteria
Both `verify` and `dependency-policy` jobs pass on the exact commit before main advances.

### Rollback
Revert the single T-003 commit.

## T-004 — Pin Servo and build the minimal embedding adapter

**Status:** READY  
**Priority:** P0  
**Type:** IMPROVE  
**Leverage:** HIGH

Introduce a reviewed exact Servo release behind `webgate-browser`; no Servo type leaks into core. Record build/platform prerequisites and upgrade policy. Start from the latest reviewed security-bearing release that can pass the build gate; do not track `main`.

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

Property/fuzz/mutation tests for schemes, IDN/Unicode, origin matching, redirects, opaque resource IDs and external-browser policy.

## T-008 — Implement transport SPI and deterministic health/failover state machine

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

State/property/mutation tests across timing, failure, retry, suspend/resume and network changes.

## T-009 — Introduce algorithm-agile device identity

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Define DeviceSigner/public-key proof contracts; prefer hardware-backed P-256/ES256 where available.

## T-010 — Implement platform secret/device adapters

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Windows DPAPI/CNG; Android Keystore; macOS Keychain/Secure Enclave where applicable; Linux secure-storage strategy with explicit fallback policy.

## T-011 — Integrate the control plane with SecureAcces

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Use SecureAcces sessions/resource resolution/authorization/revocation; never reimplement tenant authorization in WebGate.

## T-012 — Implement and qualify the primary resilient transport

**Status:** TODO  
**Priority:** P1  
**Type:** IMPROVE  
**Leverage:** HIGH

Candidate Outline SDK + AmneziaWG behind restricted proxy/dialer; Android uses a mobile-compatible/in-process adapter rather than desktop child-process assumptions.

## T-013 — Add independent fallback transport and dual-relay failover

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

Fallback differs materially in protocol/implementation/failure mode from primary; validate relay/provider diversity and hysteresis.

## T-014 — Qualify Servo/site compatibility, security and performance

**Status:** TODO  
**Priority:** P1  
**Type:** HARDEN  
**Leverage:** HIGH

REQUIRED/OPTIONAL/NOT_USED capability inventory, visual regressions, real workload performance, pinned Servo upgrade gate and known-security review.

## T-015 — Implement signed packaging, updates and one-click link UX

**Status:** TODO  
**Priority:** P2  
**Type:** HARDEN  
**Leverage:** MEDIUM

Cross-platform signed artifacts, rollback-aware update manifest, deep-link registration and Telegram launcher flow.

## T-016 — Perform full adversarial re-audit and debt deletion pass

**Status:** TODO  
**Priority:** P0 before final release  
**Type:** HARDEN  
**Leverage:** HIGH

Repeat architecture/correctness/security/concurrency/reliability/API/testing/mutation/performance/CI/dependency audit and delete obsolete paths.

## T-017 — Enforce verified-main policy in repository settings

**Status:** BLOCKED  
**Priority:** P2  
**Type:** HARDEN  
**Leverage:** MEDIUM

Configure branch protection/ruleset compatible with exact-SHA verification and non-force fast-forward. Blocker: current connector exposes protection reads but no protection/ruleset write action. Continue all other tasks.

# 12. Testing Strategy

Pure core unit/property tests first; compile/module-boundary checks; browser adapter integration; local-proxy network-escape negative tests; platform and Android lifecycle tests; transport chaos; SecureAcces integration; end-to-end deep-link→document. Critical model: `input × state × concurrency × timing × failure × permissions × configuration × external state`.

# 13. Mutation Testing Strategy

Mandatory for URL/origin policy, fail-closed decisions, transport state machine, signed policy validation, device challenge verification and authorization adapters. Initial Rust candidate: `cargo-mutants`. Surviving mutants require observable-contract analysis.

# 14. Performance Baselines

After Servo/proxy exists measure process→shell, Servo ready, proxy/transport ready, link→first-paint, warm navigation, RSS, CPU idle, scroll stability, reconnect time and Android cold/warm start/battery-sensitive reconnect.

# 15. Security Hardening

No reusable private key in bootstrap; signed/versioned policy/update formats; explicit roots/rotation; destination-restricted local proxy; no direct fallback; no arbitrary web→native bridge; structured secret redaction; per-device revocation; hardware-backed identity where possible; locked dependency review/SBOM; server authorization for every protected resource.

# 16. Migration Strategy

`characterize → introduce boundary → dual compatibility if required → migrate callers → verify → remove legacy`. Servo compatibility fallback is explicit only and uses the same protected network policy.

# 17. Deferred Work

- iOS pending platform policy/Servo maturity reevaluation;
- general-purpose full-device VPN;
- arbitrary general browsing;
- enterprise fleet/MDM orchestration;
- distributed authorization infrastructure beyond actual scale needs.

# 18. Rejected Decisions

System-wide VPN as default; bearer-secret document links; silent browser fallback; core dependency on DPAPI/Win32; shared user VPN keys; authorization inside relay/VPN layer; mutable/tag-only CI action references after T-003.

# 19. Completed Tasks

- T-001 — execution-grade plan/baseline reconciliation.
- T-002 — portable executable Rust workspace.
- T-003 — locked dependency, immutable CI action and architecture-policy gates.
- Research — Servo/browser, SecureAcces, resilience/cross-platform/Android.
- Architecture — browser/cross-platform ADRs and target topology.

# 20. Iteration Log

## Iteration 1

**Task:** T-001  
**Findings addressed:** F-002  
**Unexpected findings:** F-001, F-003, F-004, F-005, F-006  
**Changes:** Established baseline, findings, risks, DAG, atomic tasks and convergence rules.  
**Tests:** repository/tree/document inspection.  
**Commit:** `docs(plan): establish execution-grade living master plan`  
**Push:** main  
**Result:** PASS

## Iteration 2

**Task:** T-002  
**Findings addressed:** F-001; partial F-006  
**Unexpected findings:** F-007  
**Changes:** Portable Rust workspace, capability crates, loopback endpoint invariant, unit tests and minimal exact-commit CI.  
**Tests:** format/check/test/clippy all PASS in GitHub Actions on exact candidate SHA.  
**Commit:** `refactor(core): scaffold portable Rust boundaries`  
**Push:** main  
**Result:** PASS

## Iteration 3

**Task:** T-003  
**Findings addressed:** F-006  
**Unexpected findings:** F-008  
**Changes:** Committed lockfile; added cargo-deny policy; pinned executable Actions by commit SHA; added machine-enforced crate dependency DAG; all build/test/lint operations use the lockfile.  
**Tests:** architecture checker; locked metadata/check/test/clippy; cargo-deny advisories/licenses/bans/sources on exact candidate SHA.  
**Plan changes:** added blocked T-017 for repository-level main protection; T-004 becomes next READY task.  
**Commit:** `ci(security): lock dependencies and architecture gates`  
**Push:** main after exact-commit CI PASS  
**Result:** PASS

# 21. Definition of Final Done

No unresolved Critical/High release findings; P0/P1 release tasks DONE or evidence-based REJECTED; no protected-network escape in tested failures; Servo REQUIRED features pass; Windows/Android paths proven and Linux/macOS meet declared gates; SecureAcces device/session revocation works; independent transports survive chaos; critical logic is mutation-resistant; build/test/lint/security gates pass; performance targets pass without security regression; signed updates/packages verified; docs match code; final re-audit finds no fundamental blocker; final verified state is in `main`.
