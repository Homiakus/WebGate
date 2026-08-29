# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Research baseline:** 2026-08-29  
**Canonical browser:** Servo primary; compatibility engines explicit-only.

`MASTER_PLAN.md` is the single source of truth. Material findings are recorded before scope changes. An implementation task is DONE only after its acceptance checks pass and the verified state reaches `main` without force push.

# 1. Mission

Build WebGate as a secure, resilient, cross-platform protected-browser client for a small trusted-user set. Trusted Telegram/HTTPS links open WebGate; Servo renders the private application; only protected browser traffic traverses an application-local fail-closed path; the private origin may use dynamic IP/CGNAT; `SecureAcces` remains authoritative for server-side authorization.

Primary targets: Windows, Android as an early architecture gate, Linux, macOS.

# 2. Current State

The repository contains a portable Rust workspace with core/browser/transport/platform/app crates. Exact-SHA CI enforces formatting, build, tests, Clippy, lockfile, dependency/advisory/license/source policy and the internal crate dependency DAG. No Servo dependency, real transport, device-key adapter or SecureAcces control API exists yet.

Servo is treated as potentially compromised because its own renderer sandbox is not a reliable security boundary on Windows/Android. Long-lived secrets and privileged transport/authentication authority therefore live behind a separate trusted-broker capability boundary.

A local project-manager surface is being introduced in T-020 so developers have one deterministic entry point for environment diagnosis, controlled prerequisite bootstrap, CI-parity verification, compilation and platform diagnostics.

# 3. Architecture Map

```text
untrusted link
    ↓
┌────────────────────────────────────────────────────┐
│ Browser capsule — assume compromise               │
│ Servo + page/render/input                         │
│ bounded active web capability only                │
└─────────────────┬──────────────────────────────────┘
                  │ narrow authenticated capabilities
                  ▼
┌────────────────────────────────────────────────────┐
│ Trusted broker/control plane                      │
│ policy verification / device signer / session     │
│ transport control / privileged audit / updates    │
└───────────┬────────────────────────────────────────┘
            │
    destination-restricted fail-closed proxy
            │
     replaceable transports
            │
      Relay A / Relay B
            │
       private origin
            │
       SecureAcces authz
```

Portable crates own contracts. Platform implementations and Servo-specific types stay outward of core. See `docs/architecture/ADR-0003-SERVO-PROCESS-ISOLATION.md`.

# 4. Baseline

Current executable baseline:

- Rust workspace and committed `Cargo.lock`;
- workspace `unsafe_code = forbid`;
- `scripts/check_architecture.py` internal dependency guard;
- `cargo fmt --all -- --check`;
- `cargo check --workspace --all-targets --locked`;
- `cargo test --workspace --locked`;
- `cargo clippy --workspace --all-targets --locked -- -D warnings`;
- `cargo-deny check --all-features`;
- executable GitHub Actions pinned to immutable SHAs;
- T-020 candidate adds unit-tested project-manager and local CI-parity command.

The execution container lacks Rust/direct GitHub DNS, so candidate commits are verified by GitHub Actions on isolated work branches before synchronized non-force `main` fast-forward.

# 5. System Invariants

- **I-001:** Servo is the default protected browser.
- **I-002:** normal mode never changes the OS default route.
- **I-003:** transport loss fails closed; protected origin never gains direct fallback.
- **I-004:** browser failure never silently opens a compatibility/system browser.
- **I-005:** links are resource identifiers, not persistent bearer credentials.
- **I-006:** device private keys are generated on-device and never exported through browser APIs.
- **I-007:** bootstrap, policy and updates are signed/rollback-aware.
- **I-008:** remote policy cannot weaken compiled security invariants.
- **I-009:** transport implementations are replaceable providers.
- **I-010:** SecureAcces is authoritative for authorization.
- **I-011:** core contains no desktop-only DPAPI/Win32/sidecar assumption; Android is first-class.
- **I-012:** device signing/secret storage is a platform capability; hardware-backed keys are preferred.
- **I-013:** production has at least two materially independent network failure domains.
- **I-014:** browser proxy endpoints are loopback-only and bound/non-zero.
- **I-015:** internal crate dependency direction is machine-enforced.
- **I-016:** CI code-execution actions use immutable commit pins.
- **I-017:** internal path dependencies carry explicit compatible versions; wildcard dependencies are denied.
- **I-018:** the Servo/browser capsule is not trusted with long-lived device/bootstrap/transport/session-refresh secrets or generic privileged native APIs.
- **I-019:** network fail-closed and browser-compromise containment are distinct properties and require separate tests.
- **I-020:** developer-tool bootstrap is allowlisted, reviewable and separate from runtime credentials/configuration; project-manager installation never silently mutates source code.

# 6. Findings Registry

## F-001 — No executable baseline
**Status:** Resolved · **Severity:** High · **Confidence:** Confirmed  
T-002 created the portable workspace and executable CI baseline.

## F-002 — Roadmap was not execution-grade
**Status:** Resolved · **Severity:** High · **Confidence:** Confirmed  
T-001 established findings/tasks/DAG/baseline/iteration traceability.

## F-003 — Desktop-only runtime assumptions
**Status:** Planned · **Severity:** High · **Confidence:** Confirmed  
Earlier DPAPI/child-process assumptions can force Android rewrites. Addressed by T-006/T-009/T-010/T-019.

## F-004 — Fixed Ed25519 device identity is not universally hardware-backed
**Status:** Planned · **Severity:** High · **Confidence:** Strong  
Use algorithm-agile `DeviceSigner`; prefer hardware-backed P-256/ES256 where appropriate; Ed25519 remains suitable for project-controlled policy/update signatures.

## F-005 — Servo compatibility/security is release-sensitive
**Status:** Planned · **Severity:** High · **Confidence:** Confirmed  
Servo is pre-1.0 and fast-moving. T-004/T-014 require exact release pin, upgrade review and compatibility/security gates.

## F-006 — Architecture boundaries were documentation-only
**Status:** Resolved · **Severity:** Medium  
T-002/T-003 established crate and CI-enforced dependency boundaries.

## F-007 — Local execution environment lacks Rust/network access
**Status:** Resolved · **Severity:** Medium  
Exact-SHA Actions verification is the accepted proof path.

## F-008 — `main` lacks repository-enforced protection
**Status:** Planned/BLOCKED · **Severity:** Medium · **Confidence:** Confirmed  
Current connector exposes protection reads but no rule write. T-017 remains blocked; exact-SHA verification discipline mitigates.

## F-009 — cargo-deny rejected versionless internal path dependencies
**Status:** Resolved · **Severity:** Medium · **Confidence:** Confirmed  
Policy was preserved; manifests now specify explicit internal versions.

## F-010 — checkout v4 used deprecated Node 20
**Status:** Resolved · **Severity:** Low · **Confidence:** Confirmed  
CI pins checkout v5 Node-24 commit.

## F-011 — Servo is not a sufficient renderer sandbox on Windows/Android
**Status:** Planned · **Category:** Browser/Security · **Severity:** High · **Confidence:** Confirmed

**Evidence:** current Servo source has an unsupported sandbox branch for Windows/Android/ARM; Servo's browser-engine guidance has described multiprocess/sandboxing as partial and historically limited to Linux/macOS.  
**Current behavior:** embedding Servo together with long-lived secrets/transport control would share a privilege boundary with browser compromise.  
**Expected behavior:** browser compromise cannot export device keys, mutate trust roots, reconfigure unrestricted transport, or obtain long-lived refresh/bootstrap authority.  
**Root cause:** Rust memory safety is not a substitute for a mature renderer sandbox.  
**Affected invariants:** I-006, I-018, I-019.  
**Affected tasks:** T-004/T-005, T-006, T-009..T-012, T-019.  
**Direction:** keep Servo primary, introduce portable trusted-broker capability boundary and platform-specific process sandboxing as defense in depth.

## F-012 — Servo Linux build requires explicit native fontconfig prerequisite
**Status:** Planned via T-020/T-004 · **Category:** Build/Portability · **Severity:** High · **Confidence:** Confirmed

**Evidence:** the first clean Servo 0.5.0 candidate reached `yeslogic-fontconfig-sys 6.0.1` and failed because `pkg-config` could not locate `fontconfig.pc` on the Ubuntu runner. Architecture, lockfile and formatting gates had already passed.  
**Location:** Servo dependency graph / Linux development environment / CI.  
**Root cause:** native Servo prerequisites were not represented as an explicit project environment contract.  
**Impact:** clean Linux checkout cannot compile the selected Servo graph without undocumented host packages.  
**Blast radius:** T-004 CI, Linux developer bootstrap, future reproducible build containers.  
**Affected tasks:** T-020, T-004, T-014.  
**Direction:** project manager diagnoses and installs only empirically confirmed native prerequisites (`pkg-config` + fontconfig development package); T-004 must still prove the complete Servo graph after that correction. Any additional native dependency becomes a new Finding rather than an expanding guessed package list.

# 7. Risk Register

| Risk | Impact | Mitigation |
|---|---|---|
| Servo/browser RCE reaches long-lived secrets | Critical | broker capability separation + platform sandbox defense in depth |
| proxy failure escapes direct | Critical | negative tests + immutable proxy contract |
| Servo clean build depends on hidden host state | High | project-manager doctor + explicit native prerequisite findings |
| Servo misses required site capability | High | capability suite + explicit compatibility decision |
| Android lifecycle/process model breaks assumptions | High | early Android probe + idempotent broker/browser lifecycle |
| primary transport blocked | High | independent fallback + two relays |
| dependency/security drift | High | lockfile/cargo-deny/exact pins/upgrade gate |
| hardware key support varies | High | algorithm-agile DeviceSigner |

# 8. Pareto Improvements

1. Preserve portable contracts and broker/browser privilege separation before secrets exist.
2. Make development environment and verification reproducible through T-020.
3. Prove basic Servo embedding and fail-closed normal networking.
4. Implement narrow broker capability API before device/session/transport credentials.
5. Validate Android lifecycle/isolation early.
6. Add real transports only after browser/proxy boundaries are demonstrated.

# 9. Dependency DAG

```text
T-001 → T-002 → T-003 → T-018(plan) → T-020(dev manager) → T-004 → T-005 → T-019 → T-006
                                                                    │          │
                                                                    └──→ T-007 │
T-005 + T-006 + T-007 + T-019 → T-008 → T-012 → T-013
T-019 → T-009 → T-010 → T-011
T-004 + T-005 → T-014
T-010 + T-011 + T-013 + T-014 → T-015 → T-016
T-017 governance runs independently while blocked
```

# 10. Implementation Phases

- **A Foundation:** T-001..T-003, T-018, T-020.
- **B Servo capsule/containment:** T-004, T-005, T-019, T-006, T-007.
- **C Transport/device contracts:** T-008..T-010.
- **D SecureAcces + production transports:** T-011..T-013.
- **E Qualification/release/re-audit:** T-014..T-016.
- **Governance:** T-017.

# 11. Atomic Tasks

## T-001 — Establish execution-grade living plan
**Status:** DONE · **Priority:** P0

## T-002 — Scaffold portable Rust boundaries
**Status:** DONE · **Priority:** P0

## T-003 — Harden CI/dependency/architecture gates
**Status:** DONE · **Priority:** P0

## T-018 — Reconcile Servo sandbox gap into the trust architecture
**Status:** DONE · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

## T-020 — Add cross-platform project manager and controlled prerequisite bootstrap
**Status:** VERIFYING · **Priority:** P0 · **Type:** IMPROVE/HARDEN · **Leverage:** HIGH

### Problem
Developer setup, environment diagnosis, compilation and local verification are fragmented commands. F-012 proves that native prerequisites can be hidden until deep inside a Servo compile.

### Evidence
F-007, F-012 and the existing CI command set.

### Goal
Provide one interactive/menu and scriptable entry point for project checks, allowlisted developer-tool installation/repair, compilation, tests, security checks, Servo native diagnostics and Android development diagnostics.

### Scope
`scripts/project_manager.py`, PowerShell/sh launchers, unit tests, developer documentation, CI manager tests, `.gitignore`, `MASTER_PLAN.md`.

### Non-goals
No Servo dependency integration; no Android SDK license acceptance; no arbitrary package installer; no runtime credential handling; no Git push command inside the manager.

### Implementation
- Python 3.11+ standard-library manager with interactive menu and CLI subcommands;
- `doctor`, `install`, `verify`, `build`, `test`, `security`, `servo`, `android`, `clean`;
- official rustup HTTPS endpoint only for Rust bootstrap;
- system package-manager allowlist for Git and empirically confirmed Servo native packages;
- `cargo install --locked` for cargo-deny and opt-in cargo-mutants;
- no `shell=True`, arbitrary package names or implicit Android license acceptance;
- local verify mirrors repository CI and includes manager unit tests plus `git diff --check`;
- dry-run mode for installation and verification command inspection.

### Invariants
I-015, I-016, I-017, I-020.

### Edge cases
Missing Git/Rust/cargo-deny; Linux without supported package manager; Windows x64/ARM64 rustup URL; missing `pkg-config`; missing `fontconfig.pc`; non-interactive dry-run; Android SDK absent; clean confirmation.

### Tests
Python unit tests for rustup URL selection, package-plan behavior, serialization, repository-root discovery and dry-run verification command contract; existing Rust CI remains green.

### Mutation tests
Deferred: manager does not enforce runtime security policy. Critical future policy/state logic remains subject to cargo-mutants.

### Acceptance criteria
Manager tests PASS; dry-run verification PASS; architecture/lock/fmt/check/test/clippy/cargo-deny remain PASS on exact candidate SHA; documentation describes install/security boundaries.

### Rollback
Revert this single iteration commit; no persistent runtime/data migration.

### Dependencies
T-003, T-018.

### Blocks
T-004 clean-environment recovery.

## T-004 — Pin Servo and build minimal embedding adapter
**Status:** READY after T-020 · **Priority:** P0 · **Leverage:** HIGH

Pin a reviewed exact Servo release rather than upstream `main`. Keep Servo types inside a dedicated adapter boundary. Re-run the failed 0.5.0 experiment only after F-012 host prerequisites are explicit; every newly discovered native/supply-chain failure must use Unexpected Finding Protocol.

## T-005 — Prove fail-closed Servo normal networking
**Status:** TODO · **Priority:** P0 · **Type:** HARDEN  
Positive proxy route and negative direct-IP/DNS/redirect/IPv4/IPv6/subresource/restart tests. This proves normal browser-stack routing, not RCE containment.

## T-019 — Implement the trusted broker capability boundary
**Status:** TODO · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

Introduce browser↔broker semantic contracts and process/lifecycle boundary. Browser side receives no raw device key/bootstrap/transport-control secret. IPC is versioned, instance-bound, bounded and deny-by-default. Windows/Android process-isolation prototypes are evaluated separately; portable capability separation is mandatory regardless of OS sandbox support.

**Dependencies:** T-005.  
**Blocks:** T-006, T-009..T-012.

## T-006 — Early Android lifecycle/embedding/isolation probe
**Status:** TODO · **Priority:** P0

## T-007 — Strict navigation/deep-link policy
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN

## T-008 — Transport SPI + deterministic health/failover state machine
**Status:** TODO · **Priority:** P1

## T-009 — Algorithm-agile device identity
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN

## T-010 — Platform secret/device adapters
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN

## T-011 — SecureAcces control-plane integration
**Status:** TODO · **Priority:** P1

## T-012 — Primary resilient transport
**Status:** TODO · **Priority:** P1

## T-013 — Independent fallback + dual-relay failover
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN

## T-014 — Servo/site/security/performance qualification
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN

## T-015 — Signed packaging/update/one-click link UX
**Status:** TODO · **Priority:** P2 · **Type:** HARDEN

## T-016 — Final adversarial re-audit/debt deletion
**Status:** TODO · **Priority:** P0 before release

## T-017 — Enforce verified-main repository rule
**Status:** BLOCKED · **Priority:** P2  
Blocked on repository-settings write capability.

# 12. Testing Strategy

Separate test classes explicitly:

1. developer/bootstrap command tests;
2. pure core/architecture checks;
3. Servo API/compatibility tests;
4. normal network fail-closed tests;
5. browser-compromise/broker-capability negative tests;
6. Android/platform lifecycle/isolation tests;
7. transport chaos;
8. SecureAcces integration;
9. end-to-end trusted-link flow.

Analyze `input × state × concurrency × timing × failure × permissions × configuration × external state` using boundary, pairwise/high-risk N-wise, fuzz/property/metamorphic tests.

# 13. Mutation Testing Strategy

Mandatory for URL/origin policy, fail-closed decisions, broker authorization, IPC validation, transport transitions, signed policy, device proof and auth adapters. Planned tool: `cargo-mutants`; surviving mutants require observable-contract analysis. T-020 can optionally bootstrap cargo-mutants but does not make it a gate before critical policy logic exists.

# 14. Performance Baselines

Measure process→shell, Servo ready, broker ready, proxy/transport ready, link→first paint, warm navigation, RSS/CPU, frame stability, reconnect, IPC overhead and Android cold/warm/battery-sensitive recovery. Process isolation performance cost is measured rather than assumed.

# 15. Security Hardening

Browser treated as potentially compromised; long-lived secrets stay behind broker/platform signer; no reusable bootstrap key; signed policy/update; destination-restricted proxy; no direct fallback; semantic minimal IPC; secret-redacted logs; per-device revocation; hardware-backed identity where available; locked dependencies; server-side authz per resource. Developer bootstrap is constrained to explicit tool/package allowlists and never has access to WebGate runtime secrets.

# 16. Migration Strategy

`characterize → introduce boundary → dual compatibility if needed → migrate → verify → remove legacy`. Servo remains primary. Future native Servo sandbox improvements strengthen defense in depth but do not remove the broker capability boundary without a new ADR/threat-model proof.

# 17. Deferred Work

- iOS pending platform policy/Servo maturity;
- full-device VPN;
- arbitrary general browsing;
- enterprise fleet/MDM;
- distributed auth infrastructure beyond actual scale;
- automatic Android SDK installation/license acceptance until T-006 defines exact versions and reproducibility requirements.

# 18. Rejected Decisions

System-wide VPN default; secret bearer links; silent browser fallback; DPAPI/Win32 in core; shared user VPN keys; transport-layer authorization; weakening dependency policy; treating Rust memory safety as a substitute for browser privilege isolation; general-purpose arbitrary package installation in the project manager; hiding prerequisite failures by blindly installing a broad Servo package list.

# 19. Completed Tasks

T-001, T-002, T-003 and T-018 are complete after verified main pushes. T-020 is in verification. Research/ADRs cover Servo, SecureAcces, cross-platform/Android, resilience and target topology.

# 20. Iteration Log

## Iteration 1
**Task:** T-001 · **Result:** PASS  
**Commit:** `docs(plan): establish execution-grade living master plan` → main.

## Iteration 2
**Task:** T-002 · **Unexpected:** F-007 · **Result:** PASS  
Portable workspace; exact candidate format/check/test/clippy PASS; pushed main.

## Iteration 3
**Task:** T-003 · **Unexpected:** F-008/F-009/F-010 · **Result:** PASS  
First candidate correctly failed cargo-deny and never reached main. Corrected exact SHA `acae35585b00b88f854dbfacd699db34ebabaff4` passed verify + dependency-policy and was fast-forwarded to main.

## Iteration 4
**Task:** T-018 · **Finding addressed:** F-011 · **Result:** PASS  
Servo compromise-containment model, trusted-broker boundary and revised DAG were verified and pushed as `b7b5d42bcbf4006a3bc6fe7c3fbf12d1a043bebb`.

## Iteration 5
**Task:** T-020  
**Findings addressed:** F-012; improves F-007 developer ergonomics.  
**Changes:** cross-platform menu/CLI project manager, controlled tool bootstrap, local CI-parity verification, compile/test/security commands, Servo and Android doctors, tests/docs.  
**Tests:** manager unit tests + dry-run command contract + existing Rust/security CI on exact candidate SHA.  
**Plan changes:** T-020 inserted before T-004; F-012 recorded; hidden native prerequisites are now findings-driven.  
**Commit target:** `feat(dev): add project manager and bootstrap menu`  
**Push:** main only after exact-SHA PASS  
**Result:** VERIFYING

# 21. Definition of Final Done

No unresolved Critical/High release findings; all release P0/P1 complete or evidence-based rejected; normal protected networking cannot escape direct; browser compromise cannot extract long-lived keys or escalate broker capabilities in the supported threat model; clean supported-platform builds have explicit reproducible prerequisites; Servo required site capabilities pass; Windows/Android paths are proven; SecureAcces revocation works end-to-end; independent transports survive chaos; critical policy/IPC/state logic is mutation-resistant; build/lint/security gates pass; performance targets pass without security weakening; signed release/update flow verified; docs match code; final re-audit finds no fundamental blocker; verified final state is in `main`.
