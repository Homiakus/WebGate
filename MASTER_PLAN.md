# WebGate — Master Implementation Plan

Research baseline: **2026-08-29**

Canonical browser decision: **Servo is the primary protected browser engine.** See `docs/architecture/ADR-0001-BROWSER-ENGINE.md`.

This is the living implementation plan for WebGate. It is intentionally ordered by risk: prove Servo embedding and browser-scoped fail-closed networking first, then add real transports, then identity/control-plane integration, then production resilience.

## 0. Architecture invariants

These rules are non-negotiable unless an explicit ADR changes the product model:

1. **Servo is the default protected browser engine.**
2. Protected browser traffic is application-scoped by default; WebGate does not change the OS default route.
3. Transport loss must fail closed. Servo never silently retries protected traffic through direct Internet.
4. Browser-engine failure must not cause a silent switch to another engine or system browser.
5. External browser/navigation traffic is separate from protected traffic.
6. WebGate does not implement business authorization; SecureAcces remains authoritative server-side.
7. Links identify resources; they are not long-lived bearer credentials.
8. Device private keys are generated on-device and never distributed in configuration files.
9. All bootstrap bundles, remote policies and updates are signed.
10. Remote policy may tighten but may not weaken compiled hard security invariants.
11. Transport implementations are replaceable providers, not dependencies of browser/business code.
12. Browser engines sit behind a WebGate interface; Servo is canonical, WebView2 is optional explicit compatibility fallback only.
13. Production must have at least two materially independent network paths/failure domains.

---

# Phase 0 — Research and contracts

Status: **IN PROGRESS / Servo baseline accepted**

Completed:

- tooling audit;
- browser-engine audit;
- Servo primary-engine ADR;
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
- Servo required-feature compatibility matrix for the actual documents site;
- signed configuration wire schema;
- local IPC protocol schema;
- device registration wire schema;
- explicit license/distribution policy for bundled sidecars;
- production Servo release/LTS pinning policy.

Exit criteria:

- every client/server trust boundary has an owner and test strategy;
- no security-sensitive wire format remains informal;
- bundled dependency licenses are reviewed;
- required site capabilities are enumerated and mapped to Servo support/tests.

---

# Phase 1 — Servo protected-browser proof of concept

Goal: prove the central product idea using Servo before introducing a real VPN protocol.

## 1.1 Rust workspace

Create:

```text
crates/webgate-app
crates/webgate-browser
crates/webgate-browser-servo
crates/webgate-policy
crates/webgate-transport
crates/webgate-observability
crates/webgate-platform
```

Reserve, but do not require for M1:

```text
crates/webgate-browser-webview2
```

Use:

- Rust stable;
- pinned Servo library release/LTS candidate;
- Tokio where asynchronous orchestration is useful;
- native window/event-loop integration;
- Servo `Servo` / `WebView` embedding APIs;
- `RenderingContext`;
- `EventLoopWaker` / `Servo::spin_event_loop`;
- `tracing`;
- strict linting.

## 1.2 Servo embedding shell

Implement the smallest practical browser capsule:

- native top-level window;
- Servo engine initialization;
- one persistent protected WebView where practical;
- rendering context lifecycle;
- resize/DPI/input/IME handling;
- browser error surface owned by WebGate, not by arbitrary web content;
- controlled session/data directory.

## 1.3 Mock local proxy

Implement a restricted localhost HTTP/HTTPS proxy test gate.

Required behavior:

- bind `127.0.0.1:0`;
- return actual ephemeral port to parent;
- permit only configured test origins;
- reject all other destinations;
- start in fail-closed state;
- no direct fallback;
- bounded connections/timeouts;
- deny non-WebGate/general proxy usage where practical.

## 1.4 Servo network binding

Configure Servo protected networking to use the WebGate local proxy before any protected navigation.

Requirements:

- HTTP and HTTPS traffic for the protected session follows WebGate proxy policy;
- no environment/system proxy ambiguity in production configuration;
- remote DNS/name-resolution behavior is explicitly tested;
- protected-origin direct IP and alias paths cannot bypass policy.

## 1.5 Navigation policy

- protected origin allowlist;
- strict redirect policy;
- new-window/popup interception;
- external links opened only by explicit system-browser policy;
- no secrets in URLs;
- explicit download policy;
- debugging/developer features disabled or gated in release.

## 1.6 Kill-switch tests

Automated acceptance tests must prove:

```text
proxy alive   → protected site reachable
proxy stopped → protected site NOT reachable directly
```

Also test:

- direct IP;
- DNS alias;
- redirects;
- subresources;
- fetch/XHR;
- WebSocket/SSE when used;
- IPv4/IPv6 differences;
- browser crash/restart;
- unsupported-page behavior.

Exit criteria:

- Servo opens a protected test site only through the WebGate local proxy;
- Chrome/Telegram/system traffic is unaffected;
- stopping the proxy cannot produce direct protected-origin traffic;
- Servo failure cannot trigger a silent system-browser or WebView2 fallback.

---

# Phase 2 — Servo site-compatibility and performance gate

Goal: prove that Servo is not only secure enough for the capsule but compatible and fast enough for the actual documentation workload.

## 2.1 Machine-readable compatibility inventory

Cover at minimum:

- TLS/certificates;
- cookies and session handling;
- fetch/XHR;
- forms;
- CSS/layout used by the site;
- JavaScript APIs used by the site;
- document/file viewing workflow;
- downloads if required;
- printing if required;
- clipboard if required;
- WebSocket/SSE if required;
- local/session storage if required;
- Cyrillic text input and IME;
- accessibility requirements.

Classify every capability:

```text
REQUIRED
OPTIONAL
NOT USED
```

Required unsupported features block production or trigger deliberate site simplification/implementation work.

## 2.2 Visual and behavioral regression suite

- golden screenshots for representative pages;
- layout overflow/overlap checks;
- interaction scripts;
- form behavior;
- authentication/session behavior;
- long-document scrolling;
- file/document navigation.

## 2.3 Performance baseline

Measure:

- process start → native window;
- process start → Servo ready;
- deep-link click → first protected paint;
- warm navigation;
- idle/active RSS;
- CPU at idle;
- frame stability while scrolling;
- relevant JavaScript workload;
- startup under cold filesystem cache where practical;
- recovery after transport reconnect.

## 2.4 Compatibility fallback gate

Only if a production-critical requirement cannot be satisfied reasonably with Servo may an optional WebView2 adapter be enabled.

Fallback rules:

- explicit policy only;
- never silent;
- same protected browser interface;
- same fail-closed proxy path;
- same origin/navigation restrictions;
- separate compatibility telemetry;
- Servo remains default.

Exit criteria:

- the actual documents application passes the Servo REQUIRED capability suite;
- performance baseline is accepted;
- any remaining incompatibilities have explicit disposition.

---

# Phase 3 — Transport provider SPI and supervision

Goal: make network protocol implementation replaceable and independent of Servo.

## 3.1 Provider contract

Implement a Rust-side abstraction similar to:

```rust
pub trait TransportProvider {
    async fn start(&self, policy: TransportPolicy) -> Result<LocalProxy>;
    async fn health(&self) -> TransportHealth;
    async fn reconnect(&self) -> Result<()>;
    async fn stop(&self) -> Result<()>;
}
```

Servo receives only the local protected proxy endpoint. It must not know whether the provider is AWG, Xray or another implementation.

## 3.2 Sidecar supervision

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

## 3.3 Health model

Health is layered:

1. provider process alive;
2. local proxy alive;
3. secure remote transport handshake works;
4. relay is reachable;
5. private origin is reachable;
6. protected health/API semantics are correct.

Exit criteria:

- provider process can be killed/restarted without Servo escaping to direct Internet;
- provider API is protocol-agnostic.

---

# Phase 4 — Primary transport spike: Outline SDK + AmneziaWG

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

Adopt as primary only if it meets fail-closed, stability, packaging and recovery requirements. Otherwise retain the provider interface and choose another primary without changing Servo/browser code.

---

# Phase 5 — Independent fallback transport

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
- Servo remains attached only to the WebGate local proxy during provider changes;
- user does not need to select a VPN profile manually.

---

# Phase 6 — Signed bootstrap and device identity

Goal: replace hand-managed VPN configs with safe one-time enrollment.

## 6.1 `.webgate` bootstrap schema

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

## 6.2 Device key

On first activation:

- generate Ed25519 device keypair locally;
- compute stable public-key fingerprint/device ID;
- protect private key with Windows DPAPI;
- erase transient plaintext secret buffers where practical.

## 6.3 Policy verification

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

# Phase 7 — WebGate control API + SecureAcces integration

Goal: attach the client to the existing authorization system without duplicating it.

## 7.1 Go control API

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

## 7.2 SecureAcces v1 integration

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

## 7.3 Resource resolution

For every document request:

1. authenticate SecureAcces session;
2. load document from authoritative server store;
3. construct `secureaccess.Resource` from stored tenant/workspace ownership;
4. call `Authorize` with required permission;
5. only then return bytes/metadata.

The client never decides its own tenant/workspace scope.

## 7.4 Device registry

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

# Phase 8 — Relay/origin high availability

Goal: make the lack of a static origin IP irrelevant.

## 8.1 Two independent relays

Provision Relay A/B across different meaningful failure domains:

- different providers;
- different ASNs/prefixes where possible;
- independent credentials;
- reproducible deployment.

## 8.2 Origin server

Origin in Russia:

- no public document service port;
- no dependency on static public IP;
- no inbound NAT requirement;
- outbound secure connectivity to both relays;
- watchdog/reconnect service;
- local reverse proxy only on trusted/private path.

## 8.3 Origin resilience

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

# Phase 9 — Deep links and Telegram UX

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
Telegram → click → existing/new WebGate → Servo → protected document
```

with no manual VPN action.

---

# Phase 10 — Production hardening

## Client security

- Servo release/LTS pinning;
- Servo compatibility and security regression suite;
- release debugging/devtools disabled or gated;
- no arbitrary native IPC from page JavaScript;
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
- SBOM;
- pinned Servo dependency graph;
- Servo upgrade diff/review policy.

Go:

- race detector;
- fuzzing;
- `govulncheck`;
- `staticcheck`;
- SBOM.

All releases:

- documented/reproducible build environment where practical;
- signed release artifacts;
- dependency/license inventory;
- sidecar hashes in signed manifest.

---

# Phase 11 — Adversarial and resilience validation

Mandatory test families:

## Browser/network escape

- HTTP/HTTPS proxy bypass;
- DNS leak;
- redirect bypass;
- WebSocket/SSE/subresource behavior;
- direct IP origin access;
- IPv4/IPv6 differences;
- environment/system proxy interactions;
- local malicious process probing proxy port;
- Servo crash/restart;
- unsupported feature/navigation failure;
- attempted silent external-browser escape.

## Servo compatibility/regression

- required Web API inventory;
- visual regression;
- long-document rendering;
- Cyrillic/IME;
- auth/session cookies;
- storage APIs used by the site;
- downloads/printing only if product requirements enable them;
- upgrade from pinned/LTS Servo version.

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

## M1 — Servo browser capsule

Servo embedding + restricted proxy + provable fail-closed behavior.

## M2 — Servo compatibility/performance qualification

Actual documents site passes required capability, visual and performance gates.

## M3 — Real resilient transport

Primary provider running through the same browser/proxy abstraction.

## M4 — Dual-provider failover

Automatic primary ↔ independent fallback with two relays.

## M5 — Trusted device onboarding

Signed `.webgate` bootstrap + DPAPI device key + remote signed policy.

## M6 — SecureAcces-backed access

Server authentication, membership/resource authorization, session/device revocation.

## M7 — One-click Telegram workflow

Trusted link opens the correct document in Servo/WebGate without manual VPN interaction.

## M8 — Production release candidate

Pinned Servo/LTS, signed installer/updater, chaos/security suite, tested backup/restore and deployment runbook.

---

# Immediate next implementation slice

The highest-value next step is **M1 — Servo browser capsule**:

1. scaffold the Rust workspace;
2. pin a Servo release/LTS candidate;
3. build a minimal native Servo embedding shell;
4. integrate `Servo`/`WebView`, `RenderingContext`, event-loop wake/spin and input handling;
5. create the restricted loopback mock proxy;
6. force Servo protected HTTP/HTTPS traffic through that proxy;
7. implement allowlisted navigation and explicit external-browser handling;
8. build an automated negative test proving there is no direct fallback;
9. add the first required-feature compatibility tests for the real documents site;
10. package a Windows test build.

Do not start with AmneziaWG/Xray integration before M1 passes. If Servo cannot be proven to obey application-scoped fail-closed networking first, every later transport feature rests on an unsafe foundation.
