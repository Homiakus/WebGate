# 18. Transport Resilience v3 — Carrier-Independent Session Continuity

**Status:** ACTIVE EXECUTION EXTENSION  
**Accepted:** 2026-09-03  
**Research basis:** Turbo Tunnel; Snowflake; SpotProxy; CensorLess; QUIC/RFC 9000; Multipath QUIC; MASQUE; recent FOCI/USENIX/NDSS traffic-analysis and rendezvous work.

This section is part of the same living `MASTER_PLAN.md`, not a parallel roadmap. It extends T-053..T-062 and T-070..T-078. Existing task state remains authoritative unless explicitly amended below. Draft protocols and research mechanisms remain experimental until their owning qualification gate is DONE.

---

## 18.A Architectural correction

The current plan already contains the correct lower-level building blocks: Transport V2, H3/QUIC + H2/TLS providers, Adaptive Path Manager, signed relay discovery, anti-herd reconnect logic and release-binary adversarial qualification. The remaining structural gap is that protected application/session lifetime is still insufficiently separated from the lifetime of a concrete carrier connection.

Target architecture:

```text
ApplicationSession
      │ identity / authorization / browser ownership
      ▼
Client↔Origin E2E Secure Session                 T-055
      │
      ▼
ContinuitySession                               T-088/T-089
      │ logical streams / offsets / delivery ACK
      │ bounded replay window / reattachment
      │ session epoch / anti-replay / suspend-resume
      ▼
Adaptive Path Manager                           T-057/T-090/T-091
      │
      ├── qualified active path
      ├── bounded warm standby path
      └── candidate/probing paths
      │
      ▼
Carrier SPI                                     T-056
      ├── QUIC/H3
      ├── H2/TLS
      ├── optional MASQUE-compatible carrier
      └── qualified external sidecar
      │
      ▼
Relay / direct authenticated path / trusted peer
```

**Core rule:** a carrier socket, relay IP, network interface, NAT binding, QUIC connection or provider process is not the WebGate session. Losing one must not automatically destroy the protected logical session if another qualified path can be attached within policy and resource bounds.

The strongest common lesson from Turbo Tunnel, Snowflake, SpotProxy, CensorLess and QUIC migration is to keep persistent session state above transient carrier state.

---

## 18.B Mission amendment

The transport portion of the target mission becomes:

```text
trusted link
  ↓
WebGate-owned browser capsule
  ↓
destination-restricted loopback proxy
  ↓
Client↔Origin end-to-end secure session
  ↓
carrier-independent continuity layer
  ↓
adaptive path manager
  ├── QUIC/H3 path
  ├── H2/TLS path
  ├── optional qualified sidecar path
  ├── direct authenticated path when safely available
  ├── infrastructure relay path
  └── explicitly trusted peer relay path
  ↓
persistent outbound connectivity from private Origin
  ↓
WebGate data gateway
  ↓
SecureAcces authorization boundary
  ↓
registered private service
```

The loopback browser-facing connection should remain stable across remote path migration where technically possible. No migration may create a normal-OS direct route or system-browser fallback.

---

## 18.C New critical invariants

Append these invariants conceptually after I-044:

- **I-045 Session/carrier separation:** `ContinuitySession` identity and logical stream state are independent from a specific socket, relay address, transport provider, IP family, interface or QUIC connection.
- **I-046 No migration privilege escalation:** a new path must prove possession of current session/identity material and may not bypass revocation, Origin identity, authorization epoch, destination restriction or relay routing policy.
- **I-047 Replay-safe reattachment:** resume/reattach messages are nonce/epoch-bound and replay protected. Old path transcripts cannot attach to a newer session epoch.
- **I-048 Delivery continuity:** continuity-acknowledged bytes are never emitted twice to the protected application after migration. Unacknowledged data may be retransmitted only through bounded deduplication semantics owned above the carrier.
- **I-049 No TCP-over-TCP reliability stack:** continuity may keep minimal end-to-end delivery state required for migration but must not introduce a second competing congestion-control/retransmission loop over reliable TCP/H2 carriers.
- **I-050 Bounded suspension:** when no qualified path exists, a session may enter `Suspended/Degraded` only for bounded TTL and replay-buffer budgets. Expiry closes fail-closed.
- **I-051 Qualified-path semantics:** `Ready` for protected traffic means end-to-end usefulness, not only DNS/TCP/TLS/QUIC establishment. Qualification proves relay identity, intended Origin/gateway reachability and protected data-plane confirmation.
- **I-052 Failure-class awareness:** retry policy distinguishes network, protocol-family, DNS/control-plane, relay, authentication/policy, overload, clock/expiry and local-runtime failures. Security denials are never treated as ordinary transient network failures.
- **I-053 Relay identity ≠ relay address:** trust is rooted in cryptographic relay identity and signed policy; IP/domain/port/provider instance are rotatable endpoint attributes.
- **I-054 Carrier diversity over protocol monoculture:** QUIC may be preferred, but resilience claims that include QUIC/UDP filtering require at least one materially independent non-QUIC carrier family.
- **I-055 Evidence-bound blocking-resilience claims:** terms such as `blocking-resistant`, `DPI-resistant`, `censorship-resistant`, `unblockable` or equivalent require explicit measured evidence under T-095/T-096; encryption, padding, nesting or QUIC alone is insufficient evidence.

---

## 18.D New findings

- **F-056 — Carrier lifetime still owns too much session lifetime:** OPEN / Critical → T-088.
- **F-057 — Transparent reattachment semantics are undefined:** OPEN / Critical → T-089.
- **F-058 — Path readiness is not yet a multi-stage evidence model:** OPEN / High → T-090.
- **F-059 — Primary/fallback semantics are insufficient for heterogeneous path sets:** OPEN / High → T-091.
- **F-060 — QUIC availability cannot be assumed as a network class:** OPEN / High → T-092/T-096. Research documents environments where QUIC/UDP is selectively filtered; QUIC-only resilience is therefore a correlated failure mode.
- **F-061 — Relay churn is not yet a first-class lifecycle primitive:** OPEN / Medium/High → T-093.
- **F-062 — Bootstrap remains vulnerable to simultaneous directory/control-channel impairment:** OPEN / Medium → T-094.
- **F-063 — Traffic distinguishability is outside current release evidence:** OPEN / Medium/High → T-095.
- **F-064 — Current chaos coverage does not prove cross-carrier continuity:** OPEN / High → T-096.

---

## 18.E T-056 amendment — Carrier SPI, not session owner

`TransportProvider`/Transport V2 becomes conceptually the **Carrier SPI**. A provider owns one carrier and its mechanics. It does not own application-session identity, authorization, logical replay state or cross-carrier continuity.

Minimum provider capabilities:

```text
provider_id
transport_family
network_requirements
failure_domain
supports_connection_migration
supports_native_multipath
supports_bidirectional_streams
supports_datagrams
supports_0rtt
supports_path_validation
supports_local_sidecar
```

Provider failures are structured, not only `Offline/Error`.

Production baseline:

```text
QUIC/H3 carrier        preferred where qualified
H2/TLS carrier         materially independent fallback
sidecar carrier        optional compatibility path
MASQUE                 optional/experimental technique
```

A provider cannot be promoted to production `Ready` because a socket or handshake exists; T-090 owns end-to-end qualification.

---

## 18.F T-057 amendment — Adaptive Path Manager

T-057 remains the path-policy/scheduling owner but explicitly does not own continuity state.

Required path lifecycle:

```text
Discovered
  ↓
Probing
  ↓
CarrierEstablished
  ↓
RelayAuthenticated
  ↓
GatewayValidated
  ↓
Authorized/DataConfirmed
  ↓
Validated
  ├── Warm
  └── Active

Validated/Active
  ↓ degradation
Suspect
  ↓
Quarantined
  ↓
Cooldown
  ↓
Probing or Retired
```

Scheduler requirements:

- bounded candidate racing;
- hysteresis before switching for small score differences;
- immediate failover on hard failure threshold;
- diversity bonus for independent provider/ASN/region/transport family;
- per-failure-class cooldown;
- no repeated spraying of a known-broken protocol family;
- no duplication of non-idempotent application operations merely for path selection;
- support one-active + one-warm baseline and future true multipath;
- score is advisory; invariants and authorization gates remain mandatory.

---

## 18.G T-059 amendment — Signed Relay Directory

Relay records must separate stable identity from mutable endpoints:

```text
RelayRecord {
    relay_id
    relay_identity_key_id
    identity_epoch

    endpoint_generation
    endpoints[] {
        transport_family
        address/domain
        port
        alpn/capabilities
        provider_class
        region
        asn/failure_domain
        valid_from
        expires_at
    }

    policy_epoch
    record_expiry
    signature
}
```

Rules:

- endpoint rotation does not rotate identity by default;
- identity rotation uses an explicit signed key-transition path;
- an address found outside a signed record is a candidate, not trusted policy;
- a bounded previous/current endpoint-generation overlap may exist for migration;
- old generations are rejected after expiry/rollback window;
- directory mirrors distribute authoritative signed objects but are not themselves trust authorities.

---

## 18.H New execution tasks

### T-088 — Carrier-independent Continuity Session Core
**Status:** READY · **Priority:** P0 architecture/runtime · **Depends on:** stable T-055 and T-056 contracts · **Owns:** F-056 · **Protects:** I-003, I-038, I-045, I-050

Introduce a stable `ContinuitySession` boundary between E2E secure session and carriers so a browser/application session can survive loss of a carrier, relay endpoint, NAT binding, interface or protocol family while both continuity endpoints remain alive and another qualified path is available.

Preferred code boundary:

```text
crates/webgate-continuity
    session.rs
    stream.rs
    replay_window.rs
    resume.rs
    path_binding.rs
    budgets.rs
    telemetry.rs
```

Minimum session state:

```text
ContinuitySessionId      opaque; never bearer credential
SessionEpoch
SecurityContextId
PolicyEpoch
ActivePathGeneration

logical streams:
    StreamId
    send_offset
    peer_acked_offset / ack ranges as required
    recv_offset
    FIN/reset state

budgets:
    max_unacked_bytes
    max_streams
    max_suspended_time
    max_resume_attempts
```

FSM:

```text
Establishing
  ↓
Active
  ↓ path degradation
Migrating
  ├── success → Active
  └── no path → Suspended
                    ├── qualified path before TTL → Migrating → Active
                    └── TTL/budget/revocation → Closed

Any state + policy/device/session revocation → Closed
```

T-088 v1 does not imply transparent process-crash persistence; persistence across endpoint restart requires a separate threat model.

**Exit:** killing the active carrier while a qualified alternate exists does not require browser recreation and does not lose or duplicate continuity-acknowledged bytes.

### T-089 — Replay-safe Resume and Logical Stream Delivery Protocol
**Status:** TODO · **Priority:** P0 security/correctness · **Depends on:** T-055, T-088 · **Owns:** F-057 · **Protects:** I-046..I-050

Do not build a second TCP stack. QUIC/TCP owns per-path congestion control and ordinary packet loss. Continuity stores only the end-to-end state needed for cross-carrier deduplication and recovery of unacknowledged logical-stream data.

Required concepts:

```text
SessionEpoch
PathGeneration
StreamId
StreamOffset / RecordSequence
AckRanges or cumulative delivered offset
ResumeNonce
ResumeProof
SecurityContextId
PolicyEpoch
```

Resume proof binds to session ID, current security epoch, new-path transcript/nonce, Origin identity, policy context, path generation and expiry using reviewed T-055 keying/exporter mechanisms.

Security requirements include replay rejection, epoch/revocation checks, no long-lived bearer resume token, no relay-forged Client↔Origin resume and 0-RTT disabled by default unless replay safety is explicitly proven.

Delivery requirements include pre-emission deduplication, deterministic FIN/reset semantics, bounded out-of-order window, per-stream/session/global memory budgets and backpressure toward the local proxy/browser.

Negative tests: old proof replay, same proof double-use, wrong Origin/SecurityContext/PolicyEpoch, offset rewind/jump, duplicate post-migration record, FIN-then-data, reset-then-replay, buffer exhaustion, suspend TTL expiry.

### T-090 — End-to-End Path Qualification FSM and Failure Taxonomy
**Status:** TODO · **Priority:** P0 runtime · **Depends on:** T-054, T-056, T-088 · **Owns:** F-058 · **Protects:** I-005, I-051, I-052

Qualification levels:

```text
Q0 CandidateKnown
Q1 CarrierEstablished
Q2 RelayAuthenticated
Q3 IntendedGatewayReached
Q4 Session/PolicyBound
Q5 ProtectedDataChallengeConfirmed
```

Only Q5 may become an `Active` production path for an existing continuity session.

Minimum failure taxonomy:

```text
LocalNetworkDown
InterfaceChanged
DnsUnavailable
DnsAnswerUntrusted
UdpUnavailableOrBlackholed
QuicHandshakeFailed
TcpUnavailable
TlsHandshakeFailed
MtuOrPmtuFailure
RelayEndpointUnreachable
RelayOverloaded
RelayAuthRejected
OriginRouteUnknown
GatewayUnreachable
SessionResumeRejected
AuthorizationDenied
PolicyEpochMismatch
DirectoryExpired
ClockSkew
HighLoss
HighLatency
HighJitter
CaptivePortalSuspected
LocalProviderFailure
```

Each class defines retryability, retry scope, cooldown, security severity, human diagnostic and whether current session state is invalidated. Security/auth failures must never trigger blind relay/path fan-out.

### T-091 — Transport Happy Eyeballs, Warm Standby and Anti-Flap Scheduler
**Status:** TODO · **Priority:** P1/P0 for resilience profile · **Depends on:** T-057, T-089, T-090, T-071 · **Owns:** F-059 · **Protects:** I-013, I-014, I-043, I-050, R-I-003

Race only a small bounded set of materially different candidates with measurement-derived stagger. Keep at most a policy-bounded number of warm paths; they must be authenticated, recently Q5-qualified, low-rate while idle, independently budgeted and immediately revocable.

Anti-flap switch conditions:

- current path crosses a hard failure threshold; or
- candidate score exceeds active score by a configured margin for a stability window.

Initial engineering targets, subject to measured revision:

```text
warm-path migration p95 interruption: <= 750 ms
cold heterogeneous fallback p95:      <= 3 s
acknowledged-byte loss:               0
continuity duplicate delivery:        0
OS direct escape events:              0
unauthorized resume events:           0
```

### T-092 — QUIC Migration + Multipath QUIC Experimental Qualification
**Status:** EXPERIMENTAL/READY · **Priority:** P1 research/runtime · **Depends on:** T-056, T-090, T-091 · **Owns:** F-060 · **Protects:** I-042, I-054

Baseline: qualify ordinary QUIC connection migration/NAT rebinding, Wi-Fi address change, IPv4/IPv6 transition where supported, path validation and failure interaction with continuity path generation.

Track IETF Multipath QUIC and eventual RFC. Architecture must support MPQUIC without requiring it for baseline correctness. Experimental requirements include path-ID telemetry, validation before use, WebGate-owned scheduling, reordering/RTT-path testing and proof that MPQUIC is never the only fallback where QUIC itself can be filtered.

Promotion requires acceptable library/interoperability maturity and a proven H2/TLS or equivalent independent fallback.

### T-093 — Rotatable Relay Fleet and Seamless Endpoint Churn
**Status:** TODO · **Priority:** P1 resilience · **Depends on:** T-053, T-059, T-089, T-091 · **Owns:** F-061 · **Protects:** I-040, I-053

Make relay endpoint replacement routine:

- stable `RelayID`, rotatable endpoint generation;
- pre-publish/pre-qualify next endpoint;
- bounded signed old/new overlap;
- migrate active continuity sessions;
- deterministic retirement of stale endpoint generations;
- independent endpoint rotation vs identity/key rotation;
- graceful drain before relay termination;
- provider/region/ASN-aware fleet and scheduler.

Tests include active-session IP rotation, VM/process replacement, publish-new/revoke-old, stale mirror, rollback attack, address rotation without identity rotation and signed identity transition.

### T-094 — Emergency Signed Rendezvous Capsule
**Status:** EXPERIMENTAL · **Priority:** P2 resilience/research · **Depends on:** T-059 · **Owns:** F-062 · **Protects:** I-019, I-040

Define a very small signed, expiry-bounded bootstrap object:

```text
schema_version
capsule_id
issued_at
expires_at
minimum_client_version
directory_epoch
directory_digest
small bounded relay/bootstrap hints
trust-root/key id
signature
```

The capsule contains bootstrap metadata only, never arbitrary tunnel payload or long-lived credentials.

Optionally evaluate fountain/erasure coding for reconstruction from lossy/unordered permitted channels. Signature verification occurs only after reconstruction; fragments cannot authorize a relay; size/allocation is bounded; rollback is rejected.

### T-095 — Traffic Fingerprint and Distinguishability Qualification Lab
**Status:** TODO/RESEARCH · **Priority:** P1 for blocking-resilience claims, P2 otherwise · **Depends on:** T-056, T-091 · **Owns:** F-063 · **Protects:** I-055

Treat distinguishability as an empirical measurement problem. Capture only synthetic/metadata evidence needed for qualification:

```text
packet direction
packet/record size
relative timing
burst structure
connection setup phases
RTT estimates
stream concurrency
reconnect/migration events
```

Test nested TLS-handshake signatures, burst/RTT structure, cross-layer timing, connection reuse/multiplexing, reconnect/migration signatures, padding policy, H2/H3 differences and synthetic ordinary-web baselines.

Deliver a reproducible synthetic trace generator, feature extractor, simple baseline classifiers/statistical-distance metrics, release-over-release regression report and explicit FP/FN limitations. Never claim universal undetectability.

### T-096 — Network Interruption and Protocol-Selective Chaos Qualification
**Status:** TODO · **Priority:** P0 final transport-resilience gate · **Depends on:** T-088, T-089, T-090, T-091; T-092/T-093/T-094/T-095 when enabled · **Owns:** F-064 · **Protects:** I-003, I-045..I-055, R-I-016

Fault matrix with actual release binaries where practical:

```text
100% UDP blackhole while TCP/443 remains usable
selective QUIC handshake loss
TCP reset on active H2 path
relay IP blackhole
DNS timeout/NXDOMAIN/untrusted answer
directory mirror loss and signed stale/rollback/expiry
NAT rebinding
source IP/interface change
IPv4-only / IPv6-only / asymmetric reachability
loss 1/5/10/30%
reorder/duplication
high jitter / abrupt RTT increase
PMTU/fragmentation blackhole
relay kill/restart
relay endpoint-generation rotation
active path degraded while warm path healthy
all protected paths unavailable
path-score flapping stimulus
resume-proof replay
policy/device revocation during suspension
replay-buffer pressure/exhaustion
captive-portal-like interception in isolated lab
clock skew affecting directory/session expiry
```

Assertions for every applicable case:

```text
no OS direct fallback
no system-browser fallback
no authorization bypass
no cross-Origin route
no acknowledged-byte loss
no duplicate continuity delivery
bounded memory
bounded CPU/retry rate
bounded reconnection concurrency
explicit failure class
session migrates or closes deterministically
```

Correlated scenarios must include UDP blocked + Relay A down; DNS down + relay endpoint churn; Wi-Fi loss during active QUIC + H2 warm standby; same-provider relay outage + independent-provider relay; control plane down while valid signed directory remains; and all directory sources down after expiry.

**Exit:** prove at least one heterogeneous carrier-family switch and one relay-address replacement while the protected browser/session security boundary remains intact.

---

## 18.I T-071 amendment — reconnect controller

Add these rules:

- retry budgets are scoped by failure class and failure domain, not just relay address;
- `UdpUnavailableOrBlackholed` suppresses repeated QUIC spraying on the same interface for bounded cooldown while allowing H2/TLS candidates;
- `AuthorizationDenied`, identity mismatch and policy-epoch rejection are security outcomes and do not trigger relay/path fan-out;
- successful T-091 migration resets backoff only after a stability window;
- warm-standby probes consume explicit connection/battery/bandwidth budgets.

---

## 18.J T-061 and T-078 gate amendments

T-061 baseline dependencies gain:

```text
T-088 Continuity Session
T-089 Replay-safe resume/delivery
T-090 End-to-end path qualification
T-091 Heterogeneous path racing/warm standby
T-096 Protocol-selective chaos qualification
```

T-092/T-093/T-094/T-095 are required when their corresponding feature or blocking-resilience claim is enabled.

T-078 additionally requires evidence that a live session survives active carrier loss when an alternate qualified path exists; protocol-family outage does not produce retry storms; relay endpoint churn does not change trust identity; all-path loss yields bounded suspended/offline semantics; and network return is anti-herd/anti-flap.

---

## 18.K Revised transport dependency order

```text
T-053 Secure relay envelope
   ↓
T-054 Explicit routing/admission
   ↓
T-055 Client↔Origin E2E secure session
   │
   ├──────────────┐
   ▼              ▼
T-056 Carrier SPI  T-059 Signed relay directory
   │              │
   └──────┬───────┘
          ▼
T-088 Continuity Session Core
          ↓
T-089 Resume + logical-stream delivery
          │
          ├───────────────┐
          ▼               ▼
T-090 Path qualification  T-093 Relay churn
          ↓
T-057 Adaptive Path Manager (amended)
          ↓
T-091 Happy Eyeballs + warm standby + anti-flap
          │
          ├───────────────┬────────────────┐
          ▼               ▼                ▼
T-092 QUIC/MPQUIC   T-094 rendezvous   T-095 fingerprint lab
 experimental       experimental       claim-dependent
          └───────────────┬────────────────┘
                          ▼
                    T-096 chaos gate
                          ↓
                    T-061 release gate
                          ↓
                    T-078 resilience gate
```

T-071 anti-herd/liveness progresses in parallel and becomes an input to T-091/T-096.

---

## 18.L Permanent transport verification matrix amendment

```text
CONTINUITY
active carrier killed + warm path healthy
  => same ContinuitySession remains active

active carrier killed + cold fallback healthy
  => bounded migration; no browser recreation if replay window/TTL permits

all paths killed
  => Suspended/Offline, bounded memory, no direct escape

path returns before suspension TTL
  => replay-safe resume

path returns after TTL
  => old resume rejected; new session required

SECURITY
resume proof replay
  => rejected

old policy/security epoch
  => rejected

wrong Origin identity
  => rejected

relay knows SessionId but lacks E2E/session proof
  => cannot attach

DELIVERY
acked bytes then migrate
  => no duplicate upstream delivery

unacked bytes then migrate
  => retransmit/deduplicate correctly

reorder/duplicate carrier packets
  => no stream corruption

BUDGETS
remote path blackhole
  => replay buffer reaches bound; local backpressure; no OOM

1000 clients lose relay
  => jittered bounded recovery; no synchronized dial storm

DIVERSITY
UDP blackhole
  => QUIC family cooldown + H2/TLS candidate

relay/provider failure
  => independent failure-domain candidate preferred

path scores oscillate
  => hysteresis prevents flap
```

---

## 18.M Telemetry and observability contract

Add structured local metrics/events, excluding protected payloads and long-lived secrets:

```text
continuity_session_state
continuity_session_epoch
active_path_id/generation
path_state
path_qualification_level
path_failure_class
carrier_family
relay_id (safe opaque id)
endpoint_generation
migration_reason
migration_duration_ms
resume_attempt/result
unacked_bytes
replay_buffer_bytes
warm_path_count
path_score components (debug-sanitized)
retry_budget remaining
protocol-family cooldown
```

Aggregate SLIs:

- session establishment success;
- session survival after eligible path failure;
- warm migration p50/p95/p99;
- cold heterogeneous fallback p50/p95/p99;
- duplicate-delivery count = 0;
- acknowledged-byte-loss count = 0;
- unauthorized-resume count = 0;
- path flap rate;
- retries per outage/client;
- recovery herd peak concurrency;
- carrier-family distribution;
- failure causes by class/failure domain.

---

## 18.N Research anchors for Section 9

Evidence inputs, not automatic implementation mandates:

1. David Fifield, **Turbo Tunnel**, FOCI 2020 — persistent session/reliability state above transient carriers.
2. Bocovich et al., **Snowflake**, USENIX Security 2024 — ephemeral proxies and transparent upper-layer switching.
3. Kon et al., **SpotProxy**, USENIX Security 2024 — active proxy migration and controlled endpoint churn.
4. Kang et al., **CensorLess**, PoPETs 2026 — short-lived bridges and live migration.
5. RFC 9000 QUIC — baseline connection migration/address-change semantics.
6. IETF Multipath QUIC — multiple simultaneous paths with application-owned scheduling; experimental until implementation/interoperability risk is acceptable.
7. RFC 9297 HTTP Datagrams, RFC 9298 CONNECT-UDP, RFC 9484 CONNECT-IP / MASQUE — candidate carrier techniques; WebGate remains application-scoped rather than a generic full-device VPN.
8. Heitmann et al., **On Russia’s Early Introduction of QUIC SNI Censorship**, FOCI 2026 — evidence that QUIC/UDP can be a correlated failure domain.
9. Cawthon & Fifield, **Fountain codes in censorship circumvention rendezvous**, FOCI 2026 — bounded lossy-channel emergency bootstrap metadata.
10. Xue et al., **Fingerprinting Obfuscated Proxy Traffic with Encapsulated TLS Handshakes**, USENIX Security 2024 — encryption/random padding/nesting alone do not prove indistinguishability.
11. Ramesh et al., **CalcuLatency**, USENIX Security 2024 — cross-layer timing can expose proxy structure independently of payload secrecy.
12. Kamali & Barradas, **Huma**, NDSS 2026 — timing/burst/behavioral realism is a separate measurement domain, not a baseline implementation mandate.

---

## 18.O Convergence amendment

WebGate cannot claim transport-resilient next-generation convergence until:

- continuity session state is demonstrably independent of a concrete carrier;
- at least one live session survives a real carrier-family or relay-endpoint replacement without browser/system fallback;
- replay-safe reattachment and duplicate suppression are adversarially tested;
- replay buffers, suspended sessions, retries and candidate racing are bounded globally and per session;
- path `Ready` means Q5 protected data confirmation;
- QUIC outage falls back to a materially independent family in deployments claiming that resilience class;
- relay address rotation does not change trust identity and signed-directory rollback cannot abuse it;
- all-path loss degrades/closes deterministically with zero protected traffic escape;
- recovery is anti-herd and anti-flap;
- blocking-resilience claims, when made, are evidence-bound by T-095/T-096.

---

## 18.P Recommended execution order

Network architecture execution order:

```text
1. Keep T-053/T-054/T-055 as security prerequisites.
2. Freeze T-056 Carrier SPI before adding more transports.
3. Implement T-088 ContinuitySession skeleton with bounded state and deterministic test carriers.
4. Implement T-089 delivery/resume protocol + mutation/fuzz tests.
5. Implement T-090 qualification FSM/failure taxonomy.
6. Reconcile T-057 to consume Q-level paths rather than raw provider readiness.
7. Integrate real H2/TLS and QUIC/H3 carriers under T-056.
8. Implement T-091 bounded path racing/warm standby/hysteresis.
9. Run T-096 early with UDP blackhole, relay kill, DNS failure and NAT rebinding; fix architecture before feature expansion.
10. Add T-093 relay churn once continuity is real.
11. Keep T-092 MPQUIC and T-094 fountain rendezvous experimental until heterogeneous failover is stable.
12. Build T-095 traffic-analysis lab before making blocking-resistance claims.
13. Re-run T-061 and T-078 only on release binaries and exact final main SHA.
```

First vertical slice:

```text
Browser/proxy connection stays alive
        ↓
ContinuitySession active over Carrier A
        ↓
Carrier A is killed
        ↓
Carrier B becomes Q5
        ↓
same logical stream continues
        ↓
no acknowledged bytes lost
no duplicate bytes delivered
no direct OS escape
no browser recreation
```

Do not start with MPQUIC, traffic shaping, fountain bootstrap, peer relays or large relay fleets. If clean A→B carrier replacement does not work, higher-level resilience features only hide the missing abstraction.

---

## 18.Q Status truth amendment

Until T-088/T-089/T-090/T-091/T-096 are DONE, documentation should say only that WebGate has a fail-closed multi-provider transport architecture with planned heterogeneous path resilience, while carrier-independent continuity and release-binary seamless migration remain under qualification.

Do not claim:

```text
seamless network switching
connection survives any network block
DPI-resistant
unblockable
censorship-proof
zero-interruption failover
```

unless a narrower statement is supported by recorded measurement and its exact owning gate is DONE.
