# WebGate — Living Master Plan

**Repository:** `Homiakus/WebGate`
**Primary branch:** `main`
**Plan status:** ACTIVE
**Reconciled:** 2026-09-02
**Baseline converged main:** `704e1ac84f283623f27da6a60706649fd0d08b70`

This file is the only execution source of truth. Supporting documents under `docs/` are design/evidence references; they do not own task state, release readiness, or acceptance.

A task is `DONE` only when its observable production contract exists, relevant negative tests exist, qualification passes, the exact state reaches `main` without force push, and repeated `main` CI passes where CI is available.

T-047 remains the qualified convergence checkpoint for the original public WebGate scope. T-053..T-062 deliberately extend the required production contract; T-047 must not be cited as evidence that the new next-generation transport/security requirements are already implemented.

---

# 1. Mission

```text
trusted link
  ↓
WebGate-owned browser capsule
  ↓
destination-restricted loopback proxy
  ↓
end-to-end WebGate secure session
  ↓
adaptive path manager
  ├── direct path when safely available
  ├── independent infrastructure relays
  ├── trusted peer relay where explicitly permitted
  └── heterogeneous protected transport providers
  ↓
persistent outbound connectivity from private Origin
  ↓
WebGate data gateway
  ↓
loopback SecureAcces authority bridge
  ↓
private + durable SecureAcces authority
  ↓
registered private service
```

Origin must work behind dynamic IP / CGNAT with no inbound port forwarding. Protected traffic must never silently escape through normal OS Internet, an unproxied system browser, an unowned upstream, a local authorization surrogate, an unverified relay path, or an expired/offline authorization mode.

The target is not a generic VPN and not an open proxy. WebGate is an application-scoped Zero-Trust overlay for explicitly registered services. Transport reachability and application authorization remain separate security boundaries.

---

# 2. Truth hierarchy

1. Observed runtime behavior.
2. Reproducible tests/experiments.
3. Security/correctness invariants.
4. Code.
5. This plan.
6. Supporting design/evidence documents.
7. Initial assumptions.

Material unexpected evidence becomes an `F-XXX` finding before task scope/order changes.

External standards, drafts and reference implementations are design evidence only. Draft protocols may be evaluated experimentally but cannot become the sole production dependency until their interoperability and stability risk is explicitly accepted.

---

# 3. Qualified foundations

The following foundations are already qualified under their recorded scopes and must remain regression-protected:

- Rust workspace/architecture/lock/format/check/test/clippy gates.
- Go 1.26.6 gate with `go mod tidy -diff`, gofmt, vet, tests and scoped `CGO_ENABLED=0` portability.
- Split loopback-only Data/Admin listeners and bootstrap Admin-token containment.
- Explicit client/server config failure is fail-closed.
- Ed25519 device proof-of-possession with short-lived single-use challenges.
- Process spawn failure cannot report false `RUNNING`.
- **T-043:** protected service upstreams are loopback-only; environment-proxy/redirect SSRF escape is contained.
- **T-049:** reproducible SecureAcces v0.4.0 dependency/toolchain anchor.
- **T-050:** data-plane depends on context-aware `ServiceAuthorizer`; unavailable authority → `503`, policy deny → `403`.
- **T-052 public half:** loopback-only remote authority bridge, exact AccountID/DeviceID binding, strict response validation, bounded bodies/timeouts, no redirects/proxy escape, control-secret scrubbing for child services.
- **T-039:** WebGate-owned service/device/release/control/audit state is durable and recovery-qualified.
- **T-036:** real destination-restricted browser-facing loopback SOCKS5 proxy with local domain/port policy enforcement and fail-closed endpoint revocation.
- **T-037:** Origin maintains persistent authenticated outbound reverse sessions to Relay A/B without public inbound ports and multiplexes streams to loopback data gateway.
- **T-040:** Ed25519/SHA-512 device identity engine and persisted production key store with corruption fail-closed behavior.
- **T-041:** Servo BrowserCapsule with enforced loopback proxy and no system-browser fallback.
- **T-042:** dual-relay failover transport with health observation and switchback.
- **T-048:** transactional fail-closed runtime client configuration binding.
- **T-044:** race, mutation and fuzz security feedback loop.
- **T-045:** original multi-hop end-to-end qualification path.
- **T-046:** reproducible multi-binary distribution and signed release manifests.
- **T-047:** original-scope final re-audit/convergence checkpoint.

## Still not production-qualified

- Private SecureAcces provider in its own qualified `main`.
- SecureAcces-owned durable state + recovery qualification.
- SecureAcces-backed administrator management authorization.
- Atomic privileged action + durable audit transaction across all Admin routes.
- Native cryptographic protection of the WebGate relay protocol itself.
- Per-node relay/origin identity replacing a shared production cluster secret.
- Explicit multi-origin/tenant relay routing and relay admission control.
- Client↔Origin end-to-end opaque secure session through an untrusted relay.
- Real heterogeneous H3/QUIC + H2/TLS provider implementations behind the transport abstraction.
- Direct path upgrade / peer relay mode.
- Signed multi-source relay discovery and last-known-good bootstrap.
- Optional bounded control-plane outage continuity using signed authorization leases.
- Full release-binary cross-process qualification for the extended architecture.

---

# 4. Critical invariants

## Existing invariants

- **I-001 Browser ownership:** protected content only through a WebGate-qualified browser path.
- **I-002 Application-scoped routing:** normal mode never changes OS default route.
- **I-003 Fail closed:** transport/authority/browser/config/state loss never creates direct or surrogate fallback.
- **I-004 No silent engine fallback:** protected content never opens in an unproxied system browser.
- **I-005 Real readiness:** `Ready`/`Running` means required side effects and health proof succeeded.
- **I-006 Destination restriction:** browser-facing proxy is loopback-only and policy bounded.
- **I-007 Network access ≠ authorization:** transport credentials never grant application access.
- **I-008 SecureAcces authority:** production authorization comes from real SecureAcces state or an explicitly qualified SecureAcces-signed bounded lease; never from mutable WebGate-local permission maps.
- **I-009 Device binding:** SecureAcces session DeviceID and account exactly match the active WebGate device; tenant-local UserID is not global AccountID.
- **I-010 Real PoP:** activation requires cryptographic proof over a short-lived single-use challenge.
- **I-011 Production keys:** platform-backed where supported; synthetic/in-memory stores are test-only.
- **I-012 Origin no-public-IP:** no inbound NAT requirement; Origin maintains outbound connectivity.
- **I-013 Failure-domain diversity:** production relay redundancy spans materially independent failure domains.
- **I-014 Transport diversity:** fallback differs materially in protocol/implementation/failure mode.
- **I-015 Admin isolation:** privileged operations require management authorization + durable audit on isolated plane.
- **I-016 Server-owned routing:** clients cannot choose authoritative upstream/tenant/workspace/permission/process fields.
- **I-017 No generic proxy:** no SSRF/open-proxy pivot.
- **I-018 Durable security state:** restart/crash cannot silently reset authoritative state.
- **I-019 Signed policy/release:** signed/versioned/rollback-aware artifacts where applicable.
- **I-020 No false qualification:** mock/interface/work-branch state cannot mark production capability complete.
- **I-021 Trusted CI:** runtime/provider paths have build/static/test gates; critical concurrency gets race checks before release.
- **I-022 Mutation resistance:** critical allow/deny/state logic has meaningful test-of-tests evidence.
- **I-023 Private-source boundary:** public WebGate must not implicitly publish private SecureAcces implementation.
- **I-024 Control-secret containment:** authority/admin/Telegram credentials are not persisted in WebGate SQLite and are not inherited by launched protected services.
- **I-025 Registry ownership:** registries own mutable state; inputs/results are detached and mutations pass through explicit locked APIs.
- **I-026 Persist-before-memory:** durable mutation commits before authoritative memory changes; failure leaves memory unchanged.
- **I-027 Runtime is ephemeral:** process PID/state/StartedAt never restore as `RUNNING` after restart.
- **I-028 Durable identity:** persisted devices require explicit global AccountID; no UserID→AccountID inference.
- **I-029 Append-only audit:** durable WebGate audit cannot be UPDATEd/DELETEd; identical replay is idempotent and conflicting event-ID reuse is corruption.
- **I-030 Secret-free control snapshot:** durable control metadata excludes runtime secrets and service-registry records.
- **I-031 No resurrection:** an existing state DB, including an empty service registry, is authoritative and cannot be silently reseeded.
- **I-032 Qualified recovery:** backup is a consistent SQLite snapshot; restore validates schema/checksums before create-only atomic install.

## Next-generation invariants

- **I-033 Native relay-link confidentiality/integrity:** a production Origin↔Relay or Client↔Relay WebGate link is never raw unauthenticated plaintext TCP. The WebGate-owned protocol has a native authenticated encrypted envelope independent of optional external sidecars.
- **I-034 Per-node identity:** production relay/origin trust is rooted in unique rotatable node credentials. A shared `ClusterToken` is bootstrap/recovery-only and cannot remain the sole production root of trust.
- **I-035 Explicit relay routing:** every admitted client transit stream is bound to an explicitly resolved authorized relay target (`ClusterID`/`OriginID` and opaque service/session context as required); arbitrary `first active origin` selection is forbidden.
- **I-036 Relay admission control:** unauthenticated or unreserved peers cannot create unbounded streams, buffers, goroutines/tasks, reservations, bandwidth or memory pressure.
- **I-037 Relay opacity:** after T-055, infrastructure/peer relays forward authenticated ciphertext and do not need access to protected HTTP payloads, session headers or application secrets.
- **I-038 Adaptive path safety:** path optimization may select among qualified paths but cannot weaken destination restriction, device binding, authorization or fail-closed behavior.
- **I-039 Direct path is an optimization:** direct connectivity is attempted only after authenticated rendezvous and path proof; failure returns to a qualified relay path, never direct public Internet.
- **I-040 Signed discovery:** relay/provider/discovery metadata is signed, versioned, expiry-bounded and rollback-aware. DNS/DHT/gossip may discover candidates but cannot become authoritative policy.
- **I-041 Bounded offline authorization:** any authority-outage continuity mode uses short-lived signed leases with explicit expiry and policy epoch; no indefinite permission cache is allowed.
- **I-042 Protocol agility:** transport/provider choice is a replaceable capability behind a stable interface; WebGate core must not depend on one censorship/firewall/NAT behavior or one vendor implementation.
- **I-043 Failure-domain proof:** HA qualification measures provider, ASN/network, region, transport family and deployment correlation; multiple hosts in one material failure domain do not satisfy redundancy.
- **I-044 Release-binary qualification:** the final extended production path is exercised with actual release binaries/processes across the Rust and Go boundary; mock sidecars/relays cannot be the final evidence.

---

# 5. Findings registry

Historical F-001..F-028 remain in Git history.

## Existing active/recent findings

- **F-029 — False convergence:** RESOLVED by T-034.
- **F-030 — Synthetic client transport readiness:** RESOLVED/contained by T-035/T-036.
- **F-031 — No real Servo/proxied runtime:** RESOLVED by T-041.
- **F-032 — Synthetic production keystore:** RESOLVED by T-040.
- **F-033 — Real SecureAcces production authority absent:** OPEN / Critical → T-052.
- **F-034 — Origin reverse connectivity absent:** RESOLVED by T-037/T-045.
- **F-035 — Security/operations state ephemeral:** PARTIALLY RESOLVED; SecureAcces durability remains T-052.
- **F-036 — Admin auth is interim shared token:** OPEN/CONTAINED / High → T-051.
- **F-037 — Explicit config failed open:** RESOLVED by T-035.
- **F-038 — Race/mutation/fuzz CI depth missing:** RESOLVED by T-044.
- **F-039 — Runtime client config can report false success:** RESOLVED by T-048.
- **F-040 — SecureAcces dependency/auth boundaries under-modeled:** PARTIALLY RESOLVED; T-052/T-051 remain.
- **F-041 — Private/durable SecureAcces deployment channel absent:** OPEN / Critical → T-052.
- **F-042 — Device UserID/AccountID conflation:** CONTAINED; private legacy cleanup remains T-052.
- **F-043 — Child services inherited WebGate control secrets:** RESOLVED by T-052 public half.
- **F-044 — Registry state escaped through live pointers:** RESOLVED by T-039A.
- **F-045 — Existing empty durable service registry could be silently re-seeded:** RESOLVED by T-039B2.

## Newly accepted findings from 2026-09-02 transport review

- **F-046 — Relay protocol lacks a native encrypted authenticated envelope:** OPEN / Critical → T-053. `OriginReverseAgent` currently establishes ordinary TCP and WGRL framing itself provides framing/size validation but not confidentiality/authenticated encryption.
- **F-047 — Shared production relay secret concentrates trust:** OPEN / Critical → T-053. `ClusterToken` authenticates Origin sessions; compromise has cluster-wide blast radius unless deployment adds an external boundary.
- **F-048 — Relay client routing is under-specified for multi-origin operation:** OPEN / Critical → T-054. Current relay behavior can select an arbitrary active Origin rather than an explicit tenant/origin route.
- **F-049 — Relay resource/admission controls are insufficient for hostile public exposure:** OPEN / High → T-054. Existing stream/channel/goroutine allocation requires explicit quotas, reservations, rate limiting, lifecycle and memory budgets.
- **F-050 — Single-TCP reverse multiplexing creates correlated transport failure/head-of-line risk:** OPEN / High → T-056. Preserve WGRL/1 only as compatibility path; qualify independent stream-capable transport.
- **F-051 — Real transport diversity is incomplete:** OPEN / High → T-056/T-057. Provider abstraction exists, but `SecureRelayTransport` is intentionally fail-closed/stubbed and current dual failover relies on loopback SOCKS sidecars.
- **F-052 — Static relay configuration limits resilience and rotation:** OPEN / High → T-059. Signed multi-source discovery and last-known-good operation are absent.
- **F-053 — Authority outage is an intentional availability SPOF:** OPEN / Medium/High → T-060. Current default fail-closed `503` remains correct; an optional bounded lease mode may be added without weakening revocation semantics beyond its explicit TTL.
- **F-054 — Cross-stack qualification still contains mocked transport segments:** OPEN / High → T-061. Go E2E covers real relay/origin/gateway; Rust full-stack path still uses test SOCKS relay components rather than all release processes.
- **F-055 — Supporting hardening annex has stale statements relative to qualified main:** OPEN / Medium → T-062. `docs/implementation/NETWORK_SECURITY_HARDENING_PLAN.md` must become a reconciled evidence annex, not a competing active plan.

---

# 6. Task state

Status vocabulary: `DONE`, `READY`, `IN_PROGRESS`, `BLOCKED`, `REOPENED`, `NEEDS_REQUALIFICATION`, `TODO`, `DEFERRED`, `EXPERIMENTAL`.

## DONE foundations

T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(in-memory baseline), T-030, T-032, T-034, T-035, T-043, T-049, T-050, T-039A, T-039B1, T-039B2, T-039, T-036, T-037, T-040, T-041, T-042, T-048, T-044, T-045, T-046 and **T-047** are DONE under their recorded scopes.

### T-047 — Original-scope final re-audit/convergence
**Status:** DONE · **Priority:** historical baseline gate

Qualified contract:
- Original public WebGate scope converged across I-001..I-032 and F-001..F-045.
- Multi-stack static/test/race/mutation/fuzz/release checks passed under the evidence recorded in Git history.
- This task is not evidence for I-033..I-044 or F-046..F-055.

## Active SecureAcces P0 work

### T-038 — Authoritative SecureAcces + administrator authorization
**Status:** IN_PROGRESS · **Priority:** P0

Converges when T-049 + T-050 + T-052 + T-051 are complete.

### T-052 — Private durable SecureAcces production provider/deployment
**Status:** IN_PROGRESS / PRIVATE-CI-BLOCKED · **Priority:** P0

Public bridge is qualified in WebGate. Private candidate work exists in `Homiakus/SecureAcces`; complete only after executable private CI passes, normal promotion occurs, public/private protocol compatibility is exercised, and SecureAcces backup/restore evidence exists.

### T-051 — SecureAcces administrator management authorization
**Status:** TODO · **Priority:** P0

Replace shared-token-only management with request-scoped SecureAcces principal/actor authorization. Shared token may remain only as explicitly scoped bootstrap/recovery factor. Converge privileged action + durable audit into a fail-closed management transaction where feasible.

---

# 7. Next-generation transport/security execution program

## T-053 — Native Relay Secure Envelope + per-node identity
**Status:** READY · **Priority:** P0 · **Owns:** F-046, F-047 · **Protects:** I-003, I-033, I-034

### Root cause

Current WGRL framing validates magic/version/payload size and Origin authentication uses a shared `ClusterToken`, but the WebGate-owned relay protocol does not itself guarantee encrypted authenticated transport. Security must not depend on an undocumented or accidentally absent outer sidecar.

### Required design

- Add protocol-negotiated secure relay link with authenticated encryption.
- Prefer standard TLS 1.3/mTLS for node-to-relay link establishment; do not invent new cryptographic primitives.
- Give every Relay and Origin a unique identity/key/certificate and stable key ID.
- Bind identity to expected node role and cluster/origin scope.
- Add certificate/public-key pin or signed trust bundle semantics suitable for private deployments.
- Add key rotation with overlap, expiry and revocation.
- Add replay-resistant connection challenge/transcript binding.
- Keep `ClusterToken` only as explicitly configured bootstrap/recovery mechanism; production mode rejects shared-token-only operation.
- Preserve version negotiation so WGRL/1 can be rejected or used only in an explicitly insecure/dev compatibility mode.
- Control and data frames after handshake must be cryptographically integrity-protected.

### Negative/attack qualification

```text
plaintext client to production relay → rejected
wrong relay identity → rejected
wrong origin identity/role → rejected
expired/revoked credential → rejected
replayed authentication transcript → rejected
shared token only in production mode → startup/reconnect rejected
MITM certificate/key substitution → rejected
frame mutation after secure handshake → rejected
key rotation overlap → old+new valid only inside configured overlap
post-revocation old key → rejected
```

### Exit contract

No production WebGate relay/origin path can report `Ready` without native cryptographic peer authentication and encrypted integrity-protected transport.

---

## T-054 — Explicit Relay Routing + Admission Controller
**Status:** TODO · **Priority:** P0 · **Depends on:** T-053 · **Owns:** F-048, F-049 · **Protects:** I-016, I-017, I-035, I-036

### Required design

Replace `first active origin` semantics with an explicit routing contract:

```text
RelayRouteKey = {
  cluster_id,
  origin_id,
  optional tenant/routing namespace,
  protocol_version
}
```

Client admission must use a short-lived relay capability or equivalent authority-signed/relay-verifiable token that is distinct from application authorization and that binds at minimum:

```text
subject/device or anonymous bootstrap class where explicitly allowed
relay/fleet scope
cluster/origin target
not-before / expiry
nonce or token id
max stream/session envelope
policy epoch/version where applicable
```

Relay remains unable to authorize an application request; it only decides whether a transport stream may be routed to the named Origin.

Add a centralized relay admission/resource controller:

- max authenticated Origins;
- max reservations/origin;
- max active streams/client and origin;
- new-stream rate limit;
- handshake rate limit;
- per-peer and global bandwidth envelopes;
- per-stream/session byte/time limits;
- global/per-tenant memory budgets;
- bounded queues;
- real idle and dead-session expiry;
- overload shedding before allocating expensive buffers/workers;
- metrics for rejected/limited traffic without leaking secrets.

### Negative/attack qualification

```text
unknown origin id → fail closed
cross-cluster route attempt → fail closed
valid transport credential for origin A used for origin B → fail closed
expired capability → fail closed
stream flood before auth → bounded resources
stream flood after auth → quotas enforced
slowloris/half-open stream → bounded timeout
one tenant exhausts quota → other tenant remains healthy
origin disconnect → all bound client streams closed/reset deterministically
routing table replacement/race → no cross-origin crosstalk
```

---

## T-055 — Client↔Origin End-to-End Secure Session
**Status:** TODO · **Priority:** P0 · **Depends on:** T-053, T-054, T-040 · **Owns:** relay trust minimization · **Protects:** I-007, I-009, I-037

### Goal

Turn Relay into a blind transport forwarder. A compromised Relay must not be able to read or silently modify protected application payloads.

### Required design

- Establish an authenticated E2E session between WebGate Client device identity and the intended Origin/WebGate gateway identity.
- Bind E2E handshake to explicit `OriginID`, device/account/session context and negotiated protocol version.
- Use a standard reviewed secure-channel construction; do not implement bespoke cryptography.
- Relay receives only the routing metadata strictly required to deliver ciphertext.
- Encrypt protected request/response bytes including WebGate internal session headers before transit relay visibility where architecture permits.
- Provide forward-secure ephemeral session keys.
- Enforce nonce/sequence uniqueness and replay rejection.
- Define session resumption carefully; resumed state may not bypass device/session revocation checks.
- Define rekey thresholds by bytes/time and on path changes where required.
- Minimize observable metadata; document what Relay still learns (IP endpoints, timing, sizes, selected routing key, etc.).

### Negative qualification

```text
relay reads ciphertext as HTTP → impossible/no plaintext
relay flips ciphertext bit → session fails integrity check
relay replays old record → rejected
relay redirects ciphertext to wrong Origin → E2E identity mismatch/rejected
stolen relay credential without client/origin identity → no application plaintext
expired/revoked client device session → gateway authorization still denies
```

---

## T-056 — Transport V2: QUIC/H3 + H2/TLS + stable provider ABI
**Status:** TODO · **Priority:** P1 · **Depends on:** T-053, T-054 · **Owns:** F-050, F-051 · **Protects:** I-014, I-042

### Required provider contract

The existing transport abstraction must become capable of owning a real network path rather than only a local sidecar endpoint. Provider API must expose enough information for policy and adaptive routing, for example:

```text
start / stop
connect or open_stream
probe
capabilities
transport_family
network_requirements (udp/tcp/443/etc.)
failure_domain
health snapshot
session migration/resumption support
```

### Required production providers

1. **QUIC/H3 provider** — independent streams, modern congestion control, connection migration where supported.
2. **H2/TLS provider** — TCP/443-compatible fallback with multiplexed HTTP/2 semantics where UDP/H3 is unavailable.
3. **External sidecar provider** — preserve compatibility with independently maintained protected transport implementations through strict loopback-only integration.

Optional/experimental:

- MASQUE CONNECT-UDP/CONNECT-IP where it improves the scoped transport design.
- Reverse CONNECT draft experiments behind feature flags; no production sole dependency while the protocol remains an Internet-Draft.
- Multipath QUIC experiments behind feature flags; no requirement for baseline production until standardized/interoperable enough for WebGate risk tolerance.

### Required behavior

- WGRL/2 stream semantics map cleanly onto independent transport streams where available.
- A lost QUIC stream does not serialize unrelated application streams behind TCP retransmission.
- H2/TLS remains a materially different fallback failure mode.
- Provider readiness requires real end-to-end probe, not process existence.
- External sidecar providers remain loopback-only for plaintext handoff.

---

## T-057 — Adaptive Path Manager
**Status:** TODO · **Priority:** P1 · **Depends on:** T-056 · **Owns:** dynamic failover quality · **Protects:** I-005, I-013, I-014, I-038, I-043

### Path classes

```text
Direct authenticated path
Trusted peer relay
Infrastructure Relay A/B/C
H3/QUIC transport
H2/TLS transport
Qualified external sidecar transport
```

Not every deployment must enable every class, but production must have at least two materially independent qualified paths.

### Path scoring

Use measured evidence rather than static primary/fallback labels. Inputs may include:

- successful end-to-end probes;
- recent connection success rate;
- RTT and handshake time;
- packet loss/timeout rate where observable;
- consecutive failures;
- transport family;
- provider/ASN/region failure-domain correlation;
- current network compatibility (UDP availability, IPv4/IPv6 reachability, captive/restricted network conditions);
- cooldown and hysteresis to prevent route flapping.

Connection establishment may race a small bounded set of qualified candidate paths. Only establishment is raced; non-idempotent application requests are never duplicated merely to choose a path.

### Exit contract

Single relay/provider/network-family loss causes bounded failover without browser recreation, direct Internet escape, authorization bypass or path thrashing.

---

## T-058 — Direct Path Upgrade + Trusted Peer Relay
**Status:** TODO · **Priority:** P1 · **Depends on:** T-055, T-057 · **Protects:** I-012, I-036, I-037, I-039

### Direct path

Add authenticated rendezvous/NAT discovery and attempt direct Client↔Origin connectivity after the secure identities and intended route are known.

Requirements:

- no inbound port-forwarding requirement as a baseline;
- support NAT rebinding/address changes where underlying transport permits;
- direct attempt cannot expose a generic listening service;
- only the authenticated WebGate secure session may use the negotiated path;
- direct failure falls back to a qualified relay path;
- no direct-path attempt may bypass destination or service policy.

### Peer relay

Allow explicitly trusted WebGate nodes to relay traffic only under reservation/capability control.

Reservation must be:

```text
identity-bound
signed/verifiable
expiry-bounded
connection-count bounded
byte/time bounded
revocable
not an open SOCKS/HTTP proxy
```

A peer relay never becomes an authorization authority and should remain opaque to E2E payloads.

---

## T-059 — Signed Relay Directory + resilient bootstrap
**Status:** TODO · **Priority:** P1 · **Depends on:** T-053, T-057 · **Owns:** F-052 · **Protects:** I-019, I-040, I-043

### Signed directory object

At minimum:

```text
schema/version
issued_at
expires_at
policy_epoch
relay identity key id
addresses/domains
supported protocols
provider/ASN/region/failure-domain metadata
capabilities
priority/weight hints
signature
```

### Distribution

- multiple HTTPS mirrors/endpoints;
- last-known-good signed cache;
- pinned bootstrap trust root;
- rollback protection;
- explicit expiry behavior;
- DNS SVCB/HTTPS may advertise candidates/bootstrap hints;
- DHT/gossip may discover candidates experimentally, but discovered data remains untrusted until validated by signed directory/policy.

### Failure qualification

```text
one directory server offline → client still operates from another source/LKG
DNS unavailable → existing trusted directory still usable until expiry
malicious DNS answer → cannot create trusted relay
old signed directory replay → rollback rejected
expired directory with no replacement → explicit degraded/offline policy, never silent trust extension
compromised single mirror → signature validation protects integrity
```

---

## T-060 — Control-plane resilience with bounded authorization leases
**Status:** TODO · **Priority:** P1 · **Depends on:** T-052, T-051 · **Owns:** F-053 · **Protects:** I-008, I-009, I-018, I-041

Default production behavior remains fail-closed on unavailable authority until this task is fully qualified.

Optional continuity mode may issue short-lived SecureAcces-signed authorization leases containing an explicit bounded authorization statement, for example:

```text
account_id
device_id
workspace/service scope
permissions
policy_epoch
not_before
expires_at
lease_id
issuer/signing key id
signature
```

Requirements:

- lease verification works offline with pinned/rotatable authority public keys;
- maximum TTL is policy-controlled and visible to administrators;
- revocation latency is explicitly equal to or less than the accepted lease TTL unless an online revocation channel is available;
- clock skew handling is bounded;
- no lease extension is possible without authority;
- privilege elevation requires fresh authority, not stale lease;
- high-risk admin/process operations may be configured to require online authority regardless of ordinary data-plane leases.

This task is accepted only if the security/availability tradeoff is explicit in configuration and documentation. It must never appear as an accidental cache fallback.

---

## T-061 — Extended release-binary adversarial qualification
**Status:** TODO · **Priority:** P0 final next-gen gate · **Depends on:** T-053..T-060 as applicable · **Owns:** F-054 · **Protects:** I-020, I-021, I-022, I-044

### Required topology

Launch actual built binaries/processes, not in-process transport mocks:

```text
webgate-app / browser qualification client
  ↓
real local restricted proxy
  ↓
real Transport V2 provider
  ↓
real webgate-relay A/B(/C)
  ↓
real webgate-origin agent
  ↓
real webgate-server gateway
  ↓
real SecureAcces-compatible test authority
  ↓
real local protected HTTP service
```

### Adversarial matrix

- Relay A kill/restart.
- Relay B kill/restart.
- Origin agent restart.
- Gateway restart.
- Authority unavailable/recovery.
- UDP unavailable; H2/TLS fallback remains functional.
- IPv4/IPv6 asymmetry where platform supports test isolation.
- NAT rebinding/direct-path loss.
- DNS/discovery failure.
- Signed directory rollback/tamper/expiry.
- Relay credential expiry/revocation/rotation.
- Origin identity mismatch.
- Client capability replay.
- Ciphertext mutation/replay.
- stream flood / slowloris / oversized frame.
- cross-origin and cross-tenant routing attempts.
- high latency/loss causing path switch without route flapping.
- all protected paths unavailable → explicit offline/fail-closed; no OS direct fallback.

### Performance evidence

Record at minimum:

- connection setup latency p50/p95/p99;
- warm session open latency;
- failover interruption time;
- relay CPU/memory per 100/1000 concurrent streams where practical;
- bounded memory under rejected stream flood;
- throughput comparison WGRL/1 TCP vs H3/QUIC and H2/TLS for representative workloads;
- zero crosstalk/corruption across concurrent streams.

---

## T-062 — Documentation and evidence convergence
**Status:** TODO · **Priority:** P2 · **Depends on:** plan changes as they land · **Owns:** F-055

- Reconcile `docs/implementation/NETWORK_SECURITY_HARDENING_PLAN.md` with current reality.
- Mark superseded prototype-gap statements as historical evidence rather than current task truth.
- Keep `MASTER_PLAN.md` as the only owner of statuses/priorities/dependency order.
- Add architecture diagrams for secure envelope, E2E session, explicit multi-origin routing, adaptive paths and discovery trust chain.
- Document observable relay metadata and threat model after T-055.
- Document provider failure-domain qualification rules after T-057.
- Document direct/peer relay reservation security after T-058.
- Document offline lease risk/TTL semantics after T-060.

---

# 8. Dependency order / current priority

Two work lanes may progress independently until their integration gates.

```text
SECUREACCES LANE
T-049 DONE → T-050 DONE → T-052(private) → T-051 → T-038 convergence ───────────────┐
                                                                                   │
NETWORK / TRANSPORT LANE                                                           │
T-053 → T-054 → T-055 ───────────────┐                                             │
          └──────→ T-056 → T-057 ────┼→ T-058 → T-059 ────────────────────────────┤
                                     │                                             │
T-052 + T-051 ───────────────────────┴→ T-060 ─────────────────────────────────────┤
                                                                                   ▼
                                                                            T-061 final
                                                                                   ↓
                                                                            T-062 docs
```

Current execution priority:

1. **T-053 — Native Relay Secure Envelope + per-node identity.** This is the highest-leverage public P0 because the current raw WGRL/cluster-token boundary is weaker than the rest of the application security model.
2. In parallel, continue/check executable private SecureAcces CI for T-052; if unblocked, finish T-052 → T-051 → T-038.
3. **T-054 — Explicit routing + admission** immediately after T-053.
4. **T-055 — E2E Client↔Origin session** before peer/direct-path expansion.
5. T-056 → T-057 → T-058 → T-059.
6. T-060 only after SecureAcces production identity/durability/admin semantics are qualified.
7. T-061 is the next-generation final release gate.
8. T-062 continuously reconciles docs but may only be marked DONE after the implemented architecture is stable.

---

# 9. Design baselines / research anchors

These are evidence inputs, not automatic implementation mandates:

- TLS 1.3 (`RFC 8446`) for standard authenticated encrypted transport.
- QUIC (`RFC 9000`) and QUIC TLS mapping (`RFC 9001`).
- HTTP Datagrams (`RFC 9297`).
- CONNECT-UDP (`RFC 9298`).
- CONNECT-IP (`RFC 9484`).
- SVCB/HTTPS service binding (`RFC 9460`) for signed-directory-assisted bootstrap hints.
- MASQUE work for HTTP/3-based proxy/tunnel patterns.
- IETF Reverse HTTP CONNECT work as an experimental reference for outbound-Origin reverse connectivity; draft status means no sole production dependency.
- IETF Multipath QUIC work as experimental future path diversity; draft status means no baseline production dependency.
- SPIFFE/SVID principles as reference for short-lived workload/node identity.
- libp2p Circuit Relay v2 / DCUtR as reference patterns for bounded relay reservations and direct-path upgrade.
- DERP/peer-relay style architectures as reference patterns for relay fallback and path hierarchy.

Any external protocol/library addition receives its own supply-chain, interoperability and failure-mode review before promotion.

---

# 10. Permanent verification matrix

## Baseline

```text
python3 -m unittest discover -s scripts/tests -p 'test_*.py' -v
python3 scripts/project_manager.py verify --dry-run
python3 scripts/run_mutation_tests.py
python3 scripts/check_architecture.py
cargo metadata --locked --no-deps --format-version 1
cargo fmt --all -- --check
cargo check --workspace --all-targets --locked
cargo test --workspace --locked
cargo clippy --workspace --all-targets --locked -- -D warnings
cargo deny check --all-features
cd server && go mod tidy -diff
cd server && go vet ./...
cd server && go test ./...
cd server && go test -race ./pkg/persistence ./pkg/registry ./pkg/origin ./pkg/relay ./pkg/gateway
cd server && CGO_ENABLED=0 go test ./pkg/persistence ./pkg/registry ./pkg/origin ./pkg/relay
cd server && CGO_ENABLED=0 go build ./cmd/webgate-server ./cmd/webgate-relay
```

## Durability negatives

```text
caller mutates input/read snapshot → registry unchanged
durable write failure → memory unchanged
closed SQLite store → mutation fails without version/status advance
corrupt registry/control payload/checksum → load/startup fails closed
corrupt backup → restore fails; target remains absent
audit UPDATE/DELETE → trigger rejects
same audit ID + different payload → corruption
runtime PID/state/StartedAt → never resurrected
PoP challenge → never restored
device without AccountID → durable write rejected
runtime control secret → absent from durable JSON and backup bytes
existing empty state DB → defaults are not re-seeded
persist-before-memory mutant → killed
secret-persistence mutant → killed
Go module drift → `go mod tidy -diff` fails
CGO-only regression → portability gate fails
```

## Authorization negatives

```text
nil/unavailable authority → 503 before upstream unless explicit qualified signed-lease mode applies
policy deny → 403
network failure/redirect/oversized/malformed authority response → unavailable
200 allow with wrong AccountID/DeviceID or empty SessionID → unavailable
device without AccountID → deny before authority I/O
child process environment → no WEBGATE_AUTHORITY_TOKEN / WEBGATE_ADMIN_TOKEN
expired offline lease → deny
lease wrong DeviceID/AccountID/policy epoch → deny
```

## Relay/security negatives

```text
production plaintext WGRL → reject
wrong relay/origin identity → reject
revoked/expired node key → reject
auth transcript replay → reject
shared cluster token only in production → reject
unknown explicit OriginID → reject
cross-cluster route → reject
relay capability replay/expiry → reject or bounded single-use semantics
pre-auth flood → bounded resources
post-auth stream flood → quota enforced
slow stream → timeout/budget enforced
ciphertext mutation/replay → E2E session reject
relay redirect to wrong Origin → E2E identity reject
```

## Path/discovery negatives

```text
Relay A loss → alternate qualified path
UDP blocked → H2/TLS qualified fallback where configured
all protected transports down → explicit offline, no direct OS fallback
malicious DNS/DHT candidate → not trusted without signed directory/policy
signed directory rollback → reject
signed directory expiry → explicit policy outcome
path score oscillation → hysteresis/cooldown prevents flapping
direct path fails → relay fallback, not public direct service exposure
peer relay reservation expires → stream creation rejected
```

---

# 11. Process rules

1. Synchronize remote `main` and CI.
2. Select one atomic task by risk/dependency leverage.
3. State root cause, invariants, protected surfaces, rollback and verification.
4. Characterize wrong behavior first where feasible.
5. Implement the smallest root-cause fix.
6. Verify cheap → expensive and attack with negative/mutant evidence.
7. Reconcile this plan before final promotion.
8. Recheck remote HEAD before promotion.
9. Fast-forward only verified state; never force push.
10. Repeat CI on `main` before marking DONE when executable CI is available.
11. Immediately select the next task after qualification.
12. Do not mark a protocol/provider `Ready` because an interface, mock, sidecar config or handshake stub exists.
13. Do not introduce bespoke cryptography where a standard reviewed secure-channel construction satisfies the requirement.
14. Do not make a draft protocol the only production path.
15. Do not trade fail-closed behavior for availability without an explicit bounded, signed policy mechanism and negative tests.

Green CI never overrules a stronger invariant. A workflow that never starts executable steps is not code evidence.

---

# 12. Recent iteration checkpoint

- **T-039:** durable WebGate state, append-only audit, recovery, no resurrection, persist-before-memory.
- **T-036:** real restricted loopback proxy.
- **T-037:** real outbound reverse Origin connectivity and relay node.
- **T-040:** production device key/PoP cryptography.
- **T-041:** Servo BrowserCapsule proxy enforcement.
- **T-042:** dual-relay failover.
- **T-048:** transactional runtime config binding.
- **T-044:** race/mutation/fuzz feedback loop.
- **T-045:** original multi-hop E2E qualification.
- **T-046:** signed release/distribution pipeline.
- **T-047:** original-scope convergence checkpoint at `704e1ac84f283623f27da6a60706649fd0d08b70`.
- **2026-09-02 architecture review:** accepted F-046..F-055 and opened T-053..T-062. Original convergence remains historical evidence; production target is now extended by I-033..I-044.

No force push.

---

# 13. Convergence criterion

WebGate next-generation convergence requires all of the following:

- original I-001..I-032 remain green;
- I-033..I-044 are implemented and adversarially qualified;
- F-046..F-055 are resolved or explicitly risk-accepted with documented scope;
- SecureAcces private production authority/durability/admin authorization converge under T-052/T-051/T-038;
- production relay/origin links have native authenticated encryption and rotatable per-node identity;
- relay routing is explicit and multi-tenant safe;
- relay resources are reservation/quota/rate/memory bounded;
- relay cannot read or silently modify Client↔Origin application payload after T-055;
- at least two materially independent transport/failure domains are qualified, including H3/QUIC and H2/TLS or an explicitly justified equivalent diversity set;
- direct/peer relay features, if enabled, remain authenticated, bounded and non-open-proxy;
- discovery is signed, versioned, expiry-bounded and rollback-aware;
- authority outage behavior is either strict fail-closed or an explicitly configured, short-lived signed-lease mode with bounded revocation latency;
- release-binary cross-process adversarial qualification T-061 passes;
- docs/evidence are reconciled by T-062;
- the exact final state reaches `main` without force push and its available CI/qualification evidence is recorded.
