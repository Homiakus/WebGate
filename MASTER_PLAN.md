# WebGate — Master Implementation Plan

Research baseline: **2026-08-29**

This is the living implementation plan for WebGate. It is intentionally ordered by risk: prove browser-scoped routing and fail-closed semantics first, then add real transports, then identity/control-plane integration, then production resilience.

## 0. Architecture invariants

These rules are non-negotiable unless an explicit ADR changes the product model:

1. Protected browser traffic is application-scoped by default; WebGate does not change the OS default route.
2. Transport loss must fail closed. The protected WebView never silently retries through direct Internet.
3. External browser/navigation traffic is separate from protected traffic.
4. WebGate does not implement business authorization; SecureAcces remains authoritative server-side.
5. Links identify resources; they are not long-lived bearer credentials.
6. Device private keys are generated on-device and never distributed in configuration files.
7. All bootstrap bundles, remote policies and updates are signed.
8. Remote policy may tighten but may not weaken compiled hard security invariants.
9. Transport implementations are replaceable providers, not dependencies of browser/business code.
10. Production must have at least two materially independent network paths/failure domains.

---

# Phase 0 — Research and contracts

Status: **IN PROGRESS / baseline committed**

Completed:

- tooling audit;
- target architecture;
- SecureAcces compatibility analysis;
- primary/fallback transport shortlist;
- trust-boundary definition;
- fail-closed requirements;
- initial resilience model.

Remaining:

- formal threat model (STRIDE-style plus abuse cases);
- ADR for app-local proxy vs system TUN;
- ADR for primary transport provider;
- signed configuration wire schema;
- local IPC protocol schema;
- device registration wire schema;
- explicit license/distribution policy for bundled sidecars.

Exit criteria:

- every client/server trust boundary has an owner and test strategy;
- no security-sensitive wire format remains informal;
- bundled dependency licenses are reviewed.

---

# Phase 1 — Windows protected-browser proof of concept

Goal: prove the central product idea before introducing a real VPN protocol.

## 1.1 Rust workspace

Create:

```text
crates/webgate-app
crates/webgate-browser
crates/webgate-policy
crates/webgate-transport
crates/webgate-observability
crates/webgate-platform
```

Use:

- Rust stable;
- Tauri 2;
- Tokio;
- Wry/WebView2 through Tauri;
- `tracing`;
- strict linting.

## 1.2 Mock local proxy

Implement a restricted localhost HTTP CONNECT/SOCKS5 test proxy.

Required behavior:

- bind `127.0.0.1:0`;
- return actual ephemeral port to parent;
- permit only configured test origin;
- reject all other destinations;
- no direct fallback;
- bounded connections/timeouts.

## 1.3 WebView policy

- configure WebView proxy before protected navigation;
- navigation allowlist;
- new-window interception;
- external links open in system browser;
- devtools disabled in release;
- explicit download policy;
- dedicated WebView data directory.

## 1.4 Kill-switch tests

Automated acceptance tests must prove:

```text
proxy alive   → protected site reachable
proxy stopped → protected site NOT reachable directly
```

Also test direct-IP, DNS alias, redirect and subresource attempts.

Exit criteria:

- WebGate opens a protected test site through only its local proxy;
- Chrome/Telegram/system traffic is unaffected;
- stopping the proxy cannot produce direct protected-origin traffic.

---

# Phase 2 — Transport provider SPI and supervision

Goal: make network protocol implementation replaceable.

## 2.1 Provider contract

Implement a Rust-side abstraction similar to:

```rust
pub trait TransportProvider {
    async fn start(&self, policy: TransportPolicy) -> Result<LocalProxy>;
    async fn health(&self) -> TransportHealth;
    async fn reconnect(&self) -> Result<()>;
    async fn stop(&self) -> Result<()>;
}
```

## 2.2 Sidecar supervision

Implement:

- child process launch without secrets on command line;
- authenticated local control channel;
- version handshake;
- child binary hash/signature verification;
- lifecycle bound to parent;
- crash detection;
- bounded restart policy;
- graceful shutdown;
- stdout/stderr redaction strategy.

## 2.3 Health model

Health is layered:

1. provider process alive;
2. local proxy alive;
3. secure remote transport handshake works;
4. relay is reachable;
5. private origin is reachable;
6. protected health/API semantics are correct.

Exit criteria:

- provider process can be killed/restarted without browser escape to direct Internet;
- provider API is protocol-agnostic.

---

# Phase 3 — Primary transport spike: Outline SDK + AmneziaWG

Goal: validate the preferred app-local resilient path.

Create `webgate-transport` Go side service.

Research/implementation tasks:

- integrate Outline SDK dialer/proxy primitives;
- integrate current AmneziaWG v3 module where appropriate;
- expose only a destination-restricted local proxy;
- support multiple relay endpoints;
- remote DNS for protected origins;
- network transition/reconnect behavior;
- metrics for handshake/latency/failure state;
- Windows packaging as a sidecar.

Required experiments:

- Windows 11/Windows 10 where supported;
- Wi-Fi ↔ Ethernet transition;
- public IP change;
- router reconnect;
- UDP impairment/block;
- relay process restart;
- packet loss/high latency;
- origin unavailable while relay remains reachable.

Decision gate:

Adopt as primary only if it meets fail-closed, stability, packaging and recovery requirements. Otherwise retain the provider interface and choose another primary without changing browser code.

---

# Phase 4 — Independent fallback transport

Goal: eliminate single-protocol/single-implementation dependency.

Initial candidate: Xray-core provider.

Requirements:

- local restricted SOCKS/HTTP interface;
- TCP/443-class fallback path;
- independent protocol/implementation family from primary AWG path;
- sidecar binary verification;
- same health/supervisor contract;
- no direct route manipulation.

Failover order baseline:

```text
Relay A / Primary
Relay B / Primary
Relay A / Fallback
Relay B / Fallback
```

Add:

- per-endpoint circuit breaker;
- jittered exponential backoff;
- stability window before returning to preferred path;
- hysteresis to prevent transport flapping.

Exit criteria:

- simulated complete primary transport failure automatically recovers through fallback;
- user does not need to select a VPN profile manually.

---

# Phase 5 — Signed bootstrap and device identity

Goal: replace hand-managed VPN configs with safe one-time enrollment.

## 5.1 `.webgate` bootstrap schema

Define canonical signed payload containing only:

- schema version;
- bundle ID;
- control-plane bootstrap endpoints;
- short-lived one-time enrollment token;
- policy root key IDs;
- validity window.

Must not contain:

- long-lived device private key;
- SecureAcces session token;
- reusable permanent VPN private key.

## 5.2 Device key

On first activation:

- generate Ed25519 device keypair locally;
- compute stable public-key fingerprint/device ID;
- protect private key with Windows DPAPI;
- erase transient plaintext secret buffers where practical.

## 5.3 Policy verification

Implement:

- Ed25519 root trust;
- key IDs and rotation;
- schema version/migration;
- expiry;
- monotonic anti-rollback version;
- cached last-known-good signed policy;
- no local extension of expiry.

Exit criteria:

- copying a used/expired bootstrap bundle to another machine cannot recreate trusted access;
- tampering any signed field causes activation failure.

---

# Phase 6 — WebGate control API + SecureAcces integration

Goal: attach the client to the existing authorization system without duplicating it.

## 6.1 Go control API

Suggested surface:

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

## 6.2 SecureAcces v1 integration

Use existing:

- identity providers;
- `LoginWithProvider`;
- `AuthenticateSession`;
- `Authorize`;
- memberships/workspaces;
- revocation;
- audit;
- hardened HTTP middleware.

Use the WebGate device public-key fingerprint as `deviceID` for current session visibility/audit, while keeping actual device proof in WebGate's device registry.

## 6.3 Resource resolution

For every document request:

1. authenticate SecureAcces session;
2. load document from authoritative server store;
3. construct `secureaccess.Resource` from stored tenant/workspace ownership;
4. call `Authorize` with required permission;
5. only then return bytes/metadata.

The client never decides its own tenant/workspace scope.

## 6.4 Device registry

Add server-side device domain with:

- device ID;
- account ID;
- public key;
- status;
- creation/last-seen/revocation timestamps.

Add challenge-response proof of key possession for activation/refresh where appropriate.

Exit criteria:

- account/membership/session revocation is enforced immediately by SecureAcces;
- device revocation prevents new trusted WebGate sessions without affecting unrelated users/devices.

---

# Phase 7 — Relay/origin high availability

Goal: make the lack of a static origin IP irrelevant.

## 7.1 Two independent relays

Provision Relay A/B across different meaningful failure domains:

- different providers;
- different ASNs/prefixes where possible;
- independent credentials;
- reproducible deployment.

## 7.2 Origin server

Origin in Russia:

- no public document service port;
- no dependency on static public IP;
- no inbound NAT requirement;
- outbound secure connectivity to both relays;
- watchdog/reconnect service;
- local reverse proxy only on trusted/private path.

## 7.3 Origin resilience

Test:

- DHCP/public-IP rotation;
- CGNAT;
- router reboot;
- primary ISP loss and optional LTE/5G backup;
- origin service restart;
- one relay loss;
- relay credential rotation.

Exit criteria:

- users keep the same WebGate link/config while origin public addressing changes.

---

# Phase 8 — Deep links and Telegram UX

Goal: one-click user workflow.

Implement:

```text
webgate://open/d/<opaque-id>
```

Prefer public HTTPS launcher links in Telegram:

```text
https://go.example/d/<opaque-id>
```

The launcher contains no document bytes and no long-lived credential.

Requirements:

- strict parser;
- opaque IDs;
- single-instance dispatch;
- no secrets in URL/query;
- replay-safe actions;
- graceful behavior if client is not installed;
- explicit external navigation behavior.

Exit criteria:

```text
Telegram → click → existing/new WebGate → protected document
```

with no manual VPN action.

---

# Phase 9 — Production hardening

## Client security

- release devtools disabled;
- Tauri capability minimization;
- no arbitrary native IPC from page JS;
- CSP/hardened embedded content behavior;
- secure local file permissions;
- DPAPI secret store;
- sensitive memory wrappers;
- structured redacted logs;
- signed sidecars;
- signed update chain.

## Server security

- HTTPS/TLS;
- SecureAcces HTTP hardening;
- least-privileged reverse proxy;
- private database;
- rate limiting;
- brute-force/replay controls;
- audit integrity/retention;
- backup encryption and tested restore.

## Supply chain

Rust:

- `cargo audit`;
- `cargo deny`;
- `cargo nextest`;
- `cargo fuzz`;
- `clippy`;
- SBOM.

Go:

- race detector;
- fuzzing;
- `govulncheck`;
- `staticcheck`;
- SBOM.

All releases:

- reproducible-ish documented build environment;
- signed release artifacts;
- dependency/license inventory;
- sidecar hashes in signed manifest.

---

# Phase 10 — Adversarial and resilience validation

Mandatory test families:

## Network escape

- DNS leak;
- proxy bypass;
- redirect bypass;
- WebSocket/SSE/subresource behavior;
- direct IP origin access;
- IPv4/IPv6 differences;
- PAC/system proxy interactions;
- local malicious process probing proxy port.

## Parser/config

- malformed URI;
- Unicode/IDN confusion;
- duplicate JSON fields/canonicalization cases;
- truncated signature;
- future/old schema;
- expired policy;
- rollback policy;
- unknown signing key.

## Transport chaos

- kill primary sidecar;
- kill relay A;
- UDP blackhole;
- DNS poisoning/failure;
- 10–30% packet loss;
- high jitter;
- suspend/resume;
- laptop network changes;
- origin restart.

## Authz

- revoked SecureAcces session;
- suspended account;
- revoked membership;
- cross-tenant resource ID attempt;
- stale client-side authorization state;
- revoked WebGate device.

Exit criteria:

No test may produce protected content through an unauthorized or direct network path.

---

# Milestones

## M1 — Browser capsule

Tauri/WebView2 + restricted proxy + provable fail-closed behavior.

## M2 — Real resilient transport

Primary provider running through the same browser proxy abstraction.

## M3 — Dual-provider failover

Automatic primary ↔ independent fallback with two relays.

## M4 — Trusted device onboarding

Signed `.webgate` bootstrap + DPAPI device key + remote signed policy.

## M5 — SecureAcces-backed access

Server authentication, membership/resource authorization, session/device revocation.

## M6 — One-click Telegram workflow

Trusted link opens the correct document in WebGate without manual VPN interaction.

## M7 — Production release candidate

Signed installer/updater, chaos/security suite, tested backup/restore and deployment runbook.

---

# Immediate next implementation slice

The highest-value next step is **M1 only**:

1. scaffold Tauri 2/Rust workspace;
2. create restricted loopback mock proxy;
3. bind WebView2 to it with per-WebView proxy configuration;
4. implement allowlisted navigation and external-browser escape handling;
5. build an automated negative test proving there is no direct fallback;
6. package a Windows test build.

Do not start with AmneziaWG/Xray integration before M1 passes. If application-scoped fail-closed routing is not proven first, every later transport feature rests on an unsafe foundation.
