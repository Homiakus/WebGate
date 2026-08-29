# WebGate — Browser Engine Performance Audit

Research date: **2026-08-29**

## Decision summary

For the Windows-first WebGate client, the preferred production browser stack is:

```text
Rust native shell
  + wry (thin WebView host)
  + Microsoft WebView2 / Edge Chromium runtime
  + one persistent WebView/profile
  + app-local fail-closed HTTP CONNECT/SOCKS5 proxy
```

**Do not use Tauri as a mandatory runtime layer for the protected browser capsule.** Tauri remains acceptable for auxiliary/admin UI, but the protected browser window should be implementable directly with `wry` plus a thin native window/event-loop layer. This minimizes framework surface while keeping the same WebView2 rendering engine.

For an absolute Windows-only minimum-host experiment, a native **C++/Win32 + WebView2 SDK** spike is worth benchmarking against Rust+wry. It will not make Blink/V8 rendering itself faster, because both use WebView2, but it provides the lower-bound host overhead.

The current recommendation is **Rust+wry/WebView2**, unless measurement proves a material end-user advantage for the C++ host.

---

## What “fastest” means for WebGate

WebGate is not a general-purpose browser. Its workload is a small set of trusted document sites reached through an application-local protected transport. Therefore the important metrics are:

1. cold process start → visible native window;
2. cold start → WebView ready;
3. click/deep-link → first protected document paint;
4. warm navigation latency;
5. idle RSS / working set;
6. CPU at idle;
7. scroll/compositing smoothness for long documents/PDF-like pages;
8. JavaScript throughput for the actual document site;
9. number of helper processes;
10. binary/runtime distribution cost;
11. ability to bind **only this WebView** to the WebGate proxy;
12. fail-closed behavior if the local transport disappears;
13. security-update latency;
14. compatibility with the existing site.

A synthetic JS benchmark alone is not a valid browser selection criterion.

---

# Candidate 1 — WebView2 (Blink + V8) via Rust `wry`

## Verdict

**Primary production choice.**

### Why it fits WebGate

- Full modern Chromium/Edge web compatibility.
- Hardware-accelerated Chromium rendering/compositing.
- Mature multi-process sandbox architecture inherited from Edge/Chromium.
- Evergreen runtime provides rapid browser security updates independently from WebGate releases.
- `wry` supports per-WebView proxy configuration using HTTP CONNECT or SOCKS5 on Windows.
- No need to bundle a complete Chromium distribution with the application.
- Existing Edge/WebView2 binaries may already be resident in memory, improving warm startup.
- Microsoft explicitly documents startup, process and memory optimization techniques.

### Important performance characteristics

Microsoft documents that WebView2 is multi-process and that cold creation has a real startup cost. The practical optimization is therefore not to replace Chromium with another wrapper, but to:

- keep one WebView alive and navigate it instead of recreating controls;
- use a local User Data Folder on fast storage;
- use the Evergreen runtime;
- create the native shell immediately;
- pre-initialize the WebView environment while WebGate establishes transport;
- avoid multiple protected WebViews/tabs unless needed;
- avoid using the WebView to draw trivial startup UI.

### Recommended WebGate startup pipeline

```text
process start
   |
   +--> create tiny native Rust window immediately
   |
   +--> start restricted localhost proxy on ephemeral port
   |       (listening immediately, fail-closed until tunnel ready)
   |
   +--> initialize WebView2 environment/WebView against that proxy
   |
   +--> establish secure transport in parallel
   |
   +--> verify origin health
   |
   `--> navigate protected WebView to requested deep-link
```

This overlaps the two expensive operations: browser initialization and secure-network establishment.

### Rust host

Use `wry` directly rather than requiring the complete Tauri runtime for the browser capsule.

Candidate shape:

```text
webgate.exe
  crates/app-shell      Rust
  crates/browser-wry    Rust
  crates/policy         Rust
  crates/transport      Rust interface/supervisor
  sidecar/provider      transport implementation
```

`wry::WebViewBuilder::with_proxy_config()` supports HTTP CONNECT and SOCKS5. On Windows, Wry translates this to the WebView2 `--proxy-server` browser argument.

### C++ host comparison

Native Win32/C++ with the WebView2 SDK uses the same Edge rendering engine. Therefore the expected performance difference is limited to host startup/event-loop/integration overhead, not HTML layout, V8 execution or GPU compositing.

A C++ spike should be treated as the **host-overhead control**, not as a different browser engine.

### Main risks

- Chromium-family baseline memory/process footprint is not minimal.
- cold WebView initialization is measurable;
- proxy configuration must be set before protected navigation;
- browser/runtime policy must be hardened to avoid navigation or protocol escape.

### Sources

- Microsoft WebView2 performance guidance: https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/performance
- Wry WebView proxy API: https://docs.rs/wry/latest/wry/struct.WebViewBuilder.html
- Wry Windows integration: https://docs.rs/wry/latest/x86_64-pc-windows-msvc/wry/trait.WebViewBuilderExtWindows.html

---

# Candidate 2 — Native C++/Win32 + WebView2

## Verdict

**Benchmark as the absolute Windows host-overhead floor.**

Use only if it provides a measured, meaningful advantage over Rust+wry.

### Strengths

- minimum abstraction around Microsoft’s native API;
- direct control over WebView2 Environment/Controller lifecycle;
- no Rust-to-COM wrapper layer;
- straightforward access to low-level Windows diagnostics and ETW.

### Weaknesses

- same underlying Blink/V8/WebView2 processes as Rust+wry;
- C++ memory-safety burden in the security-sensitive client shell;
- more bespoke lifecycle/RAII/error-handling code;
- likely tiny difference in steady-state rendering because rendering is out-of-process inside WebView2.

### Decision rule

Do **not** choose C++ merely because C++ is assumed to be faster. Choose it only if the WebGate workload benchmark shows a user-visible gain, e.g. materially better p95 click-to-first-paint, RAM, or launch time.

---

# Candidate 3 — Ultralight (C/C++, WebKit + JavaScriptCore)

## Verdict

**Best experimental candidate for minimum footprint / custom renderer, but not current production default.**

Ultralight explicitly targets lightweight high-performance embedded HTML and offers CPU/GPU renderers. It is based on a custom WebKit port and JavaScriptCore and exposes C/C++ APIs plus a portable C API.

### Why it is attractive

- designed specifically for embedded apps instead of general browser embedding;
- customizable rendering and memory behavior;
- direct native ↔ JavaScriptCore integration;
- CPU or GPU renderer;
- likely attractive startup/resource profile for controlled HTML applications.

### Why it is not the WebGate default yet

- proprietary core components/commercial licensing considerations;
- not full browser parity: upstream documents exceptions including WebGL, WebRTC and HTML5 audio/video limitations/experimental status;
- less confidence than Chromium for arbitrary existing enterprise/document web apps;
- application-local proxy/fail-closed networking behavior needs a dedicated proof before it can satisfy the central WebGate invariant;
- Rust bindings are community bindings, while official APIs are C/C++/C.

### Use case where Ultralight can win

If the protected document site is deliberately constrained to a known HTML/CSS/JS profile and WebGate owns the entire web application, Ultralight should be included in the benchmark because it may beat Chromium-family engines on startup and memory.

### Sources

- https://ultralig.ht/
- https://github.com/ultralight-ux/Ultralight
- https://docs.ultralig.ht/docs/using-the-cpp-api
- https://docs.ultralig.ht/docs/using-the-c-api

---

# Candidate 4 — CEF / Chromium Embedded Framework (C++)

## Verdict

**Not recommended for WebGate unless a WebView2 limitation is discovered.**

CEF provides maximum Chromium control and excellent compatibility, but it brings a full Chromium embedding/distribution stack. CEF itself documents the Chromium multi-process model with browser, renderer and GPU processes.

For WebGate this duplicates capabilities already available from WebView2 while increasing packaging and update responsibility.

### When CEF would become justified

- Windows WebView2 policy prevents a required networking/sandbox capability;
- exact Chromium version pinning is mandatory;
- deep Chromium internals/handlers unavailable in WebView2 are required;
- WebGate must ship the same Chromium implementation across desktop operating systems.

### Source

- https://chromiumembedded.github.io/cef/general_usage.html

---

# Candidate 5 — Servo (Rust)

## Verdict

**Strategic R&D candidate, not production browser for WebGate in 2026.**

Servo is particularly interesting because it is a Rust browser engine explicitly pursuing a lightweight, high-performance embedding model.

However, as of the 2026 research baseline:

- Servo still describes itself as a prototype browser engine;
- the embedding API remains under active development;
- a stable C API wrapper is only now being expanded;
- not all WebView APIs are exposed through that C API;
- printing is still not supported;
- several embedding/JS bridge capabilities are active roadmap work.

For a security-oriented document client, web compatibility and predictable lifecycle are more important than theoretical parallel-engine advantages.

### Recommendation

Keep a `browser-servo` experimental crate behind the browser provider abstraction once WebGate reaches a mature state. Re-evaluate quarterly or at major Servo embedding milestones.

### Sources

- https://servo.org/
- https://github.com/servo/servo
- https://servo.org/blog/2026/07/31/june-in-servo/

---

# Candidate 6 — Sciter

## Verdict

**Excellent compact HTML UI engine, wrong default for an existing arbitrary web site.**

Sciter explicitly uses its own HTML/CSS engine and JS runtime and intentionally differs from a full W3C browser because its goal is compact desktop UI.

That can be extremely useful for WebGate’s *native application chrome*, settings or diagnostics. It is not the safest compatibility choice for loading an existing documents web application as-is.

### Sources

- https://docs.sciter.com/docs/intro/

---

# Ranking for WebGate

| Rank | Engine/host | Web compatibility | Expected footprint | Startup potential | Per-app proxy fit | Production maturity for task | Decision |
|---|---|---:|---:|---:|---:|---:|---|
| 1 | **Rust + wry + WebView2** | Excellent | Medium | High after optimization | Excellent | Excellent | **Default** |
| 2 | **C++ Win32 + WebView2** | Excellent | Medium | Potentially marginally best host overhead | Excellent | Excellent | Benchmark control |
| 3 | **Ultralight C++** | Good, not full browser parity | Low | Potentially excellent | Must be proven | Good but proprietary/core constraints | Experimental benchmark |
| 4 | **CEF C++** | Excellent | High | Medium | Excellent/control-rich | Excellent | Fallback only |
| 5 | **Servo Rust** | Improving | Potentially low | Potentially excellent | Needs integration work | Not yet | R&D |
| 6 | **Sciter** | Desktop-UI oriented, not full web parity | Very low | Excellent | Not the target model | Mature for UI | App chrome only |

---

# Architecture refinement: browser provider boundary

To avoid hard-coding the engine into all WebGate code, introduce:

```rust
pub trait ProtectedBrowser {
    async fn initialize(&mut self, cfg: BrowserConfig) -> Result<()>;
    async fn navigate(&mut self, target: ProtectedUrl) -> Result<()>;
    async fn clear_session(&mut self) -> Result<()>;
    async fn shutdown(&mut self) -> Result<()>;
}
```

Initial implementation:

```text
browser-webview2-wry
```

Benchmark implementations:

```text
browser-webview2-native-cpp   # small benchmark harness, not necessarily shipped
browser-ultralight            # experimental
browser-servo                 # future R&D
```

Do not abstract low-level DOM/browser capabilities prematurely. The abstraction should only cover the WebGate-owned lifecycle and policy boundary.

---

# Mandatory benchmark before final engine freeze

Create `bench/browser-capsule/` with the same protected site fixture and transport stub for all candidates.

## Test machine states

- cold boot / no browser runtime resident;
- WebView2/Edge warm;
- low-memory pressure;
- integrated GPU and discrete GPU where available;
- transport healthy;
- transport delayed by 100/500/1500 ms.

## Measurements

Capture at least 30 launches per variant:

```text
T_native_window
T_proxy_listening
T_engine_ready
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
GPU process usage
process count
bytes read from disk
binary/install footprint
```

Web workload:

- landing page;
- large long-form documentation page;
- client-side routed page;
- table-heavy page;
- image-heavy page;
- PDF workflow if required;
- WebSocket/SSE if used by the product;
- authentication/session flow backed by SecureAcces.

## Security measurements are co-equal with speed

Every candidate must also pass:

```text
local proxy killed -> no direct protected-origin connection
DNS failure -> no public DNS escape for protected names
redirect -> navigation policy remains enforced
new window -> cannot escape protected route
IPv6 -> cannot bypass proxy policy
```

A faster engine that fails any network-isolation invariant is disqualified.

---

# Final recommendation

For the next WebGate implementation slice:

1. Replace the assumption “Tauri 2 is the browser” with **“WebView2 is the browser engine; Wry is the preferred Rust host.”**
2. Build M1 using **Rust + Wry + WebView2 directly**.
3. Keep one long-lived WebView and profile.
4. Start the loopback fail-closed proxy first and initialize WebView2 against it while the secure transport connects in parallel.
5. Build a tiny **C++/Win32 + WebView2 benchmark harness** to establish the minimum host overhead.
6. Build an **Ultralight C++ benchmark harness** only if the existing documents site passes a compatibility probe and licensing is acceptable.
7. Do not move to CEF unless WebView2 fails a concrete requirement.
8. Keep Servo on the research watchlist rather than the production critical path.

The expected Pareto optimum is **Rust + Wry + WebView2**: essentially native Chromium rendering speed and compatibility, minimal WebGate-specific wrapper overhead, excellent per-WebView proxy fit, rapid security updates, and substantially lower implementation/supply-chain cost than shipping CEF.