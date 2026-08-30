# WebGate Verified Release and Telegram Distribution

Research snapshot: **2026-08-30**

## 1. Purpose

WebGate must let an administrator deliver the latest safe client build to a trusted user without requiring that user to understand GitHub, CI, VPN setup, package feeds, or manual configuration.

Target administrator flow:

```text
Admin
  ↓
WebGate Admin UI
  ↓
select user / users
  ↓
Send latest WebGate
  ↓
resolve user's registered platform/device
  ↓
select latest PROMOTED + VERIFIED release for that platform/channel
  ↓
Telegram delivery
  ↓
user receives installer/update + version/signature information
  ↓
client/installer verifies artifact trust before installation
```

Telegram is a delivery and notification channel. It is **not** the root of trust for WebGate binaries.

---

## 2. Definition of "latest version"

`latest` must never mean "whatever commit is currently newest on main".

A build is user-deliverable only after it enters a release state such as:

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

Failed/revoked states:

```text
REJECTED
QUARANTINED
REVOKED
SUPERSEDED
```

Only a `PROMOTED`/`AVAILABLE` artifact may be selected by "Send latest version".

The release record binds at least:

- semantic/product version;
- immutable source commit SHA;
- release channel (`stable`, optional `beta`/`test`);
- platform;
- architecture;
- package format;
- artifact SHA-256 or stronger digest profile;
- artifact size;
- signing key ID;
- build timestamp;
- minimum/maximum compatible server protocol where required;
- verification/qualification result;
- promotion actor/time;
- revocation/supersession state.

---

## 3. Build targets

Initial target matrix:

| Platform | Architecture | Preferred package |
|---|---|---|
| Windows | x86_64 | signed installer/MSIX/EXE after packaging decision |
| Windows | arm64 where supported | signed installer |
| Android | arm64 | signed APK; store-compatible package may be added later |
| Linux | x86_64/aarch64 | evidence-backed package/AppImage/deb/rpm strategy |
| macOS | arm64/x86_64 | signed/notarized package after support gate |

The build pipeline must not silently substitute one architecture/package for another.

---

## 4. Release Builder

The release builder consumes an immutable source revision, not a mutable branch reference after the build begins.

```text
approved source SHA
       ↓
clean build environment
       ↓
platform build matrix
       ↓
tests / lint / security gates
       ↓
package
       ↓
sign
       ↓
generate manifest + checksums
       ↓
release qualification
       ↓
promote
```

Required properties:

- exact source SHA recorded;
- locked dependencies;
- isolated clean runners/build environments;
- no runtime WebGate user secrets in build context;
- signing credentials isolated from untrusted browser/page code;
- artifact digest computed after final packaging/signing stage as defined by the platform contract;
- provenance/attestation added when practical;
- failed build cannot overwrite a previous good artifact;
- rebuilding the same version cannot silently replace the already-promoted artifact with different bytes;
- a promoted release is immutable; fixes use a new release record/version.

---

## 5. Release Manifest

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
  "size": 12345678,
  "signing_key_id": "release-2026-01",
  "created_at": "...",
  "min_server_protocol": 1,
  "signature": "..."
}
```

The exact wire format may differ, but these security semantics are mandatory.

The client/installer must verify release trust independently of Telegram metadata or filename.

---

## 6. Telegram identity binding

A WebGate administrator must not type an arbitrary Telegram `chat_id` and thereby gain an unaudited binary-distribution primitive.

Preferred model:

```text
SecureAcces Account
       │
verified Telegram ExternalIdentity
       │
WebGate notification binding
       │
Telegram recipient
```

Delivery resolves the recipient from already verified server-side identity state.

Manual override, if ever required, must be a separate privileged workflow with explicit verification and audit.

Never infer authorization from Telegram username display text.

---

## 7. Delivery modes

### 7.1 Direct Telegram document

For artifacts within the configured Telegram transport limit:

```text
Release Service
     ↓
Telegram Delivery Adapter
     ↓
sendDocument
     ↓
user chat
```

The cloud Bot API currently supports documents up to 50 MB. WebGate must treat this as a configurable provider capability rather than a permanent invariant because Telegram may change the limit.

For larger direct uploads, an administrator may optionally deploy Telegram's local Bot API Server. The official server currently supports uploads up to 2000 MB.

WebGate must discover/configure these limits explicitly and test the selected mode.

### 7.2 Signed short-lived download link

For artifacts too large for the configured Telegram path, or when release hosting is preferred:

```text
Telegram message
    ↓
short-lived download capability
    ↓
WebGate Release Download API
    ↓
authorized artifact
```

Requirements:

- opaque high-entropy capability or authenticated download session;
- short TTL;
- optional one-time or bounded-use policy;
- bound to release/user/device context when practical;
- no server filesystem path in URL;
- no long-lived bearer URL;
- rate limiting;
- revocation support;
- audit delivery/download separately;
- downloaded bytes still require release signature/hash verification.

### 7.3 Telegram `file_id` reuse

If the exact immutable artifact has already been uploaded by the same bot, Telegram `file_id` may be cached to avoid repeated uploads.

`file_id` is a transport optimization only. Release identity continues to be the WebGate release ID + cryptographic digest/signature.

---

## 8. User message UX

Preferred update message:

```text
WebGate 1.2.3
Windows x64 · Stable

A new verified WebGate version is available.

[Install / Download]
[What changed]
[Need help]
```

The message may include:

- version;
- platform;
- release channel;
- short human-readable change summary;
- artifact or download button;
- activation button for a new device when required.

Do not include:

- VPN private keys;
- session bearer tokens;
- device private keys;
- raw service credentials;
- long-lived admin capabilities.

---

## 9. New-user bootstrap vs update

These are different operations.

### New user / new installation

```text
admin approves user
      ↓
select platform or wait for device claim
      ↓
send generic signed installer
      +
short-lived activation/bootstrap capability
      ↓
installation generates its own device key
      ↓
SecureAcces/WebGate activation
```

The binary remains generic. Do not compile a permanent user credential into an individualized executable.

### Existing active device update

```text
admin: Send latest
      ↓
server resolves registered device platform/arch
      ↓
latest compatible promoted release
      ↓
Telegram update delivery
      ↓
client verifies + installs/launches update flow
```

---

## 10. Admin UI

Add a **Releases** section with:

- current stable/beta versions;
- source SHA;
- build/verification state;
- platform artifacts;
- signatures/checksums;
- promotion/revocation controls;
- delivery history;
- failed delivery diagnostics;
- rollout status by user/device.

User page actions:

```text
Send latest WebGate
Send activation package
Resend release
View delivery history
```

Bulk operations:

```text
Send latest to selected users
Send latest to all active users
Send latest to devices below minimum version
```

Bulk send must show a preview containing:

- number of recipients;
- platforms/architectures;
- selected release per platform;
- users with no verified Telegram binding;
- incompatible/unknown devices;
- expected direct-file vs download-link mode.

No bulk delivery executes before authorization and preview validation.

---

## 11. Rollout policy

Support at least:

- manual one-user delivery;
- selected-user delivery;
- all-active-users delivery;
- version-compliance rollout;
- staged/canary rollout later or when user count justifies it.

Default production policy should separate:

```text
build != verify != promote != deliver
```

A successful compilation alone must never trigger automatic mass distribution unless an explicit signed/admin policy enables such behavior.

---

## 12. Release revocation

Administrator must be able to mark a release `REVOKED`.

Effects:

- no new Telegram delivery;
- no new signed-link issuance;
- update service no longer recommends it;
- Admin UI shows affected users/devices;
- optionally notify users who received/downloaded the bad release;
- rollback/forward-fix recommendation follows explicit client update policy.

Release revocation does not magically remove already installed binaries. The client/server minimum-version and deny-version policy must handle dangerous versions explicitly.

---

## 13. Audit events

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

Audit metadata may contain IDs, versions, platform/arch and result codes, but not raw bot tokens, signed-link secrets or user credentials.

---

## 14. Failure handling

Required cases:

- Telegram unavailable;
- recipient blocked the bot;
- missing verified Telegram identity;
- Telegram rate limit;
- artifact exceeds provider limit;
- upload interrupted;
- stale cached `file_id`;
- download-link issuance failure;
- artifact storage unavailable;
- incompatible platform/architecture;
- revoked release raced with delivery;
- user/device revoked during delivery;
- signature/digest mismatch;
- installer/update failed on the device.

Retryable transport errors use bounded backoff/idempotency. Security/policy failures are not blindly retried.

---

## 15. Security invariants

- Telegram is not a binary trust root.
- Only promoted/available immutable releases can be sent as "latest".
- Artifact signature/digest verification is independent of message authenticity.
- Telegram bot token is server-side secret material and never reaches client/browser content.
- Release signing key is separate from Telegram bot credentials, device keys, policy keys and transport keys.
- No per-user long-lived credential is compiled into the release binary.
- Delivery recipient is resolved from verified identity state.
- Revoked user/device/release state is checked immediately before dispatch/download authorization.
- A filename/version string cannot select artifact bytes without matching immutable release metadata.
- The server cannot replace bytes under an already-promoted release ID/version without detection.
- Delivery history and privileged promotion/revocation actions are audited.

---

## 16. Verification and tests

Required tests include:

### Build/release

- immutable SHA input;
- build failure cannot promote;
- verification failure cannot promote;
- digest mismatch blocks promotion/delivery;
- wrong signing key/profile blocks client acceptance;
- same version/different bytes cannot silently replace promoted release;
- revoked release cannot be newly delivered.

### Platform selection

- Windows user gets Windows artifact;
- Android user gets Android artifact;
- wrong architecture is denied;
- unknown platform produces a clear admin-visible state rather than a guessed package.

### Telegram delivery

- verified recipient mapping;
- blocked bot;
- rate limit;
- direct-file size boundary;
- local Bot API capability configuration;
- signed-link fallback;
- idempotent retry avoids duplicate unintended messages where possible;
- `file_id` cache maps only to the exact immutable release artifact.

### Authorization

- non-admin cannot promote/revoke/send;
- admin cannot send to a tenant/user outside management authority;
- revoked user/device fails closed;
- arbitrary chat ID cannot bypass identity binding;
- CSRF/IDOR/replay/concurrency coverage for Admin release APIs.

### E2E

```text
commit → build → verify → promote → admin selects user →
resolve device/platform → Telegram delivery → download/install →
client verifies release → client reports version
```

The final adversarial release qualification must be part of the WebGate release gate.

---

## 17. Implementation placement

Recommended server packages/modules:

```text
server/
├── release/              release domain/state machine
├── buildregistry/        immutable build/provenance records
├── artifactstore/        artifact storage abstraction
├── distribution/         delivery orchestration
├── telegram/             Telegram delivery adapter
├── download/             short-lived release download API
└── admin/                release administration API
```

Exact repository layout may evolve, but domain boundaries should remain independent from SecureAcces internals and from the Telegram provider implementation.

---

## 18. Relationship to client self-update

Telegram delivery is especially useful for:

- first installation;
- manual recovery;
- administrator-driven update;
- notification that a newer version exists.

Long term, an already-installed WebGate client should also be able to consume the same signed release metadata through its protected control plane and update without needing Telegram for every release.

Both paths must use the **same release authority, same immutable artifacts, and same signature verification rules**.
