# ADR-0001 — Protected Browser Engine

- Status: **ACCEPTED — SERVO PRIMARY**
- Date: **2026-08-29**
- Scope: WebGate protected browser capsule
- Research: `docs/research/BROWSER_ENGINE_AUDIT.md`

## Context

WebGate needs a fast embedded browser that can be forced through an application-local, fail-closed transport without routing unrelated operating-system traffic. The client is security-sensitive, opens a controlled documentation application, and is intended to remain predominantly Rust.

The previous M1 baseline selected Wry + Microsoft WebView2 because it was the safest short-term compatibility choice. The project has now made an explicit product decision to prioritize a native Rust browser-engine path and to adopt **Servo as the primary browser engine**.

As of the 2026-08-29 research baseline, Servo provides a Rust embedding library, a `Servo`/`WebView` embedding model, Windows builds, HTTP and HTTPS proxy support, rendering-context APIs, and an LTS release track. Its web-platform compatibility is still behind Chromium, so WebGate must treat compatibility as a measured project responsibility rather than assume arbitrary-site compatibility.

## Decision

Use:

```text
Rust native WebGate shell
        |
        v
Servo embedding API
  Servo / WebView / WebViewDelegate
        |
RenderingContext
        |
WebGate network policy
        |
app-local fail-closed proxy
        |
secure transport provider
```

**Servo is the canonical and default protected browser engine for WebGate.**

The browser abstraction must not expose Chromium/WebView2-specific concepts to the rest of WebGate. `webgate-browser-servo` owns Servo integration; application, policy, identity and transport crates depend only on WebGate interfaces.

## WebView2 status

Microsoft WebView2 is retained only as an **optional compatibility/fallback engine**, not the default architecture.

A future `webgate-browser-webview2` adapter may be maintained for pages that fail the Servo compatibility contract, but:

1. Servo remains the default engine.
2. Compatibility fallback must be explicit and policy-controlled.
3. A fallback engine must preserve the same fail-closed network invariants.
4. The client must never silently switch engines because a page fails to render.

CEF and Ultralight are research/benchmark candidates only.

## Why Servo

### Rust-first architecture

Servo is written in Rust and exposes an embedding-oriented Rust API. This keeps the critical browser capsule much closer to WebGate's memory-safety and supply-chain model than a custom C++ browser host.

### Embeddability

Servo's current library API is centered around:

- `ServoBuilder` / `Servo`;
- `WebViewBuilder` / `WebView`;
- `ServoDelegate` / `WebViewDelegate`;
- `RenderingContext`;
- `EventLoopWaker` and `Servo::spin_event_loop`.

This fits WebGate's requirement to own the shell, navigation policy, lifecycle and transport state machine.

### App-local proxy model

Servo has HTTP and HTTPS proxy support. WebGate will configure protected network access through a WebGate-owned local proxy and will make that proxy fail closed. Proxy selection/configuration is part of the browser-adapter boundary and must not be controlled by page content.

### Cross-platform direction

Servo supports Windows, Linux, macOS, Android and OpenHarmony. WebGate remains Windows-first, but the browser-engine decision no longer hard-binds the architecture to a Windows Chromium runtime.

### Performance direction

Servo is explicitly designed as a lightweight, parallel browser engine. WebGate will measure actual performance on the documentation workload rather than infer performance from language or marketing claims.

## Known risks accepted by this ADR

Servo is not Chromium and does not yet implement every Web Platform feature required by arbitrary modern sites. The project therefore accepts these obligations:

- maintain a WebGate site-compatibility suite;
- inventory every browser API required by the documents application;
- block production rollout on unsupported security-critical functionality;
- track Servo releases and LTS updates;
- run visual/regression tests against every accepted Servo upgrade;
- keep a policy-controlled WebView2 adapter available as a contingency if required.

A Servo regression must fail closed; it must never cause protected navigation to escape through a normal system browser or direct network path.

## Browser crate boundary

Target workspace:

```text
crates/
├── webgate-browser/          engine-independent browser contract
├── webgate-browser-servo/    canonical Servo adapter
└── webgate-browser-webview2/ optional compatibility adapter
```

The engine-independent layer owns concepts such as:

```rust
trait ProtectedBrowser {
    async fn start(&mut self, policy: BrowserPolicy) -> Result<()>;
    async fn navigate(&mut self, target: ProtectedTarget) -> Result<()>;
    async fn health(&self) -> BrowserHealth;
    async fn clear_session_data(&mut self) -> Result<()>;
    async fn shutdown(&mut self) -> Result<()>;
}
```

The exact trait will be refined during M1; it must not expose Servo internals.

## Network invariant

Servo is never allowed unrestricted direct network access for protected content.

Required model:

```text
Servo
  |
  v
127.0.0.1:<ephemeral WebGate proxy>
  |
  +-- transport healthy --> protected origin
  |
  +-- transport unhealthy --> DENY
```

No automatic direct fallback is permitted.

The local proxy must be destination-restricted, bind only to loopback, and reject requests until the selected secure transport and private origin are healthy.

## M1 implementation rules

1. Build the protected browser directly around Servo's embedding library.
2. Use one persistent Servo instance/WebView where practical.
3. Integrate Servo with the native event loop through `EventLoopWaker`/`spin_event_loop`.
4. Implement a WebGate-owned rendering context/window layer.
5. Configure HTTP/HTTPS proxying before protected navigation.
6. Start the local proxy in fail-closed state before Servo is allowed to navigate.
7. Enforce navigation/origin policy outside page JavaScript.
8. Intercept external navigation and open it only via explicit policy in the system browser.
9. Keep browser/session data in a dedicated WebGate location.
10. Disable or gate debugging capabilities in production.
11. Never let a Servo error, crash, unsupported feature or proxy failure cause direct protected-origin access.

## Required M1 validation

M1 must test at least:

```text
A. Servo + healthy WebGate proxy
   -> protected site reachable

B. Servo + proxy stopped
   -> protected site unreachable
   -> no direct packets to protected origin

C. Servo + transport unavailable
   -> fail-closed error UI

D. redirect/subresource/WebSocket attempts
   -> cannot bypass policy

E. required document-site feature matrix
   -> all production-critical features pass
```

Performance measurements:

- cold process-to-window time;
- cold process-to-first-protected-paint;
- warm navigation;
- idle and active RSS;
- CPU at idle;
- long-document scroll/frame stability;
- JavaScript workload relevant to the actual site;
- process/thread count;
- recovery after transport loss;
- recovery after suspend/resume and network transition.

## Compatibility gate

Servo does not need to pass arbitrary-browser compatibility. It must pass **the WebGate application contract**.

Before production, maintain a machine-readable compatibility inventory covering at least:

- authentication/session cookies;
- TLS and certificate behavior;
- fetch/XHR;
- navigation/redirects;
- forms;
- required CSS/layout;
- required JavaScript APIs;
- file/document viewing workflow;
- downloads if enabled;
- printing if required;
- WebSocket/SSE if used;
- clipboard if used;
- accessibility requirements;
- IME/Cyrillic text input;
- CJK only if the product requires it.

Unsupported optional functionality should be removed from the site or feature-gated before replacing Servo.

## Dependency/update policy

Servo releases must be pinned. WebGate should prefer an appropriate Servo LTS line for production once the compatibility suite passes, while testing current releases in CI before promotion.

Every Servo upgrade requires:

1. build reproducibility check;
2. dependency/security scan;
3. compatibility suite;
4. visual regression tests;
5. network-escape tests;
6. performance regression tests;
7. signed WebGate release validation.

## Consequences

### Positive

- canonical browser path is Rust-first;
- less dependence on the Windows Edge runtime;
- direct control over embedding lifecycle and rendering integration;
- cross-platform engine direction;
- architecture aligns with WebGate's goal of a purpose-built protected browser rather than a generic Chromium wrapper;
- browser networking can be treated as a first-class WebGate security boundary.

### Negative

- Web compatibility risk is higher than WebView2/Chromium;
- WebGate must own a stronger compatibility/regression test suite;
- monthly Servo releases may contain breaking changes outside an LTS line;
- some browser capabilities may need product-specific workarounds or site simplification;
- the WebView2 compatibility adapter may still be needed during transition.

## Supersession rule

This ADR supersedes every older WebGate document that names **Tauri**, **Wry/WebView2**, **WebView2**, **CEF**, or **Ultralight** as the primary protected browser engine.

The canonical rule is now:

> **Servo is the primary protected browser engine. WebView2 is optional compatibility fallback only.**
