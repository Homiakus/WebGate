# ADR-0001 — Protected Browser Engine

- Status: **ACCEPTED FOR M1 BASELINE**
- Date: **2026-08-29**
- Scope: Windows-first WebGate protected browser capsule
- Research: `docs/research/BROWSER_ENGINE_AUDIT.md`

## Context

WebGate needs a fast embedded browser that can be forced through an application-local fail-closed proxy without routing unrelated OS traffic. The client is security-sensitive, opens an existing documents web application, and must retain modern web compatibility and fast browser security updates.

The previous baseline named Tauri 2 + Wry/WebView2. The browser audit showed that Tauri is not required for the protected browser capsule itself. WebView2 supplies the browser engine and Wry can host it directly from Rust while configuring an HTTP CONNECT/SOCKS5 proxy for the WebView.

## Decision

Use:

```text
Rust native application shell
        |
       wry
        |
Microsoft WebView2 Evergreen Runtime
        |
app-local WebGate proxy
        |
secure transport
```

The protected browser implementation will be **Wry + WebView2 directly**, with the smallest practical native window/event-loop layer.

Tauri is not a mandatory dependency of the protected browser path. It may still be used later for non-critical tooling if it produces a clear product benefit and does not weaken the capability/security boundary.

## Why not C++ as the primary host?

A C++/Win32 WebView2 host and a Rust/Wry WebView2 host use the same Blink/V8/Edge runtime. C++ therefore does not make web rendering intrinsically faster. It can only reduce host wrapper/lifecycle overhead.

A minimal C++/Win32+WebView2 harness will be maintained as a benchmark control. WebGate switches the production shell to C++ only if repeatable measurements show a material end-user advantage that justifies the memory-safety and maintenance tradeoff.

## Why not CEF?

CEF is a full Chromium embedding/distribution stack. It is valuable when low-level Chromium capabilities unavailable in WebView2 are required, but it increases binary/update/supply-chain responsibility without a demonstrated WebGate requirement.

CEF is a fallback decision only after a concrete WebView2 blocker is documented.

## Why not Ultralight?

Ultralight is a strong low-footprint C/C++ experimental candidate and should be benchmarked if the documents application fits its supported web feature set. It is not the baseline because WebGate currently values arbitrary modern-web compatibility, security-update maturity and proven application-scoped proxy behavior over theoretical footprint wins. Its proprietary core also requires a separate licensing decision.

## Why not Servo?

Servo is a strategic Rust R&D candidate, but its embedding interfaces remain under active development in 2026 and production browser capabilities still have gaps. It should not sit on the critical access path until the WebGate compatibility/security suite passes against a stable Servo embedding release.

## Performance implementation rules

1. Use one persistent protected WebView wherever possible.
2. Do not destroy/recreate the browser for ordinary navigation.
3. Create the native shell before browser initialization for immediate perceived startup.
4. Start the restricted loopback proxy immediately; it must listen in fail-closed mode even before the remote transport is healthy.
5. Initialize WebView2 against that proxy in parallel with transport establishment.
6. Navigate to protected content only after transport and private-origin health checks pass.
7. Keep the User Data Folder local and dedicated to WebGate.
8. Disable unnecessary browser capabilities in release builds.
9. Treat external links as system-browser navigation outside the protected capsule.
10. Never allow proxy failure to cause direct WebView networking.

## Required M1 benchmark

Compare at minimum:

```text
A: Rust + Wry + WebView2
B: C++ Win32 + WebView2
```

Optional if site compatibility is proven:

```text
C: C++ + Ultralight
```

Measure cold/warm startup, click-to-first-paint, working set, CPU, process count and transport-failure behavior. A browser is disqualified if it is faster but cannot prove fail-closed routing.

## Consequences

### Positive

- smaller application framework surface than mandatory Tauri;
- Rust memory safety for most client control logic;
- Chromium/Edge compatibility;
- per-WebView application-scoped proxy model;
- Evergreen security updates;
- no bundled CEF distribution by default;
- straightforward benchmark against native C++ using the same engine.

### Negative

- WebView2 retains Chromium-family process/memory overhead;
- Windows implementation remains tied to the WebView2 runtime;
- direct Wry hosting requires a little more application-shell work than full Tauri;
- cross-platform browser engines remain platform-specific behind the WebGate browser boundary.

## Supersession rule

Where older WebGate documentation says **“Tauri 2 + Wry/WebView2”** as a mandatory protected-browser stack, this ADR supersedes that wording with **“Rust + Wry + WebView2; Tauri optional outside the critical browser path.”**
