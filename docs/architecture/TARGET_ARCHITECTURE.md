# WebGate Target Architecture

Research snapshot: **2026-08-29**

## 1. Product definition

WebGate is a **protected browser capsule** for a small set of trusted users.

The user experience should be:

```text
open trusted link
      ↓
WebGate starts/activates
      ↓
secure transport is selected automatically
      ↓
private document opens
```

The user should not have to understand VPNs, endpoints, routes or failover.

---

# 2. System architecture

```text
                           ┌─────────────────────────┐
                           │ Telegram / HTTPS link   │
                           └────────────┬────────────┘
                                        │
                                        ▼
┌────────────────────────────────────────────────────────────────────┐
│                          WebGate Client                            │
│                         Rust / Tauri 2                             │
│                                                                    │
│  ┌────────────────┐     ┌──────────────────────┐                  │
│  │ Deep-link gate │────►│ Navigation Policy    │                  │
│  └────────────────┘     └──────────┬───────────┘                  │
│                                     │                              │
│                            ┌────────▼────────┐                     │
│                            │ WebView2 / Wry  │                     │
│                            └────────┬────────┘                     │
│                                     │ per-WebView SOCKS/HTTP       │
│                            ┌────────▼────────┐                     │
│                            │ Local Proxy Gate │                     │
│                            │ allowlist only   │                     │
│                            └────────┬────────┘                     │
│                                     │                              │
│                     ┌───────────────▼────────────────┐             │
│                     │ Transport Supervisor / Policy │             │
│                     └───────┬───────────────┬────────┘             │
│                             │               │                      │
│                     Primary │               │ Fallback             │
│                             ▼               ▼                      │
│                    Outline/AWG          Xray provider              │
│                                                                    │
│  Other security services:                                         │
│  - signed config/policy                                            │
│  - DPAPI credential vault                                          │
│  - updater verification                                            │
│  - health state machine                                            │
│  - structured/redacted logs                                        │
└───────────────────────────────┬────────────────────────────────────┘
                                │
                     encrypted independent paths
                                │
                 ┌──────────────┴───────────────┐
                 ▼                              ▼
           ┌──────────┐                   ┌──────────┐
           │ Relay A  │                   │ Relay B  │
           │ VPS/ASN A│                   │ VPS/ASN B│
           └────┬─────┘                   └────┬─────┘
                │                              │
                └──────────────┬───────────────┘
                               │ private reverse connectivity
                               ▼
                     ┌─────────────────────┐
                     │ Origin server (RU)  │
                     │ dynamic IP / CGNAT  │
                     │ inbound Internet: X │
                     └─────────┬───────────┘
                               │
                 ┌─────────────┴─────────────┐
                 ▼                           ▼
          WebGate/Docs API             SecureAcces
          protected content       auth/session/authorization
```

---

# 3. Fundamental trust boundaries

## Boundary A — untrusted external link → WebGate

Anything from Telegram, mail, browser or another process is untrusted input.

Deep links may identify a document but never carry an authorization secret.

Allowed:

```text
webgate://open/d/opaque-id
```

Forbidden:

```text
webgate://open?session_token=...
webgate://open?vpn_private_key=...
```

---

## Boundary B — web content → native application

The embedded page is not trusted with arbitrary native capabilities.

The page must not be able to:

- execute processes;
- read local files;
- read transport keys;
- obtain SecureAcces bearer tokens from native storage;
- reconfigure endpoints;
- disable the kill switch;
- invoke arbitrary Tauri commands.

Native IPC commands exposed to the WebView require an explicit capability allowlist.

---

## Boundary C — WebGate UI → transport process

Transport processes are supervised and least-privileged.

Control channel requirements:

- authenticated local IPC;
- versioned protocol;
- no secrets passed in process command-line arguments;
- parent generates a short-lived IPC capability at spawn;
- child accepts commands only from its parent/session;
- crash/restart is safe and idempotent.

Data proxy socket is separate from control IPC.

---

## Boundary D — local proxy → remote network

The local proxy is **destination restricted**.

Default policy:

```text
ALLOW docs.internal.example:443
ALLOW api.internal.example:443
ALLOW control.internal.example:443
DENY  *
```

This prevents the child process from becoming an accidental machine-wide circumvention/general Internet proxy.

---

## Boundary E — transport access → application authorization

Possession of a transport key is not sufficient to read documents.

The origin requires SecureAcces authentication and authorization.

```text
network reachability
      +
valid SecureAcces session
      +
Authorize(resource, permission)
      =
resource access
```

---

# 4. Client component model

Recommended Rust workspace:

```text
crates/
├── webgate-app/            Tauri lifecycle/UI composition
├── webgate-browser/        WebView policy/navigation/downloads
├── webgate-deeplink/       strict URI parsing + single instance dispatch
├── webgate-config/         schemas, signed bundle, migration
├── webgate-crypto/         signatures/fingerprints/secret wrappers
├── webgate-identity/       device identity + session vault interface
├── webgate-policy/         hard local policy + remote signed policy
├── webgate-transport/      provider traits + health/failover
├── webgate-supervisor/     child lifecycle/IPC/restart
├── webgate-health/         state machines and diagnostics
├── webgate-updater/        signed update policy
├── webgate-observability/  redacted tracing
└── webgate-platform/       DPAPI/Windows integration
```

Principle:

```text
UI cannot configure raw transport.
Browser cannot read secrets.
Transport cannot change hard UI/security policy.
Remote policy cannot weaken compiled hard invariants.
```

---

# 5. Transport state machine

Suggested state model:

```text
STOPPED
  ↓
STARTING
  ↓
PROBING
  ↓
CONNECTED_PRIMARY
  │
  ├── degradation ──► FAILING_OVER
  │                       │
  │                       ├──► CONNECTED_BACKUP
  │                       └──► OFFLINE
  │
  └── stop ─────────► STOPPED
```

A transport is not considered healthy merely because a protocol handshake succeeded.

Health levels:

1. local provider alive;
2. secure transport established;
3. relay reachable;
4. origin reachable;
5. protected health endpoint returns expected authenticated response semantics.

This avoids false-positive "VPN connected" states where the actual site is unreachable.

---

# 6. Failover policy

Initial endpoint matrix:

```text
Priority 1: Relay A / Outline-AWG
Priority 2: Relay B / Outline-AWG
Priority 3: Relay A / Xray fallback
Priority 4: Relay B / Xray fallback
```

The exact order should later be dynamically ranked using bounded health observations.

Rules:

- no infinite rapid retry loop;
- exponential/jittered backoff per failed endpoint;
- circuit breaker after repeated failures;
- keep at least one cold/standby independent provider;
- do not switch based on one lost packet;
- recover primary only after a stability window;
- persist only non-sensitive endpoint health hints.

---

# 7. Browser fail-closed rules

The WebView is created with a proxy only after the local proxy is ready.

If transport is unavailable:

```text
WebView protected navigation
        ↓
local proxy unavailable/failing
        ↓
WebGate offline page
```

Never:

```text
transport failed
      ↓
remove WebView proxy
      ↓
direct connection
```

The direct path is not a fallback.

External links are a different operation:

```text
protected page → https://external.example
          ↓
WebGate asks/decides according to policy
          ↓
system browser
          ↓
normal OS Internet
```

---

# 8. Origin topology

The Russian origin server does not require static IP or inbound port forwarding.

It maintains outbound connections/tunnels toward Relay A and Relay B.

```text
Origin RU
   ├──── outbound secure link ───► Relay A
   └──── outbound secure link ───► Relay B
```

Therefore these events should not require client reconfiguration:

- DHCP public IP change;
- router restart;
- CGNAT address change;
- no inbound NAT mapping;
- migration between local ISPs.

The relays have stable public endpoints; the origin does not.

---

# 9. Relay requirements

Relay A and Relay B should ideally differ by:

- hosting provider;
- ASN;
- IP prefix;
- possibly country/region if latency and policy permit;
- control credentials;
- failure domain.

Do not create "two relays" as two VMs on the same physical/provider network and call that high availability.

Each relay should be disposable and reproducible from configuration.

Relays store as little sensitive state as practical.

---

# 10. Bootstrap and policy model

Two configuration classes:

## 10.1 Bootstrap bundle

Short-lived, manually delivered once.

Purpose:

- locate initial control-plane endpoints;
- convey one-time enrollment capability;
- identify policy-signing roots/schema.

It expires and becomes useless after activation.

## 10.2 Remote signed policy

Longer-lived and refreshable.

Contains:

- allowed web origins;
- relay endpoint descriptors;
- transport preferences;
- minimum client version;
- feature policy;
- certificate/key IDs;
- policy version and expiry.

It does **not** contain the device private key.

---

# 11. Local hard policy vs remote policy

Some values must be impossible to weaken remotely.

Compiled hard invariants:

```text
fail_closed = true
allow_direct_protected_origin = false
require_policy_signature = true
allow_unsigned_update = false
allow_webview_devtools_release = false
allow_arbitrary_native_ipc = false
```

Remote policy can become stricter, not weaker.

Example:

```text
compiled origins max set
        ∩
remote allowed origins
        =
effective origins
```

---

# 12. Device lifecycle

```text
UNENROLLED
   ↓
BOOTSTRAP_CLAIMED
   ↓
KEY_GENERATED
   ↓
PENDING_APPROVAL (optional)
   ↓
ACTIVE
   ├──► SUSPENDED
   └──► REVOKED
```

A revoked device cannot restore trust merely by reinstalling WebGate and reusing an old bootstrap file.

---

# 13. Credentials

Separate credentials by purpose:

```text
Device signing key
Transport credential(s)
SecureAcces session token/cookie
Policy root public keys
Update root public keys
```

Do not derive all credentials from one master secret.

Compromise containment is better when credentials can rotate independently.

---

# 14. SecureAcces placement

SecureAcces is behind the transport, inside the trusted server application boundary.

Recommended request path:

```text
WebGate request
    ↓
reverse proxy
    ↓
SecureAcces HTTP/session middleware
    ↓
authoritative document lookup
    ↓
SecureAcces.Authorize
    ↓
document handler
```

The relay/VPN layer does not implement business authorization.

---

# 15. Availability model

The design explicitly protects against:

| Failure | Expected behavior |
|---|---|
| origin public IP changes | no client impact |
| origin behind CGNAT | supported |
| Relay A outage | automatic Relay B failover |
| AWG path blocked/degraded | switch to independent fallback |
| UDP restricted | TCP/443-class fallback |
| transport process crash | supervisor restart/failover |
| WebView process crash | app recovers without exposing direct path |
| policy endpoint A down | endpoint B / cached valid policy |
| expired policy with no refresh | fail closed according to expiry policy |
| user session revoked | protected requests denied by SecureAcces |
| one device compromised | revoke device/session without affecting other users |

---

# 16. Cached operation

For resilience, WebGate may cache the last valid signed policy.

Rules:

- cache only after signature verification;
- preserve original expiry;
- never extend expiry locally;
- record monotonic version to defend against rollback;
- if policy is expired beyond an explicitly allowed grace window, fail closed;
- bootstrap secrets are never part of the long-term cache.

---

# 17. DNS strategy

Protected host resolution should not rely solely on the local ISP resolver.

Preferred model:

```text
WebView sends hostname to local proxy
      ↓
transport provider resolves protected hostname
      ↓
protected/resilient DNS path
```

The endpoint bootstrap layer should support multiple ways to resolve relay endpoints because the transport cannot depend entirely on itself to discover its first hop.

Bootstrap options may include:

- signed literal endpoint IP candidates;
- multiple hostnames/providers;
- system DNS plus independent DoH bootstrap;
- cached last-known-good endpoint set.

---

# 18. Update architecture

Application update is part of the trust chain.

Signed release manifest should include:

```text
client version
platform/arch
package hash
sidecar hashes
minimum previous version (if needed)
release channel
signature/key id
```

At startup, WebGate verifies bundled sidecars before execution.

If a sidecar hash does not match the signed manifest, the transport is not started.

---

# 19. Security testing requirements

Required before production:

- deep-link parser property tests;
- config signature fuzzing;
- config migration tests;
- navigation allowlist bypass tests;
- IDN/punycode/Unicode hostname tests;
- local proxy destination bypass tests;
- DNS leak tests;
- direct-fallback negative tests;
- child-process IPC authentication tests;
- transport failover chaos tests;
- session revocation integration tests against SecureAcces;
- malicious/expired/rollback policy tests;
- updater tamper tests;
- Windows reinstall/user-profile/key-store cases;
- network transition tests: Wi-Fi → Ethernet → hotspot;
- suspend/resume;
- router/IP change during active session;
- Relay A hard failure;
- blocked UDP;
- TLS interception/proxy environments.

---

# 20. Architectural verdict

The target system should be considered three cooperating planes:

```text
APPLICATION PLANE
Tauri/Wry browser + UX + local policy

TRANSPORT PLANE
restricted local proxy + pluggable resilient transports + relays

AUTHORIZATION PLANE
SecureAcces + document resource ownership/permissions
```

Keeping these planes separate is the main architectural rule for WebGate.

It allows transport technology to evolve rapidly without destabilizing either the browser UX or the authorization model.
