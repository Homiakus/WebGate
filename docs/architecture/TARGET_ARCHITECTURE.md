# WebGate Target Architecture

Research snapshot: **2026-08-29**

Canonical browser engine: **Servo**. See `docs/architecture/ADR-0001-BROWSER-ENGINE.md`.

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
private document opens in Servo
```

The user should not have to understand VPNs, endpoints, routes, browser engines or failover.

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
│                              Rust                                 │
│                                                                    │
│  ┌────────────────┐     ┌──────────────────────┐                  │
│  │ Deep-link gate │────►│ Navigation Policy    │                  │
│  └────────────────┘     └──────────┬───────────┘                  │
│                                     │                              │
│                         ┌───────────▼───────────┐                  │
│                         │ Servo browser engine │                  │
│                         │ Servo / WebView      │                  │
│                         └───────────┬───────────┘                  │
│                                     │ HTTP/HTTPS proxy             │
│                            ┌────────▼─────────┐                    │
│                            │ Local Proxy Gate │                    │
│                            │ allowlist only   │                    │
│                            │ fail closed      │                    │
│                            └────────┬─────────┘                    │
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
│  - Servo compatibility gate                                        │
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

Optional compatibility path, not part of the default data path:

```text
webgate-browser interface
       ├── Servo adapter       ← default
       └── WebView2 adapter    ← explicit policy-controlled fallback only
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
- invoke arbitrary native commands;
- select or switch browser engines;
- change Servo network/proxy policy.

Native capabilities exposed to the page require an explicit capability allowlist and a narrow typed interface.

---

## Boundary C — Servo adapter → transport process

Servo does not talk to raw transport implementations directly.

The browser receives only a WebGate-owned protected proxy endpoint and browser policy. Transport processes are supervised and least-privileged.

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

The proxy binds to loopback and starts in fail-closed mode. This prevents the transport path from becoming an accidental machine-wide circumvention/general Internet proxy.

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
├── webgate-app/               native lifecycle/UI composition
├── webgate-browser/           engine-independent browser contract
├── webgate-browser-servo/     canonical Servo integration
├── webgate-browser-webview2/  optional compatibility adapter
├── webgate-deeplink/          strict URI parsing + single instance dispatch
├── webgate-config/            schemas, signed bundle, migration
├── webgate-crypto/            signatures/fingerprints/secret wrappers
├── webgate-identity/          device identity + session vault interface
├── webgate-policy/            hard local policy + remote signed policy
├── webgate-transport/         provider traits + health/failover
├── webgate-supervisor/        child lifecycle/IPC/restart
├── webgate-health/            state machines and diagnostics
├── webgate-updater/           signed update policy
├── webgate-observability/     redacted tracing
└── webgate-platform/          DPAPI/Windows integration
```

The Servo adapter owns:

- Servo engine lifecycle;
- `WebView` creation;
- `WebViewDelegate` integration;
- rendering context;
- event-loop wake/spin integration;
- input/resize/DPI/IME plumbing;
- browser storage/session location;
- proxy configuration;
- navigation/delegate callbacks translated into engine-independent WebGate events.

Principle:

```text
UI cannot configure raw transport.
Browser cannot read secrets.
Servo cannot bypass the WebGate proxy for protected traffic.
Transport cannot change hard UI/security policy.
Remote policy cannot weaken compiled hard invariants.
Web content cannot select browser engine or transport.
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
2. local proxy gate healthy;
3. secure transport established;
4. relay reachable;
5. origin reachable;
6. protected health endpoint returns expected authenticated response semantics.

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
- persist only non-sensitive endpoint health hints;
- transport failover must not recreate Servo or alter its security model.

---

# 7. Servo fail-closed rules

The WebGate local proxy is created first in a fail-closed state. Servo protected networking is configured to use it before protected navigation is allowed.

If transport is unavailable:

```text
Servo protected navigation
        ↓
WebGate local proxy
        ↓
transport unavailable
        ↓
DENY + WebGate offline/error UI
```

Never:

```text
transport failed
      ↓
remove Servo proxy / use direct network
      ↓
direct connection
```

Never:

```text
Servo page/render failure
      ↓
silently open protected URL in WebView2/system browser
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

External-browser navigation must not include protected credentials or protected-only URLs that expose sensitive information.

---

# 8. Servo compatibility contract

Servo is required to support the **WebGate documents application**, not arbitrary websites.

Maintain a machine-readable feature matrix:

```text
feature                 requirement   status
------------------------------------------------
auth cookies            REQUIRED      PASS/FAIL
fetch/XHR               REQUIRED      PASS/FAIL
forms                   REQUIRED      PASS/FAIL
site CSS/layout         REQUIRED      PASS/FAIL
Cyrillic/IME            REQUIRED      PASS/FAIL
document navigation     REQUIRED      PASS/FAIL
printing                OPTIONAL/...  PASS/FAIL
WebSocket               OPTIONAL/...  PASS/FAIL
```

Every Servo upgrade must run:

- feature compatibility suite;
- visual regression suite;
- network-escape tests;
- performance regression tests;
- crash/recovery tests.

Prefer a Servo LTS line for production after qualification. Current releases may be tested continuously before promotion.

---

# 9. Origin topology

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

# 10. Relay requirements

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

# 11. Bootstrap and policy model

Two configuration classes:

## 11.1 Bootstrap bundle

Short-lived, manually delivered once.

Purpose:

- locate initial control-plane endpoints;
- convey one-time enrollment capability;
- identify policy-signing roots/schema.

It expires and becomes useless after activation.

## 11.2 Remote signed policy

Longer-lived and refreshable.

Contains:

- allowed web origins;
- relay endpoint descriptors;
- transport preferences;
- minimum client version;
- browser compatibility/fallback policy;
- feature policy;
- certificate/key IDs;
- policy version and expiry.

It does **not** contain the device private key.

Remote policy may disable optional compatibility fallback but may not silently replace Servo for protected browsing.

---

# 12. Local hard policy vs remote policy

Some values must be impossible to weaken remotely.

Compiled hard invariants:

```text
primary_browser_engine = servo
fail_closed = true
allow_direct_protected_origin = false
allow_silent_engine_fallback = false
require_policy_signature = true
allow_unsigned_update = false
allow_browser_devtools_release = false
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

# 13. Device lifecycle

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

# 14. Credentials

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

# 15. SecureAcces placement

SecureAcces is behind the transport, inside the trusted server application boundary.

Recommended request path:

```text
Servo/WebGate request
    ↓
WebGate protected transport
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

# 16. Availability model

The design explicitly protects against:

| Failure | Expected behavior |
|---|---|
| origin public IP changes | no client impact |
| origin behind CGNAT | supported |
| Relay A outage | automatic Relay B failover |
| AWG path blocked/degraded | switch to independent fallback |
| UDP restricted | TCP/443-class fallback |
| transport process crash | supervisor restart/failover |
| Servo crash | browser capsule recovers without exposing direct path |
| Servo unsupported feature | explicit controlled error; never direct/silent engine fallback |
| policy endpoint A down | endpoint B / cached valid policy |
| expired policy with no refresh | fail closed according to expiry policy |
| user session revoked | protected requests denied by SecureAcces |
| one device compromised | revoke device/session without affecting other users |

---

# 17. Cached operation

For resilience, WebGate may cache the last valid signed policy.

Rules:

- cache only after signature verification;
- preserve original expiry;
- never extend expiry locally;
- record monotonic version to defend against rollback;
- if policy is expired beyond an explicitly allowed grace window, fail closed;
- bootstrap secrets are never part of the long-term cache.

---

# 18. DNS strategy

Protected host resolution should not rely solely on the local ISP resolver.

Preferred model:

```text
Servo sends protected hostname through WebGate proxy policy
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

# 19. Update architecture

Application and Servo-engine updates are part of the trust chain.

Signed release manifest should include:

```text
client version
Servo version/LTS line
platform/arch
package hash
sidecar hashes
minimum previous version (if needed)
release channel
signature/key id
```

At startup, WebGate verifies bundled sidecars before execution.

If a sidecar hash does not match the signed manifest, the transport is not started.

Servo upgrades are promoted only after compatibility, visual, security and performance regression suites pass.

---

# 20. Security testing requirements

Required before production:

- deep-link parser property tests;
- config signature fuzzing;
- config migration tests;
- Servo navigation allowlist bypass tests;
- Servo proxy/direct-fallback negative tests;
- Servo compatibility tests for every REQUIRED site feature;
- Servo visual regression tests;
- Servo crash/recovery tests;
- IDN/punycode/Unicode hostname tests;
- local proxy destination bypass tests;
- DNS leak tests;
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

# 21. Architectural verdict

The target system should be considered three cooperating planes:

```text
APPLICATION PLANE
Rust shell + Servo + browser/navigation policy

TRANSPORT PLANE
restricted local proxy + pluggable resilient transports + relays

AUTHORIZATION PLANE
SecureAcces + document resource ownership/permissions
```

Keeping these planes separate is the main architectural rule for WebGate.

It allows transport technology to evolve rapidly without destabilizing either the Servo browser capsule or the authorization model.

The canonical browser rule is:

> **Servo is primary. WebView2 exists only as an explicit compatibility adapter if a documented production requirement cannot be met by Servo.**
