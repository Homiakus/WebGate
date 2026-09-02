# Real Servo Renderer Requalification

Status: **implementation gate / T-041R**  
Last synchronized: **2026-09-02**

This document is an implementation annex to `MASTER_PLAN.md`. The plan remains the sole status owner.

## Why T-041 was reopened

The existing WebGate browser boundary provides valuable security contracts:

- `BrowserCapsule` owns navigation-policy checks;
- a protected loopback proxy must be attached before browser start;
- the system browser is not an allowed fallback for protected destinations;
- GUI and CLI now share one `ApplicationSessionManager` lifecycle;
- application `Open` is fail-closed when renderer proof is absent.

However, the current `ServoEmbeddingAdapter` is not a real Servo embedder. Re-audit found that it currently models renderer state locally instead of owning a Servo engine/WebView:

- `initialize()` transitions internal state to ready;
- `load_url()` stores URL/title state;
- `execute_proxied_fetch()` returns simulated response text;
- `crates/webgate-browser/Cargo.toml` has no Servo dependency;
- there is no owned `Servo`, `ServoBuilder`, `RenderingContext`, `WebView`, `WebViewDelegate` or Servo event loop.

Therefore historical T-041 evidence qualifies only the BrowserCapsule policy/proxy contract. It does **not** qualify a production protected renderer.

## Current fail-closed behavior

T-080 introduced a shared application session contract:

```text
requested
  -> authorizing
  -> transport_ready
  -> starting_protected_browser
  -> navigating
  -> renderer_unqualified   # current real result
```

The current build deliberately cannot reach `open` through the Servo path.

`POST /api/session/open` retains the `BrowserCapsule` and returns a correlation-only session ID, but returns `renderer_unqualified` with an unavailable status until a production renderer is proven. GUI and CLI must not convert that state into success or open the target in the system browser.

## Current Servo embedding contract to implement

The production WebGate adapter should follow the supported Servo embedding model rather than reproducing it with local state flags. The implementation boundary should own:

```text
ServoProtectedEngine
  EventLoopWaker
  ServoBuilder -> Servo
  RenderingContext
  WebViewBuilder -> WebView
  WebViewDelegate
  renderer/event-loop pump
  navigation/load/frame/crash lifecycle
```

Servo's embedding API requires the embedder to wake and run `Servo::spin_event_loop`, create a rendering context, construct a WebView through `WebViewBuilder`, and handle `WebViewDelegate::notify_new_frame_ready` so the rendered frame is painted/presented. Navigation is requested through `WebView::load`; processing still depends on spinning the Servo event loop.

Because Servo's embedding API is actively evolving, concrete Servo API calls must remain isolated behind `webgate-browser`. The rest of WebGate consumes a stable internal renderer contract.

## Proposed internal renderer boundary

```rust
trait ProtectedBrowserEngine {
    fn start(&mut self, proxy: SocketAddr, policy: NavigationPolicy)
        -> Result<(), BrowserEngineError>;

    fn navigate(&mut self, target: &str)
        -> Result<NavigationRequestId, BrowserEngineError>;

    fn pump_events(&mut self)
        -> Result<(), BrowserEngineError>;

    fn snapshot(&self) -> RendererQualificationSnapshot;

    fn shutdown(&mut self);
}
```

The public/internal snapshot should carry evidence rather than only a boolean:

```text
engine_instance_created
webview_created
requested_url
observed_url
load_status
frame_ready_count
last_frame_at
crashed
closed
proxy_boundary_verified
policy_epoch / profile identity where available
```

The actual types should remain as small and version-tolerant as possible. Do not expose Servo-specific structs outside the browser crate.

## `Open` qualification rule

`ApplicationSessionState::Open` is a security-relevant user-visible claim. It may be emitted only when the renderer supplies sufficient positive evidence.

Minimum STRICT/Servo proof:

1. a real Servo engine exists;
2. a real WebView exists;
3. a verified WebGate loopback proxy/security boundary is attached before protected navigation;
4. the policy-approved target was passed to `WebView::load`;
5. Servo's event loop was actually pumped after the request;
6. the WebView delegate observed the intended active URL/navigation lifecycle;
7. load state reached the accepted terminal/usable condition defined by the adapter;
8. at least one new-frame/render signal was observed and presented, unless a future Servo platform requires a documented equivalent proof;
9. no renderer crash/close/fatal load failure has occurred after the qualifying evidence;
10. no direct/system-browser fallback was used.

A successful return from `WebView::load`, construction of a WebView, or a local `Ready` enum alone is insufficient.

## WebViewDelegate evidence

The adapter should use current Servo callbacks as evidence inputs, including as applicable:

- `notify_url_changed`;
- `notify_load_status_changed`;
- `notify_new_frame_ready`;
- `notify_crashed`;
- `notify_closed`;
- `notify_animating_changed` when continuous event-loop pumping is needed.

The adapter should read `WebView::url()` and `WebView::load_status()` rather than trusting only requested values.

## Security requirements

### No direct network escape

The renderer may not silently use host/default networking for protected destinations. Before T-041R can become DONE, the implementation must prove that the WebView's protected requests remain inside the WebGate transport boundary.

If Servo's native proxy facilities cannot enforce the exact WebGate requirements on a supported platform, the adapter must remain unavailable/fail-closed on that platform until a correct integration exists.

### No external-browser fallback

Failure states such as these must never invoke the system browser:

```text
renderer_unavailable
renderer_unqualified
proxy_unavailable
navigation_denied
load_failed
renderer_crashed
renderer_closed
transport_offline
```

### Renderer crash/close semantics

A qualified `Open` session must leave `Open` immediately and deterministically when the owning WebView reports a crash or close event. The session manager must retain enough lifecycle ownership to perform cleanup and expose the new authoritative state.

### Navigation authority

Renderer callbacks are evidence of renderer behavior, not authorization authority. URL/service authorization remains in WebGate policy/security components before navigation is requested.

## Implementation phases

### R1 — Dependency and build isolation

- introduce a feature-gated real Servo dependency in `webgate-browser`;
- pin/lock a known Servo embedding API version;
- keep the default/non-Servo build fail-closed rather than falling back to the current simulator;
- rename the current simulator so its unqualified nature is obvious, or remove it from production paths.

### R2 — Minimal real engine

- construct `Servo` using `ServoBuilder`;
- implement `EventLoopWaker`;
- create a supported `RenderingContext`;
- create `WebView` with `WebViewBuilder` and a WebGate `WebViewDelegate`;
- implement deterministic event-loop pumping and shutdown.

### R3 — Renderer evidence model

- collect URL, load-status, frame-ready, crash and close evidence;
- expose a renderer qualification snapshot;
- add transition invariants and timeout semantics;
- explicitly distinguish `navigation_requested`, `loading`, `rendered`, `open`, `failed`.

### R4 — Protected network integration

- prove renderer traffic uses the WebGate loopback/restricted transport;
- negative-test direct/default networking escape;
- verify redirects cannot bypass policy/service authorization;
- test proxy loss/rebind/restart behavior.

### R5 — T-080 convergence

- connect renderer proof to `ApplicationSessionManager`;
- permit `Open` only from positive renderer proof;
- propagate crash/close/load failure back to GUI/CLI;
- real-browser/release-binary E2E: click/tap -> session open -> Servo WebView -> protected target -> visible frame;
- prove zero external-browser fallback on all negative paths.

## Required automated evidence

At minimum:

```text
real Servo/WebView construction test
real WebView::load + event-loop processing test
URL observation test
load-status observation test
new-frame/render observation test
renderer crash/close propagation test
policy-denied URL test
missing protected proxy test
transport-offline test
redirect-policy test
system-browser fallback absence test
session close releases renderer resources
repeated open/close leak test
```

Mocks may be used for unit isolation, but T-041R exit requires at least one release-binary path with the actual Servo runtime.

## Current evidence before T-041R implementation

- `cd02d1a52a82217523e3846944a4329b5990c3f4` — GUI and CLI routed through a shared session orchestrator after app check/tests/E2E/clippy qualification.
- `62793b83be546b9f7af00ba7d4c8ba578f16dcfa` — GUI uses the authoritative session endpoint; source truth tests, real headless Chromium state tests and Rust qualification passed before commit.
- `93d0dea42f12a2e5c29790bcdf3f358a6e40d8ca` — `MASTER_PLAN.md` reopens historical Servo qualification and adds T-041R.

These commits prove truthful orchestration and fail-closed semantics. They do **not** prove a real Servo renderer.

## Exit

T-041R becomes DONE only when release-binary evidence demonstrates a real embedded Servo/WebView rendering a protected target through WebGate's enforced transport boundary, with observable URL/load/frame evidence and deterministic failure propagation. At that point T-080 may qualify `Open` for STRICT/Servo mode.
