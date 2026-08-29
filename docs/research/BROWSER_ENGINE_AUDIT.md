# WebGate — Browser Engine Performance Audit

Research date: **2026-08-29**

## Decision summary

The project decision is now:

```text
Rust native WebGate shell
  + Servo embedded as a Rust library
  + one persistent protected WebView where practical
  + WebGate-owned rendering/event-loop integration
  + app-local fail-closed HTTP/HTTPS proxy
```

**Servo is the primary production target for WebGate.**

WebView2 remains an optional compatibility adapter only. CEF and Ultralight are research/benchmark alternatives, not the default path.

This supersedes the earlier audit conclusion that selected Wry + WebView2 as the primary engine. The reason for the change is product direction: WebGate is intentionally becoming a purpose-built Rust protected browser rather than a thin Chromium host.

The latest 2026 Servo state materially improves feasibility: Servo is published as an embeddable Rust crate, provides `Servo`/`WebView` APIs, Windows builds, HTTP and HTTPS proxy support, and an LTS release track. The trade-off is that WebGate must own a strict compatibility and regression suite because Servo still does not equal Chromium's arbitrary-site coverage.

---

## What “fastest” means for WebGate

WebGate is not a general-purpose browser. Its workload is a controlled private document site reached through an application-local protected transport.

The relevant metrics are:

1. cold process start → visible native window;
2. cold start → engine ready;
3. deep-link click → first protected document paint;
4. warm navigation latency;
5. idle RSS / working set;
6. CPU at idle;
7. scroll/compositing stability for long documents;
8. JavaScript throughput for the actual site;
9. process/thread count;
10. binary/runtime distribution cost;
11. ability to force protected traffic through only the WebGate proxy;
12. fail-closed behavior if local transport disappears;
13. security-update latency;
14. compatibility with the actual documents application.

Synthetic JavaScript scores alone do not decide the engine.

---

# Candidate 1 — Servo (Rust)

## Verdict

**PRIMARY / CANONICAL ENGINE.**

### Why Servo fits WebGate

- written in Rust;
- designed explicitly as an embeddable browser engine;
- exposes an embedding API around `Servo`, `WebView`, delegates and rendering contexts;
- supports Windows and has a cross-platform direction;
- supports HTTP and HTTPS proxies;
- provides a library release and LTS track;
- modular enough for WebGate to own browser lifecycle, transport policy and security boundaries;
- avoids making Chromium/WebView2 a permanent architectural dependency.

### Embedding shape

```text
webgate-app
    ↓
webgate-browser trait
    ↓
webgate-browser-servo
    ├── ServoBuilder / Servo
    ├── WebViewBuilder / WebView
    ├── WebViewDelegate
    ├── RenderingContext
    └── EventLoopWaker / spin_event_loop
```

The browser adapter translates Servo-specific lifecycle/events into WebGate-owned typed interfaces.

### Networking shape

```text
Servo
  ↓
HTTP/HTTPS proxy configuration
  ↓
127.0.0.1:<ephemeral WebGate proxy>
  ↓
restricted destination policy
  ↓
secure transport provider
  ↓
private origin
```

The proxy starts in fail-closed state. Servo is not allowed a protected direct-connect path.

### Major strengths

- memory-safe language across browser engine and WebGate control code;
- browser engine tailored toward embedding rather than generic desktop browsing;
- parallel/concurrent design;
- direct Rust integration;
- no Edge Evergreen runtime dependency;
- better long-term cross-platform consistency than a Windows-only WebView2 design;
- browser/transport architecture can evolve together behind explicit interfaces.

### Main risks

- lower arbitrary-web compatibility than Chromium;
- embedding API still evolves and monthly releases may break callers;
- some browser capabilities can remain incomplete;
- production requires aggressive compatibility, visual and network-isolation testing;
- WebGate may need to simplify/adjust its own documentation site around Servo rather than demand arbitrary-site support.

### Production release rule

Prefer a qualified Servo LTS line after the WebGate site suite passes. Test newer Servo releases continuously, but promote only after:

- build/reproducibility checks;
- security/dependency scan;
- REQUIRED capability tests;
- visual regression;
- network-escape tests;
- performance regression;
- crash/recovery tests.

### Sources

- Servo project: https://servo.org/
- Servo library/docs: https://doc.servo.org/
- crates.io/LTS announcement: https://servo.org/blog/2026/04/13/servo-0.1.0-release/
- proxy support: https://servo.org/blog/2026/01/23/december-in-servo/
- HTTPS proxy support: https://servo.org/blog/2026/02/28/january-in-servo/
- current embedding API progress: https://servo.org/blog/2026/07/31/june-in-servo/

---

# Candidate 2 — WebView2 via Rust/Wry

## Verdict

**Compatibility fallback only.**

WebView2 remains technically excellent for existing arbitrary modern web applications:

- Blink/V8 compatibility;
- mature Chromium sandbox/process model;
- Evergreen security updates;
- strong Windows integration;
- Wry can configure WebView-specific HTTP CONNECT/SOCKS5 proxy behavior.

However, it is no longer the project baseline because WebGate has chosen Servo as a first-class Rust browser engine rather than a Chromium host.

A `webgate-browser-webview2` adapter may be implemented only when a documented production requirement cannot reasonably be met with Servo.

Fallback must be:

- explicit;
- policy-controlled;
- observable;
- protected by the same local proxy;
- subject to the same navigation/origin policy;
- never triggered silently by a Servo page error.

---

# Candidate 3 — Native C++/Win32 + WebView2

## Verdict

**Benchmark/control only.**

A native C++ host can establish the lower bound of host overhead around the same WebView2 engine, but it does not make Blink/V8 intrinsically faster than Rust/Wry. It also introduces unnecessary memory-safety burden into a security-sensitive shell.

Use only for comparative measurements if needed.

---

# Candidate 4 — Ultralight (C/C++)

## Verdict

**Experimental low-footprint candidate, not default.**

Strengths:

- optimized for embedded HTML UI;
- potentially excellent startup and memory profile;
- custom CPU/GPU rendering;
- direct C/C++ APIs.

Weaknesses for WebGate:

- proprietary/commercial considerations;
- incomplete browser parity;
- app-local fail-closed transport behavior would need separate proof;
- official path is not Rust-first.

Keep as a benchmark candidate only if a future ultra-low-footprint requirement justifies it.

---

# Candidate 5 — CEF

## Verdict

**Not recommended unless a future hard requirement demands bundled Chromium control.**

CEF offers excellent compatibility but brings:

- a large Chromium runtime;
- more helper processes;
- larger packaging/update burden;
- greater browser supply-chain ownership.

Servo is the default; WebView2 is the compatibility fallback. CEF therefore has no current production role.

---

# Candidate 6 — Sciter

## Verdict

**Potential app-chrome/UI tool, not the protected site engine.**

Its compact desktop-UI focus is useful for custom application surfaces but it is not the target engine for loading the existing web application contract.

---

# Updated ranking for WebGate

| Rank | Engine/host | Rust-first | Web compatibility | Footprint potential | App-local proxy fit | Role |
|---|---|---:|---:|---:|---:|---|
| 1 | **Servo** | Excellent | Improving / must be tested | Excellent potential | Good, native proxy support | **Primary** |
| 2 | **Rust + Wry + WebView2** | Good host, Chromium engine | Excellent | Medium | Excellent | Explicit compatibility fallback |
| 3 | **C++ + Ultralight** | No | Good for controlled apps | Excellent | Must be proven | Experimental benchmark |
| 4 | **C++ + WebView2** | No | Excellent | Medium | Excellent | Benchmark control |
| 5 | **CEF** | No | Excellent | High | Excellent/control-rich | Last-resort Chromium option |
| 6 | **Sciter** | No | UI-oriented | Excellent | Not target model | App chrome only |

---

# Browser provider boundary

WebGate must not expose engine-specific APIs to unrelated crates.

```rust
pub trait ProtectedBrowser {
    async fn start(&mut self, policy: BrowserPolicy) -> Result<()>;
    async fn navigate(&mut self, target: ProtectedTarget) -> Result<()>;
    async fn health(&self) -> BrowserHealth;
    async fn clear_session_data(&mut self) -> Result<()>;
    async fn shutdown(&mut self) -> Result<()>;
}
```

Implementations:

```text
webgate-browser-servo        # canonical
webgate-browser-webview2     # optional compatibility adapter
```

Do not prematurely abstract arbitrary DOM internals. The interface exists to isolate lifecycle, navigation, health and security policy.

---

# Servo compatibility suite

Maintain a machine-readable inventory of actual site requirements.

At minimum classify:

```text
auth/session cookies
TLS/certificate behavior
fetch/XHR
forms
CSS/layout used by product
JavaScript APIs used by product
storage used by product
file/document navigation
downloads
printing
clipboard
WebSocket/SSE
Cyrillic + IME
accessibility
```

Every item is:

```text
REQUIRED
OPTIONAL
NOT USED
```

A REQUIRED failure blocks production or results in a deliberate site-side redesign/implementation plan.

---

# Mandatory performance benchmark

Use the real protected site fixture with the same WebGate proxy/transport stub.

Measure at least:

```text
T_native_window
T_proxy_listening
T_servo_ready
T_transport_ready
T_first_navigation_start
T_DOMContentLoaded
T_first_contentful_paint
T_interactive
```

System metrics:

```text
peak working set
idle working set after 30 s
CPU time to first paint
process/thread count
bytes read from disk
binary/install footprint
```

Web workload:

- landing page;
- long documentation page;
- client-side routed page;
- table-heavy page;
- image-heavy page;
- file/document workflow;
- authentication/session flow backed by SecureAcces.

---

# Security measurements are co-equal with speed

Servo must pass:

```text
local proxy killed -> no direct protected-origin connection
transport unavailable -> protected content unavailable
DNS failure -> no protected-name escape
redirect -> navigation policy remains enforced
external navigation -> explicit system-browser policy only
IPv4/IPv6 -> cannot bypass protected proxy policy
Servo crash -> no silent direct or engine fallback
unsupported page feature -> controlled error, not security downgrade
```

A faster engine that fails any network-isolation invariant is disqualified.

---

# Final recommendation

For the next WebGate implementation slice:

1. Build M1 directly around **Servo**.
2. Pin a Servo release/LTS candidate.
3. Implement the minimal Rust native embedding shell.
4. Bind protected HTTP/HTTPS traffic to the WebGate local fail-closed proxy before navigation.
5. Keep one persistent Servo/WebView where practical.
6. Build the required-site capability suite immediately, not after the browser is finished.
7. Add visual and performance regression baselines.
8. Keep WebView2 behind an optional compatibility adapter only.
9. Do not silently fall back from Servo to another browser engine.
10. Treat Servo upgrades as security-sensitive dependency upgrades with qualification gates.

The project decision is now unambiguous: **Servo is the browser engine WebGate is built around.**
