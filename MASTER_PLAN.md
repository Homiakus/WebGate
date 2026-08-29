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

The repository contains a portable Rust workspace with core/browser/transport/platform/app crates. Exact-SHA CI enforces formatting, build, tests, Clippy, lockfile, dependency/advisory/license/source policy and the internal crate dependency DAG.

T-004 is integrating a dedicated `webgate-browser-servo` adapter with Servo pinned exactly to `0.5.0`. The generated dependency graph is committed and must pass WebGate's own CI before Servo is accepted into `main`. No real protected proxy, transport provider, device-key adapter or SecureAcces control API exists yet.

Servo is treated as potentially compromised because its renderer sandbox is not a reliable security boundary on Windows/Android. Long-lived secrets and privileged transport/authentication authority therefore live behind the planned trusted-broker capability boundary.

# 3. Architecture Map

```text
untrusted link
    ↓
┌────────────────────────────────────────────────────┐
│ Browser capsule — assume compromise               │
│ webgate-browser contract                          │
│        ↑                                           │
│ webgate-browser-servo → exact Servo release       │
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

Portable crates own contracts. Servo-specific types remain inside `webgate-browser-servo`; platform implementations remain outside core. See `docs/architecture/ADR-0003-SERVO-PROCESS-ISOLATION.md`.

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
- executable GitHub Actions pinned to immutable SHAs.

T-004 adds a large third-party browser dependency graph. The graph is not accepted merely because Servo is published: exact WebGate candidate verification remains mandatory.

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
- **I-020:** Servo types and lifecycle objects do not leak through the portable `webgate-browser` API.
- **I-021:** Servo is pinned to an exact reviewed release; upstream `main`/floating semver is not a production dependency source.

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

**Evidence:** current Servo source has an unsupported sandbox branch for Windows/Android/ARM; Servo browser-engine guidance describes multiprocess/sandbox support as incomplete outside supported paths.  
**Current behavior:** embedding Servo together with long-lived secrets/transport control would share the browser compromise boundary.  
**Expected behavior:** browser compromise cannot export device keys, mutate trust roots, reconfigure unrestricted transport, or obtain long-lived refresh/bootstrap authority.  
**Root cause:** Rust memory safety is not equivalent to a mature renderer sandbox.  
**Blast radius:** browser architecture, identity, transport, Android/Windows platform runtime, session handling.  
**Affected invariants:** I-006, I-018, I-019.  
**Affected tasks:** T-004/T-005, T-006, T-009..T-012, T-019.  
**Direction:** keep Servo primary; enforce trusted-broker capability separation and platform sandboxing as defense in depth. ADR-0003 records the decision.

# 7. Risk Register

| Risk | Impact | Mitigation |
|---|---|---|
| Servo/browser RCE reaches long-lived secrets | Critical | broker capability separation + platform sandbox defense in depth |
| proxy failure escapes direct | Critical | negative tests + immutable proxy contract |
| Servo misses required site capability | High | capability suite + explicit compatibility decision |
| Servo dependency graph introduces advisory/license/native-build blockers | High | exact lockfile + cargo-deny + exact-SHA build gate |
| Android lifecycle/process model breaks assumptions | High | early Android probe + idempotent broker/browser lifecycle |
| primary transport blocked | High | independent fallback + two relays |
| hardware key support varies | High | algorithm-agile DeviceSigner |

# 8. Pareto Improvements

1. Preserve portable contracts and broker/browser privilege separation before secrets exist.
2. Prove exact Servo embedding and normal fail-closed networking before real transport work.
3. Implement narrow broker capability API before device/session/transport credentials.
4. Validate Android lifecycle/isolation early.
5. Keep the Servo dependency graph continuously locked and security-scanned.

# 9. Dependency DAG

```text
T-001 → T-002 → T-003 → T-018(plan) → T-004 → T-005 → T-019 → T-006
                                                   │          │
                                                   └──→ T-007 │
T-005 + T-006 + T-007 + T-019 → T-008 → T-012 → T-013
T-019 → T-009 → T-010 → T-011
T-004 + T-005 → T-014
T-010 + T-011 + T-013 + T-014 → T-015 → T-016
T-017 governance runs independently while blocked
```

# 10. Implementation Phases

- **A Foundation:** T-001..T-003 plus T-018 architecture reconciliation.
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
Exact corrected SHA passed verify + cargo-deny before `main` fast-forward.

## T-018 — Reconcile Servo sandbox gap into the trust architecture
**Status:** DONE · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH  
F-011 recorded; ADR-0003 accepted; exact candidate passed existing CI and reached `main`.

## T-004 — Pin Servo and build minimal embedding adapter
**Status:** VERIFYING · **Priority:** P0 · **Type:** IMPROVE · **Leverage:** HIGH

### Problem
The browser boundary existed only as a project-owned trait; Servo was still conceptual rather than a pinned, compile-verified dependency.

### Evidence
F-005 and the current Servo 0.5.0 public embedding API (`ServoBuilder`, `Servo::spin_event_loop`).

### Goal
Accept exactly one reviewed Servo release behind a dedicated adapter without contaminating portable browser/core contracts.

### Scope
- add `webgate-browser-servo`;
- pin `servo = "=0.5.0"`;
- commit the generated lock graph;
- prove minimal builder/handle/event-loop API integration;
- update architecture dependency policy;
- keep temporary lock-generation workflow out of the final candidate.

### Non-goals
No real window/rendering surface, navigation, protected proxy, broker IPC, device secrets or production session handling.

### Implementation
`ServoBrowser` privately owns `Option<servo::Servo>`, constructs via `ServoBuilder::default().build()`, exposes only the existing `ProtectedBrowser` contract, and delegates event-loop pumping without exposing Servo types.

### Invariants
I-001, I-018, I-020, I-021.

### Compatibility constraints
Rust floor remains compatible with Servo 0.5.0. Servo-specific native/build prerequisites discovered by CI must become Findings before any workaround.

### Edge cases
Adapter type existence without runtime window, idempotent shutdown representation, no Servo dependency edge from portable core/browser contracts.

### Tests
Compile-time trait conformance plus full locked workspace CI and dependency policy.

### Mutation tests
N/A: this task contains adapter wiring rather than security policy logic.

### Acceptance criteria
Architecture check, locked metadata, format, workspace check/test/clippy and cargo-deny all PASS on the exact clean candidate SHA; no temporary lock-generation workflow exists in the candidate tree.

### Dependencies
T-018.

### Blocks
T-005, T-014.

### Risk
High dependency/native-build risk; low application behavior risk.

### Rollback
Revert the single T-004 main commit.

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
Validate Servo, proxy, pause/resume/recreate, broker lifecycle and Android isolation options without desktop-only core assumptions.

## T-007 — Strict navigation/deep-link policy
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN  
Pure property/fuzz/mutation-tested URL/origin policy.

## T-008 — Transport SPI + deterministic health/failover state machine
**Status:** TODO · **Priority:** P1

## T-009 — Algorithm-agile device identity
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN

## T-010 — Platform secret/device adapters
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN

## T-011 — SecureAcces control-plane integration
**Status:** TODO · **Priority:** P1  
Authorization remains server authoritative; broker obtains only necessary session capabilities.

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

1. pure core/architecture checks;
2. Servo API/compatibility tests;
3. normal network fail-closed tests;
4. browser-compromise/broker-capability negative tests;
5. Android/platform lifecycle/isolation tests;
6. transport chaos;
7. SecureAcces integration;
8. end-to-end trusted-link flow.

Analyze `input × state × concurrency × timing × failure × permissions × configuration × external state` using boundary, pairwise/high-risk N-wise, fuzz/property/metamorphic tests.

# 13. Mutation Testing Strategy

Mandatory for URL/origin policy, fail-closed decisions, broker authorization, IPC validation, transport transitions, signed policy, device proof and auth adapters. Planned tool: `cargo-mutants`; surviving mutants require observable-contract analysis.

# 14. Performance Baselines

Measure process→shell, Servo ready, broker ready, proxy/transport ready, link→first paint, warm navigation, RSS/CPU, frame stability, reconnect, IPC overhead and Android cold/warm/battery-sensitive recovery. Process isolation performance cost is measured rather than assumed.

# 15. Security Hardening

Browser treated as potentially compromised; long-lived secrets stay behind broker/platform signer; no reusable bootstrap key; signed policy/update; destination-restricted proxy; no direct fallback; semantic minimal IPC; secret-redacted logs; per-device revocation; hardware-backed identity where available; locked dependencies; server-side authz per resource.

# 16. Migration Strategy

`characterize → introduce boundary → dual compatibility if needed → migrate → verify → remove legacy`. Servo remains primary. Future native Servo sandbox improvements strengthen defense in depth but do not remove the broker capability boundary without a new ADR/threat-model proof.

# 17. Deferred Work

- iOS pending platform policy/Servo maturity;
- full-device VPN;
- arbitrary general browsing;
- enterprise fleet/MDM;
- distributed auth infrastructure beyond actual scale.

# 18. Rejected Decisions

System-wide VPN default; secret bearer links; silent browser fallback; DPAPI/Win32 in core; shared user VPN keys; transport-layer authorization; weakening dependency policy; treating Rust memory safety as a substitute for browser privilege isolation; tracking Servo `main` or floating releases in production.

# 19. Completed Tasks

T-001, T-002, T-003 and T-018 are complete after verified `main` pushes. Research/ADRs cover Servo, SecureAcces, cross-platform/Android, resilience and target topology.

# 20. Iteration Log

## Iteration 1
**Task:** T-001 · **Result:** PASS  
**Commit:** `docs(plan): establish execution-grade living master plan` → main.

## Iteration 2
**Task:** T-002 · **Unexpected:** F-007 · **Result:** PASS  
Portable workspace; exact candidate format/check/test/clippy PASS; pushed main.

## Iteration 3
**Task:** T-003 · **Unexpected:** F-008/F-009/F-010 · **Result:** PASS  
First candidate correctly failed cargo-deny and never reached main. Corrected SHA `acae35585b00b88f854dbfacd699db34ebabaff4` passed verify + dependency-policy and was fast-forwarded to main.

## Iteration 4
**Task:** T-018  
**Finding addressed:** F-011  
**Changes:** Servo compromise-containment model; trusted broker boundary; revised DAG.  
**Tests:** verify + dependency-policy PASS on exact candidate.  
**Commit:** `docs(security): define Servo compromise boundary`  
**Push:** main  
**Result:** PASS

## Iteration 5
**Task:** T-004  
**Changes:** exact Servo 0.5.0 pin, dedicated adapter crate, generated lock graph, architecture-edge update.  
**Tests:** architecture + locked metadata + fmt/check/test/clippy + cargo-deny on clean candidate.  
**Plan changes:** added I-020/I-021; temporary lock-generator explicitly excluded from final tree.  
**Commit target:** `feat(browser): pin Servo behind adapter boundary`  
**Push:** main only after exact-SHA PASS  
**Result:** VERIFYING

# 21. Definition of Final Done

No unresolved Critical/High release findings; all release P0/P1 complete or evidence-based rejected; normal protected networking cannot escape direct; browser compromise cannot extract long-lived keys or escalate broker capabilities in the supported threat model; Servo required site capabilities pass; Windows/Android paths are proven; SecureAcces revocation works end-to-end; independent transports survive chaos; critical policy/IPC/state logic is mutation-resistant; build/lint/security gates pass; performance targets pass without security weakening; signed release/update flow verified; docs match code; final re-audit finds no fundamental blocker; verified final state is in `main`.
