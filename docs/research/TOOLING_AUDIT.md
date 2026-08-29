# WebGate Tooling Audit

Research snapshot: **2026-08-29**

This document evaluates the best current building blocks for a desktop client that opens a private web application inside an embedded browser and sends **only that browser's traffic** through a resilient protected transport.

## 1. Evaluation criteria

Each candidate is evaluated against the actual WebGate constraints, not against generic VPN-client requirements.

Priority criteria:

1. **Application-scoped routing** — must avoid changing the OS default route.
2. **Fail-closed behavior** — protected web traffic must never silently bypass the secure path.
3. **Windows production quality** — Windows is the first target.
4. **Low operational complexity** — a few trusted users should not require enterprise orchestration.
5. **DPI/blocking resilience** — multiple independent transports should be possible.
6. **Embeddability** — transport must be controllable from the client rather than manually configured by the user.
7. **Security boundaries** — navigation, downloads, local IPC, updates and config must be policy-controlled.
8. **License suitability** — bundling must not accidentally force an unwanted product licensing model.
9. **Maintainability** — active upstreams and well-defined APIs matter more than novelty.
10. **Compatibility with SecureAcces** — WebGate must reuse server-side authz rather than duplicate it.

Scoring is architectural guidance, not a benchmark.

---

# 2. Embedded browser / desktop shell

## 2.1 Tauri 2 + Wry + WebView2 — **RECOMMENDED**

**Score: 9.5/10**

Why it fits:

- Tauri 2 is Rust-native on the backend and uses the operating-system WebView stack.
- On Windows, Wry uses Microsoft WebView2.
- Current Tauri/Wry APIs expose a **per-WebView proxy URL**.
- The proxy can be `http://` or `socks5://`.
- Navigation, new-window, download and resource events can be intercepted.
- WebView data directory/incognito behavior can be controlled.
- Tauri provides mature packaging, updater and application lifecycle infrastructure without bundling a full Chromium build.
- MIT/Apache-2.0 licensing is favorable.

Most important capability for WebGate:

```rust
WebviewWindowBuilder::new(...)
    .proxy_url(local_proxy_url)
    .on_navigation(...)
    .on_new_window(...)
    .on_download(...)
```

This is the architectural reason Tauri/Wry currently wins: WebGate can scope the transport **at the browser object itself**, so no system TUN is necessary in the normal operating mode.

### Recommended use

Use **Tauri 2** as the application framework and drop to Wry-specific APIs only where Tauri does not expose a required low-level behavior.

Avoid building the whole product directly on raw Wry unless Tauri creates a hard restriction. Tauri already solves packaging, updates, deep links, lifecycle and permission capability management.

### Production hardening

- disable devtools in release builds;
- disable browser extensions;
- explicit navigation allowlist;
- explicit download policy;
- block `file:`, arbitrary custom schemes and untrusted local content;
- isolate WebView persistent data into a dedicated application directory;
- external links open in the system browser;
- never give web content generic Tauri command access;
- keep the command capability allowlist minimal.

Sources:

- https://github.com/tauri-apps/tauri
- https://github.com/tauri-apps/wry
- https://docs.rs/tauri/latest/tauri/webview/struct.WebviewWindowBuilder.html
- https://docs.rs/wry/latest/wry/struct.WebViewBuilder.html

---

## 2.2 Raw Wry + Tao — viable minimal alternative

**Score: 8.7/10**

Advantages:

- minimal framework surface;
- direct WebView control;
- same per-WebView HTTP CONNECT/SOCKS5 proxy capability;
- easy to reason about for a very small application.

Disadvantages:

- WebGate would have to build its own updater, installer integration, deep-link handling, single-instance coordination, permissions/capability model and more platform glue.

Decision: keep as a fallback architecture, not first choice.

---

## 2.3 Chromium Embedded Framework (CEF)

**Score: 7.0/10**

CEF is extremely mature and exposes deep Chromium networking/request controls. It is appropriate when the application needs Chromium itself as a product dependency.

For WebGate it is currently excessive:

- much larger binary/runtime footprint;
- more update responsibility;
- more browser-process attack surface owned by WebGate;
- more packaging complexity;
- C++ integration overhead for a Rust-first application.

CEF becomes interesting only if WebView2 policy limitations become a blocker.

Source: https://chromiumembedded.github.io/cef/general_usage.html

---

## 2.4 Servo

**Score: 5.5/10 for production WebGate today**

Servo is strategically interesting because it is Rust-native, but WebGate's first requirement is production compatibility with an existing documentation website and the broad web platform. It is not the conservative choice for the first release.

Decision: track, do not depend on it for v1.

---

# 3. Application-local transport engines

The transport engine must expose a local proxy/dialer to the WebView. A normal full-device VPN is a secondary mode, not the default.

## 3.1 Outline SDK + AmneziaWG v3 — **PRIMARY RESEARCH PATH**

**Score: 9.4/10 for architectural fit; maturity risk must be tested**

This combination is unusually well aligned with WebGate.

### Outline SDK

Outline SDK explicitly targets embedding interference-resistant networking into applications. It provides composable `StreamDialer` / `PacketDialer` abstractions and supports a **side-service integration model** for desktop applications.

Important capabilities:

- local forward proxy patterns;
- custom transport composition;
- resilient DNS;
- DNS override;
- TCP/TLS fragmentation mechanisms;
- SOCKS5 and Shadowsocks components;
- HTTP/2 and HTTP/3 CONNECT evolution;
- Windows/macOS/Linux support;
- Apache-2.0 licensing.

Source: https://github.com/OutlineFoundation/outline-sdk

### AmneziaWG v3

The current AmneziaWG Go module is now `github.com/amnezia-vpn/amneziawg-go/v3` and depends directly on Outline SDK packages. The upstream has actively evolved during 2026 and includes AWG3 work intended to reduce statistical/DPI recognizability.

Important for WebGate:

- WireGuard-derived authenticated encrypted tunnel semantics;
- current Amnezia-specific anti-DPI evolution;
- MIT license for `amneziawg-go`;
- Windows support through Amnezia/Wintun ecosystem;
- Outline integration means we can investigate a **dialer/proxy mode** instead of forcing a system-wide interface.

Sources:

- https://github.com/amnezia-vpn/amneziawg-go
- https://docs.amnezia.org/documentation/amnezia-wg/
- https://github.com/amnezia-vpn/amneziawg-windows

### Why a Go side service is acceptable in a Rust application

The transport layer is a protocol subsystem, not UI/business logic. Running it as a supervised child process gives valuable isolation:

```text
webgate.exe (Rust/Tauri)
       │
       │ authenticated local IPC
       ▼
webgate-transport.exe (Go)
       │
       ├── restricted local SOCKS/HTTP listener
       └── Outline/AWG transports
```

Benefits:

- upstream Go networking code can be used without unsafe Rust FFI;
- a crash in the transport engine does not corrupt the UI process;
- transport can be restarted independently;
- transport implementations can evolve without rewriting browser code;
- the Rust process can enforce policy around the transport process.

### Risk

AWG3 is very new. The upstream issue tracker still shows 2026 transport/compatibility defects. Therefore it must be behind a `TransportProvider` interface and accompanied by an independent fallback implementation.

Decision: **prototype and benchmark as the preferred primary path, but do not make WebGate single-transport.**

---

## 3.2 Xray-core — **RECOMMENDED INDEPENDENT FALLBACK**

**Score: 9.1/10**

Strengths:

- mature local SOCKS/HTTP proxy model;
- VLESS/REALITY/XHTTP-class resilient transport options;
- very active development;
- independent implementation/protocol family from AWG;
- straightforward sidecar process model;
- MPL-2.0 licensing is considerably easier to isolate in a multi-process product than a GPL engine.

Source:

- https://github.com/XTLS/Xray-core

Why it should remain independent instead of being hidden behind the same network implementation as AWG:

A backup path is most useful when it fails differently.

```text
Primary failure domain:
Outline/AWG/UDP/QUIC-like path

Fallback failure domain:
Xray/VLESS/REALITY/TCP-443/XHTTP-like path
```

This provides implementation, protocol and transport diversity.

---

## 3.3 sing-box — technically excellent, licensing decision required

**Technical score: 9.6/10**

sing-box is arguably the strongest universal proxy engine in this comparison:

- broad inbound/outbound support;
- routing engine;
- WireGuard endpoint support;
- VLESS and other modern protocols;
- mature local proxy modes;
- fast upstream development.

However, upstream sing-box is GPLv3. WebGate must not make it the default bundled engine until the project explicitly chooses how it will distribute and license the combined product.

This is not a statement that separate-process aggregation necessarily changes WebGate's license; it is a warning that the distribution model deserves a deliberate legal review rather than an accidental dependency decision.

Source: https://github.com/SagerNet/sing-box

Decision: keep as:

- benchmark/reference engine;
- optional user-supplied engine;
- potential default only after licensing policy is explicit.

---

## 3.4 GotaTun / BoringTun

### GotaTun

**Score: 7.0/10**

Mullvad's GotaTun is a modern Rust userspace WireGuard implementation and a fork of BoringTun. It supports Windows as a library and has independent audits.

Source: https://github.com/mullvad/gotatun

### BoringTun

BoringTun remains important historically and is deployed at significant scale, but its own repository warns about restructuring and recommends not relying on the master branch directly.

Source: https://github.com/cloudflare/boringtun

### Why they are not the WebGate v1 transport core

They implement WireGuard cryptography/protocol, but an app-local browser tunnel still needs:

- a userspace IP/network stack or custom stream abstraction;
- DNS behavior;
- proxy exposure;
- anti-DPI transport behavior;
- failover policy;
- endpoint health management.

Using them would move too much transport engineering into WebGate itself.

Decision: GotaTun is a useful future Rust-native transport provider, especially for environments where plain WireGuard is sufficient.

---

## 3.5 Official AmneziaWG Windows tunnel DLL / Wintun

**Score: 8/10 for full VPN mode; 5/10 for default WebGate mode**

The official Windows embeddable tunnel library is production-relevant and MIT-licensed. However its normal model creates a Windows tunnel interface and therefore changes OS networking/routing.

That conflicts with WebGate's central requirement that unrelated traffic remain unaffected.

Use cases:

- optional full-device/admin mode;
- emergency compatibility mode;
- not the default protected-browser data path.

Source: https://github.com/amnezia-vpn/amneziawg-windows

---

# 4. Recommended transport abstraction

WebGate must never couple browser logic to a concrete VPN protocol.

Conceptual Rust interface:

```rust
#[async_trait]
pub trait TransportProvider: Send + Sync {
    async fn start(&self, policy: TransportPolicy) -> Result<LocalProxy>;
    async fn health(&self) -> TransportHealth;
    async fn reconnect(&self) -> Result<()>;
    async fn stop(&self) -> Result<()>;
}
```

Candidate providers:

```text
TransportProvider
├── OutlineAwgProvider      primary candidate
├── XrayProvider            independent fallback
├── SingBoxProvider         optional/experimental
└── GotaTunProvider         future pure-Rust WG
```

The browser only receives:

```text
socks5://127.0.0.1:<ephemeral-port>
```

It does not know or care which remote transport is active.

---

# 5. Local proxy design

The local proxy is a security boundary, not merely a convenience.

## Requirements

- bind only to loopback;
- use a random ephemeral port;
- child process lifecycle tied to WebGate;
- explicit destination host/port allowlist;
- no generic Internet proxying;
- deny unknown DNS names;
- prefer remote/transport-side DNS for protected origins;
- no direct fallback;
- connection timeout and bounded queues;
- no secrets in command-line arguments;
- control IPC separate from data proxy socket;
- transport process must reject configuration not authenticated by the parent.

The destination allowlist is particularly valuable. Even if another local process discovers the proxy port, it cannot turn WebGate into a general-purpose tunnel.

---

# 6. Configuration and cryptography tools

## Serialization

Recommended:

- `serde`
- `serde_json` for human-readable diagnostics/export
- optionally CBOR (`ciborium`) for canonical compact signed payloads

Do not make YAML the signed on-wire format. YAML's parsing surface and canonicalization ambiguity are unnecessary for security-sensitive configuration.

## Signature

Recommended:

- Ed25519 signed policy/enrollment payloads;
- `ed25519-dalek` in Rust;
- equivalent standard Go implementation server-side.

Model:

```text
bundle.payload
bundle.signature
key_id
schema_version
not_before
expires_at
```

The client binary contains a small set of trusted root policy public keys. Root-key rotation must be explicit and cross-signed.

## Secret memory handling

Recommended Rust crates/patterns:

- `zeroize`;
- `secrecy` or an equivalent small wrapper;
- avoid `Debug` on secret types;
- redact logs structurally, not by regex after formatting.

---

# 7. OS secret storage

## Windows v1

Prefer Windows-native protection:

- DPAPI (`CryptProtectData` / `CryptUnprotectData`);
- user scope for per-user application credentials;
- optional additional entropy tied to WebGate application metadata.

Microsoft documents that DPAPI-protected data is normally decryptable only by the same user's credentials on the same computer.

Source: https://learn.microsoft.com/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata

Rust integration options:

1. call DPAPI through the `windows` crate directly — preferred for a small security-critical surface;
2. use a reviewed wrapper;
3. use `keyring` as a future cross-platform abstraction for non-Windows targets.

Do not store:

- SecureAcces session tokens;
- transport private keys;
- enrollment refresh secrets;

as plaintext JSON/TOML files.

---

# 8. Device key

Each WebGate installation should generate its own asymmetric keypair locally.

The configuration file should contain only bootstrap material. The long-lived private key is created after installation and stored in OS-protected storage.

Recommended initial algorithm:

- Ed25519 for application/device challenge signatures.

Potential later Windows hardening:

- TPM-backed key through CNG/NCrypt for non-exportability.

The system must support revoking a **device** independently from revoking the whole SecureAcces account.

---

# 9. Deep links

Recommended URI ownership:

```text
webgate://open/<opaque-resource-id>
```

Public Telegram links should preferably be ordinary HTTPS links that redirect/offer to open WebGate rather than containing a credential.

Example:

```text
https://go.example.net/d/8H37K
               ↓
        webgate://open/d/8H37K
```

Rules:

- resource identifiers are opaque;
- no session token in URL;
- no VPN private key in URL;
- deep link parser uses strict URL parsing (`url` crate);
- reject unexpected scheme, authority, Unicode ambiguity and extra parameters;
- one application instance receives subsequent deep links.

---

# 10. Updates and supply-chain security

Tauri's updater infrastructure is preferred for the application shell, but WebGate should define stronger product rules around it:

- signed update manifest;
- signed binary/packages;
- HTTPS is transport protection, not the root of trust;
- minimum allowed version / anti-rollback policy for emergency security releases;
- sidecar hashes pinned in the signed manifest;
- sidecar executable signature/hash verified before launch;
- SBOM generated per release;
- dependency-license policy checked in CI.

Recommended CI tools:

- `cargo audit`;
- `cargo deny`;
- `cargo nextest`;
- `cargo fuzz`;
- `proptest`;
- `loom` for concurrency state machines where appropriate;
- `clippy` with warnings denied for security-sensitive crates;
- Go `govulncheck`, race detector, fuzzing and staticcheck for transport/control components.

---

# 11. Observability

Recommended Rust stack:

- `tracing` + structured spans;
- bounded rotating local log;
- optional OpenTelemetry export for admin diagnostics.

Never log:

- session tokens;
- enrollment secrets;
- device private keys;
- full signed config payload when it contains bearer bootstrap fields;
- full sensitive document URLs if their path/query leaks business data.

Useful health dimensions:

```text
browser
local-proxy
transport-provider
endpoint-A
endpoint-B
origin
SecureAcces session
policy version
update status
```

---

# 12. Recommended v1 stack

## Client

```text
Rust
Tauri 2
Wry/WebView2
Tokio
Serde
url
tracing
Ed25519
Windows DPAPI
```

## Transport

```text
webgate-transport (Go side service)
    ├── Outline SDK
    └── AmneziaWG v3 candidate

xray-core sidecar/provider
    └── independent TCP/443-style fallback
```

## Server/control plane

```text
Go
SecureAcces
Caddy or equivalent hardened reverse proxy
WebGate device/enrollment API
2 independent VPS/relay endpoints
private origin with outbound-only connectivity
```

---

# 13. Decision matrix

| Area | Candidate | Fit | Decision |
|---|---|---:|---|
| Desktop | Tauri 2 + Wry/WebView2 | 9.5 | **Adopt** |
| Desktop | Raw Wry/Tao | 8.7 | fallback |
| Desktop | CEF | 7.0 | only if WebView2 blocks requirements |
| Desktop | Servo | 5.5 | track |
| Primary transport | Outline SDK + AWG v3 | 9.4 | **prototype/adopt if tests pass** |
| Fallback | Xray-core | 9.1 | **adopt as independent provider** |
| Universal proxy | sing-box | 9.6 technical | hold pending license/distribution decision |
| Pure Rust WG | GotaTun | 7.0 | future provider |
| Full VPN | AmneziaWG Windows/Wintun | 8.0 full VPN | optional compatibility mode |
| Windows secrets | DPAPI | 9.5 | **adopt** |
| Cross-platform secrets | keyring | 8.5 | abstraction/future |
| Config signature | Ed25519 | 9.5 | **adopt** |

---

# 14. Key architectural conclusion

The best solution is **not** to write a new VPN stack in Rust.

The highest-leverage design is:

```text
Tauri/Wry WebView
      ↓ per-WebView proxy
restricted loopback proxy
      ↓
pluggable transport provider
      ├── Outline/AWG
      └── Xray fallback
      ↓
redundant relay layer
      ↓
private origin
```

Rust owns UI, policy, lifecycle, update trust, config trust and supervision. Specialized transport engines own network protocol implementation. SecureAcces owns server-side identity, session and authorization semantics.

This keeps every component inside the boundary where it is strongest.
