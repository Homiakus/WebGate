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

The operating-system default route must remain untouched in normal mode. Only traffic created by the embedded WebGate browser is allowed into the protected transport.

## Developer project manager

WebGate includes a cross-platform project manager for environment checks, controlled installation of missing developer tools, compilation and CI-parity verification.

Windows:

```powershell
.\scripts\webgate.ps1
```

Linux/macOS:

```sh
./scripts/webgate.sh
```

Running either launcher without arguments opens the interactive menu. Scriptable examples:

```sh
python3 scripts/project_manager.py doctor
python3 scripts/project_manager.py install --dry-run
python3 scripts/project_manager.py install --yes
python3 scripts/project_manager.py verify
python3 scripts/project_manager.py build
python3 scripts/project_manager.py build --release
python3 scripts/project_manager.py servo
python3 scripts/project_manager.py android
```

The installer is deliberately allowlisted: it does not accept arbitrary package names or shell commands, and it never handles WebGate runtime credentials. See [`docs/development/PROJECT_MANAGER.md`](docs/development/PROJECT_MANAGER.md) for the full command/security contract.

## Project goals

1. One-click access to a private documentation site for a small set of trusted users.
2. No requirement for a static public IP on the origin server.
3. No system-wide VPN in normal mode and no accidental routing of unrelated applications.
4. Fail-closed behavior: if the secure transport is unavailable, the browser must not fall back to direct Internet access.
5. Multiple independent paths/transports for resilience against VPS, provider, UDP, DNS, routing and DPI failures.
6. Per-user/per-device configuration and revocation.
7. Signed configuration and signed application updates.
8. Compatibility with [`Homiakus/SecureAcces`](https://github.com/Homiakus/SecureAcces) as the server-side authentication/authorization control plane.
9. Shared security core across Windows, Android, Linux and macOS.
10. A Rust-first protected-browser path built around Servo.
11. Android as a first-class Tier-1 target rather than a late desktop port.

## Current architecture decision

Research snapshot: **2026-08-29**.

The canonical architecture is:

- **Primary browser engine:** **Servo embedded as the canonical browser engine**.
- **Shared core:** Rust for policy, browser/transport orchestration, deep links, signed configuration, device protocol, health state and SecureAcces client integration.
- **Platform shells:** narrow Windows / Android / Linux / macOS adapters.
- **Compatibility browser:** a platform browser adapter may exist only as explicit policy-controlled fallback; there is no silent engine switch.
- **Browser isolation:** protected network access is forced through a WebGate-owned app-local HTTP/HTTPS proxy; no system TUN is required for normal mode.
- **Local proxy:** random loopback port, destination allowlist, always fail-closed; it is not a general-purpose proxy.
- **Primary resilient transport candidate:** Outline SDK / MobileProxy plus AmneziaWG-class transport where qualified.
- **Independent fallback:** Xray-class transport through the same browser-facing local proxy contract.
- **Transport runtime:** protocol/provider is independent from execution model (`Sidecar`, `InProcess`, `PlatformVpnService`).
- **Android normal mode:** Servo AAR → local HTTP(S) proxy → Outline MobileProxy/generated mobile library. The rest of the phone is untouched.
- **Android compatibility VPN:** `VpnService` restricted to the WebGate package only, used only when TUN/IP semantics are required.
- **Device identity:** hardware-backed ES256/P-256 preferred; policy/update signing remains separate and may use Ed25519.
- **Secrets:** OS-native protected storage and hardware key providers where available.
- **Config:** signed enrollment bundle; private device keys are generated locally and never distributed in plaintext configuration files.
- **Authorization:** SecureAcces remains server-side. WebGate is a secure browser/network client, not a replacement authorization database.

Authoritative decisions:

- [`ADR-0001-BROWSER-ENGINE.md`](docs/architecture/ADR-0001-BROWSER-ENGINE.md) — Servo primary browser.
- [`ADR-0002-CROSS-PLATFORM-RUNTIME.md`](docs/architecture/ADR-0002-CROSS-PLATFORM-RUNTIME.md) — cross-platform/runtime model.
- [`ADR-0003-SERVO-PROCESS-ISOLATION.md`](docs/architecture/ADR-0003-SERVO-PROCESS-ISOLATION.md) — Servo compromise-containment boundary.

## Platform tiers

```text
Tier 1  Windows x86_64
Tier 1  Android arm64 after acceptance gate
Tier 2  Linux x86_64/aarch64
Tier 2  macOS arm64/x86_64
Tier 3  OpenHarmony research
Deferred iOS
```

Servo upstream currently supports Windows, macOS, Linux, Android and OpenHarmony. WebGate nevertheless qualifies a pinned Servo LTS release itself: upstream platform availability does not automatically mean WebGate production support.

## Android architecture

Android deliberately follows the same security model without pretending to be desktop:

```text
Telegram
   ↓
verified HTTPS App Link
   ↓
WebGate Android shell
   ↓
shared Rust security core
   ↓
Servo AAR
   ↓
127.0.0.1:<ephemeral HTTP(S) proxy>
   ↓
Outline MobileProxy / transport provider
   ↓
Relay A/B
   ↓
private origin + SecureAcces
```

The Android shell should remain intentionally thin and own only OS-native concerns such as Activity lifecycle, verified App Links, Keystore, permission prompts, notifications/foreground services and optional `VpnService` fallback.

Normal Android mode does **not** require VPN permission and does not alter the networking of Chrome, Telegram or other applications.

## Why Servo

Servo aligns with WebGate's product model better than wrapping a general-purpose system browser:

- Rust-first engine and embedding API;
- `Servo`, `WebView`, delegate and rendering-context APIs;
- Windows/Linux/macOS/Android direction from the same engine family;
- HTTP and HTTPS proxy support fits WebGate's app-local networking boundary;
- an LTS line exists for embedders;
- modular design enables a purpose-built protected browser.

Servo is still pre-1.0 and does not yet match Chromium for arbitrary-site compatibility. WebGate therefore treats browser compatibility as a tested contract for the documentation site. Production releases must pass a site capability suite, visual regressions, network-escape tests and performance gates against a pinned Servo LTS patch release.

## Why app-local transport instead of a normal VPN

A traditional VPN changes routing at the OS level. WebGate instead forces its browser through a protected application-local path:

```text
Chrome ───────────────→ normal Internet
Telegram ─────────────→ normal Internet
system applications ─→ normal Internet

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

This reduces blast radius, avoids many conflicts with other VPN clients, simplifies the kill switch, and gives the user an "open link → document" workflow.

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

The local proxy should remain configured for the lifetime of the browser. During transport failure it transitions to a safe deny/offline state rather than being removed or replaced with direct networking.

A Servo crash, unsupported feature, DNS error, proxy failure or transport failure must not trigger a direct-network fallback or silent engine switch.

## Device identity

Device identity and policy/update signing are separate key purposes.

Preferred model:

```text
Device proof             ES256 / ECDSA P-256
Policy/update signatures Ed25519 or separately versioned signing profile
```

P-256 allows WebGate to use hardware-backed native stores where available:

- Windows TPM through CNG Platform Crypto Provider;
- Android Keystore / TEE / StrongBox;
- macOS Secure Enclave;
- Linux TPM2 where available, with controlled software/keyring fallback.

The server records the device key algorithm and security level so access policy can distinguish hardware-backed and software-backed devices when needed.

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

WebGate integrates with those contracts rather than duplicating them.

`SecureAcces.Session.DeviceID` is currently a session/audit attribute rather than cryptographic device proof. WebGate therefore keeps device proof in a dedicated device registry and uses the fingerprint as the SecureAcces `deviceID` for visibility. SecureAcces remains authoritative for account/session/resource authorization.

## Repository map

```text
WebGate/
├── README.md
├── MASTER_PLAN.md
├── scripts/
│   ├── project_manager.py
│   ├── webgate.ps1
│   ├── webgate.sh
│   └── tests/
└── docs/
    ├── architecture/
    │   ├── ADR-0001-BROWSER-ENGINE.md
    │   ├── ADR-0002-CROSS-PLATFORM-RUNTIME.md
    │   ├── ADR-0003-SERVO-PROCESS-ISOLATION.md
    │   └── TARGET_ARCHITECTURE.md
    ├── development/
    │   └── PROJECT_MANAGER.md
    ├── implementation/
    │   └── CROSS_PLATFORM_RESILIENCE_PLAN.md
    ├── integration/
    │   └── SECUREACCESS.md
    └── research/
        ├── BROWSER_ENGINE_AUDIT.md
        ├── RESILIENCE_CROSS_PLATFORM_AUDIT.md
        └── TOOLING_AUDIT.md
```

## Non-negotiable security properties

- Servo is the default protected browser engine;
- fail closed on transport failure;
- no automatic direct-connect fallback from the protected browser;
- no silent browser-engine fallback;
- normal mode does not alter unrelated OS/app networking;
- no secret VPN/private key inside a shareable static config;
- one device key per installation;
- all enrollment bundles and remote policies are signed;
- remote policy cannot weaken compiled hard security invariants;
- protected browser has an explicit origin allowlist;
- external navigation goes to the system browser only by explicit policy;
- no generic page-to-native privileged bridge;
- local transport IPC/proxy endpoints are restricted;
- SecureAcces authorizes protected resources server-side;
- links are identifiers, not bearer credentials;
- secrets/tokens are redacted from logs and crash reports;
- updates are cryptographically signed and rollback-aware;
- restored application/browser state is never treated as authorization proof.

## Research and plans

- [`docs/research/TOOLING_AUDIT.md`](docs/research/TOOLING_AUDIT.md)
- [`docs/research/BROWSER_ENGINE_AUDIT.md`](docs/research/BROWSER_ENGINE_AUDIT.md)
- [`docs/research/RESILIENCE_CROSS_PLATFORM_AUDIT.md`](docs/research/RESILIENCE_CROSS_PLATFORM_AUDIT.md)
- [`docs/implementation/CROSS_PLATFORM_RESILIENCE_PLAN.md`](docs/implementation/CROSS_PLATFORM_RESILIENCE_PLAN.md)
- [`docs/development/PROJECT_MANAGER.md`](docs/development/PROJECT_MANAGER.md)
- [`docs/architecture/ADR-0001-BROWSER-ENGINE.md`](docs/architecture/ADR-0001-BROWSER-ENGINE.md)
- [`docs/architecture/ADR-0002-CROSS-PLATFORM-RUNTIME.md`](docs/architecture/ADR-0002-CROSS-PLATFORM-RUNTIME.md)
- [`docs/architecture/ADR-0003-SERVO-PROCESS-ISOLATION.md`](docs/architecture/ADR-0003-SERVO-PROCESS-ISOLATION.md)

## Status

**Foundation tooling is executable and verified. Servo remains the primary engine; Windows and Android are the first two architecture-validation targets.**
