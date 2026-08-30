# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`  
**Primary branch:** `main`  
**Plan status:** ACTIVE  
**Research baseline:** 2026-08-30  
**Last verified implementation state before current planning expansions:** `d0c8199756fd204caa335f59a83e41a4787c7bc8`  
**Canonical browser:** Servo primary; compatibility engines explicit-only.  
**Server direction:** Go-first WebGate Server Gateway + SecureAcces authoritative authorization.  
**Release direction:** immutable verified/promoted platform builds + cryptographic signing + Telegram/bootstrap delivery + protected self-update path.

`MASTER_PLAN.md` is the single execution source of truth. Material new evidence becomes a Finding before scope or ordering changes. A runtime implementation task is DONE only after its acceptance checks pass and the verified state reaches `main` without force push.

Detailed release/distribution contract: [`docs/architecture/RELEASE-TELEGRAM-DISTRIBUTION.md`](docs/architecture/RELEASE-TELEGRAM-DISTRIBUTION.md).

---

# 1. Mission

Build WebGate as a secure, resilient, cross-platform protected-access platform for a small trusted-user set, with:

1. a protected WebGate client/browser;
2. resilient application-local transport;
3. a centrally administered server gateway for multiple private services;
4. SecureAcces as the sole authorization authority;
5. first-class device/session administration;
6. a verified release factory able to build the latest approved client and deliver it to users through Telegram or a protected download path.

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
 authoritative ProtectedService Registry
        ↓
 Docs / FactoryOS / Files / Monitoring / ...
```

Administrator product surface:

```text
Admin
  ↓
WebGate Admin UI / API
  ├── Users
  ├── Protected Services
  ├── Access Matrix (User × Service)
  ├── Devices
  ├── Sessions
  ├── Releases
  ├── Telegram Delivery
  ├── Audit
  └── Service Health
```

Release/distribution path:

```text
immutable source SHA
      ↓
clean platform build matrix
      ↓
tests/security/qualification
      ↓
sign + manifest + digest
      ↓
PROMOTED release
      ↓
Admin: Send latest WebGate
      ↓
resolve verified user + device platform
      ↓
Telegram file OR short-lived protected download link
      ↓
installer/client verifies release trust independently
```

Primary client targets are Windows and Android. Linux and macOS follow the same portable contracts after their support gates. The private origin may use dynamic public IP / CGNAT and may host multiple local web services.

---

# 2. Current State

The repository currently has:

- portable Rust workspace boundaries (`webgate-core`, `webgate-browser`, `webgate-transport`, `webgate-platform`, `webgate-app`);
- `unsafe_code = forbid`, lockfile and cargo-deny policy;
- machine-enforced internal crate dependency direction;
- exact-SHA CI verification process;
- cross-platform developer project manager and controlled prerequisite bootstrap;
- Windows/POSIX launchers and project-manager tests;
- architecture ADRs for Servo primary, cross-platform runtime and Servo compromise containment;
- documented SecureAcces integration boundary;
- documented multi-service/admin target architecture;
- documented verified release + Telegram distribution target architecture.

Not yet implemented in production WebGate code:

- real Servo adapter;
- fail-closed browser proxy;
- production transport/relay providers;
- trusted broker IPC boundary;
- device-key platform adapters;
- WebGate Server Gateway;
- ProtectedService registry;
- SecureAcces control API adapter;
- Admin API/UI;
- first-class WebGate Device Registry;
- release registry/artifact store;
- production signing/package pipeline;
- Telegram binary delivery pipeline;
- protected self-update flow.

SecureAcces already supplies account/user/workspace/membership/session/permission and management primitives. WebGate must reuse those primitives rather than create a parallel RBAC database.

---

# 3. Target Architecture

```text
UNTRUSTED CONTENT / USER DEVICE
      │
      ▼
┌──────────────────────────────────────────────┐
│ Browser capsule — assume compromise         │
│ Servo + page/render/input state              │
│ short-lived bounded web capability only     │
└───────────────────┬──────────────────────────┘
                    │ narrow semantic IPC
                    ▼
┌──────────────────────────────────────────────┐
│ Trusted client broker                       │
│ policy verification                         │
│ device signer                               │
│ session refresh authority                   │
│ transport control                           │
│ update verification                         │
└───────────────────┬──────────────────────────┘
                    │ destination-restricted proxy
                    ▼
           replaceable secure transports
                    │
               Relay A / Relay B
                    │
                    ▼
┌───────────────────────────────────────────────────────────────┐
│ PRIVATE ORIGIN / SERVER                                       │
│                                                               │
│ WebGate Server Gateway / Control Plane                        │
│   ├── SecureAcces adapter                                     │
│   ├── ProtectedService Registry                               │
│   ├── Device Registry                                         │
│   ├── Release Registry / Artifact Store                       │
│   ├── Telegram Delivery Adapter                               │
│   ├── Admin API / UI                                          │
│   └── Audit / Health                                          │
│                                                               │
│ request: authenticate → resolve service → authorize → proxy    │
│                                                               │
│       Docs      FactoryOS      Files      Monitoring      ...  │
└───────────────────────────────────────────────────────────────┘
```

The Go-first server direction allows WebGate Server to use SecureAcces natively. Rust client protocols remain narrow, versioned wire contracts; SecureAcces internal Go structs are not exposed as public client protocol.

---

# 4. Server Domain Model

## 4.1 ProtectedService

Every routable private application is a first-class server record.

Conceptual model:

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
    Version     uint64
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

Typical registry:

```text
svc-docs       → workspace-docs       → 127.0.0.1:8081
svc-factory    → workspace-factory    → 127.0.0.1:8082
svc-files      → workspace-files      → 127.0.0.1:8083
svc-monitoring → workspace-monitoring → 127.0.0.1:8084
```

`UpstreamRef`, `TenantID` and `WorkspaceID` are server-owned facts. Browser/client input cannot override them.

## 4.2 SecureAcces remains authoritative

```text
ProtectedService
      │
      └── authoritative WorkspaceID
                       │
Account/User ─ Membership ─ PermissionBits
                       │
                    ALLOW / DENY
```

Initial mapping:

| Service action | SecureAcces permission |
|---|---|
| open/view | `PermView` |
| download/export | `PermDownload` |
| upload | `PermUpload` |
| edit/write | `PermEdit` |
| delete | `PermDelete` |
| manage members | `PermManageMembers` |
| manage workspace/service access | `PermManageWorkspace` |

The Admin access matrix is only a projection/mutation UX for SecureAcces memberships.

## 4.3 Protected request sequence

```text
request
  ↓
authenticate session/device context
  ↓
resolve registered ProtectedService
  ↓
resolve authoritative resource metadata
  ↓
derive tenant/workspace from server state
  ↓
SecureAcces.Authorize
  ↓
method/header/body policy
  ↓
proxy to registered upstream only
```

Unknown/ambiguous route, service, workspace binding, method, host or upstream state fails closed.

---

# 5. Admin Product Model

## 5.1 Users

Admin can create/preapprove users, manage enrollment, inspect safe identity/status data, suspend/reactivate/revoke, inspect effective service access and permitted session/device operations.

## 5.2 Services

Admin can register a protected service, bind it to its SecureAcces workspace, configure a policy-valid upstream, suspend/disable it, inspect health and perform audited reconfiguration.

## 5.3 Access Matrix

Primary UX:

```text
                 Docs   FactoryOS   Files   Monitoring
Ivan              View      —       Download     —
Sergey            Edit     View     Download    View
Anna              View     Edit        —         —
Administrator     Admin    Admin      Admin      Admin
```

Cell edits translate to SecureAcces membership operations. No independent ACL table is created.

## 5.4 Devices

Admin manages first-class device lifecycle:

```text
PENDING → ACTIVE ↔ SUSPENDED → REVOKED
```

Device and session revocation remain distinct semantics.

## 5.5 Sessions

Admin sees bounded metadata and can perform only management-authorized revocations. Raw bearer tokens are never exposed.

## 5.6 Audit / Health

Security audit and operational health are separate streams. Sensitive topology, raw IP details, credentials and transport secrets are redacted according to role/policy.

## 5.7 Releases and Telegram Delivery

Admin gets a first-class **Releases** surface:

- current `stable` and optional `beta/test` release;
- immutable source commit SHA;
- build/verification/promotion status;
- platform/architecture artifacts;
- digest/signature/signing-key ID;
- release revocation/supersession;
- delivery history and rollout state;
- failed-delivery diagnostics without credential leakage.

User page actions:

```text
Send latest WebGate
Send activation package
Resend release
View delivery history
```

Bulk actions:

```text
Send latest to selected users
Send latest to all active users
Send latest to devices below minimum version
```

Bulk delivery requires a preview showing recipient count, platform/architecture resolution, selected release per platform, unavailable Telegram bindings, incompatible devices and chosen direct-file/link delivery mode.

`latest` means the newest **PROMOTED + AVAILABLE + compatible** release in the selected channel, never the newest arbitrary `main` commit.

New-install flow and update flow are separate:

```text
NEW USER:
generic signed installer + short-lived activation capability
→ installation generates its own device key

EXISTING DEVICE:
resolve device platform/arch
→ latest compatible promoted release
→ Telegram delivery
→ local verification/install
```

No long-lived per-user credential is compiled into an executable.

---

# 6. Release Domain Model

Release lifecycle:

```text
SOURCE_CANDIDATE
   ↓
BUILDING
   ↓
BUILT
   ↓
VERIFYING
   ↓
VERIFIED
   ↓
PROMOTED
   ↓
AVAILABLE
```

Terminal/exception states include `REJECTED`, `QUARANTINED`, `REVOKED`, `SUPERSEDED`.

A user-deliverable release binds at least:

- WebGate version;
- immutable source SHA;
- release channel;
- platform and architecture;
- package format;
- artifact digest and size;
- signing key ID;
- build/provenance metadata;
- verification result;
- server protocol compatibility;
- promotion actor/time;
- revocation/supersession state.

Conceptual manifest:

```json
{
  "schema": "webgate.release/v1",
  "version": "1.2.3",
  "channel": "stable",
  "source_commit": "<immutable sha>",
  "platform": "windows",
  "arch": "x86_64",
  "artifact": "WebGate-1.2.3-windows-x86_64.exe",
  "sha256": "...",
  "signing_key_id": "release-2026-01",
  "min_server_protocol": 1,
  "signature": "..."
}
```

A successful compilation is not sufficient for promotion or distribution.

---

# 7. Telegram Distribution Contract

Telegram is transport/notification, not the release trust root.

Recipient resolution:

```text
SecureAcces Account
      ↓
verified Telegram ExternalIdentity
      ↓
server-side notification binding
      ↓
Telegram chat recipient
```

Admin does not normally enter an arbitrary `chat_id`.

Delivery modes:

1. **Direct document** for artifacts within configured Bot API capability.
2. **Local Telegram Bot API Server** when deliberately deployed for larger direct uploads.
3. **Short-lived protected download capability** when direct upload is unsuitable.
4. Cached Telegram `file_id` may optimize resends of the exact immutable artifact but never defines artifact identity/trust.

Provider size limits are runtime/provider capabilities, not compiled security assumptions. As of the 2026-08-30 research snapshot, the Telegram cloud Bot API documents direct bot file sends up to 50 MB, while the official local Bot API Server can upload up to 2000 MB; WebGate must keep these values configurable and revalidated.

Every downloaded/received package still verifies WebGate release signature/digest locally.

Telegram message UX should show version, platform/channel, brief changes and a clear Install/Download action without exposing secret material.

---

# 8. Baseline Verification

Current Rust/project gates:

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

As Go server code lands:

```text
go test ./...
go test -race ./...
go vet ./...
```

Release pipeline adds clean build/package/sign/manifest/verification gates per supported platform plus adversarial distribution tests.

---

# 9. System Invariants

- **I-001:** Servo is the default protected browser engine.
- **I-002:** normal WebGate mode does not change the OS default route.
- **I-003:** transport loss fails closed; protected traffic never silently falls back to direct Internet.
- **I-004:** browser failure never silently switches browser engine/system browser for protected content.
- **I-005:** links identify resources; they are not persistent bearer credentials.
- **I-006:** device private keys are generated on-device and never distributed in config/build artifacts.
- **I-007:** bootstrap, policy, release and update artifacts are signed/versioned/rollback-aware where applicable.
- **I-008:** remote policy can tighten but cannot weaken compiled hard security invariants.
- **I-009:** transport implementations remain replaceable behind stable contracts.
- **I-010:** SecureAcces is authoritative for account/session/workspace/resource authorization.
- **I-011:** shared client core contains no mandatory desktop-only assumption; Android is first-class.
- **I-012:** device signing/secret storage is a platform capability; hardware-backed keys are preferred.
- **I-013:** production has at least two materially independent network failure domains.
- **I-014:** browser-facing proxy endpoints are loopback-only and fail closed.
- **I-015:** internal dependency direction is machine-enforced.
- **I-016:** CI code-execution actions use immutable pins where supported.
- **I-017:** dependency versions/locks remain explicit and reviewed.
- **I-018:** browser capsule cannot access long-lived device/bootstrap/transport/session-refresh secrets or generic privileged native APIs.
- **I-019:** network fail-closed and browser-compromise containment are separate test properties.
- **I-020:** developer bootstrap is allowlisted/reviewable and has no runtime credential role.
- **I-021:** each routable private application has a server-authoritative `ProtectedService` record.
- **I-022:** service→tenant/workspace binding is server-owned.
- **I-023:** Admin access matrix derives from SecureAcces memberships and never becomes parallel authorization state.
- **I-024:** gateway proxies only to registered policy-valid upstreams; no generic client-controlled proxy target.
- **I-025:** protected upstreams are not intentionally public as the normal deployment path.
- **I-026:** unknown/ambiguous service/path/method/routing metadata fails closed.
- **I-027:** privileged admin operations require explicit management authorization and audit.
- **I-028:** device status and session status are separate lifecycles.
- **I-029:** raw bearer/session/device/transport/upstream/release-signing secrets never appear in ordinary Admin UI/audit.
- **I-030:** health/diagnostics follow least-information disclosure.
- **I-031:** Admin API may identify objects by opaque ID but cannot submit authoritative security facts the server should resolve.
- **I-032:** service→workspace rebind is privileged/audited and invalidates stale security decisions.
- **I-033:** `latest` user version means latest compatible `PROMOTED`/`AVAILABLE` release, never arbitrary newest source commit.
- **I-034:** a promoted release is immutable; different bytes require a new release identity/version and cannot silently replace an existing promoted artifact.
- **I-035:** Telegram is never a release trust root; artifact signature/digest verification is independent of message authenticity/filename.
- **I-036:** release signing keys are separate from Telegram bot token, device keys, transport keys and policy keys.
- **I-037:** no long-lived per-user credential is compiled into individualized release binaries.
- **I-038:** Telegram recipients are resolved from verified server-side identity binding; arbitrary chat IDs are not a normal delivery authority.
- **I-039:** user/device/release state is rechecked immediately before dispatch or protected download authorization.
- **I-040:** revoked/quarantined release cannot be newly distributed or recommended by update service.
- **I-041:** compilation, verification, promotion and delivery are separate state transitions; successful build alone cannot trigger mass user distribution by default.
- **I-042:** release/download/Telegram delivery actions are bounded, idempotent where appropriate, rate-aware and audited without secret leakage.
- **I-043:** direct Telegram file-size limits are provider configuration/capability, not hard-coded product security invariants.

---

# 10. Findings Registry

## Existing findings

- **F-001:** repository originally had no executable baseline — resolved by T-002.
- **F-002:** original roadmap was not execution-grade — resolved by T-001.
- **F-003:** desktop-only assumptions existed — planned mitigation T-006/T-009/T-010/T-019.
- **F-004:** fixed Ed25519 device identity is not universally hardware-backed — planned T-009/T-010.
- **F-005:** Servo compatibility/security is release-sensitive — planned T-004/T-014.
- **F-006:** architecture boundaries were documentation-only — mitigated T-002/T-003.
- **F-007:** local execution environment may lack Rust/native prerequisites — mitigated T-020.
- **F-008:** `main` lacks repository-enforced branch protection — T-017 blocked on connector capability.
- **F-009:** cargo-deny rejected versionless internal path deps — resolved T-003.
- **F-010:** CI action runtime drift — resolved T-003.
- **F-011:** Servo is not a sufficient mature renderer sandbox — planned containment T-019 and platform defense in depth.
- **F-012:** Servo Linux build exposed fontconfig native prerequisite — mitigated T-020, final proof T-004.
- **F-013:** multi-service origin lacked first-class service domain — T-021.
- **F-014:** no implemented WebGate server gateway/control API — T-011/T-022/T-023.
- **F-015:** admin workflow incomplete — T-024.
- **F-016:** device administration needs first-class registry/proof — T-009/T-010/T-025.
- **F-017:** access matrix could become second authorization database — prevented by I-023/T-023/T-024/T-027.
- **F-018:** generic configurable reverse proxy creates SSRF/pivot risk — T-021/T-022/T-027.
- **F-019:** Admin control plane is a high-value target — T-023/T-024/T-027.

## F-020 — No user-facing verified release distribution pipeline

**Status:** Planned  
**Severity:** High  
**Category:** Release / Product / Operations

A trusted user currently has no WebGate-native path to receive the current production client without manually navigating repository/build tooling.

**Resolution:** T-015 + T-028.

## F-021 — “Latest version” is unsafe if mapped directly to newest main commit

**Status:** Planned prevention  
**Severity:** Critical  
**Category:** Supply Chain / Release Safety

Newest source is not necessarily tested, signed, compatible or promoted. Distribution must select an immutable promoted release record, not a mutable branch head.

**Resolution:** I-033/I-034/I-041, T-015, T-028.

## F-022 — Telegram delivery can be mistaken for binary authenticity

**Status:** Planned prevention  
**Severity:** Critical  
**Category:** Supply Chain / Identity

A file arriving from the expected Telegram bot is insufficient proof that the bytes are an authorized WebGate release.

**Resolution:** I-035/I-036, T-015, T-028.

## F-023 — Per-user compiled installers would create secret sprawl

**Status:** Planned prevention  
**Severity:** High  
**Category:** Device Bootstrap / Release

Embedding persistent user/device credentials into custom binaries makes revocation, leakage containment and build security worse.

**Resolution:** generic signed package + short-lived activation capability; I-037; T-025/T-028.

## F-024 — Telegram provider limits/failures require a delivery fallback

**Status:** Planned  
**Severity:** Medium  
**Category:** Reliability

Cloud Bot API file-size/rate/availability limits can prevent direct delivery. WebGate needs configurable provider capability plus short-lived protected download fallback and optional local Bot API Server support.

**Resolution:** T-028.

---

# 11. Risk Register

| Risk | Impact | Mitigation |
|---|---|---|
| browser compromise reaches long-lived secrets | Critical | trusted broker + platform sandbox defense in depth |
| network proxy escapes direct | Critical | T-005 negative escape tests |
| access matrix diverges from authorization | Critical | SecureAcces-only grants; matrix is projection |
| gateway becomes SSRF/open-proxy pivot | Critical | registered upstream-only routing + adversarial tests |
| Admin API compromise changes access/routing | Critical | management authz + strong auth + audit + negative tests |
| arbitrary newest commit is distributed as “latest” | Critical | immutable release registry + promotion gate |
| Telegram-delivered file is treated as trusted solely due to sender | Critical | independent signature/digest verification |
| release artifact silently replaced | Critical | immutable release identity + digest/signature/provenance |
| stale service→workspace binding preserves access | Critical | authoritative lookup/cache invalidation/versioning |
| wrong platform/arch sent to user | High | Device Registry + explicit build matrix + fail closed on unknown |
| bot token or signing key leaks | Critical | separate secret domains, server-only storage, least privilege |
| release revoked while delivery is in-flight | High | pre-dispatch/download recheck + revocation state |
| Telegram unavailable/rate-limited/file too large | Medium/High | bounded retry + direct/link modes + optional local Bot API |
| device identifier mistaken for cryptographic proof | High | T-009/T-025 proof-of-possession |
| dependency/security drift | High | lockfiles, reviewed pins, clean builds, release qualification |

---

# 12. Pareto Improvements

1. Keep portable client contracts and browser/broker privilege separation before secrets exist.
2. Keep developer setup reproducible through T-020.
3. Prove exact Servo integration before production browser networking.
4. Define `ProtectedService` and authorization contracts before hardening the control plane.
5. Prove fail-closed browser networking before real transport providers.
6. Build gateway authorization/routing before Admin UI.
7. Keep access matrix a SecureAcces projection.
8. Establish immutable release domain before implementing “send latest”.
9. Build/package/sign once per platform; never create secret-bearing per-user binaries.
10. Use Telegram as delivery UX with cryptographic verification and protected-link fallback.
11. Validate Android lifecycle before desktop assumptions harden.
12. Run adversarial admin/service/release/distribution qualification before release.

---

# 13. Dependency DAG

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
T-021 + T-011 ───────────────┼→ T-022 → T-026
T-021 + T-011 ───────────────└→ T-023 → T-024 → T-026
T-009 + T-010 + T-011 + T-023 → T-025 → T-026

TRANSPORT TRACK
T-008 → T-012 → T-013

QUALIFICATION / RELEASE
T-004 + T-005 → T-014
T-022 + T-023 + T-024 + T-025 + T-026 + T-013 + T-014 → T-027
T-010 + T-011 + T-013 + T-014 + T-024 + T-025 + T-026 + T-027 → T-015
T-015 + T-023 + T-024 + T-025 → T-028
T-028 → T-016

T-017 runs independently when repository-settings write capability exists.
```

**Next selected client task:** T-004.  
**Parallel server-domain task:** T-021.  
**Release/distribution architecture:** specified; implementation follows T-015 prerequisites and Admin/Device surfaces.

---

# 14. Implementation Phases

- **A — Executable foundation:** T-001, T-002, T-003, T-018, T-020 — DONE.
- **B — Servo capsule and containment:** T-004, T-005, T-019, T-006, T-007.
- **C — Portable transport/device contracts:** T-008, T-009, T-010.
- **D — Server domain and authorization control plane:** T-021, T-011, T-022, T-023.
- **E — Administrator/fleet operations:** T-024, T-025, T-026.
- **F — Production transports:** T-012, T-013.
- **G — Qualification:** T-014, T-027.
- **H — Packaging/release/distribution:** T-015, T-028.
- **I — Final adversarial re-audit:** T-016.
- **Governance parallel:** T-017.

---

# 15. Atomic Tasks

## Completed foundation

- **T-001 — Establish execution-grade living plan:** DONE.
- **T-002 — Scaffold portable Rust boundaries:** DONE.
- **T-003 — Harden CI/dependency/architecture gates:** DONE.
- **T-018 — Reconcile Servo sandbox gap into trust architecture:** DONE.
- **T-020 — Cross-platform project manager and controlled prerequisite bootstrap:** DONE.

## T-004 — Pin Servo and build minimal embedding adapter

**Status:** READY · **Priority:** P0 · **Type:** IMPROVE/HARDEN

Pin reviewed exact Servo release, isolate Servo types, prove builder/event loop/rendering integration and make evidence-backed native prerequisites explicit.

## T-005 — Prove fail-closed Servo normal networking

**Status:** TODO · **Priority:** P0 · **Type:** HARDEN

Positive protected-proxy path plus negative direct-IP/DNS/redirect/IPv4/IPv6/subresource/restart tests.

## T-019 — Implement trusted broker capability boundary

**Status:** TODO · **Priority:** P0 · **Type:** HARDEN

Versioned bounded instance-bound semantic IPC; browser receives no raw long-lived secrets or generic native execution capability.

## T-006 — Android lifecycle/embedding/isolation probe

**Status:** TODO · **Priority:** P0

Validate Servo/proxy/broker lifecycle across Android pause/resume/recreate and absence of desktop-only core assumptions.

## T-007 — Strict navigation/deep-link policy

**Status:** TODO · **Priority:** P1

Fuzz/property/mutation tests for schemes, IDN/Unicode, origins, redirects, opaque IDs and external navigation.

## T-008 — Transport SPI and deterministic failover state machine

**Status:** TODO · **Priority:** P1

## T-009 — Algorithm-agile device identity

**Status:** TODO · **Priority:** P1

## T-010 — Platform secret/device adapters

**Status:** TODO · **Priority:** P1

Windows CNG/TPM where possible; Android Keystore; macOS Keychain/Secure Enclave where applicable; explicit Linux policy.

## T-021 — ProtectedService registry and server authorization domain

**Status:** READY · **Priority:** P0 · **Type:** ARCHITECTURE/HARDEN

Define service identity/lifecycle, authoritative tenant/workspace binding, bounded upstream model, route collision policy, configuration versioning, persistence/migration, audit vocabulary, disable/rebind semantics and cache invalidation.

Acceptance: service resolution is deterministic from server-owned state and client fields cannot choose authorization workspace or proxy destination.

## T-011 — Integrate SecureAcces control plane

**Status:** TODO · **Priority:** P0 · **Type:** SECURITY/INTEGRATION

Reuse SecureAcces accounts/users/workspaces/memberships/sessions/enrollment/management/revocation. Provide WebGate-oriented API rather than exposing SecureAcces structs.

Required flows include bootstrap claim, device challenge/activation, session create/refresh/revoke, `me/policy`, server-side service/resource authorization and admin management operations.

## T-022 — WebGate Server Gateway and safe multi-service router

**Status:** TODO · **Priority:** P0 · **Type:** SECURITY/SERVER

Implement authenticated service resolution + SecureAcces authorization + registered-upstream reverse proxy. Include SSRF, path normalization, header handling, body/time limits, cancellation, service disable/drain and direct-bypass tests.

## T-023 — Admin Control API

**Status:** TODO · **Priority:** P0 · **Type:** SECURITY/PRODUCT

Typed/versioned operations for users, services, memberships/effective access, devices, sessions, releases, audit and health. Every privileged mutation maps to explicit management authorization and audit. Browser API uses hardened cookie/Origin/CSRF policy. No raw credential retrieval endpoints.

## T-024 — Admin Web UI

**Status:** TODO · **Priority:** P1 · **Type:** PRODUCT/UX/HARDEN

Sections:

1. Dashboard
2. Users
3. Services
4. Access Matrix
5. Devices
6. Sessions
7. **Releases**
8. **Delivery**
9. Audit
10. Settings/Health

Release UX includes build/verification state, promotion/revocation, per-platform artifacts, “Send latest WebGate”, activation package, bulk rollout preview and delivery history.

## T-025 — WebGate Device Registry and admin lifecycle

**Status:** TODO · **Priority:** P0 · **Type:** SECURITY/FLEET

Public-key device registry with proof-of-possession and PENDING/ACTIVE/SUSPENDED/REVOKED lifecycle. Device record supplies authoritative platform/arch when trusted/known for release selection.

## T-026 — Audit, health and operational administration

**Status:** TODO · **Priority:** P1

Structured security audit, operational health, service probes, incident state, redaction, retention/rotation/export and backup/restore contract.

## T-012 — Primary resilient transport

**Status:** TODO · **Priority:** P1

Outline SDK/MobileProxy + AmneziaWG-class candidate behind restricted contract after qualification.

## T-013 — Independent fallback and dual-relay failover

**Status:** TODO · **Priority:** P1

Fallback differs materially in implementation/protocol/failure mode.

## T-014 — Qualify Servo/site compatibility, security and performance

**Status:** TODO · **Priority:** P1

## T-027 — Full admin/service authorization adversarial E2E qualification

**Status:** TODO · **Priority:** P0 before release

Test multidimensional space:

```text
user × service × permission × device × session × service state ×
routing version × timing × concurrency × transport failure
```

Mandatory scenarios include multi-service rights, membership/user/device/session/service revocation, workspace rebind, matrix projection integrity, forged tenant/workspace/upstream denial, SSRF/path/header attacks, Admin IDOR/CSRF/replay/concurrency, simultaneous admins and transport failover preserving authorization semantics.

## T-015 — Signed packaging, update authority and one-click client UX

**Status:** TODO · **Priority:** P0 for user distribution · **Type:** SUPPLY-CHAIN/HARDEN

### Goal
Create the immutable release authority used by both Telegram distribution and future protected self-update.

### Scope

- platform build/package matrix;
- immutable source SHA input;
- clean build environment contract;
- locked dependency graph;
- artifact storage abstraction;
- signed release manifest;
- digest/signing key ID/provenance metadata;
- `stable` plus optional non-production channels;
- `VERIFIED → PROMOTED → AVAILABLE` state transitions;
- release revoke/supersede/minimum-version policy;
- client-side signature/digest/version/protocol compatibility verification;
- installer/update UX and rollback-aware policy;
- no user/device secrets in artifacts.

### Required tests

- failed build/verification cannot promote;
- same version with different bytes cannot silently replace promoted release;
- signature/digest mismatch rejected;
- wrong platform/arch rejected;
- revoked release no longer recommended;
- old/rollback release policy enforced.

## T-028 — Build latest verified client and deliver it to users via Telegram

**Status:** TODO  
**Priority:** P0 for operational release  
**Type:** RELEASE / PRODUCT / SECURITY / AUTOMATION  
**Leverage:** VERY HIGH

### Goal
Give Admin a safe one-click flow to compile/build the production client through the release pipeline and deliver the latest compatible promoted version to selected trusted users through Telegram.

### Core flow

```text
Admin chooses build/release candidate
        ↓
immutable source SHA
        ↓
clean platform build matrix
        ↓
T-015 verification/signing/promotion
        ↓
Admin selects user(s)
        ↓
server resolves verified Telegram identity
        ↓
server resolves registered device platform/arch or explicit bootstrap target
        ↓
select latest compatible PROMOTED release
        ↓
pre-dispatch user/device/release authorization/state check
        ↓
Telegram file OR protected short-lived download link
        ↓
user installs
        ↓
client verifies signed release and reports version when available
```

### Build targets

At minimum:

- Windows x86_64 signed installer;
- Android arm64 signed APK/package after Android acceptance gate;
- Linux/macOS artifacts only when those platform support gates are declared production-ready.

Do not guess platform/architecture. Unknown target is an admin-visible error/state.

### Telegram adapter

- resolve recipient from verified server-side Telegram identity/binding;
- bot token stored only server-side;
- direct `sendDocument` within configured provider capability;
- optional local Telegram Bot API Server for intentionally supported larger uploads;
- short-lived protected download link fallback;
- cached Telegram `file_id` only for exact immutable artifact;
- bounded retries/backoff and rate-limit handling;
- delivery idempotency key;
- no credentials/secrets in captions or filenames.

### New-user delivery

Send generic signed installer plus a **short-lived activation/bootstrap capability**. Installation creates its device private key locally. Never compile long-lived user/device secrets into individualized binaries.

### Existing-device update

Resolve registered platform/architecture, select latest compatible stable promoted release, deliver package/link, verify locally and report installed version when supported.

### Admin operations

- `Build release candidate`;
- `Promote release`;
- `Revoke release`;
- `Send latest WebGate`;
- `Send activation package`;
- `Send latest to selected users`;
- `Send latest to all active users`;
- `Send latest to outdated devices`;
- `View delivery/install history`.

Bulk operations require preview and explicit management authorization.

### Delivery audit events

At minimum:

```text
RELEASE_BUILD_STARTED
RELEASE_BUILD_SUCCEEDED
RELEASE_BUILD_FAILED
RELEASE_VERIFIED
RELEASE_PROMOTED
RELEASE_REVOKED
RELEASE_DELIVERY_REQUESTED
RELEASE_DELIVERY_SENT
RELEASE_DELIVERY_FAILED
RELEASE_DOWNLOAD_ISSUED
RELEASE_DOWNLOADED
RELEASE_INSTALL_REPORTED
```

### Mandatory failure cases

- Telegram unavailable;
- bot blocked by user;
- no verified Telegram identity;
- rate limit;
- artifact too large for configured mode;
- interrupted upload;
- stale Telegram `file_id`;
- download service unavailable;
- wrong/unknown platform or architecture;
- release revoked during dispatch;
- user/device revoked during dispatch;
- digest/signature mismatch;
- installer/update failure.

### Security tests

- non-admin cannot build/promote/revoke/send;
- admin cannot distribute outside management authority;
- arbitrary `chat_id` cannot bypass verified binding;
- revoked user/device/release fails closed;
- Telegram message/file metadata cannot override immutable release metadata;
- different bytes cannot be served under the same promoted release identity;
- signing key and bot token compromise domains are independent;
- Admin release endpoints receive CSRF/IDOR/replay/concurrency tests.

### E2E acceptance

```text
source commit
→ clean build
→ tests/qualification
→ sign/manifest
→ promote
→ Admin selects user
→ platform/device resolve
→ Telegram delivery
→ user obtains package
→ local signature/digest verification
→ install/launch
→ version report / successful protected access
```

All supported branches of this flow pass before T-028 is DONE.

## T-016 — Final adversarial re-audit and debt deletion

**Status:** TODO · **Priority:** P0 before release

## T-017 — Enforce verified-main repository rule

**Status:** BLOCKED · **Priority:** P2

Repository settings write capability is currently unavailable through the connector; continue independent implementation work.

---

# 16. Testing Strategy

Testing layers:

1. developer/bootstrap tests;
2. architecture/dependency gates;
3. Servo compile/integration;
4. browser network-escape negatives;
5. browser-compromise/broker capability tests;
6. Android lifecycle/isolation;
7. ProtectedService registry/route policy;
8. SecureAcces integration/management authorization;
9. gateway SSRF/path/header/proxy negatives;
10. device proof/lifecycle/revocation;
11. Admin API IDOR/CSRF/replay/concurrency;
12. Admin UI contract/E2E;
13. transport chaos/failover;
14. release state-machine/signature/artifact immutability tests;
15. platform build/package tests;
16. Telegram recipient/provider/rate/size/fallback tests;
17. release revoke/race tests;
18. end-to-end build→promote→Telegram→verify→install flow;
19. full User × Service × Device × Session × Release × Failure qualification.

Critical logic uses:

`input × state × concurrency × timing × failure × permissions × configuration × external state`.

Use boundary partitions, high-risk N-wise/pairwise, fuzzing, property tests, metamorphic tests, model-based state tests, chaos tests and regression fixtures.

---

# 17. Mutation Testing Strategy

Mandatory for critical:

- URL/deep-link/origin policy;
- fail-closed decisions;
- broker IPC authorization;
- transport state machine;
- signed config/policy/release validation;
- device proof verification;
- SecureAcces adapters;
- service registry validation;
- service→workspace resolution;
- gateway allow/deny/upstream decisions;
- Admin management authorization;
- access-matrix mutation translation;
- release promotion/revocation state machine;
- artifact-selection platform compatibility logic;
- Telegram recipient mapping and delivery authorization.

Use `cargo-mutants` for Rust where applicable. Select and pin/review an appropriate Go mutation tool after server package structure exists.

---

# 18. Performance / Reliability Baselines

Client:

- process/shell/Servo/broker/proxy readiness;
- trusted link → first paint;
- warm navigation;
- RSS/CPU;
- reconnect/failover;
- broker IPC overhead;
- Android cold/warm lifecycle.

Server:

- authn + service resolve + authorize latency;
- gateway overhead;
- concurrent active requests;
- Admin list/matrix latency;
- permission/service/device/session revoke convergence;
- health probe/audit overhead.

Release/distribution:

- clean build duration per platform;
- package/sign/manifest duration;
- artifact storage size;
- build cache effectiveness without compromising reproducibility;
- Telegram upload throughput/retry behavior;
- direct-file vs protected-link delivery latency;
- bulk rollout rate limiting and queue boundedness;
- promotion → first successful install/report latency.

Performance optimization cannot weaken fail-closed behavior, authorization freshness or supply-chain verification.

---

# 19. Security Hardening

- browser capsule treated as compromise-prone;
- long-lived secrets behind broker/platform signer;
- signed/versioned configuration/policy/release/update formats;
- destination-restricted proxy;
- no direct protected-origin fallback;
- no generic page→native bridge;
- per-device revocation and hardware-backed identity where available;
- server-authoritative service registry;
- no open/generic gateway proxy;
- SecureAcces authorization on protected resources;
- explicit management authorization on admin mutations;
- access matrix is projection only;
- service rebind privileged/audited;
- release build source immutable;
- promoted release immutable;
- separate release-signing and Telegram credentials;
- Telegram not trusted for artifact authenticity;
- generic signed installers + short-lived activation, not persistent custom secrets;
- revoked release/user/device blocked before delivery/download;
- raw secrets excluded from UI/logs/crash reports;
- hardened Admin cookie/origin/CSRF controls;
- bounded requests, queues, retries, payloads and filters;
- locked dependency graph and reviewed supply-chain updates.

---

# 20. Migration Strategy

Major change:

`characterize → introduce boundary → migrate callers/data → verify → remove legacy`.

Service registry change:

`schema version → migrate/validate → rebuild route index → invalidate security caches → verify effective access → activate`.

Release schema/signing change:

`new schema/key profile → dual verification window if required → promote compatible clients/server → revoke/deprecate old profile → remove legacy only after fleet evidence`.

---

# 21. Deferred Work

- iOS until platform/Servo policy reevaluation;
- general-purpose full-device VPN mode;
- arbitrary general web browsing;
- large-enterprise MDM orchestration beyond first-class small-group device admin;
- distributed authorization infrastructure beyond demonstrated scale;
- generic arbitrary reverse-proxy product;
- public app-store distribution as the only installation method;
- automatic unreviewed mass rollout immediately after every successful build.

---

# 22. Rejected Decisions

- system-wide VPN as default;
- bearer-secret document links;
- silent browser-engine fallback;
- shared user VPN keys;
- authorization in relay/VPN layer;
- client-provided authoritative tenant/workspace/permission fields;
- client-selectable upstream;
- second ACL database for access matrix;
- public exposure of each internal service as normal architecture;
- generic open reverse proxy;
- weakening dependency/security gates to make CI green;
- treating Rust memory safety as mature renderer sandbox;
- arbitrary package install in developer manager;
- defining “latest” as newest `main` commit;
- distributing a build merely because compilation succeeded;
- using Telegram sender/file name as proof of binary authenticity;
- compiling long-lived per-user secrets into customized installers;
- reusing one release version/ID for different bytes;
- silently choosing a package for unknown user platform/architecture.

---

# 23. Completed Tasks

- T-001 — living execution plan.
- T-002 — portable Rust workspace/executable baseline.
- T-003 — dependency/security/architecture CI gates.
- T-018 — Servo compromise-containment architecture.
- T-020 — cross-platform project manager and controlled prerequisite bootstrap.

---

# 24. Iteration Log

## Iterations 1–5

Foundation plan/workspace/CI/containment/project-manager tasks completed and pushed to `main`; exact verified implementation baseline remains recorded above.

## Iteration 6 — Server admin + multi-service planning expansion

Added F-013..F-019, I-021..I-032 and T-021..T-027. WebGate Server Gateway, ProtectedService registry, Admin UI/API and User×Service matrix became release scope while SecureAcces remained sole authorization authority.

## Iteration 7 — Verified release + Telegram distribution planning expansion

**Date:** 2026-08-30  
**Type:** Architecture / Product / Supply Chain  
**Findings added:** F-020..F-024.  
**Invariants added:** I-033..I-043.  
**Task added:** T-028.  
**T-015 expanded:** immutable release authority, package/sign/manifest/promotion/revocation.  
**Admin UI expanded:** Releases + Delivery + per-user/bulk “Send latest WebGate”.  
**Decision:** “latest” means latest compatible promoted release, not latest source commit. Telegram is a delivery channel only; local signature/digest verification remains authoritative. Generic signed packages are paired with short-lived activation capability instead of persistent per-user secrets.  
**Architecture document:** `docs/architecture/RELEASE-TELEGRAM-DISTRIBUTION.md`.

---

# 25. Definition of Final Done

- no unresolved Critical/High release findings;
- all release P0/P1 tasks DONE or evidence-based explicitly rejected/deferred;
- protected browser cannot escape direct networking under tested failures;
- browser compromise cannot reach long-lived trusted capabilities in the supported threat model;
- supported clean builds have explicit reproducible prerequisites;
- Servo required site capabilities pass;
- Windows and Android production runtime paths are proven;
- SecureAcces revocation works end-to-end;
- ProtectedService registry is authoritative/versioned/validated/audited;
- multiple services coexist behind one WebGate Server Gateway;
- clients cannot forge tenant/workspace/permission/upstream authority;
- Admin can manage users, services, access, devices, sessions, releases, delivery, audit and health;
- User×Service matrix exactly projects SecureAcces authorization;
- gateway is proven not to be an open proxy/SSRF pivot within the supported threat model;
- device proof-of-possession and lifecycle work end-to-end;
- primary + independent fallback transports survive chaos without changing authorization semantics;
- service/membership/user/device/session revocation converges fail closed according to defined timing guarantees;
- immutable release registry exists with clean builds, platform artifacts, digest/signature/provenance and explicit promotion/revocation;
- “Send latest WebGate” selects only a compatible promoted release for the verified user/device target;
- Windows user delivery and Android user delivery pass supported platform gates;
- Telegram direct-file path works within configured provider limits;
- protected short-lived download fallback works for large/unavailable direct delivery;
- arbitrary Telegram chat ID cannot bypass verified identity binding;
- Telegram-delivered binaries are rejected if local release signature/digest verification fails;
- no persistent per-user credential is compiled into release binaries;
- revoked user/device/release is denied immediately before new dispatch/download authorization;
- release promotion/revocation/build/delivery/install history is audited without secret leakage;
- full build→verify→promote→Telegram→download/install→local verify→version report E2E passes;
- critical policy/state/authorization/release/distribution code is mutation-resistant;
- Rust + Go format/build/test/race/lint/security/static gates pass;
- performance/reliability targets pass without security regression;
- signed update/self-update path and Telegram bootstrap/manual-update path use the same release authority;
- documentation matches code;
- final T-027 admin/service adversarial qualification passes;
- final T-028 release/distribution E2E qualification passes;
- final adversarial re-audit finds no fundamental blocker;
- final verified state and synchronized `MASTER_PLAN.md` are in `main`.
