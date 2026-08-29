# WebGate

WebGate is a secure, application-scoped access client for private web resources.

The core UX is intentionally simple:

```text
Telegram / trusted link
        ↓
      WebGate
        ↓
Servo embedded browser
        ↓
app-local secure transport
        ↓
redundant public relay/VPS layer
        ↓
private origin server
```

The operating-system network route must remain untouched. Only traffic created by the embedded WebGate browser is allowed into the protected transport.

## Project goals

1. One-click access to a private documentation site for a small set of trusted users.
2. No requirement for a static public IP on the origin server.
3. No system-wide VPN and no accidental routing of unrelated applications.
4. Fail-closed behavior: if the secure transport is unavailable, the browser must not fall back to direct Internet access.
5. Multiple independent paths/transports for resilience against VPS, provider, UDP, DNS, routing and DPI failures.
6. Per-user/per-device configuration and revocation.
7. Signed configuration and signed application updates.
8. Compatibility with [`Homiakus/SecureAcces`](https://github.com/Homiakus/SecureAcces) as the server-side authentication/authorization control plane.
9. Windows-first implementation with an architecture that can later support Linux/macOS.
10. A Rust-first protected-browser path built around Servo.

## Current architecture decision

Research snapshot: **2026-08-29**.

The canonical architecture is:

- **Primary browser engine:** **Servo embedded directly as a Rust library**.
- **Browser shell:** thin native Rust shell around Servo's `Servo`/`WebView` embedding APIs and a WebGate-owned rendering/event-loop integration.
- **Compatibility fallback:** Microsoft WebView2 may exist behind an optional adapter for explicit policy-controlled compatibility cases, but it is not the default engine.
- **Browser isolation:** protected network access is forced through a WebGate-owned app-local HTTP/HTTPS proxy; no system TUN is required for the default mode.
- **Local proxy:** random loopback port, destination allowlist, fail-closed; it is not a general-purpose proxy.
- **Primary resilient transport candidate:** a small Go `webgate-transport` side service built around Outline SDK and AmneziaWG v3 integration.
- **Independent fallback:** Xray-core with VLESS/REALITY/XHTTP-class transport exposed only through a local SOCKS/HTTP proxy.
- **Alternative universal transport engine:** sing-box is technically strong, but its GPLv3 distribution implications require an explicit product/licensing decision before it becomes the default bundled engine.
- **Secrets:** OS-native protected storage (Windows DPAPI first; keyring abstraction for future cross-platform support).
- **Config:** signed enrollment bundle; private keys are generated on the device and are never distributed in plaintext configuration files.
- **Authorization:** SecureAcces remains server-side. WebGate is a secure browser/network client, not a replacement authorization database.

The authoritative browser decision is recorded in [`docs/architecture/ADR-0001-BROWSER-ENGINE.md`](docs/architecture/ADR-0001-BROWSER-ENGINE.md).

## Why Servo

Servo aligns with WebGate's product model better than wrapping a general-purpose system browser:

- browser engine and application control plane remain Rust-first;
- Servo is designed for embedding and exposes `Servo`, `WebView`, delegate and rendering-context APIs;
- Servo supports Windows and has a cross-platform direction;
- HTTP and HTTPS proxy support can be used as part of WebGate's app-local networking boundary;
- its modular design gives WebGate a path toward a purpose-built protected browser rather than a Chromium distribution.

Servo does not yet match Chromium for arbitrary-site compatibility. WebGate therefore treats browser compatibility as a tested contract for the documentation site. Production releases must pass a site capability suite, visual regressions, network-escape tests and performance gates against the pinned Servo release/LTS line.

## Why app-local transport instead of a normal VPN

A traditional VPN changes routing at the OS level. WebGate instead forces its embedded browser through a protected application-local network path:

```text
Chrome ───────────────→ normal Internet
Telegram ─────────────→ normal Internet
Windows Update ───────→ normal Internet

WebGate / Servo
      ↓
127.0.0.1:<ephemeral>
      ↓
WebGate restricted proxy
      ↓
secure transport
      ↓
private infrastructure
```

This sharply reduces accidental blast radius, avoids conflicts with other VPN clients, simplifies the kill switch, and makes the user experience essentially "open link → document".

## Browser safety invariant

The protected browser must never gain direct access to the protected origin.

```text
Servo
  |
  v
WebGate local proxy
  |
  +-- transport healthy --> private origin
  |
  +-- transport unhealthy --> DENY
```

A Servo crash, unsupported feature, DNS error, proxy failure or transport failure must not trigger a direct-network fallback or silent engine switch.

## SecureAcces role

SecureAcces already provides the correct server-side concepts for WebGate:

- global Account identity;
- tenant-local User;
- Workspace and Membership;
- persistent permission bits;
- short-lived sessions;
- enrollment and login challenges;
- provider-agnostic identity verification;
- HTTP authorization middleware;
- Telegram adapters;
- fail-closed authorization and revocation.

WebGate will integrate with those contracts rather than duplicating them.

A key compatibility finding is that SecureAcces `Session.DeviceID` is currently an audit/session attribute rather than a cryptographic device credential. WebGate can be fully compatible today by using normal SecureAcces sessions and storing their token in OS-protected storage. A later hardening phase should introduce a first-class device public-key binding/provider so possession of the device key can be proven during session issuance/refresh.

## Repository map

```text
WebGate/
├── README.md
├── MASTER_PLAN.md
└── docs/
    ├── architecture/
    │   ├── ADR-0001-BROWSER-ENGINE.md
    │   └── TARGET_ARCHITECTURE.md
    ├── integration/
    │   └── SECUREACCESS.md
    └── research/
        ├── BROWSER_ENGINE_AUDIT.md
        └── TOOLING_AUDIT.md
```

The repository starts documentation-first. Code should only be committed after the transport, trust-boundary, browser-engine and configuration contracts are explicit enough to test.

## Non-negotiable security properties

- Servo is the default protected browser engine;
- fail closed on transport failure;
- no automatic direct-connect fallback from the protected browser;
- no silent browser-engine fallback;
- no secret VPN/private key inside a shareable static config;
- one device key per installation;
- all enrollment bundles and remote policies are signed;
- remote policy cannot weaken local hard security invariants;
- protected browser has an explicit origin allowlist;
- external navigation goes to the system browser only by explicit policy, never through the protected transport;
- debug/devtools are disabled or tightly gated in production;
- local transport IPC is authenticated/restricted;
- local proxy accepts only WebGate-approved target origins;
- SecureAcces authorizes every protected resource request server-side;
- links themselves are identifiers, not bearer credentials;
- secrets/tokens are redacted from logs and crash reports;
- updates are cryptographically signed and rollback-aware.

## Research sources

Primary sources and comparative decisions are tracked in:

- [`docs/research/TOOLING_AUDIT.md`](docs/research/TOOLING_AUDIT.md)
- [`docs/research/BROWSER_ENGINE_AUDIT.md`](docs/research/BROWSER_ENGINE_AUDIT.md)
- [`docs/architecture/ADR-0001-BROWSER-ENGINE.md`](docs/architecture/ADR-0001-BROWSER-ENGINE.md)

## Status

**Phase 0 — architecture/tooling research in progress. Servo is fixed as the primary browser-engine baseline.**
