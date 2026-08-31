# WebGate — Network, Security & Resilience Hardening Plan

**Repository:** `Homiakus/WebGate`  
**Branch:** `main`  
**Status:** ACTIVE / IMPLEMENTATION-READY  
**Created:** 2026-08-31  
**Scope:** network path, server-without-public-IP topology, security hardening, performance, failover, origin process supervision, production qualification.  

> This document is an implementation annex to `MASTER_PLAN.md`. It does not weaken any existing WebGate security invariant. If an implementation detail conflicts with a compiled security invariant, the stricter invariant wins.

---

# 0. Why this plan exists

The current repository contains a strong target architecture, but several runtime components are still prototypes or state-machine simulations while README-level wording can be read as production readiness.

The hardening program must close the gap between **declared architecture** and **actual network/security behavior** before WebGate is used for real protected access.

The most important verified gaps in the current codebase are:

1. `DynamicRelayTransport` reports `Ready` without establishing a real transport.
2. `SecureRelayTransport::start_tunnel()` changes state but does not establish a network tunnel.
3. `probe_latency()` does not probe a real relay/origin path.
4. GUI launch currently opens Edge/Chrome/default browser without binding the protected browser traffic to the WebGate transport.
5. device proof-of-possession accepts any non-empty signature string instead of cryptographically verifying the challenge.
6. the active client uses `InMemoryDeviceKeyStore` with synthetic key material.
7. session authorization does not yet strictly bind the presented session to the presented device identity.
8. server authorization state is in-memory rather than authoritative SecureAcces-backed runtime state.
9. server registries and audit/process state are mostly ephemeral.
10. Admin API and user data-plane endpoints are registered on the same HTTP server/mux.
11. default `ListenAddr = ":8787"` can bind all host interfaces even though logging implies loopback-only operation.
12. Admin API contains process/config/device mutation surfaces that must never be reachable without strong administrator authentication.
13. `ProcessManager` can mark a service as running even when `cmd.Start()` failed.
14. current upstream URL validation is not a complete SSRF defense.
15. current failover state machine is not backed by a real multi-provider/multi-relay data path.
16. high-latency thresholds exist in configuration but are not yet part of a complete route health decision.
17. current tests validate useful domain logic but do not yet qualify the complete client → relay → reverse origin → gateway → protected service path.

The goal of this plan is not to redesign WebGate from scratch. The goal is to make the existing target architecture real, measurable, fail-closed and production-qualifiable.

---

# 1. Production success definition

WebGate may be called production-ready for protected private-service access only when all of the following are true:

```text
Trusted user
   ↓
WebGate-owned browser capsule
   ↓
WebGate destination-restricted loopback proxy
   ↓
real protected transport provider
   ↓
real Relay A / Relay B
   ↓
real persistent outbound reverse connectivity from Origin
   ↓
WebGate data-plane gateway
   ↓
SecureAcces authoritative authn/authz
   ↓
allowlisted protected local service
```

and the system remains fail-closed when any protected layer is unavailable.

A protected URL must never silently open through:

- direct OS Internet;
- the system default browser;
- an unproxied Chrome/Edge instance;
- a non-WebGate generic SOCKS/HTTP proxy;
- a stale or unauthenticated relay;
- a revoked session/device;
- an expired unsigned/rollback policy.

---

# 2. Non-negotiable invariants

These invariants are acceptance gates, not preferences.

## INV-NET-001 — application-scoped routing

WebGate protected traffic must be scoped to the WebGate browser/proxy boundary. The OS default route must not be modified in normal mode.

## INV-NET-002 — no direct protected egress

If all protected transports fail, protected navigation returns an explicit offline/fail-closed result. No direct fallback is permitted.

## INV-NET-003 — origin requires no public IP

The protected origin must operate behind dynamic IP and CGNAT with no inbound port forwarding. Origin establishes outbound persistent connections to relay infrastructure.

## INV-NET-004 — relay diversity

Production redundancy requires at least two relay failure domains. Two VMs on the same provider/ASN are not sufficient for HA qualification.

## INV-NET-005 — transport diversity

At least one production fallback path must fail differently from the primary transport family. Initial target:

```text
Primary:  Outline/AWG-class protected transport
Fallback: Xray/VLESS/REALITY/TCP-443-class transport
```

Exact provider implementation may evolve behind the stable `TransportProvider` contract.

## INV-AUTH-001 — possession of tunnel credentials is not application authorization

Network reachability alone never grants service access. Every protected request is authorized by SecureAcces-backed policy.

## INV-AUTH-002 — session ↔ device binding

A valid session can only be used by the device identity to which it was issued/bound.

## INV-ID-001 — real device proof-of-possession

Device activation requires verification of a cryptographic signature over a server-generated short-lived challenge.

## INV-ID-002 — no synthetic production keys

Production builds must use platform-backed key storage where supported. `InMemoryDeviceKeyStore` is test-only.

## INV-ADMIN-001 — admin/data plane separation

Admin API and user gateway must not share a remotely exposed unauthenticated listener. Admin surface defaults to loopback/private-management access only.

## INV-ADMIN-002 — no unauthenticated process execution

No request without authenticated administrator authorization may mutate service executable path/arguments or start/stop/restart a process.

## INV-POLICY-001 — signed configuration

Production endpoint/policy/config updates are accepted only after signature, version and expiry validation.

## INV-STATE-001 — no false Ready/Running

A component can report `Ready`/`Running` only after its real external side effects have succeeded and required health checks pass.

## INV-OBS-001 — health is end-to-end

A transport is not healthy merely because the local process exists or a protocol handshake succeeds. Production health must include the protected origin/gateway path.

---

# 3. Target production topology

```text
                       CONTROL / POLICY PLANE
                 ┌────────────────────────────┐
                 │ SecureAcces + WebGate Admin│
                 │ users/devices/policy/audit │
                 └──────────────┬─────────────┘
                                │ signed policy
                                ▼

┌──────────────────────────────────────────────────────────────┐
│                        WEBGATE CLIENT                        │
│                                                              │
│ Browser Capsule                                              │
│      │                                                       │
│      ▼                                                       │
│ Destination Restricted Proxy                                 │
│ 127.0.0.1:<random ephemeral>                                 │
│      │                                                       │
│ Transport Supervisor                                        │
│      ├── Provider A: AWG/Outline-class                       │
│      └── Provider B: Xray-class                              │
│                                                              │
│ Platform Device Key Store                                    │
│ Signed Policy Store                                          │
└──────────────┬───────────────────────────┬───────────────────┘
               │                           │
               ▼                           ▼
      ┌────────────────┐          ┌────────────────┐
      │ Relay A        │          │ Relay B        │
      │ Provider/ASN A │          │ Provider/ASN B │
      └────────┬───────┘          └────────┬───────┘
               │                           │
               └────────────┬──────────────┘
                            │
                   persistent reverse links
                            │
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                         ORIGIN SERVER                        │
│ dynamic IP / CGNAT / NO inbound port forwarding             │
│                                                              │
│ webgate-origin-agent                                         │
│      ├── reverse link → Relay A                              │
│      └── reverse link → Relay B                              │
│                                                              │
│ Data Gateway: 127.0.0.1:<data-port>                          │
│ Admin API:    127.0.0.1:<admin-port>                         │
│                                                              │
│ Gateway → SecureAcces → Service Registry                     │
│                 │                                            │
│                 ├── Docs                                     │
│                 ├── FactoryOS                                │
│                 ├── Files                                    │
│                 └── Monitoring                               │
└──────────────────────────────────────────────────────────────┘
```

---

# 4. Delivery strategy

Implementation is intentionally ordered to avoid building new features on top of false health/security states.

```text
Phase 0  freeze unsafe production claims + establish gates
Phase 1  close exposed admin/data-plane security defects
Phase 2  make identity/session cryptographically real
Phase 3  make server state durable and SecureAcces authoritative
Phase 4  implement origin agent + reverse connectivity
Phase 5  implement real client local proxy + transport providers
Phase 6  bind real browser traffic to the protected path
Phase 7  implement end-to-end health and 4-way failover
Phase 8  harden service routing, SSRF and process supervision
Phase 9  signed policy/config/update trust chain
Phase 10 performance and connection lifecycle optimization
Phase 11 real E2E/chaos/security qualification
Phase 12 staged production rollout and SLO enforcement
```

No later phase may be used to justify skipping an earlier P0 gate.

---

# 5. Phase 0 — Reality Gate & production claim correction

**Priority:** P0  
**Goal:** prevent prototype state from being mistaken for qualified production behavior.

## WG-HARD-0001 — runtime capability matrix

Create a machine-readable capability matrix containing at least:

```text
feature                         state
real_browser_proxy              IMPLEMENTED/QUALIFIED/NO
real_primary_transport          ...
real_fallback_transport         ...
reverse_origin_connectivity     ...
secureaccess_runtime_adapter    ...
platform_keystore               ...
admin_authentication            ...
policy_signature_validation     ...
end_to_end_failover             ...
```

Acceptance:

- generated/validated in CI;
- README release-ready wording is derived from or consistent with capability state;
- no feature marked QUALIFIED without its qualification test artifact.

## WG-HARD-0002 — forbid fake readiness in production paths

Replace prototype implementations with explicit state such as `Unsupported`, `Prototype`, `NotConfigured`, or error returns.

Acceptance:

- no production code path reports `TransportState::Ready` without real listener/provider/connection establishment;
- no process reports `Running` after failed spawn;
- CLI/UI distinguishes `configured`, `starting`, `healthy`, `degraded`, `offline`.

## WG-HARD-0003 — test-only marker enforcement

`InMemoryDeviceKeyStore`, mock relay providers and synthetic sessions must be test/dev-only.

Acceptance:

- production release build fails if mock provider/key-store feature is active;
- CI includes a `production-capabilities` build gate.

---

# 6. Phase 1 — Admin/data-plane security isolation

**Priority:** P0  
**Goal:** remove the highest-risk remote management attack surface before transport exposure.

## WG-HARD-0101 — split listeners

Introduce distinct listeners/configuration:

```text
DataPlaneAddr = 127.0.0.1:<gateway-port>
AdminAddr     = 127.0.0.1:<admin-port>
HealthAddr    = optional restricted listener
```

Production default must never be `:port` for admin.

Acceptance:

- admin routes cannot be reached through the data listener;
- gateway routes cannot accidentally expose admin handlers;
- startup logs print actual bound addresses from `Listener.Addr()`, not assumed addresses.

## WG-HARD-0102 — authenticated admin middleware

Admin mutations require SecureAcces-backed administrator identity and explicit administrative permission.

Required coverage:

- service create/update/delete;
- route changes;
- executable/path/args mutation;
- process start/stop/restart;
- config bind/update;
- device enrollment/status/revoke;
- release promotion/revoke;
- Telegram administrative actions;
- audit export where sensitive.

Acceptance:

- unauthenticated request → `401`;
- authenticated non-admin → `403`;
- administrator session/device binding verified;
- CSRF protection or non-browser bearer semantics defined explicitly for browser admin UI;
- authorization tests cover every mutating route.

## WG-HARD-0103 — network exposure policy

Add config validation that rejects unsafe public admin binds unless an explicit high-friction override is supplied.

Acceptance:

- `0.0.0.0:*`, `[::]:*`, `:*` for admin rejected by default;
- startup fails closed rather than logging a warning and continuing.

## WG-HARD-0104 — process-control permission split

Create a dedicated high-risk permission/capability for executable/process operations.

Acceptance:

- ordinary WebGate admin cannot implicitly execute arbitrary binaries unless granted process-control permission;
- process command mutation is audited with actor/device/session/old value/new value.

## WG-HARD-0105 — request body and resource limits

All admin/data endpoints receive bounded body size, header size, timeout and concurrency policy.

Acceptance:

- oversized body rejected deterministically;
- slowloris test does not exhaust server goroutines/file descriptors;
- server has ReadHeaderTimeout, IdleTimeout and sane max headers.

---

# 7. Phase 2 — Real device identity and session binding

**Priority:** P0  
**Goal:** make device trust cryptographically meaningful.

## WG-HARD-0201 — challenge verification

Replace the non-empty signature check with actual Ed25519/P-256 verification.

Challenge payload must bind:

```text
challenge_id
nonce
server_id / audience
requested_device_id
public_key fingerprint
issued_at
expires_at
protocol_version
```

Acceptance:

- wrong key fails;
- modified challenge fails;
- replay fails;
- expired challenge fails;
- challenge is single-use;
- malformed encoding fails without panic.

## WG-HARD-0202 — platform key-store adapters

Implement production device key storage:

- Windows: CNG/DPAPI/TPM-backed path where practical;
- Android: Android Keystore;
- macOS: Keychain/Secure Enclave path where practical;
- Linux: documented protected file/secret-service fallback with clearly weaker assurance tier.

Acceptance:

- private key is not exportable through WebGate API;
- key never appears in logs/CLI/process args;
- reinstall/re-enrollment behavior defined and tested.

## WG-HARD-0203 — stable device identity

Device ID is derived/issued in a way that survives normal restarts and cannot be chosen by untrusted page content.

Acceptance:

- restarting client does not create a new identity each launch;
- duplicate/replayed enrollment cannot silently replace another device.

## WG-HARD-0204 — strict session-device-user binding

Authorization must verify:

```text
session exists
session not expired/revoked
session.UserID == authenticated user
session.DeviceID == presented verified device
service workspace permission present
device status == ACTIVE
```

Acceptance:

- session from device A + active device B → denied;
- revoked device invalidates active access;
- revoked session invalidates access without server restart.

## WG-HARD-0205 — credential separation

Maintain distinct credentials for:

- device signing;
- transport;
- SecureAcces session;
- policy roots;
- update roots.

Acceptance:

- rotation of one credential type does not require replacing all others.

---

# 8. Phase 3 — Durable server state & authoritative SecureAcces

**Priority:** P0/P1  
**Goal:** remove in-memory authorization truth and restart-induced state loss.

## WG-HARD-0301 — SecureAcces runtime adapter

Replace the local in-memory authorization emulator in production with a real adapter to SecureAcces.

Define narrow interfaces:

```go
AuthenticateSession(ctx, sessionToken, deviceIdentity)
AuthorizeService(ctx, principal, workspaceID, permission)
RevokeSession(...)
ResolveMembership(...)
```

Acceptance:

- SecureAcces is authoritative for user/session/workspace permissions;
- WebGate does not maintain a competing RBAC truth source;
- adapter failures fail closed;
- bounded cache may improve availability but cannot extend revoked/expired authority beyond defined policy.

## WG-HARD-0302 — SQLite persistence

Persist WebGate-owned state that is not SecureAcces authority:

- protected service registry;
- device metadata/status references;
- release registry;
- signed-policy metadata/version;
- audit log/index;
- process desired state;
- non-sensitive health history.

Initial recommendation: SQLite + WAL + transactional migrations.

Acceptance:

- restart preserves state;
- corruption/recovery procedure documented;
- migrations atomic;
- backup/restore qualification test exists.

## WG-HARD-0303 — immutable audit semantics

Audit records include:

```text
event_id
UTC timestamp
actor user
device
session id/fingerprint (redacted form)
action
target
result
reason
request correlation id
```

Acceptance:

- sensitive tokens/private keys never logged;
- audit mutation/deletion is separately privileged;
- concurrent writes race-safe.

---

# 9. Phase 4 — Origin agent and server without public IP

**Priority:** P0  
**Goal:** make dynamic-IP/CGNAT operation real.

## WG-HARD-0401 — `webgate-origin-agent`

Create a supervised origin component responsible only for outbound connectivity and local gateway exposure.

Responsibilities:

- establish outbound authenticated link to Relay A;
- establish outbound authenticated link to Relay B;
- reconnect with jittered exponential backoff;
- expose only approved local gateway/service targets into relay streams;
- emit health/metrics;
- never open an inbound Internet listener.

Acceptance:

- works behind NAT and CGNAT;
- origin IP can change without client config change;
- router restart causes bounded recovery;
- no inbound port-forward is required.

## WG-HARD-0402 — persistent multiplexed reverse sessions

Avoid one tunnel setup per HTTP request.

Target model:

```text
Origin → Relay A : long-lived authenticated connection
Origin → Relay B : long-lived authenticated connection
                     ├── logical stream 1
                     ├── logical stream 2
                     └── logical stream N
```

Acceptance:

- multiple concurrent HTTP/WebSocket streams supported where required;
- connection-level backpressure and stream limits defined;
- reconnect does not leak stale streams.

## WG-HARD-0403 — relay authentication

Relay must mutually authenticate the origin agent and clients/transport side.

Acceptance:

- unknown origin cannot register arbitrary service route;
- one tenant/origin cannot impersonate another;
- credential rotation supported;
- replayed registration denied.

## WG-HARD-0404 — relay least-state design

Relay stores minimal durable secrets/state.

Acceptance:

- compromise of Relay A alone does not yield SecureAcces user credentials or device private keys;
- relay can be rebuilt from infrastructure configuration.

## WG-HARD-0405 — dual-failure-domain qualification

Production Relay A and Relay B differ by provider/ASN/failure domain.

Acceptance evidence records:

- provider;
- ASN;
- region;
- public endpoint;
- transport family support;
- incident isolation assumptions.

---

# 10. Phase 5 — Real client proxy and transport providers

**Priority:** P0  
**Goal:** replace state simulations with a real protected local data plane.

## WG-HARD-0501 — destination-restricted local proxy

Implement WebGate-owned loopback proxy with:

- random ephemeral port;
- bind only to loopback;
- destination allowlist;
- bounded queues/timeouts;
- no generic Internet proxy use;
- remote/protected DNS strategy;
- fail-closed if provider unavailable;
- separate control IPC and data socket.

Acceptance:

- another local process discovering the port cannot use it for arbitrary Internet destinations;
- forbidden hostname/IP denied;
- direct-IP bypass denied unless explicitly allowlisted;
- DNS leak test passes.

## WG-HARD-0502 — asynchronous `TransportProvider` contract

Evolve provider interface to model real lifecycle:

```text
STOPPED
STARTING
PROBING
CONNECTED
DEGRADED
FAILING_OVER
OFFLINE
```

Suggested operations:

```rust
start(policy) -> Result<LocalProxy>
health() -> TransportHealth
reconnect() -> Result<()>
stop() -> Result<()>
```

Acceptance:

- `Ready/Connected` only after real provider and relay/origin verification;
- provider crash changes health state;
- shutdown closes listeners/children deterministically.

## WG-HARD-0503 — primary provider

Implement and benchmark the selected AWG/Outline-class primary provider behind a supervised sidecar or safe library boundary.

Acceptance:

- actual encrypted data reaches Relay A/B;
- startup/stop/reconnect idempotent;
- sidecar hash/version verified;
- secrets not passed in command-line args.

## WG-HARD-0504 — independent fallback provider

Implement Xray-class fallback using an independent protocol/implementation path.

Acceptance:

- fallback works when UDP is blocked;
- fallback works when primary provider process is killed;
- destination restrictions remain enforced by WebGate proxy.

## WG-HARD-0505 — authenticated local control IPC

Transport sidecar control uses a parent-created short-lived capability.

Acceptance:

- unrelated local process cannot reconfigure transport;
- stale capability invalid after restart;
- IPC protocol is versioned and fuzz-tested.

---

# 11. Phase 6 — Real protected browser binding

**Priority:** P0  
**Goal:** ensure the actual browser rendering protected content is on the protected path.

## WG-HARD-0601 — eliminate unproxied protected browser fallback

Protected URL must never be opened through generic `msedge.exe`, `chrome.exe`, `cmd /c start`, `open`, or `xdg-open` as a silent fallback.

External public links remain a separate policy-controlled operation.

Acceptance:

- test kills all transports and confirms protected URL never appears in system browser;
- test injects browser start failure and verifies explicit fail-closed UI.

## WG-HARD-0602 — Servo production adapter / qualified compatibility adapter

Bind the real browser engine to the WebGate local proxy before navigation.

Acceptance:

- all protected HTTP/HTTPS/DNS/WebSocket traffic follows the WebGate proxy path as applicable;
- browser startup fails if proxy not available;
- changing transport does not disable browser proxy policy;
- network escape tests pass.

## WG-HARD-0603 — navigation policy hardening

Test/deny:

- `file:`;
- unsafe custom schemes;
- localhost bypass to arbitrary ports;
- credential-bearing external redirects;
- IDN/punycode confusion;
- redirect to non-allowlisted origin;
- download-to-execute paths.

## WG-HARD-0604 — browser compromise boundary

Assume page/renderer compromise.

Acceptance:

- web content cannot read transport/device keys;
- cannot mutate relay endpoints;
- cannot disable proxy/fail-closed policy;
- cannot launch arbitrary native process;
- native IPC capability allowlist is minimal and typed.

---

# 12. Phase 7 — End-to-end health and failover

**Priority:** P1  
**Goal:** move from Primary/Fallback state switching to evidence-driven resilient routing.

## WG-HARD-0701 — health ladder

A path has layered health:

```text
H1 provider process alive
H2 local proxy alive
H3 transport established
H4 relay reachable
H5 reverse origin session alive
H6 gateway reachable
H7 SecureAcces dependency healthy enough for required operation
H8 protected health endpoint returns expected semantics
```

A path is user-ready only at the required top level.

## WG-HARD-0702 — 4-way route matrix

Initial route candidates:

```text
1 Relay A / Primary provider
2 Relay B / Primary provider
3 Relay A / Fallback provider
4 Relay B / Fallback provider
```

Acceptance:

- each combination independently probeable;
- failure state tracked per combination, not globally.

## WG-HARD-0703 — route scoring

Introduce bounded scoring using:

- recent availability;
- RTT EWMA;
- jitter;
- loss/failure rate;
- origin/gateway health;
- recent stability;
- transport/provider penalty;
- circuit-breaker state.

Do not switch on a single slow sample.

## WG-HARD-0704 — hysteresis and circuit breaker

Acceptance:

- no route flapping under marginal packet loss;
- repeated failure opens circuit;
- recovery requires stability window;
- jittered exponential backoff prevents retry storms.

## WG-HARD-0705 — fail-closed all-path failure

Acceptance:

- all 4 paths unavailable → protected browser receives deterministic offline state;
- no direct fallback;
- cached content/session policy behavior explicitly defined.

## WG-HARD-0706 — network transition recovery

Qualification scenarios:

- Wi-Fi → Ethernet;
- Wi-Fi → mobile hotspot;
- suspend/resume;
- DHCP/IP change;
- router reboot;
- DNS outage;
- UDP blocked mid-session.

---

# 13. Phase 8 — Gateway, SSRF and service process hardening

**Priority:** P0/P1

## WG-HARD-0801 — immutable upstream resolution

Client request chooses only a service slug/resource path. It cannot submit an arbitrary upstream URL.

Acceptance:

- upstream target comes from authoritative registry;
- scheme/host/port policy validated at service registration/update time.

## WG-HARD-0802 — SSRF defense

Protect against:

- arbitrary LAN targets;
- cloud metadata endpoints;
- link-local ranges;
- unapproved loopback ports;
- DNS rebinding;
- redirects to forbidden destinations;
- IPv4/IPv6 encoding tricks.

Preferred model: upstream allowlist by service identity and resolved socket policy, not generic URL filtering alone.

## WG-HARD-0803 — safe proxy header policy

Explicitly strip/normalize:

- `X-WebGate-*` internal headers before upstream;
- hop-by-hop headers;
- forwarded identity headers that upstream must not trust from clients;
- conflicting `Host`/forwarding metadata.

## WG-HARD-0804 — ProcessManager truthfulness

If `cmd.Start()` fails:

```text
state = FAILED
pid = 0
error recorded
no fake running instance
```

Acceptance:

- unit/integration test verifies failed binary cannot appear `Running`;
- exit hook updates runtime state;
- unexpected process exit is visible to health system.

## WG-HARD-0805 — supervised restart policy

Optional automatic restart must have:

- max restart count/window;
- exponential backoff;
- crash-loop state;
- manual override;
- audit trail.

## WG-HARD-0806 — privilege reduction

Where practical, protected service processes and origin agent run with least required OS privileges.

---

# 14. Phase 9 — Signed policy, configuration and update trust chain

**Priority:** P1

## WG-HARD-0901 — canonical signed policy envelope

Include:

```text
schema_version
policy_version
issued_at
not_before
expires_at
key_id
payload_hash
signature
```

Use deterministic/canonical serialization suitable for signing.

## WG-HARD-0902 — anti-rollback

Client stores highest accepted policy version and rejects older policy unless an explicit signed recovery mechanism exists.

## WG-HARD-0903 — endpoint bootstrap resilience

Bootstrap supports multiple independently resolvable relay/control endpoint candidates:

- signed literal IP candidates where appropriate;
- multiple hostnames/providers;
- system DNS + independent DoH bootstrap;
- cached last-known-good endpoints.

## WG-HARD-0904 — signed sidecar/update manifest

Before transport sidecar execution verify:

- artifact hash;
- version;
- platform/arch;
- signer/key ID;
- minimum compatible client version.

## WG-HARD-0905 — config parser hardening

Replace permissive ad-hoc production parsing with schema-backed parsing and validation.

Acceptance:

- malformed/unknown critical fields fail;
- unsafe bind addresses rejected;
- signed payload canonicalization test suite exists.

---

# 15. Phase 10 — Performance program

**Priority:** P2 after security/data-plane correctness

## WG-HARD-1001 — baseline benchmark harness

Measure separately:

```text
client → relay RTT
relay → origin RTT
client → origin effective RTT
TTFB
small response latency
large download throughput
origin upload saturation
CPU/memory per active session
proxy overhead
reconnect/failover duration
```

Never report performance from simulated `probe_latency()`.

## WG-HARD-1002 — persistent connections

Use persistent/multiplexed transport connections where protocol permits.

Acceptance:

- repeated HTTP requests do not perform unnecessary full tunnel setup;
- connection reuse visible in metrics.

## WG-HARD-1003 — stream/backpressure limits

Define:

- max concurrent streams/client;
- max streams/origin connection;
- queue length;
- per-request body limit;
- idle timeout;
- large upload/download behavior.

## WG-HARD-1004 — HTTP transport tuning

Evaluate:

- keepalive;
- HTTP/2 where compatible;
- WebSocket requirements;
- compression at application layer;
- bounded buffering vs streaming.

## WG-HARD-1005 — origin uplink awareness

Document and surface that effective aggregate download to clients is bounded by origin upload capacity.

Add warning/metric when origin uplink is saturated.

## WG-HARD-1006 — target initial SLOs

Initial targets for a small trusted fleet, subject to real benchmarks:

```text
local proxy overhead p95          < 2 ms on localhost
healthy path connect p95          < 3 s
single relay failover p95         < 5 s
all-path fail-closed reaction     < 2 s after confirmed failure threshold
control API local p95             < 100 ms excluding external SecureAcces latency
no memory growth under 8h soak    statistically bounded
```

These are qualification targets, not claims until measured.

---

# 16. Phase 11 — Security, E2E, chaos and reliability qualification

**Priority:** P1/P2  
**Goal:** prove real behavior rather than unit-level intent.

## WG-HARD-1101 — real topology test environment

CI/nightly lab topology must include:

```text
Windows client
Linux/Windows origin behind NAT simulation
Relay A
Relay B
Primary provider
Fallback provider
WebGate gateway
SecureAcces test instance
real protected service
```

## WG-HARD-1102 — full E2E happy path

Test:

```text
fresh client
→ enroll device
→ approve/authenticate
→ receive signed policy
→ establish protected transport
→ open protected service
→ authorize through SecureAcces
→ receive content
```

## WG-HARD-1103 — network escape suite

Explicit negative tests:

- kill local proxy;
- kill provider;
- block relay;
- remove DNS;
- revoke policy;
- redirect protected URL externally;
- browser crash/restart.

Acceptance: protected request never exits directly.

## WG-HARD-1104 — identity attack suite

- forged device signature;
- replayed challenge;
- wrong device/session pairing;
- expired session;
- revoked device;
- stolen session on another machine;
- malformed public keys.

## WG-HARD-1105 — admin attack suite

- unauthenticated process start;
- non-admin process start;
- CSRF attempt;
- config path abuse;
- oversized body;
- method confusion;
- route exposure through data listener.

## WG-HARD-1106 — SSRF suite

Include metadata IPs, IPv6 loopback/link-local, redirects, DNS rebinding harness and encoded IP representations.

## WG-HARD-1107 — chaos matrix

Inject:

- Relay A hard kill;
- Relay B hard kill;
- both relays sequentially;
- primary transport process crash;
- fallback crash;
- origin agent crash;
- origin gateway restart;
- SecureAcces outage;
- 1–20% packet loss;
- latency 100/300/1000/2000 ms;
- jitter;
- bandwidth throttling;
- DNS poisoning/unavailability;
- clock skew within supported bounds.

## WG-HARD-1108 — long soak

Minimum qualification:

- 8h routine session;
- 24h transport/origin soak;
- repeated network transitions;
- no unbounded memory/goroutine/thread/file-descriptor growth.

## WG-HARD-1109 — mutation/test-of-tests

Apply mutation testing to security-critical pure logic:

- permission mapping;
- session/device binding;
- challenge expiry/replay;
- navigation allowlist;
- failover threshold/hysteresis;
- policy version/expiry checks;
- SSRF predicates.

Mutation survivors in P0 security logic block production promotion unless explicitly reviewed/waived.

---

# 17. Phase 12 — Staged production rollout

## WG-HARD-1201 — release channels

```text
dev → lab → canary → stable
```

Production policy must be able to pin minimum/maximum qualified client versions.

## WG-HARD-1202 — canary fleet

Start with a minimal trusted fleet and enable:

- one primary relay;
- one fallback relay;
- both transport families;
- detailed redacted telemetry.

## WG-HARD-1203 — rollback

Rollback mechanism must not violate anti-rollback trust rules. Use a specifically signed rollback authorization/recovery release where needed.

## WG-HARD-1204 — operational runbooks

Required runbooks:

- Relay A outage;
- Relay B outage;
- origin moved to another ISP;
- SecureAcces unavailable;
- device stolen;
- suspected relay compromise;
- policy signing key rotation;
- update signing key rotation;
- database backup/restore;
- origin agent recovery;
- transport sidecar crash loop.

---

# 18. Dependency graph

```text
P0 admin isolation ───────────────┐
                                  ├─► production exposure allowed
real identity/session binding ────┤
                                  │
SecureAcces adapter ──────────────┤
                                  │
origin reverse connectivity ──────┤
                                  │
real local proxy/transports ──────┤
                                  │
real protected browser binding ───┘
             │
             ▼
     end-to-end health
             │
             ▼
       4-way failover
             │
             ▼
 security/chaos qualification
             │
             ▼
          canary
             │
             ▼
          stable
```

Performance optimization begins only after the real path exists and P0 security semantics are enforced.

---

# 19. Atomic P0 checklist

The following items block any claim of production readiness:

- [ ] Admin listener separated from data plane.
- [ ] Admin default bind is loopback/private-management only.
- [ ] Every admin mutation authenticated and authorized.
- [ ] Process execution mutation requires high-risk admin capability.
- [ ] `cmd.Start()` failure cannot produce fake Running state.
- [ ] Real device challenge signature verification implemented.
- [ ] Production device key-store implemented for Tier-1 platforms.
- [ ] Session strictly bound to device.
- [ ] Production SecureAcces adapter replaces local authorization truth.
- [ ] Durable server state implemented.
- [ ] Real origin outbound reverse connectivity exists.
- [ ] At least Relay A and Relay B are reachable through real authenticated links.
- [ ] Real client loopback destination-restricted proxy exists.
- [ ] Real primary protected transport exists.
- [ ] Real independent fallback transport exists.
- [ ] Actual protected browser engine is bound to WebGate proxy.
- [ ] Protected URL cannot silently open in an unproxied system browser.
- [ ] All-path loss fails closed.
- [ ] Gateway upstream selection is registry-owned.
- [ ] SSRF protection qualified.
- [ ] Signed production policy/config verification exists.
- [ ] Real E2E test passes through client → relay → origin → SecureAcces → protected service.
- [ ] Network escape negative suite passes.

---

# 20. P1/P2 backlog after P0

## P1

- durable audit improvements;
- health ladder;
- 4-way relay/provider matrix;
- circuit breaker/hysteresis;
- DNS bootstrap resilience;
- signed sidecar/update verification;
- NAT/router/network-transition qualification;
- chaos test automation;
- backup/restore qualification;
- 24h soak.

## P2

- dynamic health scoring;
- performance optimization;
- stream multiplexing tuning;
- compression/caching policy;
- advanced telemetry;
- automated relay capacity scaling;
- broader Tier-2 platform qualification.

---

# 21. Required observability

Metrics must distinguish configuration from actual runtime health.

Minimum metrics:

```text
webgate_transport_state{provider,relay}
webgate_transport_rtt_ms{provider,relay}
webgate_transport_failures_total{provider,relay,reason}
webgate_failovers_total{from,to,reason}
webgate_reverse_session_state{relay}
webgate_origin_reconnects_total{relay}
webgate_gateway_requests_total{service,status}
webgate_gateway_duration_ms{service}
webgate_auth_denials_total{reason}
webgate_device_revocations_total
webgate_process_state{service}
webgate_process_restarts_total{service,reason}
webgate_policy_version
webgate_policy_expiry_seconds
```

Never use high-cardinality raw session tokens, URLs with secrets, device private identifiers or bearer tokens as metric labels.

---

# 22. Error taxonomy

Failures must be actionable and non-ambiguous.

Suggested stable classes:

```text
WG-NET-NO-PROXY
WG-NET-PRIMARY-DOWN
WG-NET-FALLBACK-DOWN
WG-NET-ALL-PATHS-DOWN
WG-NET-ORIGIN-OFFLINE
WG-NET-DNS-BOOTSTRAP
WG-AUTH-SESSION-EXPIRED
WG-AUTH-SESSION-DEVICE-MISMATCH
WG-AUTH-ACCESS-DENIED
WG-ID-CHALLENGE-INVALID
WG-ID-DEVICE-REVOKED
WG-POLICY-INVALID-SIGNATURE
WG-POLICY-EXPIRED
WG-POLICY-ROLLBACK
WG-ADMIN-UNAUTHENTICATED
WG-ADMIN-FORBIDDEN
WG-PROC-SPAWN-FAILED
WG-UPSTREAM-DENIED
```

UI should translate these to concise user-facing Russian messages while preserving structured diagnostics for logs.

---

# 23. Definition of Done for every implementation task

A task is DONE only when:

1. implementation is merged/pushed to `main`;
2. positive tests exist;
3. negative/adversarial tests exist where security-relevant;
4. race/concurrency behavior is tested where applicable;
5. logs are redacted;
6. no new direct-egress path is introduced;
7. documentation/config schema is updated;
8. CI passes from the exact resulting commit SHA;
9. the capability matrix is updated if runtime capability changed;
10. unexpected findings are added back to `MASTER_PLAN.md` / this annex before proceeding.

For P0 work, a unit test alone is insufficient if the behavior crosses process/network boundaries.

---

# 24. Recommended first execution tranche

Implementation should begin in this exact order:

### Tranche A — stop dangerous exposure

1. `WG-HARD-0101` split admin/data listeners.
2. `WG-HARD-0103` reject unsafe admin binds.
3. `WG-HARD-0102` admin authentication/authorization middleware.
4. `WG-HARD-0104` high-risk process-control permission.
5. `WG-HARD-0804` remove fake process Running state.
6. add adversarial tests for all five changes.

### Tranche B — make identity real

7. `WG-HARD-0201` real cryptographic challenge verification.
8. `WG-HARD-0204` session-device binding.
9. `WG-HARD-0202/0203` persistent Tier-1 device key store and identity.
10. add replay/forgery/revocation tests.

### Tranche C — make server authority real

11. `WG-HARD-0301` production SecureAcces adapter.
12. `WG-HARD-0302` SQLite persistence.
13. `WG-HARD-0303` durable/redacted audit.

### Tranche D — make the network real

14. `WG-HARD-0401/0402` origin agent + persistent reverse session.
15. deploy/provision Relay A/B test infrastructure.
16. `WG-HARD-0501` real restricted local proxy.
17. `WG-HARD-0503` primary provider.
18. `WG-HARD-0504` independent fallback provider.
19. `WG-HARD-0601/0602` bind actual browser to the proxy.

### Tranche E — prove resilience

20. `WG-HARD-0701` end-to-end health ladder.
21. `WG-HARD-0702` four route combinations.
22. `WG-HARD-0704/0705` hysteresis/circuit breaker/fail-closed.
23. `WG-HARD-1101..1108` real E2E + chaos + soak.
24. only then tune `Phase 10` performance.

---

# 25. Final release gate

A stable release is blocked if any of the following is true:

```text
admin endpoint publicly exposed without strong auth
OR device proof is synthetic
OR production key store is in-memory
OR session/device mismatch is accepted
OR SecureAcces is bypassed by local RBAC truth
OR origin still requires inbound public-IP access
OR relay provider reports Ready without real path validation
OR protected browser can bypass WebGate proxy
OR all-path transport failure can fall back direct
OR process spawn failure can report Running
OR SSRF qualification fails
OR policy/update signatures are not enforced
OR real E2E topology test does not pass
OR network escape suite has any survivor
```

When all gates pass, WebGate can legitimately claim the intended operational property:

> A trusted user can open a WebGate link and securely reach an authorized private service hosted behind a dynamic IP/CGNAT origin, without a system-wide VPN, with fail-closed behavior and independent relay/transport failover.
