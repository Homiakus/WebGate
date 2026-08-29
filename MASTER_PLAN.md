# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Research baseline:** 2026-08-29  
**Canonical browser:** Servo primary; compatibility engines explicit-only.

`MASTER_PLAN.md` is the single source of truth. Every material finding is recorded before scope changes. Every task is complete only after verification and a non-force push/fast-forward to `main`.

# 1. Mission

Build a secure, resilient, cross-platform protected-browser client for a small trusted-user set. A trusted Telegram/HTTPS link opens WebGate; Servo renders the private web application; only WebGate browser traffic traverses an application-local fail-closed transport; the private origin may use dynamic IP/CGNAT; `SecureAcces` remains authoritative for server-side authorization.

Primary targets: Windows, Android as an early architecture gate, Linux, macOS.

# 2. Current State

The repository has a portable Rust workspace with separate core/browser/transport/platform/app crates and an exact-commit GitHub Actions verification workflow. No Servo runtime, real transport provider, platform key store, or control API exists yet.

# 3. Architecture Map

```text
untrusted link
    ↓
webgate-app (composition)
    ├── webgate-browser ── Servo adapter later
    ├── webgate-transport ─ provider/state boundary
    ├── webgate-platform ─ OS capability boundary
    └── webgate-core ─ pure portable invariants
                ↓
       loopback fail-closed proxy
                ↓
      replaceable secure transports
                ↓
        Relay A / Relay B
                ↓
          private origin
                ↓
          SecureAcces authz
```

Internal crate direction is explicit and machine-checked. Core never imports OS/browser/VPN implementations.

# 4. Baseline

Initial baseline: documentation only; build/tests N/A.

Current baseline after foundation work:

- Rust workspace: present;
- committed `Cargo.lock`: required from T-003 onward;
- workspace `unsafe_code = forbid`;
- architecture dependency checker: required;
- `cargo fmt --all -- --check`: required;
- `cargo check --workspace --all-targets --locked`: required;
- `cargo test --workspace --locked`: required;
- `cargo clippy --workspace --all-targets --locked -- -D warnings`: required;
- `cargo-deny check --all-features`: required;
- executable GitHub Actions refs: immutable commit SHAs;
- mutation/coverage/benchmarks: introduced when corresponding critical logic exists.

The execution container lacks Rust and direct GitHub DNS. Verification therefore uses an isolated work branch; GitHub Actions validates the exact candidate SHA; `main` advances only after PASS and synchronization.

# 5. System Invariants

- **I-001:** Servo is the default protected browser.
- **I-002:** normal mode never changes the OS default route.
- **I-003:** transport loss fails closed; no protected direct fallback.
- **I-004:** browser failure never silently opens WebView2/system browser.
- **I-005:** links identify resources; they are not persistent bearer credentials.
- **I-006:** device private keys are generated on-device.
- **I-007:** bootstrap, remote policy and updates are signed/rollback-aware.
- **I-008:** remote policy cannot weaken compiled security invariants.
- **I-009:** transport implementations remain replaceable providers.
- **I-010:** SecureAcces remains authoritative for authorization.
- **I-011:** shared core has no DPAPI/Win32/desktop-sidecar assumption; Android is first-class.
- **I-012:** secret/device key storage is a platform capability; hardware-backed identity is preferred where available.
- **I-013:** production has at least two materially independent network failure domains.
- **I-014:** browser proxy endpoints are loopback-only and non-zero/bound.
- **I-015:** internal crate dependency direction is machine-enforced.
- **I-016:** CI code-execution actions are immutable-SHA pinned.
- **I-017:** internal path dependencies carry explicit workspace-compatible versions; wildcard dependency policy is denied.

# 6. Findings Registry

## F-001 — No executable baseline
**Status:** Resolved · **Severity:** High · **Confidence:** Confirmed  
Documentation-only start prevented code verification. Resolved by T-002 portable workspace and executable CI baseline.

## F-002 — Roadmap was not execution-grade
**Status:** Resolved · **Severity:** High · **Confidence:** Confirmed  
Resolved by T-001: findings, tasks, DAG, risks, baseline and iteration log became canonical.

## F-003 — Desktop-only runtime assumptions
**Status:** Planned · **Category:** Cross-platform · **Severity:** High · **Confidence:** Confirmed  
Earlier DPAPI/sidecar assumptions can force Android rewrites. T-006/T-009/T-010 keep platform capabilities outside core.

## F-004 — Fixed Ed25519 device identity is not universally hardware-backed
**Status:** Planned · **Category:** Security/Cross-platform · **Severity:** High · **Confidence:** Strong  
Use algorithm-agile `DeviceSigner`; prefer hardware-backed P-256/ES256 where appropriate; project-controlled Ed25519 remains suitable for policy/update signatures.

## F-005 — Servo compatibility/security is release-sensitive
**Status:** Planned · **Category:** Browser/Security · **Severity:** High · **Confidence:** Confirmed  
Servo is pre-1.0 and fast-moving. T-004/T-014 require exact pin, upgrade review, compatibility and security gates.

## F-006 — Architecture boundaries were documentation-only
**Status:** Resolved · **Severity:** Medium · **Confidence:** Confirmed  
T-002 created crate boundaries; T-003 adds machine-enforced internal dependency policy.

## F-007 — Local execution environment lacks Rust/network access
**Status:** Resolved · **Category:** Environment · **Severity:** Medium · **Confidence:** Confirmed  
Exact-SHA GitHub Actions verification on isolated branches is the accepted proof path.

## F-008 — `main` is not repository-enforced protected
**Status:** Planned / BLOCKED capability · **Category:** Governance · **Severity:** Medium · **Confidence:** Confirmed  
GitHub reports `protected: false` and no required checks. Current process still verifies exact SHA before non-force fast-forward. T-017 needs a repository-settings write capability not exposed by the connector.

## F-009 — cargo-deny correctly rejected versionless internal path dependencies
**Status:** Resolved in T-003 · **Category:** Dependency policy · **Severity:** Medium · **Confidence:** Confirmed  
**Evidence:** candidate CI reported `error[wildcard]` for each `{ path = "..." }` internal dependency.  
**Root cause:** Cargo path-only dependencies semantically omit a version and trigger `wildcards = "deny"`.  
**Decision:** do not weaken policy; add `version = "0.1.0"` alongside each path. This makes internal compatibility intent explicit and keeps wildcard denial meaningful.

## F-010 — checkout v4 is now a Node-20 legacy action
**Status:** Resolved in T-003 · **Category:** Supply chain/CI · **Severity:** Low · **Confidence:** Confirmed  
**Evidence:** GitHub runner warned that checkout v4 targets deprecated Node 20. Current checkout v5 declares `using: node24`.  
**Decision:** pin checkout v5 commit `fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09` rather than freezing a known legacy runtime.

# 7. Risk Register

| Risk | Impact | Mitigation |
|---|---|---|
| proxy failure escapes direct | Critical | negative tests before real VPN; immutable protected proxy contract |
| Servo misses required site capability | High | machine-readable capability suite + explicit compatibility decision |
| Android lifecycle destroys network state | High | early lifecycle probe + idempotent state machine |
| primary protocol/provider blocked | High | independent fallback + dual relay |
| stale/compromised dependency | High | lockfile, cargo-deny, exact pins, SBOM/security review |
| platform key hardware differs | High | algorithm-agile device signer |
| authz duplicated client-side | High | SecureAcces authoritative on every protected resource |

# 8. Pareto Improvements

1. Preserve portable boundaries before dependency growth.
2. Make fail-closed/navigation decisions pure and mutation-testable.
3. Prove Servo proxy isolation before real transport work.
4. Run Android architecture probe early.
5. Keep dependency/security gates green as Servo expands the graph.
6. Add transport diversity only after browser isolation passes.

# 9. Dependency DAG

```text
T-001 → T-002 → T-003 → T-004 → T-005 → T-006
                              │        │
                              └────→ T-007
T-005 + T-006 + T-007 → T-008 → T-012 → T-013
T-002 → T-009 → T-010 → T-011
T-004 + T-005 → T-014
T-010 + T-011 + T-013 + T-014 → T-015 → T-016
T-017 governance runs independently while blocked on tooling capability
```

# 10. Implementation Phases

- **A Foundation:** T-001..T-003.
- **B Servo capsule:** T-004..T-007.
- **C Portable transport/device contracts:** T-008..T-010.
- **D SecureAcces + production transports:** T-011..T-013.
- **E Qualification/release/re-audit:** T-014..T-016.
- **Governance:** T-017.

# 11. Atomic Tasks

## T-001 — Establish execution-grade living plan
**Status:** DONE · **Priority:** P0 · **Leverage:** HIGH

## T-002 — Scaffold portable Rust boundaries
**Status:** DONE · **Priority:** P0 · **Leverage:** HIGH  
Created core/browser/transport/platform/app crates and loopback endpoint invariant. Exact candidate passed format/check/test/clippy before main fast-forward.

## T-003 — Harden CI, dependency policy and architecture checks
**Status:** VERIFYING · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

**Problem:** initial CI lacked lock enforcement, dependency/advisory policy, immutable action pins and a machine-enforced crate DAG.  
**Scope:** `Cargo.lock`, `deny.toml`, internal manifests, `scripts/check_architecture.py`, `rust-ci.yml`, plan.  
**Non-goals:** no Servo, coverage, mutation framework or branch-settings mutation.  
**Implementation:** committed lockfile; `--locked` everywhere; cargo-deny; immutable action SHAs; explicit versions on internal path dependencies; architecture checker.  
**Tests:** architecture checker, metadata lock verification, format/check/test/clippy, cargo-deny advisory/license/ban/source checks.  
**Acceptance:** both `verify` and `dependency-policy` PASS on the exact corrected candidate SHA.  
**Rollback:** revert this one iteration commit.  
**Unexpected findings:** F-009, F-010.  
**Failure recovery:** first candidate `1552fc3...` intentionally did not reach main because cargo-deny caught versionless internal path deps; corrected candidate is built cleanly from current main.

## T-004 — Pin Servo and build minimal embedding adapter
**Status:** READY after T-003 PASS · **Priority:** P0 · **Leverage:** HIGH  
Select a reviewed exact Servo release, add adapter crate without leaking Servo types into core, document native prerequisites, prove construction/event-loop/rendering-context lifecycle in CI where feasible.

## T-005 — Prove fail-closed Servo networking
**Status:** TODO · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH  
Positive proxy route and negative direct-IP/DNS/redirect/IPv4/IPv6/subresource/restart tests. Mutation testing required for allow/deny/fallback logic.

## T-006 — Build early Android lifecycle/embedding probe
**Status:** TODO · **Priority:** P0 · **Leverage:** HIGH  
Servo Android embedding, protected proxy path, pause/resume/recreate and transport lifecycle without desktop-only core assumptions.

## T-007 — Implement strict navigation/deep-link policy
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH  
Pure core logic with property/fuzz/mutation tests for schemes, IDN/Unicode, redirects, origins and opaque IDs.

## T-008 — Implement transport SPI and deterministic failover state machine
**Status:** TODO · **Priority:** P1 · **Leverage:** HIGH  
Property/mutation tests over timing/failure/retry/suspend/network-change space.

## T-009 — Introduce algorithm-agile device identity
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

## T-010 — Implement platform secret/device adapters
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH  
Windows DPAPI/CNG; Android Keystore; macOS Keychain/Secure Enclave where applicable; explicit Linux policy.

## T-011 — Integrate control plane with SecureAcces
**Status:** TODO · **Priority:** P1 · **Leverage:** HIGH  
Reuse sessions/resource resolution/Authorize/revocation; no client-side tenant authority.

## T-012 — Implement primary resilient transport
**Status:** TODO · **Priority:** P1 · **Leverage:** HIGH  
Candidate Outline SDK + AmneziaWG behind restricted proxy/dialer; Android uses mobile/in-process architecture rather than mandatory child processes.

## T-013 — Add independent fallback and dual-relay failover
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

## T-014 — Qualify Servo compatibility/security/performance
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

## T-015 — Signed packaging, updates and one-click link UX
**Status:** TODO · **Priority:** P2 · **Type:** HARDEN · **Leverage:** MEDIUM

## T-016 — Final adversarial re-audit and debt deletion
**Status:** TODO · **Priority:** P0 before release · **Type:** HARDEN · **Leverage:** HIGH

## T-017 — Enforce verified-main repository rule
**Status:** BLOCKED · **Priority:** P2 · **Type:** HARDEN  
Connector can read branch protection/rulesets but exposes no write action. Do not block implementation; continue exact-SHA verification discipline.

# 12. Testing Strategy

Pure core tests → architecture/compile checks → browser adapter integration → proxy escape negatives → Android lifecycle/platform tests → transport chaos → SecureAcces integration → end-to-end trusted-link flow. Analyze `input × state × concurrency × timing × failure × permissions × configuration × external state` with boundary partitions, pairwise/high-risk N-wise, fuzz/property/metamorphic tests.

# 13. Mutation Testing Strategy

Mandatory for URL/origin policy, fail-closed decisions, transport transitions, signed-policy validation, device proof and auth adapters. Planned tool: `cargo-mutants`; surviving mutants require contract analysis.

# 14. Performance Baselines

After Servo/proxy exists: process→shell, Servo-ready, proxy/transport-ready, trusted-link→first paint, warm nav, RSS/CPU, scroll stability, reconnect recovery, Android cold/warm start and battery-sensitive reconnect.

# 15. Security Hardening

No reusable private key in bootstrap; signed/versioned policy/update formats; explicit roots/rotation; destination-restricted proxy; no direct fallback; minimal web→native capabilities; secret-redacted logs; per-device revocation; hardware-backed identity where possible; locked reviewed dependency graph; server-side authorization per resource.

# 16. Migration Strategy

`characterize → introduce boundary → dual compatibility only if needed → migrate → verify → remove legacy`. Compatibility browser remains explicit and uses identical protected-network policy.

# 17. Deferred Work

- iOS until Servo/platform policy is reevaluated;
- general-purpose system VPN;
- arbitrary general browsing;
- enterprise MDM/fleet orchestration;
- distributed authorization infrastructure beyond observed scale.

# 18. Rejected Decisions

System-wide VPN as default; secret bearer links; silent browser fallback; DPAPI/Win32 in core; shared user VPN keys; authorization in transport; weakening cargo-deny wildcard policy to accommodate internal path deps; mutable/tag-only CI execution refs.

# 19. Completed Tasks

T-001 and T-002 are complete. T-003 becomes complete only when the corrected exact candidate passes both CI jobs and is fast-forwarded to main. Research and ADR work for Servo, SecureAcces, cross-platform/Android and target topology already exists.

# 20. Iteration Log

## Iteration 1
**Task:** T-001  
**Unexpected:** F-001, F-003..F-006  
**Commit:** `docs(plan): establish execution-grade living master plan`  
**Push:** main  
**Result:** PASS

## Iteration 2
**Task:** T-002  
**Unexpected:** F-007  
**Changes:** portable workspace + first enforced invariants.  
**Tests:** format/check/test/clippy exact candidate PASS.  
**Commit:** `refactor(core): scaffold portable Rust boundaries`  
**Push:** main  
**Result:** PASS

## Iteration 3
**Task:** T-003  
**Unexpected:** F-008, F-009, F-010  
**Changes:** lockfile, cargo-deny, immutable CI pins, Node-24 checkout, explicit internal dependency versions, architecture DAG checker.  
**First candidate:** `1552fc311cb2c785ce3d5f5f6222ccd607049511` — verify job PASS, dependency-policy FAIL on wildcard path deps; not pushed to main.  
**Recovery decision:** preserve `wildcards = deny`, fix manifests, rebuild candidate from current main.  
**Tests required:** architecture + lock + fmt/check/test/clippy + cargo-deny.  
**Commit target:** `ci(security): lock dependencies and architecture gates`  
**Push:** main only after corrected exact-SHA PASS.  
**Result:** VERIFYING

# 21. Definition of Final Done

No unresolved Critical/High release findings; P0/P1 release tasks complete or evidence-based rejected; no tested protected-network escape; Servo required capabilities pass; Windows/Android paths proven and Linux/macOS meet declared gates; SecureAcces revocation works end-to-end; independent transports survive chaos; critical policies are mutation-resistant; build/lint/security gates pass; performance targets pass without security weakening; signed packaging/update verified; docs match code; final re-audit finds no fundamental blocker; final verified state is in `main`.
