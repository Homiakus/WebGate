# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Research baseline:** 2026-08-29  
**Last verified implementation state:** `d0c8199756fd204caa335f59a83e41a4787c7bc8`  
**Canonical browser:** Servo primary; compatibility engines explicit-only.

`MASTER_PLAN.md` is the single execution source of truth. Material new evidence must become a Finding before scope or ordering changes. An implementation task is DONE only after its acceptance checks pass and the verified state reaches `main` without force push.

---

# 1. Mission

Build WebGate as a secure, resilient, cross-platform protected-browser client for a small trusted-user set.

```text
trusted Telegram / HTTPS link
        ↓
      WebGate
        ↓
 Servo browser capsule
        ↓
 application-local fail-closed network path
        ↓
 replaceable resilient transports
        ↓
 Relay A / Relay B
        ↓
 private origin
        ↓
 SecureAcces authorization
```

Primary targets: Windows and Android first; Linux and macOS follow the same portable contracts.

---

# 2. Current State

The repository now has:

- a portable Rust workspace with `webgate-core`, `webgate-browser`, `webgate-transport`, `webgate-platform`, and `webgate-app`;
- compile-time/lint policy with `unsafe_code = forbid`;
- a committed lockfile and cargo-deny dependency policy;
- machine-enforced internal crate dependency direction;
- exact-SHA GitHub Actions verification before `main` fast-forward;
- a cross-platform developer project manager with interactive menu and scriptable commands;
- controlled bootstrap for missing developer tools and empirically confirmed Servo native prerequisites;
- dedicated Windows PowerShell and POSIX launchers;
- project-manager tests integrated into CI;
- architecture ADRs for Servo primary selection, cross-platform runtime, and Servo compromise containment.

No production Servo adapter, fail-closed proxy implementation, real VPN/relay transport, device-key adapter, or SecureAcces control API exists yet.

Servo is treated as potentially compromised. Long-lived secrets and privileged transport/authentication authority must stay behind a separate trusted-broker capability boundary.

---

# 3. Architecture Map

```text
UNTRUSTED CONTENT
      │
      ▼
┌──────────────────────────────────────────────┐
│ Browser capsule — assume compromise         │
│ Servo + document/page/render/input state     │
│ short-lived bounded web capability only     │
└───────────────────┬──────────────────────────┘
                    │ narrow semantic IPC
                    ▼
┌──────────────────────────────────────────────┐
│ Trusted broker / control plane               │
│ policy verification                          │
│ device signer                                │
│ session issuance / refresh authority         │
│ transport control                            │
│ update trust roots / privileged audit        │
└───────────────────┬──────────────────────────┘
                    │
          destination-restricted proxy
                    │
          replaceable secure transports
                    │
               Relay A / Relay B
                    │
                private origin
                    │
              SecureAcces authz
```

Portable crates own contracts. Servo, operating-system APIs, concrete secret stores, and concrete transports stay in outward adapters.

Developer tooling is separate from runtime trust: `scripts/project_manager.py` may bootstrap build tools but has no runtime credential role.

---

# 4. Baseline

Current required verification:

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
```

GitHub Actions jobs:

- `verify` — project-manager tests, command contract, architecture, lockfile, format, build/check, tests, Clippy;
- `dependency-policy` — cargo-deny advisories/licenses/bans/sources.

T-020 exact implementation SHA `2ad6f51053cb360f35b3942645cf75984e3b5282` passed both jobs. The README exposure follow-up SHA `d0c8199756fd204caa335f59a83e41a4787c7bc8` also passed both jobs before `main` fast-forward.

The assistant execution container itself does not provide the full Rust/native toolchain, so exact-commit GitHub Actions verification remains the authoritative proof path.

---

# 5. System Invariants

- **I-001:** Servo is the default protected browser engine.
- **I-002:** normal WebGate mode does not change the OS default route.
- **I-003:** transport loss fails closed; protected traffic never silently falls back to direct Internet.
- **I-004:** browser failure never silently switches to WebView2/system browser.
- **I-005:** links identify resources; they are not persistent bearer credentials.
- **I-006:** device private keys are generated on-device and never exported into browser APIs/config bundles.
- **I-007:** bootstrap, policy, and update artifacts are signed and rollback-aware.
- **I-008:** remote policy may tighten but cannot weaken compiled hard security invariants.
- **I-009:** transport implementations remain replaceable behind stable application contracts.
- **I-010:** SecureAcces remains authoritative for account/session/workspace/resource authorization.
- **I-011:** shared core has no Win32/DPAPI/desktop-sidecar assumption; Android is first-class.
- **I-012:** device signing/secret storage is a platform capability; hardware-backed keys are preferred.
- **I-013:** production has at least two materially independent network failure domains.
- **I-014:** browser-facing proxy endpoints are loopback-only and already bound to a non-zero port.
- **I-015:** internal crate dependency direction is machine-enforced.
- **I-016:** CI code-execution actions use immutable commit SHA pins.
- **I-017:** internal path dependencies carry explicit compatible versions; wildcard dependency policy remains denied.
- **I-018:** the Servo/browser capsule is not trusted with long-lived device/bootstrap/transport/session-refresh secrets or generic privileged native APIs.
- **I-019:** normal network fail-closed and browser-compromise containment are different properties and need different tests.
- **I-020:** developer bootstrap is allowlisted/reviewable, does not accept arbitrary package names or shell commands, and never silently mutates WebGate source/runtime credentials.

---

# 6. Findings Registry

## F-001 — Repository had no executable baseline

**Status:** Resolved by T-002  
**Category:** Delivery / Architecture  
**Severity:** High  
**Confidence:** Confirmed

Documentation-first state prevented code-level verification. T-002 created the portable Rust workspace and executable baseline.

## F-002 — Original roadmap was not execution-grade

**Status:** Resolved by T-001  
**Category:** Process  
**Severity:** High  
**Confidence:** Confirmed

The plan now carries findings, invariants, dependency ordering, atomic tasks, verification, and iteration history.

## F-003 — Desktop-only assumptions remained in early runtime design

**Status:** Planned  
**Category:** Cross-platform  
**Severity:** High  
**Confidence:** Confirmed

**Impact:** Android could otherwise require a core rewrite.  
**Affected tasks:** T-006, T-009, T-010, T-019.  
**Direction:** OS-specific APIs remain adapters; shared logic uses capability contracts only.

## F-004 — Fixed Ed25519 device identity is not universally hardware-backed

**Status:** Planned  
**Category:** Security / Cross-platform  
**Severity:** High  
**Confidence:** Strong

Use algorithm-agile `DeviceSigner`; prefer hardware-backed P-256/ES256 where appropriate. Keep policy/update-signing keys separate.

## F-005 — Servo compatibility/security is release-sensitive

**Status:** Planned  
**Category:** Browser / Supply chain  
**Severity:** High  
**Confidence:** Confirmed

Servo is pre-1.0 and fast-moving. Exact pin, upgrade review, site compatibility tests, and security qualification are mandatory.

## F-006 — Architecture boundaries were documentation-only

**Status:** Resolved by T-002/T-003  
**Category:** Architecture / Testing  
**Severity:** Medium  
**Confidence:** Confirmed

Crate boundaries and internal dependency DAG are now machine-enforced.

## F-007 — Local execution environment can lack Rust/native build tools

**Status:** Mitigated by T-020  
**Category:** Developer Experience / Build  
**Severity:** Medium  
**Confidence:** Confirmed

The project manager now provides `doctor`, controlled `install`, `verify`, `build`, `test`, `security`, `servo`, and `android` commands. GitHub Actions remains final exact-SHA authority.

## F-008 — `main` lacks repository-enforced branch protection

**Status:** Planned / BLOCKED on connector capability  
**Category:** Governance  
**Severity:** Medium  
**Confidence:** Confirmed

Current process mitigates with isolated branch verification and non-force fast-forward. T-017 remains blocked until a branch-rules write capability is available.

## F-009 — cargo-deny rejected versionless internal path dependencies

**Status:** Resolved by T-003  
**Category:** Dependency policy  
**Severity:** Medium  
**Confidence:** Confirmed

Policy remained strict; internal path dependencies now carry explicit versions.

## F-010 — checkout v4 used legacy Node 20 runtime

**Status:** Resolved by T-003  
**Category:** CI / Supply chain  
**Severity:** Low  
**Confidence:** Confirmed

CI now pins checkout v5 by immutable SHA.

## F-011 — Servo is not a sufficient renderer sandbox on Windows/Android

**Status:** Planned containment work  
**Category:** Browser / Security  
**Severity:** High  
**Confidence:** Confirmed

**Root cause:** Rust memory safety is not equivalent to a mature renderer sandbox.  
**Impact:** browser-engine compromise must not share long-lived secrets/transport authority.  
**Resolution direction:** Servo stays primary, but trusted broker/capability separation is mandatory; platform sandboxing is defense in depth.  
**Affected tasks:** T-004, T-005, T-006, T-009..T-012, T-019.

## F-012 — Servo Linux build has an explicit native fontconfig prerequisite

**Status:** Mitigated by T-020; verification remains in T-004  
**Category:** Build / Portability  
**Severity:** High  
**Confidence:** Confirmed

**Evidence:** the first clean Servo 0.5.0 candidate reached `yeslogic-fontconfig-sys 6.0.1` and failed because `pkg-config` could not locate `fontconfig.pc` on Ubuntu. Architecture/lock/format checks had already passed.  
**Root cause:** native Servo prerequisites were hidden host state rather than an explicit project environment contract.  
**T-020 result:** project manager now detects this condition and offers allowlisted package-manager remediation (`pkg-config` plus the platform fontconfig development package).  
**Remaining proof:** T-004 must rerun the exact Servo graph with prerequisites installed. Any next missing native dependency becomes a new Finding; do not blindly add Servo's full upstream package list.

---

# 7. Risk Register

| Risk | Impact | Current mitigation |
|---|---|---|
| Servo/browser RCE reaches long-lived secrets | Critical | trusted broker + platform sandbox defense in depth |
| proxy/transport failure escapes direct | Critical | T-005 negative network-escape tests |
| hidden native build state | High | project-manager doctor/bootstrap + findings-driven prerequisites |
| Servo misses required site capability | High | capability/visual/site contract in T-014 |
| Android lifecycle breaks desktop assumptions | High | early T-006 probe |
| primary protocol/provider is blocked | High | independent fallback + dual relays |
| dependency/security drift | High | lockfile, cargo-deny, immutable CI pins, exact Servo pin |
| hardware key support varies | High | algorithm-agile `DeviceSigner` |

---

# 8. Pareto Improvements

1. Keep portable contracts and browser/broker privilege separation before secrets exist.
2. Keep environment/setup reproducible through the T-020 project manager.
3. Prove the exact Servo adapter/build before implementing networking.
4. Prove fail-closed browser networking before real VPN transports.
5. Validate Android lifecycle/isolation before desktop patterns harden.
6. Add real primary/fallback transports only after browser isolation is proven.

---

# 9. Dependency DAG

```text
T-001 → T-002 → T-003 → T-018 → T-020 → T-004 → T-005 → T-019 → T-006
                                                     │          │
                                                     └──→ T-007 │

T-005 + T-006 + T-007 + T-019 → T-008 → T-012 → T-013
T-019 → T-009 → T-010 → T-011
T-004 + T-005 → T-014
T-010 + T-011 + T-013 + T-014 → T-015 → T-016

T-017 runs independently when repository-settings write capability exists.
```

**Next selected task:** T-004.

---

# 10. Implementation Phases

- **A — Executable foundation:** T-001, T-002, T-003, T-018, T-020 — DONE.
- **B — Servo capsule and containment:** T-004, T-005, T-019, T-006, T-007.
- **C — Portable transport/device contracts:** T-008, T-009, T-010.
- **D — SecureAcces and production transports:** T-011, T-012, T-013.
- **E — Qualification/release/re-audit:** T-014, T-015, T-016.
- **Governance parallel:** T-017.

---

# 11. Atomic Tasks

## T-001 — Establish execution-grade living plan
**Status:** DONE · **Priority:** P0 · **Leverage:** HIGH

## T-002 — Scaffold portable Rust boundaries
**Status:** DONE · **Priority:** P0 · **Leverage:** HIGH

## T-003 — Harden CI/dependency/architecture gates
**Status:** DONE · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

## T-018 — Reconcile Servo sandbox gap into trust architecture
**Status:** DONE · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

## T-020 — Add cross-platform project manager and controlled prerequisite bootstrap

**Status:** DONE  
**Priority:** P0  
**Type:** IMPROVE / HARDEN  
**Leverage:** HIGH

### Problem
Developer setup, diagnosis, compilation and local verification were fragmented. F-012 proved native prerequisites can remain hidden until deep inside a Servo build.

### Goal
One menu/scriptable entry point for checks, controlled missing-tool bootstrap, compilation, CI-parity verification, Servo native diagnostics, and Android diagnostics.

### Scope
- `scripts/project_manager.py`;
- `scripts/webgate.ps1`;
- `scripts/webgate.sh`;
- `scripts/tests/test_project_manager.py`;
- project-manager developer documentation;
- CI manager tests/command-contract gate;
- README discovery surface;
- plan/F-012 reconciliation.

### Non-goals
No Servo dependency integration; no arbitrary package installer; no Android SDK license acceptance; no runtime credential management; no Git push command inside the manager.

### Implementation
- Python standard-library manager with menu plus subcommands;
- `doctor`, `install`, `verify`, `build`, `test`, `security`, `servo`, `android`, `clean`;
- official rustup HTTPS bootstrap only;
- system package-manager allowlist for Git and empirically confirmed native prerequisites;
- `cargo install --locked` for cargo-deny and optional cargo-mutants;
- no `shell=True` and no user-supplied package names;
- `install --dry-run` for inspection;
- local verification mirrors repository CI.

### Edge cases covered
Windows x64/ARM64 rustup URL; unsupported Linux package manager; missing Git/Rust/cargo-deny; missing `pkg-config`/`fontconfig.pc`; JSON doctor output; non-interactive dry-run; Android SDK absent; destructive-clean confirmation.

### Tests
Project-manager unit tests plus dry-run command-contract test are CI-gated. Existing architecture/lock/fmt/check/test/clippy/cargo-deny gates also pass on the final implementation state.

### Acceptance result
PASS on exact SHA `d0c8199756fd204caa335f59a83e41a4787c7bc8`; pushed to `main` by non-force fast-forward.

### Rollback
Revert T-020 implementation/documentation commits; no runtime/data migration.

## T-004 — Pin Servo and build minimal embedding adapter

**Status:** READY  
**Priority:** P0  
**Type:** IMPROVE / HARDEN  
**Leverage:** HIGH

### Problem
Servo is architecturally selected but not present in production code. The first 0.5.0 experiment exposed F-012 and did not reach `main`.

### Goal
Pin a reviewed exact Servo release, isolate all Servo types in a dedicated adapter crate, prove the builder/event-loop/rendering-context integration at compile time, and make native prerequisites explicit.

### Scope
Servo adapter crate, exact dependency/lock changes, CI native prerequisite step(s), architecture-policy update, focused compile tests, plan reconciliation.

### Non-goals
No production proxy, real transport, credentials, or browser-broker IPC yet.

### Acceptance criteria
- exact Servo version, never upstream `main`;
- all Servo-specific types remain outside portable contracts;
- Linux clean CI installs only evidence-backed prerequisites;
- architecture/lock/fmt/check/test/clippy/cargo-deny all pass;
- any additional native/supply-chain failure is recorded through Unexpected Finding Protocol before remediation.

### Dependencies
T-020.

## T-005 — Prove fail-closed Servo normal networking
**Status:** TODO · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

Prove positive proxy path and negative direct-IP/DNS/redirect/IPv4/IPv6/subresource/restart paths. This proves normal browser-stack network isolation, not arbitrary-code-execution containment.

## T-019 — Implement trusted broker capability boundary
**Status:** TODO · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

Browser side receives no raw device/bootstrap/transport-control secret. IPC must be versioned, bounded, instance-bound, deny-by-default, and semantic rather than generic native execution.

## T-006 — Early Android lifecycle/embedding/isolation probe
**Status:** TODO · **Priority:** P0 · **Leverage:** HIGH

Validate Servo, proxy, pause/resume/recreate, broker lifecycle, Android isolation choices, and absence of desktop-only core assumptions.

## T-007 — Implement strict navigation/deep-link policy
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

Pure policy with property/fuzz/mutation tests for schemes, Unicode/IDN, origin matching, redirects, opaque IDs, and external-browser policy.

## T-008 — Implement transport SPI and deterministic failover state machine
**Status:** TODO · **Priority:** P1 · **Leverage:** HIGH

## T-009 — Introduce algorithm-agile device identity
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

## T-010 — Implement platform secret/device adapters
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

Windows CNG/TPM where possible; Android Keystore; macOS Keychain/Secure Enclave where applicable; explicit Linux secure-storage policy.

## T-011 — Integrate SecureAcces control plane
**Status:** TODO · **Priority:** P1 · **Leverage:** HIGH

Reuse SecureAcces session/resource authorization/revocation; never duplicate tenant authority in WebGate.

## T-012 — Implement primary resilient transport
**Status:** TODO · **Priority:** P1 · **Leverage:** HIGH

Candidate Outline SDK/MobileProxy + AmneziaWG-class transport behind the restricted browser-facing contract.

## T-013 — Add independent fallback and dual-relay failover
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

Fallback must differ materially in protocol/implementation/failure mode from the primary.

## T-014 — Qualify Servo/site compatibility, security, and performance
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

## T-015 — Implement signed packaging, updates, and one-click link UX
**Status:** TODO · **Priority:** P2 · **Type:** HARDEN · **Leverage:** MEDIUM

## T-016 — Final adversarial re-audit and debt deletion
**Status:** TODO · **Priority:** P0 before release · **Type:** HARDEN · **Leverage:** HIGH

## T-017 — Enforce verified-main repository rule
**Status:** BLOCKED · **Priority:** P2 · **Type:** HARDEN

Blocker: current connector can read branch protection but does not expose a compatible write action. Continue all independent implementation work.

---

# 12. Testing Strategy

Testing layers:

1. developer/bootstrap unit and dry-run command-contract tests;
2. architecture and dependency gates;
3. Servo adapter compile/integration tests;
4. browser network-escape negative tests;
5. browser-compromise/broker-capability tests;
6. Android/platform lifecycle/isolation tests;
7. transport chaos/failover tests;
8. SecureAcces integration tests;
9. end-to-end trusted-link → protected-document tests.

Critical logic uses the multidimensional model:

`input × state × concurrency × timing × failure × permissions × configuration × external state`.

Use boundary partitions, pairwise/high-risk N-wise, fuzzing, property tests, and metamorphic tests where appropriate.

---

# 13. Mutation Testing Strategy

Mandatory for:

- URL/origin/deep-link policy;
- fail-closed decisions;
- broker authorization/IPC validation;
- transport state transitions;
- signed policy/config validation;
- device proof verification;
- SecureAcces adapters.

Planned Rust tool: `cargo-mutants`. T-020 can bootstrap it explicitly, but mutation testing becomes a mandatory gate only when relevant critical logic exists.

---

# 14. Performance Baselines

After Servo/proxy exists, measure:

- process → shell ready;
- Servo ready;
- broker ready;
- proxy/transport ready;
- trusted link → first paint;
- warm navigation;
- idle/active RSS and CPU;
- frame/scroll stability;
- reconnect/failover time;
- broker IPC overhead;
- Android cold/warm start and battery-sensitive recovery.

No performance optimization may weaken fail-closed or privilege-separation invariants.

---

# 15. Security Hardening

- treat browser capsule as compromise-prone;
- keep long-lived secrets behind broker/platform signer;
- no reusable private key in bootstrap bundles;
- signed/versioned configuration/policy/update formats;
- destination-restricted local proxy;
- no direct protected-origin fallback;
- no generic page→native bridge;
- per-device revocation;
- hardware-backed identity where available;
- secret-redacted logs/crash diagnostics;
- locked dependency graph and reviewed updates;
- server authorization on every protected resource;
- developer bootstrap separated from runtime credentials.

---

# 16. Migration Strategy

For major changes:

`characterize → introduce boundary → dual compatibility if needed → migrate callers → verify → remove legacy`.

Servo remains primary. Future Servo-native sandbox improvements are defense in depth and do not automatically remove the trusted-broker boundary.

---

# 17. Deferred Work

- iOS until platform policy and Servo maturity are reevaluated;
- general-purpose full-device VPN mode;
- arbitrary general web browsing;
- enterprise fleet/MDM management;
- distributed authorization infrastructure beyond demonstrated scale;
- automatic Android SDK installation/license acceptance before T-006 fixes exact versions/reproducibility requirements.

---

# 18. Rejected Decisions

- system-wide VPN as default;
- bearer-secret document links;
- silent browser-engine fallback;
- Win32/DPAPI types in portable core;
- shared user VPN keys;
- authorization in relay/VPN layer;
- weakening dependency policy to make CI green;
- treating Rust memory safety as a renderer sandbox;
- general-purpose arbitrary package installation in the project manager;
- hiding Servo prerequisite failures by installing an unreviewed broad package list.

---

# 19. Completed Tasks

- T-001 — living execution plan.
- T-002 — portable Rust workspace and first executable baseline.
- T-003 — lock/dependency/security/architecture CI gates.
- T-018 — Servo compromise-containment architecture.
- T-020 — cross-platform project manager, controlled bootstrap, build/verify menu and docs.

---

# 20. Iteration Log

## Iteration 1
**Task:** T-001  
**Result:** PASS  
**Push:** main.

## Iteration 2
**Task:** T-002  
**Unexpected:** F-007  
**Result:** PASS  
**Push:** main.

## Iteration 3
**Task:** T-003  
**Unexpected:** F-008, F-009, F-010  
**Result:** PASS  
**Evidence:** corrected exact SHA passed both verify and dependency-policy jobs before main fast-forward.

## Iteration 4
**Task:** T-018  
**Finding addressed:** F-011  
**Result:** PASS  
**Push:** `b7b5d42bcbf4006a3bc6fe7c3fbf12d1a043bebb` → main.

## Iteration 5
**Task:** T-020  
**Findings addressed:** F-012; mitigates F-007 developer setup friction.  
**Changes:** project manager, controlled installer, menu/CLI, build/test/security/verify commands, Servo/Android doctors, Windows/POSIX launchers, tests, CI gate, docs/README.  
**Tests:** project-manager unit tests; dry-run command contract; architecture; lockfile; fmt; check; test; clippy; cargo-deny.  
**Verification:** exact SHA `d0c8199756fd204caa335f59a83e41a4787c7bc8` — both `verify` and `dependency-policy` PASS.  
**Push:** non-force fast-forward to `main`.  
**Plan changes:** T-020 DONE; T-004 becomes next READY task; F-012 remains an explicit T-004 build qualification condition.  
**Result:** PASS

---

# 21. Definition of Final Done

- no unresolved Critical/High release findings;
- all release P0/P1 tasks DONE or evidence-based REJECTED/explicitly DEFERRED;
- protected browser normal networking cannot escape direct under tested failures;
- browser compromise cannot extract long-lived device keys or escalate broker capabilities in the supported threat model;
- clean supported-platform builds have explicit reproducible prerequisites;
- Servo required site capabilities pass;
- Windows and Android runtime paths are proven; Linux/macOS meet declared support gates;
- SecureAcces revocation works end-to-end;
- primary and independent fallback transports survive chaos tests;
- critical policy/parser/state/authorization logic is mutation-resistant;
- format/build/test/lint/security/static checks pass;
- performance targets pass without security regression;
- signed packaging/update flow is verified;
- documentation matches code;
- final re-audit finds no new fundamental blocker;
- final verified state and synchronized MASTER_PLAN are in `main`.
