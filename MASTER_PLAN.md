# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Research baseline:** 2026-08-29  
**Last verified implementation state:** `d0c8199756fd204caa335f59a83e41a4787c7bc8`  
**Canonical browser:** Servo primary; compatibility engines explicit-only.  
**Server direction:** Go-first WebGate Server Gateway + SecureAcces authoritative authorization.

`MASTER_PLAN.md` is the single execution source of truth. Material new evidence must become a Finding before scope or ordering changes. An implementation task is DONE only after its acceptance checks pass and the verified state reaches `main` without force push.

---

# 1. Mission

Build WebGate as a secure, resilient, cross-platform protected-access platform for a small trusted-user set, with both a protected browser client and a centrally administered server-side gateway for multiple private services.

```text
trusted Telegram / HTTPS link
        ↓
      WebGate Client
        ↓
 Servo browser capsule
        ↓
 application-local fail-closed network path
        ↓
 replaceable resilient transports
        ↓
 Relay A / Relay B
        ↓
 WebGate Server Gateway
        ↓
 SecureAcces authentication / authorization
        ↓
 authoritative Service Registry
        ↓
 ┌───────────┬───────────┬────────────┬────────────┐
 │ Docs      │ FactoryOS │ Files      │ Monitoring │ ...
 └───────────┴───────────┴────────────┴────────────┘
```

The administrator experience is a first-class product surface:

```text
Admin
  ↓
WebGate Admin UI / API
  ├── Users
  ├── Protected Services
  ├── Access Matrix (User × Service)
  ├── Devices
  ├── Sessions
  ├── Audit
  └── Service Health
```

Primary client targets: Windows and Android first; Linux and macOS follow the same portable contracts. The server control plane is designed for a private origin that may host multiple local web services and may itself have dynamic public IP / CGNAT.

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
- architecture ADRs for Servo primary selection, cross-platform runtime, and Servo compromise containment;
- a documented SecureAcces integration boundary in which server-side authorization remains authoritative.

No production Servo adapter, fail-closed proxy implementation, real VPN/relay transport, device-key adapter, WebGate Server Gateway, ProtectedService registry, Admin API, Admin UI, or SecureAcces control-plane integration exists in WebGate yet.

SecureAcces already supplies the core account/user/workspace/membership/session/permission management primitives. WebGate must reuse them rather than create a parallel RBAC database.

Servo is treated as potentially compromised. Long-lived secrets and privileged transport/authentication authority must stay behind a separate trusted-broker capability boundary.

---

# 3. Target Architecture

```text
UNTRUSTED CONTENT / USER DEVICE
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
│ Trusted client broker                       │
│ policy verification                         │
│ device signer                               │
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
                    ▼
┌──────────────────────────────────────────────────────────────────┐
│ PRIVATE ORIGIN / SERVER                                          │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ WebGate Server Gateway / Control Plane                     │  │
│  │                                                            │  │
│  │  Admin API / UI                                            │  │
│  │       │                                                    │  │
│  │       ├── ProtectedService Registry                        │  │
│  │       ├── Device Registry                                  │  │
│  │       ├── Audit / Health                                   │  │
│  │       └── SecureAcces management adapter                   │  │
│  │                                                            │  │
│  │ Request path:                                              │  │
│  │ authenticate → resolve service/resource → authorize → proxy│  │
│  └───────────────────────┬────────────────────────────────────┘  │
│                          │                                       │
│        ┌─────────────────┼──────────────────┐                    │
│        ▼                 ▼                  ▼                    │
│  Docs 127.0.0.1     FactoryOS local    Monitoring local          │
│  / private LAN      / private LAN      / private LAN             │
└──────────────────────────────────────────────────────────────────┘
```

Portable client crates own contracts. Servo, operating-system APIs, concrete secret stores, and concrete transports stay in outward adapters.

The WebGate Server Gateway is preferably Go-first so it can embed/use SecureAcces natively without duplicating its domain model over a Rust boundary. A narrow versioned HTTP/wire contract is exposed to WebGate clients; SecureAcces internal structs are not a public client protocol.

Developer tooling is separate from runtime trust: `scripts/project_manager.py` may bootstrap build tools but has no runtime credential role.

---

# 4. Server Domain Model

## 4.1 ProtectedService is a first-class server entity

A physical/private origin may host many independently authorized services. WebGate therefore requires an authoritative registry rather than treating the entire server as one undifferentiated protected origin.

Conceptual Go model:

```go
type ProtectedService struct {
    ID          ServiceID
    TenantID    secureaccess.ID
    WorkspaceID secureaccess.ID

    Name        string
    Slug        string
    Description string

    Upstream    UpstreamRef
    PublicPath  string
    Status      ServiceStatus

    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

`UpstreamRef` is server-owned configuration. It is never accepted from a browser/client request. The server resolves it only after selecting an already-registered service.

Typical registry:

```text
svc-docs       → workspace-docs       → http://127.0.0.1:8081
svc-factory    → workspace-factory    → http://127.0.0.1:8082
svc-files      → workspace-files      → http://127.0.0.1:8083
svc-monitoring → workspace-monitoring → http://127.0.0.1:8084
```

## 4.2 SecureAcces remains the authorization authority

The access matrix is a projection of SecureAcces state, not a second permissions database.

```text
ProtectedService
      │
      └── authoritative WorkspaceID
                       │
User / Account ── Membership ── PermissionBits
                       │
                       ▼
                 ALLOW / DENY
```

Initial permission mapping remains:

| Service action | SecureAcces permission |
|---|---|
| open/view | `PermView` |
| download/export | `PermDownload` |
| upload | `PermUpload` |
| edit/write | `PermEdit` |
| delete | `PermDelete` |
| manage service members | `PermManageMembers` |
| manage service/workspace | `PermManageWorkspace` |

A future service-specific permission may be introduced only when an actual requirement cannot be represented safely with the existing bits.

## 4.3 Request authorization sequence

```text
GET /apps/factory/orders/123
              │
              ▼
WebGate Server Gateway
              │
              ├── authenticate session/device context
              ├── resolve route → registered ProtectedService
              ├── resolve authoritative resource metadata
              ├── derive TenantID/WorkspaceID from server state
              ├── SecureAcces.Authorize(...)
              │
          ┌───┴────┐
          │        │
        ALLOW     DENY
          │
          ▼
proxy only to the service's registered upstream
```

The client cannot provide authoritative `TenantID`, `WorkspaceID`, permission bits, upstream host, upstream port, role, or service-to-workspace binding.

---

# 5. Admin Product Model

The administrator workflow is part of the release definition, not a deferred enterprise feature.

## 5.1 Users

Admin can:

- create/preapprove a tenant user;
- start/approve/reject enrollment;
- inspect status and identity bindings without exposing credentials;
- suspend/reactivate/revoke a user;
- inspect effective service access;
- revoke memberships;
- perform permitted session/device revocation operations;
- see security-relevant audit history.

## 5.2 Services

Admin can:

- register a protected service;
- bind it to an authoritative SecureAcces workspace;
- configure a safe upstream target from server-side policy;
- enable/disable/suspend service exposure;
- inspect health and last successful probe;
- change display metadata without changing authorization authority;
- rotate upstream configuration under audit.

## 5.3 Access Matrix

Primary small-organization UX:

```text
                 Docs   FactoryOS   Files   Monitoring
Ivan              View      —       Download     —
Sergey            Edit     View     Download    View
Anna              View     Edit        —         —
Administrator     Admin    Admin      Admin      Admin
```

Editing a matrix cell must translate into reviewed SecureAcces membership operations. The matrix itself stores no independent access grant.

Preferred cell editor:

```text
No access / View / Work / Admin
+ explicit permission toggles
+ permanent or ValidUntil
+ reason/note for audit where policy requires
```

## 5.4 Devices

Admin can inspect registered installations and transition:

```text
PENDING → ACTIVE ↔ SUSPENDED → REVOKED
```

Device revocation prevents new trusted sessions from that device and revokes associated sessions where safely identifiable. Session revocation and device revocation remain separate concepts.

## 5.5 Sessions

Admin can inspect bounded session metadata and revoke allowed sessions/accounts according to SecureAcces management rules. Raw bearer/session tokens are never visible.

## 5.6 Audit and Health

Audit covers at least:

- user created/suspended/reactivated/revoked;
- membership granted/changed/suspended/revoked;
- service registered/changed/disabled;
- service-to-workspace binding changed;
- device registered/suspended/revoked;
- session revoked;
- protected service access allow/deny where policy requires;
- privileged admin action;
- significant transport fallback/security event.

Operational health is distinct from authorization audit. Internal topology, IPs, credentials, and transport secrets are redacted from normal tenant views.

---

# 6. Baseline Verification

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

As the Go server track lands, add reproducible server gates including at minimum:

```text
go test ./...
go test -race ./...
go vet ./...
```

and security/property/mutation gates for critical server policy code.

GitHub Actions remains final exact-SHA authority when the assistant environment lacks the full native toolchain.

---

# 7. System Invariants

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
- **I-011:** shared client core has no Win32/DPAPI/desktop-sidecar assumption; Android is first-class.
- **I-012:** device signing/secret storage is a platform capability; hardware-backed keys are preferred.
- **I-013:** production has at least two materially independent network failure domains.
- **I-014:** browser-facing proxy endpoints are loopback-only and already bound to a non-zero port.
- **I-015:** internal crate dependency direction is machine-enforced.
- **I-016:** CI code-execution actions use immutable commit SHA pins.
- **I-017:** internal path dependencies carry explicit compatible versions; wildcard dependency policy remains denied.
- **I-018:** the Servo/browser capsule is not trusted with long-lived device/bootstrap/transport/session-refresh secrets or generic privileged native APIs.
- **I-019:** normal network fail-closed and browser-compromise containment are different properties and need different tests.
- **I-020:** developer bootstrap is allowlisted/reviewable, does not accept arbitrary package names or shell commands, and never silently mutates WebGate source/runtime credentials.
- **I-021:** every routable private application is represented by a server-authoritative `ProtectedService` or an explicitly equivalent immutable registry record.
- **I-022:** the service → tenant/workspace binding is server-owned; clients cannot choose or override it.
- **I-023:** the Admin access matrix is derived from SecureAcces memberships and never becomes a parallel authorization store.
- **I-024:** the gateway can proxy only to pre-registered, policy-valid upstreams; no client-controlled generic reverse-proxy target exists.
- **I-025:** protected upstream services are not intentionally exposed as public Internet endpoints; normal access enters through WebGate gateway/authorized internal paths.
- **I-026:** unknown service IDs, hosts, paths, methods, malformed routing metadata, or stale bindings fail closed.
- **I-027:** privileged admin operations require explicit management authorization and are audited; UI visibility alone never grants authority.
- **I-028:** device status and session status are separate authoritative lifecycles; revoking one cannot silently mutate the semantics of the other.
- **I-029:** raw session tokens, device private keys, transport credentials, and upstream secrets are never displayed in Admin UI or ordinary audit logs.
- **I-030:** service health/diagnostics exposed to users are least-information; internal topology and credentials remain restricted/redacted.
- **I-031:** an admin API request may name an existing object by opaque ID but cannot submit authoritative permission, tenant, workspace, or upstream facts that the server should resolve itself.
- **I-032:** changing a `ProtectedService.WorkspaceID` is a privileged, audited security operation and must not silently preserve stale cached authorization decisions.

---

# 8. Findings Registry

## F-001 — Repository had no executable baseline
**Status:** Resolved by T-002 · **Severity:** High

## F-002 — Original roadmap was not execution-grade
**Status:** Resolved by T-001 · **Severity:** High

## F-003 — Desktop-only assumptions remained in early runtime design
**Status:** Planned · **Severity:** High  
**Affected tasks:** T-006, T-009, T-010, T-019.

## F-004 — Fixed Ed25519 device identity is not universally hardware-backed
**Status:** Planned · **Severity:** High  
Use algorithm-agile `DeviceSigner`; prefer hardware-backed P-256/ES256 where appropriate.

## F-005 — Servo compatibility/security is release-sensitive
**Status:** Planned · **Severity:** High

## F-006 — Architecture boundaries were documentation-only
**Status:** Resolved by T-002/T-003 · **Severity:** Medium

## F-007 — Local execution environment can lack Rust/native build tools
**Status:** Mitigated by T-020 · **Severity:** Medium

## F-008 — `main` lacks repository-enforced branch protection
**Status:** Planned / BLOCKED on connector capability · **Severity:** Medium

## F-009 — cargo-deny rejected versionless internal path dependencies
**Status:** Resolved by T-003 · **Severity:** Medium

## F-010 — checkout v4 used legacy Node 20 runtime
**Status:** Resolved by T-003 · **Severity:** Low

## F-011 — Servo is not a sufficient renderer sandbox on Windows/Android
**Status:** Planned containment work · **Severity:** High  
**Affected tasks:** T-004, T-005, T-006, T-009..T-012, T-019.

## F-012 — Servo Linux build has an explicit native fontconfig prerequisite
**Status:** Mitigated by T-020; verification remains in T-004 · **Severity:** High

## F-013 — Multi-service private origin has no first-class service domain

**Status:** Planned  
**Category:** Server Architecture / Authorization  
**Severity:** High  
**Confidence:** Confirmed

The previous plan could authorize a workspace/resource but did not model several independent applications on one private origin. Without a `ProtectedService` registry, routing, administration, and service-level access become ad hoc.

**Resolution:** T-021.

## F-014 — WebGate has no implemented server gateway/control API

**Status:** Planned  
**Category:** Server / Product  
**Severity:** High  
**Confidence:** Confirmed

SecureAcces primitives exist, but WebGate does not yet have a server process that resolves services/resources, calls SecureAcces, manages device/bootstrap policy, and safely reverse-proxies to registered upstreams.

**Resolution:** T-011, T-022, T-023.

## F-015 — Administrator workflow is incomplete as a product surface

**Status:** Planned  
**Category:** UX / Operations  
**Severity:** High  
**Confidence:** Confirmed

There is no integrated Admin UI for Users, Services, Access Matrix, Devices, Sessions, Audit, and Health.

**Resolution:** T-024.

## F-016 — Device administration needs a first-class server registry

**Status:** Planned  
**Category:** Security / Fleet  
**Severity:** High  
**Confidence:** Confirmed

`SecureAcces.Session.DeviceID` is useful metadata but is not cryptographic device proof or a complete WebGate device lifecycle.

**Resolution:** T-009, T-010, T-025.

## F-017 — An access matrix could accidentally become a second authorization database

**Status:** Planned prevention  
**Category:** Authorization Integrity  
**Severity:** Critical  
**Confidence:** Strong

The UI must project and mutate SecureAcces memberships through management operations rather than store independent booleans/roles in WebGate.

**Resolution:** I-023, T-023, T-024, T-027.

## F-018 — Generic configurable reverse proxy would create SSRF/pivot risk

**Status:** Planned prevention  
**Category:** Server Security  
**Severity:** Critical  
**Confidence:** Strong

An admin-controlled upstream still needs validation and a bounded destination policy. A client-controlled upstream is forbidden.

**Resolution:** I-024/I-026, T-021, T-022, T-027.

## F-019 — Admin control plane is a high-value target

**Status:** Planned  
**Category:** Security / Administration  
**Severity:** Critical  
**Confidence:** Strong

Compromise of Admin API could alter memberships, services, devices, or routing. Privileged actions require explicit management authorization, strong authentication policy, CSRF/origin protection where browser-based, rate limits, immutable audit evidence where practical, and negative authorization tests.

**Resolution:** T-023, T-024, T-027.

---

# 9. Risk Register

| Risk | Impact | Planned/current mitigation |
|---|---|---|
| Servo/browser RCE reaches long-lived secrets | Critical | trusted broker + platform sandbox defense in depth |
| proxy/transport failure escapes direct | Critical | T-005 negative network-escape tests |
| access matrix diverges from real authorization | Critical | SecureAcces-only grants; matrix is projection |
| gateway becomes generic SSRF/pivot proxy | Critical | registered service-only routing + upstream validation |
| Admin API compromise changes access/routing | Critical | management authz + strong auth + audit + negative tests |
| stale service→workspace binding preserves access | Critical | authoritative lookup per request/cache invalidation/versioning |
| hidden native build state | High | project-manager doctor/bootstrap + findings-driven prerequisites |
| Servo misses required site capability | High | T-014 capability/visual/site contract |
| Android lifecycle breaks desktop assumptions | High | early T-006 probe |
| primary protocol/provider is blocked | High | independent fallback + dual relays |
| device identifier mistaken for device proof | High | T-009/T-025 proof-of-possession registry |
| service health leaks internal topology | Medium | redaction + role-scoped operational views |
| dependency/security drift | High | lockfile, cargo-deny, immutable CI pins, exact pins |

---

# 10. Pareto Improvements

1. Keep portable contracts and browser/broker privilege separation before secrets exist.
2. Keep environment/setup reproducible through the T-020 project manager.
3. Prove the exact Servo adapter/build before implementing client networking.
4. Define the server `ProtectedService`/authorization contracts before T-011 hardens an incomplete control plane.
5. Prove fail-closed browser networking before real VPN transports.
6. Build gateway authorization/routing before Admin UI so UI cannot invent security semantics.
7. Keep the access matrix as a projection of SecureAcces, never a duplicate store.
8. Validate Android lifecycle/isolation before desktop patterns harden.
9. Add real primary/fallback transports only after browser isolation is proven.
10. Run adversarial service-routing/admin authorization tests before release qualification.

---

# 11. Dependency DAG

```text
CLIENT TRACK
T-001 → T-002 → T-003 → T-018 → T-020 → T-004 → T-005 → T-019 → T-006
                                                     │          │
                                                     └──→ T-007 │

T-005 + T-006 + T-007 + T-019 → T-008
T-019 → T-009 → T-010

SERVER / ADMIN TRACK
T-020 → T-021 ───────────────┐
T-010 ───────────────────────┼→ T-011
                             │
T-021 + T-011 ───────────────┼→ T-022 → T-026
T-021 + T-011 ───────────────└→ T-023 → T-024 → T-026
T-009 + T-010 + T-011 + T-023 → T-025 → T-026

TRANSPORT TRACK
T-008 → T-012 → T-013

QUALIFICATION / RELEASE
T-004 + T-005 → T-014
T-022 + T-023 + T-024 + T-025 + T-026 + T-013 + T-014 → T-027
T-010 + T-011 + T-013 + T-014 + T-024 + T-025 + T-026 + T-027 → T-015 → T-016

T-017 runs independently when repository-settings write capability exists.
```

**Next selected client task:** T-004.  
**Parallel server-plan task:** T-021 may proceed independently of Servo implementation because it defines server-domain/security contracts, not client rendering.

---

# 12. Implementation Phases

- **A — Executable foundation:** T-001, T-002, T-003, T-018, T-020 — DONE.
- **B — Servo capsule and containment:** T-004, T-005, T-019, T-006, T-007.
- **C — Portable transport/device contracts:** T-008, T-009, T-010.
- **D — Server domain and authorization control plane:** T-021, T-011, T-022, T-023.
- **E — Administrator/fleet operations:** T-024, T-025, T-026.
- **F — Production transports:** T-012, T-013.
- **G — Qualification/release/re-audit:** T-014, T-027, T-015, T-016.
- **Governance parallel:** T-017.

---

# 13. Atomic Tasks

## T-001 — Establish execution-grade living plan
**Status:** DONE · **Priority:** P0 · **Leverage:** HIGH

## T-002 — Scaffold portable Rust boundaries
**Status:** DONE · **Priority:** P0 · **Leverage:** HIGH

## T-003 — Harden CI/dependency/architecture gates
**Status:** DONE · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

## T-018 — Reconcile Servo sandbox gap into trust architecture
**Status:** DONE · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

## T-020 — Add cross-platform project manager and controlled prerequisite bootstrap
**Status:** DONE · **Priority:** P0 · **Type:** IMPROVE / HARDEN · **Leverage:** HIGH

The manager provides controlled `doctor/install/verify/build/test/security/servo/android/clean` workflows, allowlisted prerequisite installation, tests, CI command-contract coverage, and no runtime credential role. Exact verified implementation state remains `d0c8199756fd204caa335f59a83e41a4787c7bc8`.

## T-004 — Pin Servo and build minimal embedding adapter

**Status:** READY  
**Priority:** P0  
**Type:** IMPROVE / HARDEN  
**Leverage:** HIGH

Pin a reviewed exact Servo release; isolate Servo types in a dedicated adapter; prove builder/event-loop/rendering-context integration; make evidence-backed native prerequisites explicit. No proxy, transport, credentials, or browser-broker secrets yet.

## T-005 — Prove fail-closed Servo normal networking
**Status:** TODO · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

Prove positive proxy path and negative direct-IP/DNS/redirect/IPv4/IPv6/subresource/restart paths.

## T-019 — Implement trusted broker capability boundary
**Status:** TODO · **Priority:** P0 · **Type:** HARDEN · **Leverage:** HIGH

Browser side receives no raw device/bootstrap/transport-control secret. IPC is versioned, bounded, instance-bound, deny-by-default, and semantic rather than generic native execution.

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

## T-021 — Define ProtectedService registry and server authorization domain

**Status:** READY  
**Priority:** P0  
**Type:** ARCHITECTURE / HARDEN  
**Leverage:** VERY HIGH

### Problem
The private origin may host multiple services, but the current WebGate model has no first-class service registry or canonical service→workspace binding.

### Goal
Create the server domain contract that makes multi-service routing and administration safe before SecureAcces integration is finalized.

### Scope
- `ProtectedService` identity and lifecycle;
- `ServiceStatus` state model (`ACTIVE`, `SUSPENDED`, `DISABLED` or evidence-backed equivalent);
- stable `Slug`/opaque service ID rules;
- authoritative `TenantID`/`WorkspaceID` binding;
- bounded `UpstreamRef` model;
- upstream validation rules;
- route ownership/path collision rules;
- service configuration versioning;
- audit event vocabulary;
- persistence interface and migration/version strategy;
- policy for localhost/private-LAN/container-network upstreams;
- service disable/rebind semantics;
- cache invalidation requirements after security-relevant change.

### Non-goals
No Admin UI and no generic user-configurable reverse proxy.

### Required tests
- invalid/ambiguous service slug rejection;
- duplicate route/upstream policy conflicts;
- tenant/workspace cross-binding rejection;
- malicious URL/upstream parser cases;
- loopback/private-range policy boundaries;
- stale-version update rejection where optimistic concurrency is used;
- mutation tests for allow/deny validation.

### Acceptance
A service can be resolved deterministically from server-owned routing data, and no client field can choose the authorization workspace or proxy destination.

## T-011 — Integrate SecureAcces control plane

**Status:** TODO  
**Priority:** P0  
**Type:** SECURITY / INTEGRATION  
**Leverage:** VERY HIGH

Reuse SecureAcces account/user/workspace/membership/session/enrollment/management/revocation contracts. Build a WebGate-oriented API adapter; do not expose SecureAcces internals as the client protocol and never duplicate tenant authority.

Required server flows include:

```text
bootstrap claim
device challenge/activation
session create/refresh/revoke
GET me/policy
server-side service/resource authorization
admin management operations through explicit management authz
```

**Dependency:** T-021 for service authorization semantics; T-010 where device/secret adapters are required.

## T-022 — Implement WebGate Server Gateway and safe multi-service router

**Status:** TODO  
**Priority:** P0  
**Type:** SECURITY / SERVER  
**Leverage:** VERY HIGH

### Goal
Create the production request path in front of all protected services.

### Request pipeline

```text
request
  ↓
transport/gateway boundary
  ↓
authenticate session/device context
  ↓
resolve registered ProtectedService
  ↓
resolve authoritative resource metadata
  ↓
SecureAcces.Authorize
  ↓
method/header/body policy
  ↓
proxy to registered upstream only
```

### Security requirements
- no open proxy behavior;
- no client-controlled upstream/host/port;
- strict Host/path normalization;
- reject encoded path traversal and ambiguous normalization;
- bounded request/header/body/timeouts;
- hop-by-hop header handling;
- safe forwarded identity headers with overwrite/strip policy;
- WebSocket/streaming only after explicit per-service support decision;
- upstream TLS verification where TLS is used;
- deny unknown service/method by default;
- graceful service disable/drain semantics;
- no error page leaks upstream topology or secrets.

### Tests
SSRF suites, path confusion, header smuggling boundaries, cancellation/timeouts, slow upstream, upstream crash, concurrent reconfiguration, authorization revocation during load, service disable, wrong tenant/workspace binding, direct-upstream bypass checks.

## T-023 — Implement Admin Control API

**Status:** TODO  
**Priority:** P0  
**Type:** SECURITY / PRODUCT  
**Leverage:** VERY HIGH

Expose typed, versioned administration operations for:

- users/enrollment/status;
- services/registry/status;
- memberships/effective access;
- access-matrix projection and mutations;
- devices;
- sessions;
- audit;
- health summaries.

Requirements:

- every mutation mapped to explicit management authorization;
- phishing-resistant/high-assurance admin auth policy where available;
- browser Admin API protected by secure cookies/session rules, CSRF and Origin policy;
- pagination and bounded filters;
- optimistic concurrency/version checks for sensitive configuration;
- idempotency for retryable mutations where appropriate;
- no raw credential retrieval endpoints;
- audit every privileged mutation and relevant denied attempt;
- access matrix derived from memberships, not stored separately.

## T-024 — Implement Admin Web UI

**Status:** TODO  
**Priority:** P1  
**Type:** PRODUCT / UX / HARDEN  
**Leverage:** HIGH

Primary sections:

1. Dashboard — health, alerts, recent privileged events.
2. Users — status, identities, effective services, enrollment, suspend/revoke.
3. Services — registry, workspace binding, state, upstream health, safe configuration.
4. Access — User × Service matrix with clear role/permission editor and expiry.
5. Devices — status, last seen, security level, suspend/revoke.
6. Sessions — bounded metadata and allowed revoke actions.
7. Audit — searchable security/admin history with redaction.
8. Settings — server policy only; never expose raw secrets.

UX requirements:

- destructive actions show exact blast radius;
- deny/disabled/revoked states visually distinct;
- bulk changes have preview and atomic/partial-failure semantics defined;
- permission editor shows effective result before commit;
- keyboard and mobile-responsive administration where practical;
- no hidden security side effects from generic toggles;
- service route/upstream edits explain reachability without exposing secrets.

## T-025 — Implement first-class WebGate Device Registry and admin lifecycle

**Status:** TODO  
**Priority:** P0  
**Type:** SECURITY / FLEET  
**Leverage:** VERY HIGH

Server device record includes account binding, public key, algorithm, hardware/security level when attested/known, name, status, created/last-seen/revoked timestamps and policy version metadata.

Required behavior:

- proof-of-possession challenge with domain separation;
- one installation = one device identity unless explicitly migrated;
- PENDING/ACTIVE/SUSPENDED/REVOKED lifecycle;
- revoked device cannot establish a new trusted session;
- admin device revoke also revokes associated active sessions when mapping is authoritative;
- reinstall does not resurrect an old revoked identity/bootstrap capability;
- device-private key never leaves device secure storage.

## T-026 — Implement audit, health and operational administration

**Status:** TODO  
**Priority:** P1  
**Type:** RELIABILITY / SECURITY / OPERATIONS  
**Leverage:** HIGH

Add:

- structured security audit stream;
- operational service health model separate from authorization audit;
- gateway/relay/origin health summaries;
- per-service health probes with bounded frequency/timeouts;
- admin-visible incidents and last-known state;
- secret/topology redaction policy;
- retention/rotation/export policy;
- audit failure telemetry and explicit behavior for critical audit failure classes;
- backup/restore contract for service/device/control-plane state.

## T-012 — Implement primary resilient transport
**Status:** TODO · **Priority:** P1 · **Leverage:** HIGH

Candidate Outline SDK/MobileProxy + AmneziaWG-class transport behind the restricted browser-facing contract.

## T-013 — Add independent fallback and dual-relay failover
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

Fallback must differ materially in protocol/implementation/failure mode from the primary.

## T-014 — Qualify Servo/site compatibility, security, and performance
**Status:** TODO · **Priority:** P1 · **Type:** HARDEN · **Leverage:** HIGH

## T-027 — Full admin/service authorization adversarial E2E qualification

**Status:** TODO  
**Priority:** P0 before release  
**Type:** TEST / HARDEN  
**Leverage:** VERY HIGH

Run end-to-end scenarios across:

```text
user × service × permission × device state × session state ×
service state × routing version × timing × concurrency × failure
```

Mandatory scenario families:

- admin creates user → enrolls → grants one service → user can access only that service;
- one user has different rights on several services;
- membership revoke denies immediately on subsequent authorized request;
- user suspend/revoke denies all affected service access;
- service suspend/disable denies all users without altering unrelated memberships;
- service workspace rebind invalidates stale decisions;
- device suspend/revoke blocks new trust/session flow;
- session revoke does not silently delete device identity;
- matrix UI/API cannot create access not represented by SecureAcces;
- client cannot forge service/workspace/tenant/upstream;
- malicious route/header/body cannot pivot gateway to arbitrary internal addresses;
- Admin API authorization bypass/IDOR/CSRF/replay/concurrency tests;
- simultaneous admins conflict safely;
- relay failover does not change authorization semantics;
- origin restart/reconfiguration preserves or fails closed according to defined state contracts.

Use property tests, fuzzing, pairwise/high-risk N-wise generation, state-machine/model-based tests and mutation testing for critical policy code.

## T-015 — Implement signed packaging, updates, and one-click link UX
**Status:** TODO · **Priority:** P2 · **Type:** HARDEN · **Leverage:** MEDIUM

## T-016 — Final adversarial re-audit and debt deletion
**Status:** TODO · **Priority:** P0 before release · **Type:** HARDEN · **Leverage:** HIGH

## T-017 — Enforce verified-main repository rule
**Status:** BLOCKED · **Priority:** P2 · **Type:** HARDEN

Blocker: current connector can read branch protection but does not expose a compatible write action. Continue all independent implementation work.

---

# 14. Testing Strategy

Testing layers:

1. developer/bootstrap unit and dry-run command-contract tests;
2. architecture and dependency gates;
3. Servo adapter compile/integration tests;
4. browser network-escape negative tests;
5. browser-compromise/broker-capability tests;
6. Android/platform lifecycle/isolation tests;
7. ProtectedService registry and route-policy tests;
8. SecureAcces integration/management authorization tests;
9. gateway SSRF/path/header/proxy negative tests;
10. device proof/lifecycle/revocation tests;
11. Admin API authorization/CSRF/IDOR/concurrency tests;
12. Admin UI contract/E2E tests;
13. transport chaos/failover tests;
14. end-to-end trusted-link → authorized service tests;
15. full User × Service × Device × Session × Failure qualification in T-027.

Critical logic uses the multidimensional model:

`input × state × concurrency × timing × failure × permissions × configuration × external state`.

Use boundary partitions, pairwise/high-risk N-wise, fuzzing, property tests, metamorphic tests, model-based state tests, chaos tests, and regression fixtures.

---

# 15. Mutation Testing Strategy

Mandatory for:

- URL/origin/deep-link policy;
- fail-closed decisions;
- broker authorization/IPC validation;
- transport state transitions;
- signed policy/config validation;
- device proof verification;
- SecureAcces adapters;
- service registry validation;
- service→workspace authorization resolution;
- gateway route/upstream allow/deny decisions;
- Admin management authorization;
- access-matrix mutation translation.

Planned Rust tool: `cargo-mutants`. Use an appropriate Go mutation tool after the Go server package structure is established; pin/review it before CI adoption.

---

# 16. Performance Baselines

Client:

- process → shell ready;
- Servo ready;
- broker ready;
- proxy/transport ready;
- trusted link → first paint;
- warm navigation;
- idle/active RSS and CPU;
- reconnect/failover time;
- broker IPC overhead;
- Android cold/warm start and battery-sensitive recovery.

Server:

- authn + service resolve + authorization latency;
- gateway overhead vs direct upstream baseline;
- concurrent active requests;
- memory per active service/connection;
- Admin list/matrix query latency at expected user/service cardinality;
- membership change → effective deny latency;
- service disable/rebind → routing/authorization convergence latency;
- health probe overhead;
- audit write overhead and failure behavior.

No performance optimization may weaken fail-closed, authorization freshness, or privilege-separation invariants.

---

# 17. Security Hardening

- treat browser capsule as compromise-prone;
- keep long-lived secrets behind broker/platform signer;
- no reusable private key in bootstrap bundles;
- signed/versioned configuration/policy/update formats;
- destination-restricted local proxy;
- no direct protected-origin fallback;
- no generic page→native bridge;
- per-device revocation;
- hardware-backed identity where available;
- server-authoritative service registry;
- no generic gateway/open proxy behavior;
- SecureAcces authorization on every protected resource request;
- explicit management authorization on every admin mutation;
- access matrix is projection, not authority;
- service→workspace binding changes are privileged and audited;
- raw secrets excluded from UI/logs/crash diagnostics;
- Admin browser routes use hardened cookie/origin/CSRF controls;
- bounded payloads, timeouts, pagination and filters;
- locked dependency graph and reviewed updates;
- developer bootstrap separated from runtime credentials.

---

# 18. Migration Strategy

For major changes:

`characterize → introduce boundary → dual compatibility if needed → migrate callers → verify → remove legacy`.

For service registry changes:

`schema version → migrate/validate → rebuild route index → invalidate security-sensitive caches → verify effective access → commit new active version`.

Servo remains primary. Future Servo-native sandbox improvements are defense in depth and do not automatically remove the trusted-broker boundary.

---

# 19. Deferred Work

- iOS until platform policy and Servo maturity are reevaluated;
- general-purpose full-device VPN mode;
- arbitrary general web browsing;
- large-enterprise MDM/fleet orchestration beyond the first-class small-group device administration in T-025;
- distributed authorization infrastructure beyond demonstrated scale;
- generic arbitrary reverse-proxy product behavior;
- automatic Android SDK installation/license acceptance before T-006 fixes exact versions/reproducibility requirements.

---

# 20. Rejected Decisions

- system-wide VPN as default;
- bearer-secret document links;
- silent browser-engine fallback;
- Win32/DPAPI types in portable core;
- shared user VPN keys;
- authorization in relay/VPN layer;
- client-provided authoritative tenant/workspace/permission fields;
- client-selectable upstream address/port;
- access matrix as a separate permissions database;
- service ACL stored only in WebGate client configuration;
- public exposure of each internal service as the normal deployment model;
- generic open reverse proxy in the gateway;
- weakening dependency policy to make CI green;
- treating Rust memory safety as a renderer sandbox;
- general-purpose arbitrary package installation in the project manager;
- hiding Servo prerequisite failures by installing an unreviewed broad package list.

---

# 21. Completed Tasks

- T-001 — living execution plan.
- T-002 — portable Rust workspace and first executable baseline.
- T-003 — lock/dependency/security/architecture CI gates.
- T-018 — Servo compromise-containment architecture.
- T-020 — cross-platform project manager, controlled bootstrap, build/verify menu and docs.

---

# 22. Iteration Log

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
**Verification:** exact SHA `d0c8199756fd204caa335f59a83e41a4787c7bc8` — verify and dependency-policy PASS.  
**Result:** PASS.

## Iteration 6 — Plan expansion: server admin + multi-service control plane

**Type:** Architecture / Planning  
**Findings added:** F-013..F-019.  
**Invariants added:** I-021..I-032.  
**Tasks added:** T-021..T-027.  
**Decision:** WebGate Server Gateway and `ProtectedService` registry become first-class product architecture; Admin UI/API and `User × Service` access matrix become release scope; SecureAcces remains the sole authorization authority.  
**Plan effect:** T-021 is READY as the parallel server-domain task; T-011 now depends on the service-domain contract; final release gates depend on admin/service adversarial qualification.

---

# 23. Definition of Final Done

- no unresolved Critical/High release findings;
- all release P0/P1 tasks DONE or evidence-based REJECTED/explicitly DEFERRED;
- protected browser normal networking cannot escape direct under tested failures;
- browser compromise cannot extract long-lived device keys or escalate broker capabilities in the supported threat model;
- clean supported-platform builds have explicit reproducible prerequisites;
- Servo required site capabilities pass;
- Windows and Android runtime paths are proven; Linux/macOS meet declared support gates;
- SecureAcces revocation works end-to-end;
- `ProtectedService` registry is authoritative, versioned, validated, and audited;
- multiple local/private services can coexist behind one WebGate Server Gateway;
- each service is bound server-side to the correct SecureAcces workspace/authorization scope;
- clients cannot forge tenant/workspace/permission/upstream routing authority;
- Admin can manage users, services, memberships/access, devices, sessions, audit and health through authorized APIs/UI;
- the `User × Service` matrix exactly projects SecureAcces effective authorization and cannot diverge into a second ACL store;
- gateway is proven not to be an open proxy/SSRF pivot under the supported threat model;
- device proof-of-possession and suspend/revoke work end-to-end;
- primary and independent fallback transports survive chaos tests without changing authorization semantics;
- service disable/rebind and membership/user/device/session revocation converge according to defined fail-closed timing guarantees;
- critical policy/parser/state/authorization/admin logic is mutation-resistant;
- format/build/test/race/lint/security/static checks pass across Rust and Go scopes;
- performance targets pass without security regression;
- signed packaging/update flow is verified;
- documentation matches code;
- final T-027 adversarial admin/service E2E qualification passes;
- final re-audit finds no new fundamental blocker;
- final verified state and synchronized `MASTER_PLAN.md` are in `main`.
