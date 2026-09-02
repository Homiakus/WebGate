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

---

# 14. Commercialization and productization execution program

This program is part of the same living plan but does **not** weaken or replace the technical/security convergence contract above. Commercial readiness is downstream of security truth. A product, pricing page, sales document or README may not use a stronger readiness claim than the strongest qualification gate actually passed.

## Commercial positioning hypothesis

WebGate should not be positioned primarily as a VPN replacement. The highest-value product hypothesis is:

> **Self-hosted private enterprise workspace for controlled access to internal web applications without exposing the Origin publicly and without routing the whole operating system through a VPN.**

Primary product attributes to validate:

```text
application-scoped access
private Origin behind NAT/CGNAT
no inbound port-forwarding requirement
WebGate-owned controlled browser path
Zero-Trust account/device/service authorization
self-hosted control and data-plane option
Android/field-device applicability
multiple independent protected paths
strong auditability and fail-closed behavior
```

Primary ICP hypotheses, in priority order for validation rather than assumed truth:

1. Industrial/manufacturing organizations with internal web HMI, MES, ERP, dashboards, documentation and engineering systems.
2. Laboratories/regulated technical organizations with private LIMS, reports, monitoring and internal portals.
3. Organizations granting constrained access to contractors, suppliers or unmanaged/BYOD endpoints.
4. Distributed branches/sites whose local application server is behind CGNAT or dynamic addressing.
5. Android/field/warehouse/service-device workflows where a controlled private workspace is preferable to a full-device VPN.

Explicit non-targets for the first commercial product:

- generic consumer VPN;
- unrestricted Internet proxy;
- general-purpose L3 corporate network replacement;
- full PAM replacement for SSH/database/Kubernetes workflows;
- global consumer censorship-circumvention service as the core business proposition.

## Product invariants

- **P-I-001 Evidence-bound claims:** `Production`, `Enterprise`, `HA`, `E2E`, `Zero Trust`, `offline-capable` and similar claims require the exact owning technical/product gate to be DONE. README/website/demo wording cannot outrun the plan.
- **P-I-002 Renderer-independent security boundary:** adding a compatibility browser engine may change rendering implementation but may not bypass WebGate navigation policy, loopback/restricted transport, device identity, service authorization, audit or fail-closed behavior.
- **P-I-003 Explicit compatibility:** supported applications/browser engines are represented by a tested compatibility matrix. Unsupported behavior is reported explicitly; there is no silent insecure browser fallback.
- **P-I-004 Fast reversible onboarding:** the standard pilot path must install, enroll, register one private service, authorize one user/device and open the first page without inbound NAT changes; uninstall/rollback must be documented and leave no hidden network changes.
- **P-I-005 Enterprise lifecycle:** managed deployment, upgrade, rollback, credential rotation, backup/restore, observability, audit export and support diagnostics are first-class product behavior, not manual tribal knowledge.
- **P-I-006 Security-core transparency:** Apache-2.0/open-source security foundations must not be intentionally crippled to force payment. Commercial differentiation should primarily come from operations, fleet management, enterprise integration, support/SLA and hosted services.
- **P-I-007 Pilot evidence before GA:** general availability requires repeatable evidence from representative non-developer deployments, not only repository-local integration tests.
- **P-I-008 Privacy-bounded telemetry:** product analytics/diagnostics are opt-in or explicitly administrator-controlled, minimize content/identity collection, and never require sending protected application payloads to a vendor service.

## Product/commercial findings accepted 2026-09-02

- **P-F-001 — Category ambiguity:** HIGH → T-063. If marketed as "another VPN", WebGate competes directly with mature mesh/ZTNA products while hiding its browser/application-scoped differentiation.
- **P-F-002 — Browser compatibility is the largest product-fit risk:** HIGH → T-064. Servo control is strategically valuable, but commercial adoption requires measured real-world web application compatibility and a safe compatibility strategy.
- **P-F-003 — Time-to-first-service is not yet a commercial acceptance gate:** HIGH → T-065. Security architecture is sophisticated, but customers will judge installation and first-value time before appreciating internal design quality.
- **P-F-004 — No external pilot evidence:** HIGH → T-066. Repository tests cannot demonstrate administrator usability, application compatibility, deployment friction, support cost or purchasing value.
- **P-F-005 — Enterprise operational surface is incomplete as a product:** HIGH → T-067. SSO/directory integration, managed endpoint deployment, SIEM integration, fleet operations, upgrades and supportability materially affect enterprise purchase decisions.
- **P-F-006 — Apache-2.0 code alone is not a durable commercial moat:** MEDIUM/HIGH → T-068. Monetization must rely on product operations, enterprise management, hosted relay/control services, integrations, support, SLA and brand/trust rather than artificial binary scarcity.
- **P-F-007 — Readiness wording can exceed current extended architecture truth:** HIGH → T-069. Historical original-scope convergence cannot justify `Enterprise Qualified` for the next-generation/commercial product.

---

## T-063 — Product positioning, ICP and competitive validation
**Status:** READY · **Priority:** P1 product · **Owns:** P-F-001

### Deliverables

- Define one canonical product category/name and one-sentence value proposition.
- Build a competitive capability matrix against at least representative ZTNA/mesh, private-tunnel and enterprise-browser approaches.
- Separate table-stakes capabilities from true differentiation.
- Define the top 3 ICP hypotheses with buyer, administrator and end-user personas.
- Define top 10 jobs-to-be-done and top 10 reasons a prospect would reject WebGate.
- Interview/observe representative administrators/users where accessible; record evidence separately from assumptions.
- Define `do not build` boundaries so WebGate does not expand into generic VPN/PAM/SWG scope without evidence.

### Acceptance

A prospective customer can understand within one minute:

```text
what WebGate protects
why it is not just a VPN
what must be installed
where traffic/control state lives
what remains self-hosted
which problem it replaces or simplifies
```

The product narrative must stay compatible with P-I-001 and actual implemented state.

---

## T-064 — Browser compatibility strategy and qualification matrix
**Status:** READY · **Priority:** P0 product risk · **Protects:** P-I-002, P-I-003

### Goal

Preserve Servo as a high-control secure engine while eliminating browser-engine compatibility as a hidden blocker to commercial adoption.

### Required architecture decision

Evaluate and qualify a two-mode strategy unless evidence proves one engine is sufficient:

```text
STRICT mode
  Servo
  maximum WebGate-owned rendering/network control
  for qualified internal applications

COMPAT mode
  hardened Chromium/CEF/WebView2-or-equivalent adapter where platform permits
  same WebGate proxy/navigation/device/auth/audit boundary
  no direct network fallback
```

This is not permission to launch the system browser. A compatibility engine is acceptable only if WebGate owns and verifies its network path and lifecycle strongly enough to preserve P-I-002.

### Compatibility corpus

Build automated and manual qualification for representative classes:

- static/SSR applications;
- SPA/CSR React/Vue/Angular-like applications;
- WebSocket/SSE applications;
- file upload/download workflows;
- large tables/canvas/chart-heavy dashboards;
- authentication redirects/OIDC flows where permitted;
- clipboard/download restrictions when policy exists;
- responsive/mobile web applications;
- representative internal engineering, monitoring, ERP/LIMS-like and documentation applications that can legally be tested.

For every application/test case record:

```text
engine
platform
WebGate version
result PASS/PARTIAL/FAIL
known workaround
security-boundary result
performance notes
```

### Exit contract

- compatibility claims are machine-readable/versioned where practical;
- no engine can bypass the restricted proxy;
- switching engine never weakens service authorization or audit;
- application incompatibility produces explicit diagnostics instead of silent external-browser fallback.

---

## T-065 — 15-minute first-service onboarding
**Status:** TODO · **Priority:** P1 product · **Depends on:** stable T-053/T-054 identity/routing contracts · **Owns:** P-F-003 · **Protects:** P-I-004

### Target journey

From clean supported systems, a competent administrator following the standard pilot path should be able to reach first protected value with a target of **≤15 minutes**, excluding OS/package download time outside WebGate control:

```text
install Origin/server package
  ↓
enroll Origin identity
  ↓
connect/validate relay path
  ↓
register existing local HTTP service
  ↓
create/import user + device authorization
  ↓
generate safe invite/deep link
  ↓
install/open WebGate client
  ↓
first authorized private page
```

No inbound router/NAT changes are allowed in the standard path.

### Required product work

- one canonical installer/launcher per Tier-1 platform;
- interactive setup wizard plus non-interactive automation mode;
- preflight network, clock, certificate, loopback-service and authority checks;
- automatically generated but reviewable safe defaults;
- QR/deep-link enrollment for Android where secure;
- deterministic diagnostics when relay/authority/service is unreachable;
- `doctor` output understandable by support without secrets;
- uninstall/cleanup and documented rollback;
- idempotent reinstall/upgrade path;
- no need for users to understand relay ports, routes or transport internals in ordinary operation.

### Qualification

Run clean-machine onboarding repeatedly and record median/p95 time, error rate, steps requiring manual intervention and top failure causes. The 15-minute target may be changed only from measured evidence, not to conceal friction.

---

## T-066 — Design-partner and pilot evidence program
**Status:** TODO · **Priority:** P1 commercial · **Depends on:** T-038, T-053, T-054, T-055, T-064, T-065 · **Owns:** P-F-004 · **Protects:** P-I-007, P-I-008

### Pilot stages

```text
Stage A — internal/lab dogfood
Stage B — trusted technical design partner
Stage C — controlled external business pilot
Stage D — repeatable multi-customer pilot template
```

Each external pilot must have explicit scope and must not be represented as GA/Enterprise Qualified.

### Minimum pilot diversity

Seek evidence across at least three materially different scenarios, for example:

1. private internal web application behind CGNAT/dynamic IP;
2. contractor/BYOD restricted access;
3. Android/field or industrial-site workflow.

### Evidence collected

- setup time and administrator intervention;
- application compatibility;
- daily active use/session success;
- reconnect/failover behavior;
- support tickets by root cause;
- upgrade/rollback success;
- CPU/memory/bandwidth cost at client/relay/origin;
- user task completion versus previous access method;
- security/admin objections from customer review;
- willingness-to-pay/packaging feedback without treating stated intent as booked revenue.

Protected application contents are not collected as product analytics.

### Exit contract

At least three representative pilot deployments can be reproduced from documented procedures, with known issues, measured support burden and explicit evidence of what value users/admins received.

---

## T-067 — Enterprise operations and integration surface
**Status:** TODO · **Priority:** P1 commercial · **Depends on:** T-038 and stable client/origin management contracts · **Owns:** P-F-005 · **Protects:** P-I-005

### Required enterprise capabilities or explicit integrations

- OIDC/SAML SSO through SecureAcces or a defined identity boundary;
- SCIM/directory lifecycle where appropriate;
- group/role-to-service policy mapping;
- managed device enrollment and revocation;
- MDM/config-profile deployment for supported endpoint platforms where practical;
- signed policy/config rollout rings;
- staged update, rollback and version fleet visibility;
- relay/origin/client health inventory;
- HA control-plane deployment documentation;
- backup/restore drill for authoritative state;
- JSON/CEF/syslog-or-equivalent audit/SIEM export without secret leakage;
- configurable retention/export boundaries;
- support bundle generation with aggressive secret/content redaction;
- certificate/node-key rotation workflows;
- maintenance-mode and break-glass semantics with durable audit;
- capacity/SLO dashboards for relay and authority fleet.

### Acceptance

A customer should be able to operate more than one Origin, more than one relay and a fleet of devices without SSHing into every machine for normal lifecycle actions.

---

## T-068 — Commercial packaging, licensing and service model
**Status:** TODO · **Priority:** P2 commercial · **Depends on:** T-063, informed by T-066 · **Owns:** P-F-006 · **Protects:** P-I-006

Current Apache-2.0 licensing remains the baseline unless a separate deliberate legal/compatibility decision is made. This task must not silently relicense existing contributions.

### Packaging hypothesis to test

```text
Community
  core client/origin/relay
  baseline self-hosted access
  transparent security foundations

Business
  fleet management
  SSO/directory integrations
  advanced policy/audit
  managed updates
  operational dashboards

Enterprise
  HA patterns
  HSM/KMS/PKI integrations
  air-gapped/offline deployment support
  compliance evidence packs
  premium support/SLA
  advanced SIEM/MDM integrations

Managed services (optional)
  hosted relay fleet
  hosted/managed control components where architecture/privacy permits
  managed upgrades/monitoring
```

### Commercial model work

- evaluate per-user, per-device, per-active-seat, per-site and managed-relay pricing units;
- quantify relay bandwidth/compute/support cost so pricing is not structurally loss-making;
- model gross-margin sensitivity for managed relay traffic;
- define support tiers, response targets and exclusions;
- define trademark/brand policy and commercial distribution rules compatible with Apache-2.0;
- keep security-critical interoperability and export/backup paths free from deliberate lock-in.

No price is considered validated until design-partner/pilot evidence exists.

---

## T-069 — Commercial launch and claim gate
**Status:** TODO · **Priority:** P0 commercial final gate · **Depends on:** T-061, T-062, T-064, T-065, T-066, T-067, T-068 · **Owns:** P-F-007 · **Protects:** P-I-001..P-I-008

### Release vocabulary

Use explicit maturity levels:

```text
Experimental
Technical Preview
Pilot
Production
Enterprise Qualified
```

Minimum interpretation:

- **Experimental:** research/prototype behavior; no production claim.
- **Technical Preview:** usable for controlled evaluation; incomplete production contract is visible.
- **Pilot:** explicitly scoped real deployment with support and known limitations.
- **Production:** T-061 technical gate and owning production dependencies are DONE; supported deployment contract exists.
- **Enterprise Qualified:** Production plus T-064/T-065/T-066/T-067/T-068/T-062 evidence and this T-069 launch review are DONE.

### Final launch review

Before `Enterprise Qualified`:

- reconcile README, website, installer and release notes with actual gate state;
- publish supported platform/browser/application matrix;
- publish reference production architecture and threat model;
- publish support/upgrade/rollback/backup procedures;
- verify no standard installation requires public Origin ports;
- verify standard client operation does not alter OS default route;
- verify at least one clean install-to-first-service onboarding run meets the accepted onboarding SLO;
- review pilot evidence and unresolved high-severity product findings;
- review licensing/SBOM/third-party notices and commercial distribution rights;
- establish vulnerability intake/security response path;
- establish release signing/key-custody procedure;
- establish customer-facing incident/support escalation process;
- explicitly list unsupported/non-goal scenarios.

No marketing statement may promote the product above the achieved maturity level.

---

## Product/commercial dependency lane

This lane runs in parallel but cannot overtake the security gates it depends on:

```text
POSITIONING / PRODUCT
T-063 ───────────────┬→ T-068 ──────────────────────────────┐
                    │                                       │
T-064 ───────────────┼→ T-066 ──────────────────────────────┤
                    │                                       │
T-053 → T-054 ─→ T-065 ┘                                   │
T-038 + T-055 + T-064 + T-065 ─→ T-066                     │
T-038 ──────────────────────────→ T-067                     │
                                                            │
TECHNICAL FINAL: T-061 + T-062 ─────────────────────────────┤
                                                            ▼
                                                     T-069 commercial gate
```

Commercial work must not interrupt the current security priority order. T-063 and T-064 can progress in parallel with T-053; T-065 can begin against stable contracts but cannot be qualified before the required secure identity/routing path exists.

---

## Product metrics and evidence dashboard

Track trends, not vanity totals. At minimum:

### Activation

- install success rate by Tier-1 platform;
- median/p95 time to first protected service;
- percentage of standard deployments requiring manual firewall/NAT work (target: zero);
- percentage requiring manual config-file editing;
- enrollment failure reasons.

### Compatibility

- tested application classes by engine/platform;
- PASS/PARTIAL/FAIL compatibility rate;
- regressions per release;
- percentage of deployments requiring COMPAT engine;
- zero insecure external-browser fallback events.

### Reliability

- successful session establishment rate;
- reconnect/failover success;
- crash-free client sessions;
- origin/relay availability evidence;
- upgrade and rollback success rate.

### Operations/support

- support incidents per active deployment;
- median time to diagnose from `doctor`/support bundle;
- top recurring administrator errors;
- percentage of lifecycle actions possible without direct machine shell access.

### Commercial validation

- pilots started/completed;
- pilots converted to paid usage when/if commercial offering exists;
- dominant ICP/use case by observed usage;
- primary replacement/alternative named by customers;
- support and relay cost per active deployment;
- price/package objections and lost-pilot reasons.

Do not optimize metrics by weakening fail-closed/security behavior or by collecting protected application content.

---

# 15. Commercial convergence criterion

Technical convergence in Section 13 is necessary but not sufficient for a commercially mature WebGate.

`Enterprise Qualified` commercial convergence requires:

- T-069 DONE;
- T-061 technical next-generation qualification DONE;
- T-062 documentation/evidence reconciliation DONE;
- T-064 browser compatibility strategy qualified with no security-boundary escape;
- T-065 repeatable onboarding measured on clean supported systems;
- T-066 representative external pilot evidence exists;
- T-067 enterprise lifecycle/operations contract exists;
- T-068 packaging/licensing/support model is internally coherent and legally reviewable;
- no unresolved Critical product/security finding;
- unresolved High findings are either fixed or explicitly accepted with visible scope and customer impact;
- product and README claims match the actual maturity level;
- a new customer can understand deployment ownership, data paths, trust boundaries, backup/upgrade responsibilities and unsupported scenarios before purchase/deployment;
- the exact commercial release state is tagged/signed and backed by the available technical qualification evidence.

---

# 16. Total resilience audit and execution program

This section extends the target beyond ordinary HA. “Total resilience” is an engineering objective meaning **survival of every explicitly modeled single failure and of selected correlated failures without violating security invariants**. It is not a claim that software can survive arbitrary global Internet loss, destruction of all trust roots, simultaneous loss of every deployment site, or failure of a single unreplicated protected backend that is outside WebGate’s control.

T-061 remains the next-generation security/release-binary gate. T-078 becomes the final **system-resilience qualification gate**. From this section onward, `Enterprise Qualified` under T-069 additionally depends on T-078.

## 16.1 Resilience invariants

- **R-I-001 No unacknowledged single point of failure:** every production-critical component is either replicated across a materially independent failure domain or explicitly declared as a deployment-level SPOF with user-visible impact.
- **R-I-002 Correlated-failure awareness:** replicas count as independent only when provider, ASN/network, region/site, power/control dependency, transport family, software rollout ring and administrative credential blast radius are sufficiently independent.
- **R-I-003 Stable recovery:** reconnect/retry behavior uses bounded exponential backoff with jitter, retry budgets and circuit breaking; a mass outage must not create synchronized reconnect storms.
- **R-I-004 Backpressure before collapse:** every public or fan-in boundary has bounded queues, concurrency, memory, bandwidth and work admission. Overload sheds work before exhausting process/host resources.
- **R-I-005 Readiness is end-to-end:** a PID, open socket or successful process spawn is never sufficient for `Ready`. Readiness proves that the component can complete the required protected operation through its downstream dependencies.
- **R-I-006 Graceful drain:** planned restart/update/key rotation removes a node from new work, drains or safely resets existing sessions, and only then terminates where protocol semantics allow.
- **R-I-007 Deterministic degraded modes:** every dependency loss maps to an explicit state such as `Ready`, `Degraded`, `ReadOnly`, `Offline`, or `RecoveryRequired`; no hidden fallback may weaken security.
- **R-I-008 State continuity:** durable state has explicit RPO/RTO, consistent backup/restore, corruption detection and a qualified enterprise HA strategy. Network filesystems are not used for shared live SQLite unless explicitly proven safe; multi-writer semantics require a storage design built for them.
- **R-I-009 Split-brain prevention:** active/passive or replicated control-plane components use fencing/leases/consensus semantics appropriate to their state. Two authorities may not both mutate supposedly singleton state merely because a partition occurred.
- **R-I-010 Reversible releases:** client, relay, Origin, server, policy and schema changes support staged rollout, compatibility windows and automatic/manual rollback without accepting unsigned or older-disallowed artifacts.
- **R-I-011 Time resilience:** security expiry uses wall clock only where required; retry/timeout logic uses monotonic time. Clock skew is detected, bounded and never silently extends credentials/leases indefinitely.
- **R-I-012 Bulkheads:** relay, Origin, authority, protected-service and tenant failures are isolated so one unhealthy dependency or tenant cannot consume all worker, connection, memory or retry capacity.
- **R-I-013 Observability independence:** health and diagnostics remain locally available when central telemetry is unavailable. Telemetry loss must not block the data plane or trigger insecure behavior.
- **R-I-014 Operator-error resilience:** destructive or high-blast-radius operations require preview/validation, scoped authorization, durable audit, safe defaults and rollback/recovery paths.
- **R-I-015 Trust-root resilience:** release, node and authority key loss/rotation/revocation have documented recovery paths that do not require disabling signature verification or accepting arbitrary new roots.
- **R-I-016 Chaos evidence:** a resilience capability is not considered qualified until fault injection or a real failure demonstrates the expected bounded behavior.

## 16.2 Findings accepted from the 2026-09-02 resilience audit

- **R-F-001 — Fixed reconnect cadence can produce herd behavior:** HIGH → T-071. `OriginReverseAgent` currently defaults to a fixed reconnect interval and waits that interval after failure. Large fleets can synchronize after relay/network recovery.
- **R-F-002 — Heartbeat liveness is incomplete:** HIGH → T-071. Origin sends Ping and accepts Pong, but the current loop does not explicitly require a Pong within a bounded heartbeat-ack deadline before declaring the relay dead.
- **R-F-003 — Relay idle lifecycle is under-enforced:** HIGH → T-071/T-054. `IdleTimeout` exists in relay configuration and activity is tracked, but the inspected server path does not yet provide a complete idle-session/stream reaper contract.
- **R-F-004 — Process `Running` can precede application readiness:** CRITICAL runtime correctness → T-072. `ProcessManager` marks a service running after successful `cmd.Start()`/runtime-state write; it does not prove that the protected HTTP service is actually ready.
- **R-F-005 — Child-process supervision lacks restart budgets and backoff:** HIGH → T-072. Exit is observed, but there is no qualified crash-loop policy, restart budget, startup timeout or escalating degraded state.
- **R-F-006 — Stop/restart is not graceful:** HIGH → T-072. Current process stop uses immediate process kill where a real child exists; no generic drain/TERM/grace-period/KILL sequence is qualified.
- **R-F-007 — Local SQLite is durable but still node-local:** HIGH for enterprise HA → T-073. WAL/FULL/checksums and backup/restore protect local durability, but local storage alone does not make the control state survive complete host/site loss with near-zero RTO.
- **R-F-008 — Backend service availability is outside current relay HA guarantee:** HIGH → T-074. Multiple relays do not help when the only Origin/backend instance or its site is down.
- **R-F-009 — Multi-Origin service failover semantics are not yet defined:** HIGH → T-074. Active-active vs active-passive routing, readiness, session affinity, drain and fencing must be explicit before multiple Origins can be called HA.
- **R-F-010 — Signed releases exist without full fleet-safe rollout:** HIGH → T-075. Signing/rollback protection is not equivalent to canary rollout, compatibility gating, health-based automatic rollback and schema rollback safety.
- **R-F-011 — Clock is a security and availability dependency:** MEDIUM/HIGH → T-076. Certificates, signed leases, challenges and directory expiry can fail simultaneously under large time skew; the product needs bounded clock diagnostics/recovery semantics.
- **R-F-012 — Central telemetry/support cannot be a hidden runtime dependency:** MEDIUM → T-077. Resilience requires local health snapshots, event history and support evidence even when an external monitoring path is down.
- **R-F-013 — Current qualification kills individual components but does not yet run sustained correlated-failure game days:** HIGH → T-078.

---

## T-070 — Formal resilience model, service classes and SLO/RTO/RPO budgets
**Status:** READY · **Priority:** P0 architecture · **Owns:** resilience scope · **Protects:** R-I-001, R-I-002, R-I-007, R-I-008

### Deliverables

Define component and service availability classes rather than using one vague `HA` label.

Suggested service classes:

```text
S0 — single-site / single-backend
  transport redundancy may exist
  backend/site loss causes outage

S1 — redundant access path
  ≥2 independent relays/transports
  one Origin/backend site

S2 — redundant Origin/backend
  ≥2 Origins or backend instances
  health-aware failover
  explicit state/session semantics

S3 — enterprise multi-failure-domain
  ≥3 relay failure domains
  ≥2 Origin/backend sites where application permits
  HA authority/control state
  tested DR and staged release
```

For each class define at minimum:

- target monthly availability/SLO;
- session-establishment success SLI;
- maximum bounded failover interruption;
- RTO for relay, Origin, server, authority and protected service;
- RPO for WebGate state and SecureAcces state;
- maximum accepted authorization-lease revocation delay;
- which correlated failures are in/out of scope;
- minimum independent failure domains;
- capacity headroom after losing the largest failure domain.

### Capacity rule

A deployment is not resilient if surviving nodes cannot carry the load after failover. For N+1/N+2 designs, qualification must prove usable capacity after removing the largest required failure domain, not merely prove that another IP exists.

---

## T-071 — Connection recovery, anti-herd control and liveness hardening
**Status:** TODO · **Priority:** P0 runtime · **Depends on:** T-053/T-054 contracts where applicable · **Owns:** R-F-001, R-F-002, R-F-003 · **Protects:** R-I-003, R-I-004, R-I-006, R-I-012

### Origin reconnect controller

Replace fixed retry timing with a bounded state machine:

```text
Healthy
  ↓ failure
FastRetry (small bounded attempts)
  ↓
Backoff(min..max, exponential, full jitter)
  ↓ successful authenticated probe
Warmup
  ↓ stable interval
Healthy
```

Requirements:

- full/decorrelated jitter;
- configurable min/max backoff;
- reset backoff only after a stability window, not after one transient success;
- independent retry budget per relay/failure domain;
- global retry budget per Origin so simultaneous relay loss cannot create unbounded dials;
- circuit breaker for repeatedly failing destinations;
- signed-directory changes may wake a breaker only under bounded policy;
- bounded Happy-Eyeballs/path racing, never infinite parallel dialing.

### Liveness

- Ping includes sequence/timestamp or equivalent liveness correlation.
- Require Pong/ack before a bounded deadline.
- Distinguish write success from peer responsiveness.
- Apply read/write/idle deadlines that are compatible with long-lived sessions.
- Reap orphan streams and dead sessions deterministically.
- Heartbeat traffic itself is rate/budget bounded.

### Storm qualification

Simulate at least 1k logical Origins/clients where practical:

```text
relay outage 60 s
all clients disconnected
relay returns
```

Pass only if reconnect attempts spread over the configured recovery window, relay CPU/memory remain bounded, useful traffic recovers progressively, and no global lockstep retry spike occurs.

---

## T-072 — Production service supervision, readiness and graceful lifecycle
**Status:** TODO · **Priority:** P0 runtime · **Owns:** R-F-004, R-F-005, R-F-006 · **Protects:** I-005, R-I-005, R-I-006, R-I-007, R-I-014

### Architectural rule

WebGate should not unnecessarily reimplement a full init/orchestrator. Prefer a pluggable `ServiceSupervisor` contract capable of using native managers where available:

```text
Linux      systemd or qualified container runtime
Windows    Windows Service / SCM where appropriate
macOS      launchd where appropriate
embedded   explicit WebGate supervisor only where required
```

The existing direct `exec.Cmd` path may remain for development/simple deployments, but enterprise readiness requires a qualified supervisor backend.

### Runtime states

```text
Stopped
Starting
Unready
Ready
Draining
RestartBackoff
Crashed
FailedPermanent
```

### Required probes

- startup probe: process may take time without being restarted prematurely;
- liveness probe: detect wedged process;
- readiness probe: prove the registered protected endpoint can serve the expected minimal request;
- dependency-aware readiness where an application explicitly requires local dependencies;
- probe failure thresholds/hysteresis to avoid flapping.

`Ready` must not be emitted merely because a PID exists.

### Restart policy

- exponential backoff + jitter;
- restart budget per time window;
- max consecutive failures;
- stable-run window before counter reset;
- crash-loop → `FailedPermanent`/operator-visible degraded state;
- never infinitely restart a deterministic configuration failure.

### Graceful stop/update

```text
mark Unready/Draining
stop new routed sessions
wait bounded drain period
send graceful termination
wait grace period
hard kill only after deadline
verify process gone
```

---

## T-073 — State HA, disaster recovery and control-plane continuity
**Status:** TODO · **Priority:** P0 enterprise resilience · **Depends on:** T-039, T-052/T-051 for authority-owned state · **Owns:** R-F-007 · **Protects:** R-I-008, R-I-009, R-I-014

### Deployment modes

Do not force one storage architecture on every deployment.

```text
Standalone
  local SQLite
  qualified backups
  explicit non-zero RTO/RPO

Warm-standby
  continuous/scheduled replicated snapshots or event stream
  one writer
  fenced promotion

Enterprise HA
  storage/control backend designed for replicated multi-node operation
  consensus/transaction semantics appropriate to authoritative state
```

Do not place a live multi-writer SQLite database on an arbitrary network filesystem as an HA shortcut.

### Required enterprise properties

- explicit leader/writer ownership where singleton mutation exists;
- fencing token/epoch on promotion;
- stale writer cannot continue mutating after loss of leadership;
- backup encryption and integrity verification;
- off-host and off-site backup copies;
- restore to clean host/site;
- restore version/schema compatibility checks;
- corruption detection before promotion;
- periodic restore drills, not backup-success logs only;
- documented RPO/RTO evidence.

### DR scenarios

```text
server disk loss
server host loss
site loss
backup store temporarily unavailable
latest backup corrupt
operator deletes state
bad schema migration
old standby attempts promotion
partition creates two apparent leaders
```

Every scenario must have a deterministic outcome and no silent state resurrection/rollback.

---

## T-074 — Multi-Origin / multi-site protected-service resilience
**Status:** TODO · **Priority:** P1/P0 for S2/S3 deployments · **Depends on:** T-054, T-055, T-057, T-070 · **Owns:** R-F-008, R-F-009 · **Protects:** R-I-001, R-I-002, R-I-005, R-I-009

### Core principle

Relay redundancy protects access infrastructure, not the application itself. A protected service that exists on one machine/site remains a SPOF.

Add a service-cluster model such as:

```text
ServiceID
  ├─ Origin A / Backend A / failure-domain A
  ├─ Origin B / Backend B / failure-domain B
  └─ optional Origin C / Backend C
```

Routing must use authority-owned service membership and WebGate-observed health; clients may not arbitrarily select an upstream.

### Modes

- active/passive for stateful applications requiring one active writer;
- active/active for applications that are themselves safe for multi-site concurrency;
- sticky-session mode where required;
- stateless preference for new deployments where possible.

WebGate must not pretend to solve an application database’s replication/consistency. The application’s own data-plane RPO/RTO is a separate declared dependency.

### Failover requirements

- only `Ready` Origins receive new sessions;
- planned maintenance drains before route removal;
- unplanned loss stops new routing within bounded detection time;
- split-brain-sensitive active/passive service requires external/qualified fencing;
- route recovery uses hysteresis to avoid ping-pong between sites;
- capacity remains sufficient after losing one required Origin/site.

---

## T-075 — Fleet-safe releases, migrations and automatic rollback
**Status:** TODO · **Priority:** P1 resilience · **Depends on:** T-046 and stable protocol/version contracts · **Owns:** R-F-010 · **Protects:** I-019, R-I-010, R-I-014

### Rollout rings

```text
0 dev/CI
1 internal canary
2 small pilot ring
3 partial production
4 broad production
```

Requirements:

- signed release + signed rollout policy;
- maximum concurrent unavailable nodes per failure domain;
- never upgrade all relays/Origins in one failure domain simultaneously;
- protocol compatibility window across at least adjacent supported versions;
- schema migrations classified as reversible or irreversible before rollout;
- pre-migration backup/checkpoint for destructive migrations;
- health gates on startup, relay connectivity, authorization and protected-service readiness;
- automatic pause/rollback on statistically or absolutely significant regression;
- rollback cannot violate anti-rollback security policy: recovery artifacts must be explicitly authorized/signed, not simply older binaries accepted ad hoc.

### Negative qualification

```text
bad client release → canary catches before broad rollout
bad relay release → remaining domains keep capacity
bad Origin release → service route drains/falls back
bad schema migration → restore/forward-fix path proven
mixed versions during rollout → protocol remains safe or incompatible node stays unready
release-signing service unavailable → existing version continues; no unsigned bypass
```

---

## T-076 — Time, identity and trust-root continuity
**Status:** TODO · **Priority:** P1 security/resilience · **Depends on:** T-053, T-055, T-059, T-060 · **Owns:** R-F-011 · **Protects:** R-I-011, R-I-015

### Time model

- use monotonic clocks for retry, heartbeat, drain and timeout measurement;
- use wall clock only for externally meaningful validity windows;
- detect large forward/backward wall-clock jumps;
- expose `ClockUntrusted`/degraded diagnostics where credential validation cannot be trusted;
- bounded clock-skew tolerance must never become indefinite expiry extension;
- preserve last-known-good signed time/epoch hints only as auxiliary evidence, never as an unchecked authority.

### Trust-root recovery

Document and qualify:

- relay/origin node-key rotation;
- authority signing-key rotation;
- release-signing-key rotation;
- compromise revocation;
- lost-key recovery;
- offline root + online intermediate model where appropriate;
- dual-control/quorum for highest-blast-radius trust-root replacement in enterprise deployments;
- clients must never be instructed to “disable certificate/signature verification” as a recovery step.

---

## T-077 — Resilience observability, local diagnostics and bounded automation
**Status:** TODO · **Priority:** P1 operations · **Depends on:** T-070 and runtime tasks as they land · **Owns:** R-F-012 · **Protects:** R-I-007, R-I-012, R-I-013

### Required local signals

Each component exposes a local, authentication-bounded health snapshot containing at least:

```text
component version
identity/key id (non-secret)
state Ready/Degraded/Offline/etc.
active failure domain/path
last successful end-to-end probe
last failure reason/category
retry/backoff state
stream/session counts
queue/memory/bandwidth budget use
clock health
configuration/policy version
```

### Event history

- bounded local structured event ring survives enough restart context for diagnosis where appropriate;
- important state transitions are durable/auditable without storing protected application payload;
- external telemetry export is asynchronous and bounded;
- telemetry outage cannot block protected traffic or consume unbounded disk/memory;
- support bundle redacts tokens, private keys, application content and unnecessary personal data.

### Automation guardrails

Automatic recovery may:

- restart a crashed local component within a budget;
- remove an unready path from selection;
- trigger failover;
- rollback a canary under signed rollout policy.

It may not automatically:

- weaken authorization;
- disable certificate validation;
- expand allowed destinations;
- promote an unverified backup;
- create a new trust root;
- keep retrying an overload indefinitely.

---

## T-078 — Chaos, correlated-failure and disaster qualification
**Status:** TODO · **Priority:** P0 final resilience gate · **Depends on:** T-070..T-077 as applicable plus T-061 · **Owns:** R-F-013 · **Protects:** R-I-001..R-I-016

### Qualification levels

```text
L1 component fault
L2 single failure-domain loss
L3 correlated infrastructure fault
L4 control-plane/identity fault
L5 disaster recovery / site loss
```

### Minimum fault matrix

- kill/restart each relay independently;
- lose an entire relay provider/ASN/region simulation;
- kill all instances in one rollout ring;
- drop UDP, then degrade TCP, then restore in different order;
- packet loss, jitter, duplication, reordering and high RTT;
- blackhole where TCP remains established but peer stops responding;
- synchronized disconnect of 1k+ logical clients/Origins followed by recovery;
- DNS unavailable/poisoned candidate;
- directory mirror loss and stale directory replay;
- clock jumps forward/backward;
- node certificate expires/revokes during traffic;
- authority unavailable, then recovery with/without valid lease mode;
- local protected service hangs without exiting;
- crash-looping protected service;
- Origin host loss;
- full Origin/backend site loss for S2/S3 service;
- active/passive split-brain attempt;
- state DB corruption;
- latest backup corrupt, previous backup valid;
- loss of primary control node;
- failed leader fencing attempt;
- disk full / read-only filesystem;
- memory pressure and stream flood;
- telemetry sink unavailable;
- bad canary release;
- partial mixed-version rollout;
- operator applies invalid config/policy;
- release-signing or directory-signing service unavailable.

### Measured evidence

For every scenario record:

- detection time;
- user-visible interruption;
- whether established sessions survived;
- failover/recovery time;
- data/state loss against RPO;
- capacity after failure;
- whether any security invariant weakened;
- automatic action taken;
- operator action required;
- time to full redundancy restoration.

### Pass condition

T-078 is DONE only when every failure required by the chosen service class meets its SLO/RTO/RPO and security invariants, and no tested failure causes direct-Internet fallback, authorization bypass, unbounded retry storm, unbounded resource growth, split-brain mutation or unsigned/unverified recovery.

---

## 16.3 Revised dependency order

The resilience lane augments, rather than replaces, the existing security/product lanes:

```text
SECURITY CORE
T-053 → T-054 → T-055 → T-056 → T-057 ───────────────┐
                                                       │
RUNTIME RESILIENCE                                     │
T-070 → T-071                                          │
T-070 → T-072                                          │
T-070 + T-039 + SecureAcces lane → T-073              │
T-054 + T-055 + T-057 + T-070 → T-074                 │
T-046 + stable protocols → T-075                       │
T-053 + T-055 + T-059 + T-060 → T-076                 │
T-070 + runtime tasks → T-077                          │
                                                       │
T-061 security final + T-070..T-077 ─────────────────→ T-078 resilience final
                                                       │
                                                       └→ T-069 Enterprise Qualified gate
```

Current execution priority remains:

1. T-053 security envelope.
2. T-054 explicit routing/admission.
3. T-055 E2E Client↔Origin security.
4. **In parallel:** T-070 formal resilience model can start immediately because it is architecture/evidence work.
5. T-071 and T-072 are the first runtime-resilience implementations after their required interfaces are stable.
6. T-073/T-074 establish true host/site continuity rather than relay-only HA.
7. T-075/T-076/T-077 harden release, trust and operations continuity.
8. T-061 proves the next-generation protected path; T-078 then proves failure survival.
9. T-069 cannot declare `Enterprise Qualified` before T-078 is DONE.

---

## 16.4 Resilience SLI dashboard

Track at minimum by service class and failure domain:

### Availability

- successful protected session establishment ratio;
- successful request ratio excluding explicit policy denies;
- availability by relay/provider/Origin/service;
- minutes in `Degraded` vs `Offline`;
- redundancy margin: healthy independent paths/sites remaining.

### Recovery

- failure detection p50/p95/p99;
- reconnect p50/p95/p99;
- failover interruption p50/p95/p99;
- time to full redundancy restoration;
- reconnect-attempt distribution during fleet recovery;
- circuit-breaker open/half-open counts.

### Runtime supervision

- startup/readiness duration;
- liveness failures;
- restart count and restart-budget exhaustion;
- crash-loop detections;
- graceful-drain completion ratio;
- hard-kill fallback count.

### State/DR

- backup age;
- verified restore age;
- measured RPO in drills;
- measured RTO in drills;
- failed backup/restore validations;
- leadership/fencing transitions.

### Capacity

- CPU/memory/network headroom after largest required failure domain is removed;
- queue saturation;
- rejected work due to admission limits;
- stream/session count by tenant/origin/path;
- disk headroom and telemetry spool usage.

### Release resilience

- canary rollback count;
- mixed-version compatibility failures;
- rollout pauses caused by health regression;
- time to rollback;
- percentage of fleet on each release ring/version.

Do not claim `99.9%`, `99.99%` or similar availability until measured production/pilot evidence and the service-class scope make the number meaningful.

---

## 16.5 Total-resilience convergence criterion

A deployment may be called `Resilience Qualified` only when:

- its service class S0/S1/S2/S3 is explicit;
- all required SPOFs are removed or visibly accepted for that class;
- capacity remains sufficient after loss of the largest required failure domain;
- reconnect/liveness logic is storm-resistant and bounded;
- local services use real readiness/liveness and bounded supervision;
- planned maintenance drains safely;
- state RPO/RTO and restore evidence meet the selected class;
- multi-Origin routing is qualified for S2/S3 where required;
- updates are staged and rollback-qualified;
- trust-root and clock failures have safe recovery semantics;
- telemetry loss does not impair the protected path;
- T-061 security qualification is DONE;
- T-078 chaos/disaster matrix passes for all failures required by the selected class;
- no tested resilience mechanism weakens I-001..I-044, P-I-001..P-I-008 or fail-closed behavior;
- `Enterprise Qualified` additionally requires the commercial/product gates and T-069.
