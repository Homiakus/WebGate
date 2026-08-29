# WebGate ↔ SecureAcces Integration

Research snapshot: **2026-08-29**

Target dependency: [`Homiakus/SecureAcces`](https://github.com/Homiakus/SecureAcces)

## 1. Boundary decision

**SecureAcces stays on the server. WebGate stays on the client.**

Do not compile the Go authorization library into the Rust desktop client and do not duplicate its domain model in Rust.

The correct boundary is HTTP/API:

```text
WebGate Client (Rust)
      │
      │ protected transport
      ▼
WebGate / Docs API (Go)
      │
      ├── SecureAcces session authentication
      ├── SecureAcces Authorize(...)
      ├── WebGate device registry
      └── protected document service
```

SecureAcces answers **who may access which resource**. WebGate's transport answers **how the trusted client reaches that server without exposing the origin or affecting unrelated OS traffic**.

---

# 2. Capabilities already available in SecureAcces

The current repository already has the major domain concepts WebGate requires:

- `Account` — global identity;
- tenant-local `User`;
- `Workspace`;
- `Membership` with permission bits;
- `ExternalIdentity`;
- short-lived server-side `Session`;
- enrollment;
- login challenge;
- audit events;
- provider-agnostic `IdentityProvider`;
- `Authorize(ctx, principal, Resource, PermissionBits)`;
- HTTP hardening middleware;
- Telegram identity/provider adapters;
- session and account revocation.

Relevant source files:

- `SecureAcces/secureaccess/interfaces.go`
- `SecureAcces/secureaccess/types.go`
- `SecureAcces/secureaccess/service.go`
- `SecureAcces/httpauth/`
- `SecureAcces/adapter/telegram/`

SecureAcces deliberately separates persistent authorization from short-lived authentication credentials. That is exactly the model WebGate should preserve.

---

# 3. Immediate compatibility — no SecureAcces change required

WebGate v1 can integrate with SecureAcces without modifying it.

## Login/session sequence

```text
WebGate
  │
  │ authenticate using configured provider
  ▼
Go API
  │
  ├─ SecureAcces.LoginWithProvider(..., deviceID)
  │
  └─ returns one-time session token
            │
            ▼
      WebGate stores token
      in OS protected storage
```

Every subsequent protected HTTP request:

```text
request
  ↓
AuthenticateSession(token)
  ↓
server resolves authoritative Resource
  ↓
Authorize(principal, resource, permission)
  ↓
ALLOW / DENY
```

This matches SecureAcces's existing fail-closed model.

## Important rule

The WebGate client must never supply authoritative `TenantID`, `WorkspaceID` or permission information for authorization.

The server resolves the requested document in its own database and constructs:

```go
secureaccess.Resource{
    TenantID:    storedTenantID,
    WorkspaceID: storedWorkspaceID,
    Kind:        "document",
    ID:          storedDocumentID,
}
```

Only then does it call `Authorize`.

---

# 4. Current device identity gap

SecureAcces currently stores:

```go
type Session struct {
    ...
    DeviceID string
    ...
}
```

`LoginWithProvider(..., deviceID)` checks the size of this string and stores it in the session. It does **not** cryptographically verify that the requester possesses a device key associated with that `DeviceID`.

Therefore:

```text
DeviceID != device authentication
```

For v1 this is acceptable if:

- the real authentication provider is strong;
- the returned session token is protected by DPAPI/OS key storage;
- session TTL and idle TTL are bounded;
- sessions can be remotely revoked;
- `DeviceID` is the stable fingerprint/identifier used for audit and administration.

For the hardened target architecture, possession of a device key should be proven.

---

# 5. Recommended v1 device model

On first successful WebGate activation:

```text
1. Client generates Ed25519 device keypair locally.
2. Private key enters DPAPI-protected storage.
3. Client computes device key fingerprint.
4. Fingerprint becomes SecureAcces deviceID for audit/session display.
5. Existing SecureAcces authentication flow establishes the session.
6. Session token enters DPAPI-protected storage.
```

No device private key is sent to Telegram, saved in `.webgate` config or persisted as plaintext.

This makes the migration to first-class proof-of-possession later straightforward.

---

# 6. Recommended hardened device extension

The clean target is a separate **WebGate Device Registry** plus a SecureAcces identity-provider adapter.

Do not overload `Session.DeviceID` with device credential state.

Proposed server domain:

```go
type Device struct {
    ID              string
    AccountID       secureaccess.ID
    PublicKey       []byte
    KeyAlgorithm    string
    Name            string
    Status          DeviceStatus
    CreatedAt       time.Time
    LastSeenAt      time.Time
    RevokedAt       *time.Time
}
```

Device status:

```text
PENDING
ACTIVE
SUSPENDED
REVOKED
```

Proof sequence:

```text
WebGate                    Server
   │                          │
   ├──── request nonce ──────►│
   │                          │
   │◄──── nonce/context ──────┤
   │                          │
   ├─ sign(device private key)►
   │                          │
   │                  verify public key
   │                          │
   │                  map Device → Account
   │                          │
   │                  issue/refresh session
```

The signature context must bind at least:

- protocol version;
- server/audience;
- challenge nonce;
- device ID;
- timestamp/expiry;
- requested operation.

Never sign a bare random nonce without domain separation.

---

# 7. Why not model every device as a normal SecureAcces account identity today

SecureAcces's current enrollment finalization is designed to bind an unbound tenant `User` to a newly created global `Account` and `ExternalIdentity`.

A user can have multiple verified identities in storage, and the `Store` interface includes `CreateIdentity` / `ListIdentitiesByAccount`, but the reviewed public `Service` surface does not currently expose a dedicated management operation for safely adding another external identity to an already-existing account.

Therefore WebGate should not bypass the service layer and call `Store.CreateIdentity` directly merely to register devices.

Two safe choices:

1. **v1:** keep device registry in WebGate and SecureAcces authentication separate;
2. **later:** add a reviewed SecureAcces service operation / provider flow for binding an additional identity to an authenticated account.

The first option avoids forcing a premature change into SecureAcces.

---

# 8. Enrollment bundle vs SecureAcces Enrollment

These are separate concepts and should have separate names.

## SecureAcces Enrollment

Purpose:

> bind a verified external human identity to an account/user authorization model.

## WebGate Device Bootstrap Bundle

Purpose:

> teach one installation how to find the control plane and how to perform one-time device bootstrap.

Recommended extension:

```text
*.webgate
```

Conceptual payload:

```json
{
  "schema": "webgate.bootstrap/v1",
  "bundle_id": "...",
  "control_plane": [
    "https://control-a.example",
    "https://control-b.example"
  ],
  "enrollment_token": "one-time-secret",
  "expires_at": "...",
  "policy_key_id": "root-2026-01",
  "signature": "..."
}
```

The bootstrap bundle must **not** contain the long-lived device private key.

The bootstrap token:

- expires quickly;
- is one-time use;
- is bound server-side to the intended SecureAcces account/user or enrollment action;
- cannot itself authorize document access after activation.

---

# 9. Suggested onboarding flows

## A. Admin-preapproved user — preferred for a very small trusted group

```text
Admin
  ↓
creates/approves SecureAcces user
  ↓
creates WebGate bootstrap token
  ↓
sends .webgate bundle
  ↓
client activates
  ↓
SecureAcces auth/provider verification
  ↓
device registered
  ↓
session issued
```

## B. Telegram-assisted activation

SecureAcces already has Telegram adapters and challenge/enrollment support.

Possible UX:

```text
WebGate shows activation request
        ↓
Telegram bot asks user to approve
        ↓
SecureAcces verifies provider assertion
        ↓
server completes WebGate device activation
```

Telegram remains an identity/approval channel. It never carries the document bytes or permanent device secrets.

---

# 10. Session storage

On Windows:

```text
SecureAcces session token
       ↓
DPAPI user-scope protection
       ↓
WebGate local credential store
```

Never place the session token in:

- `.webgate` bootstrap file;
- application command line;
- environment variable inherited by arbitrary children;
- log file;
- URL query string;
- crash-report metadata.

The WebView may use an HttpOnly secure cookie issued by the server, or WebGate may maintain an API token outside page JavaScript. The choice depends on the existing document site's architecture.

For a normal browser-based documentation site, SecureAcces's hardened `__Host-` cookie middleware is the preferred baseline.

---

# 11. Resource permissions mapping

Recommended initial mapping:

| Site action | SecureAcces permission |
|---|---|
| open/view document | `PermView` |
| download original/PDF | `PermDownload` |
| upload attachment | `PermUpload` |
| edit document | `PermEdit` |
| delete document | `PermDelete` |
| manage workspace users | `PermManageMembers` |
| manage workspace | `PermManageWorkspace` |

Do not create WebGate-specific duplicate permission bits unless a requirement cannot be represented by SecureAcces.

---

# 12. Revocation semantics

WebGate must expose two distinct revocation operations.

## Revoke a session

Immediate loss of current login, but device may authenticate again.

Use SecureAcces session revocation.

## Revoke a device

Device public key is disabled and it cannot establish new sessions.

Use WebGate Device Registry, then revoke associated SecureAcces sessions.

## Revoke an account/user/membership

SecureAcces remains authoritative.

The current SecureAcces `Authorize` path re-checks session/account and rebuilds effective access, which is desirable for immediate fail-closed permission revocation.

---

# 13. Audit integration

WebGate security events should be reflected into SecureAcces audit where they refer to protected resources or authenticated accounts.

Examples:

```text
WEBGATE_DEVICE_REGISTERED
WEBGATE_DEVICE_REVOKED
WEBGATE_DEVICE_LOGIN
WEBGATE_TRANSPORT_FALLBACK
DOCUMENT_VIEWED
DOCUMENT_DOWNLOADED
```

Transport telemetry should remain a separate operational log when it is not an authorization event.

Do not place IP addresses, endpoint topology or transport secrets into broadly visible tenant audit data without a requirement.

---

# 14. API boundary recommendation

Do not expose SecureAcces internals directly to the Rust client.

Create a small WebGate-oriented API:

```text
POST /v1/bootstrap/claim
POST /v1/device/challenge
POST /v1/device/activate
POST /v1/session/refresh
POST /v1/session/revoke
GET  /v1/me
GET  /v1/policy
GET  /v1/transport/endpoints
```

Protected document routes continue to use normal site APIs and SecureAcces HTTP middleware.

The server translates between WebGate wire DTOs and SecureAcces domain calls.

This gives SecureAcces freedom to evolve without locking its internal Go structs into a public Rust protocol.

---

# 15. Compatibility verdict

**WebGate and SecureAcces are architecturally compatible.**

SecureAcces is already a strong fit for:

- human/account identity;
- memberships;
- resource authorization;
- sessions;
- Telegram-assisted identity flows;
- audit;
- revocation.

WebGate should add, outside the existing authorization core:

- device registry;
- signed bootstrap/policy format;
- device proof-of-possession;
- transport endpoint policy;
- desktop lifecycle/security.

The only likely future SecureAcces enhancement is a clean service-level mechanism for binding/removing additional identity credentials to an existing authenticated account if WebGate device keys are eventually promoted into SecureAcces `ExternalIdentity` objects.
