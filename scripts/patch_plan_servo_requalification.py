#!/usr/bin/env python3
from pathlib import Path

path = Path('MASTER_PLAN.md')
s = path.read_text()


def replace_once(old: str, new: str) -> None:
    global s
    count = s.count(old)
    assert count == 1, (old[:160], count)
    s = s.replace(old, new, 1)


replace_once(
    '- **F-031 — No real Servo/proxied runtime:** RESOLVED by T-041.',
    '- **F-031 — No real Servo/proxied runtime:** REOPENED / Critical → T-041R + T-080. Re-audit on 2026-09-02 proved that the current `ServoEmbeddingAdapter` is a contract/state simulator: it has no Servo dependency or real `Servo`/`WebView`/rendering event loop, `load_url()` only stores state, and `execute_proxied_fetch()` returns a simulated `proxied_response`. Historical T-041 evidence therefore does not qualify a production renderer.',
)

replace_once(
    'T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(in-memory baseline), T-030, T-032, T-034, T-035, T-043, T-049, T-050, T-039A, T-039B1, T-039B2, T-039, T-036, T-037, T-040, T-041, T-042, T-048, T-044, T-045, T-046 and **T-047** are DONE under their recorded scopes.',
    'T-001, T-002, T-003, T-006(probe), T-007, T-009, T-021(baseline), T-022(baseline), T-024(UI only), T-025(in-memory baseline), T-030, T-032, T-034, T-035, T-043, T-049, T-050, T-039A, T-039B1, T-039B2, T-039, T-036, T-037, T-040, T-042, T-048, T-044, T-045, T-046 and **T-047** are DONE under their recorded scopes. Historical T-041 is **NEEDS_REQUALIFICATION** under T-041R because its proxy/policy capsule contract was implemented but its claimed real Servo renderer/runtime proof was not.',
)

replace_once(
    '- **T-041:** Servo BrowserCapsule with enforced loopback proxy and no system-browser fallback.',
    '- **T-041:** HISTORICAL/PARTIAL — BrowserCapsule proxy/policy boundary and no-system-browser-fallback contract exist, but real Servo renderer/runtime qualification is reopened as T-041R.',
)

replace_once(
    '- **U-F-005 — GUI launch semantics do not yet prove a real BrowserCapsule launch:** CRITICAL → T-080. The inspected GUI navigation handler verifies request/transport state but does not itself own the real `BrowserCapsule::start/navigate` flow used by the CLI path.',
    '- **U-F-005 — GUI launch semantics do not yet prove a real BrowserCapsule launch:** PARTIALLY RESOLVED by T-080. GUI and CLI now share `ApplicationSessionManager`; `/api/session/open` owns `BrowserCapsule::attach_proxy/start/navigate`, retains the capsule and returns a stable correlation session ID. Final `Open` remains correctly blocked as `renderer_unqualified` until T-041R proves a real embedded renderer/navigation commit.',
)

human_anchor = '- **U-F-016 — User/operator runbooks are incomplete:** MEDIUM/HIGH → T-087. Existing docs emphasize architecture/development/operations; complete first-service, first-user, lost-device, offboarding, degraded/offline, update and recovery journeys are not yet the qualification source.\n'
human_insert = human_anchor + '- **U-F-017 — Protected renderer proof is synthetic:** CRITICAL → T-041R + T-080. The current `ServoEmbeddingAdapter::initialize()` only changes internal state, `load_url()` only records a URL/title, and `execute_proxied_fetch()` returns a simulated response. `Open` is forbidden until an actual Servo/WebView runtime produces observable renderer/load/frame/crash lifecycle evidence through the same WebGate security boundary.\n'
replace_once(human_anchor, human_insert)

replace_once(
    '**Status:** IN_PROGRESS · **Priority:** P0 client · **Depends on:** T-079 and stable T-055/T-064 browser/session contracts · **Owns:** U-F-005 · **Protects:** I-001, I-004, P-I-002, P-I-003, U-I-001, U-I-003, U-I-014',
    '**Status:** IN_PROGRESS · **Priority:** P0 client · **Depends on:** T-079, T-041R and stable T-055/T-064 browser/session contracts · **Owns:** U-F-005, U-F-017 · **Protects:** I-001, I-004, P-I-002, P-I-003, U-I-001, U-I-003, U-I-014',
)

# Insert explicit requalification gate immediately before T-080.
t080_anchor = '## T-080 — Real GUI BrowserCapsule/session orchestration\n'
assert s.count(t080_anchor) == 1, s.count(t080_anchor)
requalification = '''## T-041R — Real Servo protected-renderer requalification
**Status:** READY · **Priority:** P0 security/product blocker · **Reopens:** F-031, historical T-041 · **Owns:** U-F-017 · **Protects:** I-001, I-004, P-I-002, P-I-003, U-I-001, U-I-003, U-I-014

### Re-audit truth

The existing `BrowserCapsule` correctly enforces a loopback proxy attachment and navigation policy boundary, but the current `ServoEmbeddingAdapter` is **not** a production Servo embedder. It currently has no Servo crate dependency and no real `Servo`, `ServoBuilder`, `RenderingContext`, `WebView`, `WebViewDelegate` or renderer event loop. `initialize()` changes internal state, `load_url()` records an URL/title, and `execute_proxied_fetch()` returns simulated text.

Therefore:

- historical T-041 is not a valid proof of a real protected renderer;
- `BrowserState::Ready` from the current adapter is not sufficient evidence for application `Open`;
- T-080 must remain fail-closed at `renderer_unqualified` with zero system-browser fallback;
- README/product/qualification language must not call the current adapter a real Servo runtime until this gate is DONE.

### Required implementation

Use the current supported Servo embedding API rather than a local state simulator. The production adapter must own at least:

```text
actual servo crate/runtime
EventLoopWaker
ServoBuilder
RenderingContext (window/offscreen/software or qualified custom context)
WebViewBuilder + WebView
WebViewDelegate
application event loop
Servo::spin_event_loop
WebView::load for navigation
renderer/load/frame/crash/close callbacks
```

The concrete Servo types remain behind the `webgate-browser` boundary; security policy, WebGate transport and session orchestration remain renderer-independent.

### Renderer qualification proof

A session may transition to `Open` only after all of the following are demonstrated by the owning runtime, not inferred from a method return:

1. a real Servo engine and WebView instance were created;
2. the verified WebGate loopback transport boundary is attached and direct egress remains impossible;
3. `WebView::load` was issued for the policy-validated intended target;
4. the Servo event loop was pumped and the delegate observed the intended committed URL/load lifecycle;
5. at least one qualified render/frame-ready signal exists for the intended document, or an explicitly documented equivalent proof is justified by the current Servo API;
6. crash/close/load failure deterministically transitions the application session out of `Open`;
7. no protected URL can escape to the system browser;
8. tests prove malformed/disallowed URLs, missing proxy, renderer crash and unavailable renderer all fail closed.

### Exit contract

Release-binary evidence must prove a real embedded Servo/WebView path through the WebGate protected transport to a representative target. Source-level structs, simulated response strings, internal state transitions or mocked renderer callbacks are insufficient. Only after this gate passes may T-080 qualify application `Open` for STRICT/Servo mode.

---

'''
s = s.replace(t080_anchor, requalification + t080_anchor, 1)

qualification_anchor = '### Qualification\n\nRelease-binary GUI E2E must prove click/tap → real BrowserCapsule → real protected route → target page and prove all negative states without false success.\n'
qualification_new = qualification_anchor + '''
### Implemented partial evidence — 2026-09-02

- `ApplicationSessionManager` now owns one GUI/CLI/deep-link-ready lifecycle contract with transition history: `requested → authorizing → transport_ready → starting_protected_browser → navigating → ...`.
- `/api/session/open` and `/api/session/close` own retained BrowserCapsule lifecycle and stable non-credential correlation session IDs.
- GUI and CLI now use the same session orchestrator instead of separate launch implementations.
- Current Servo contract adapter terminates at `renderer_unqualified`; API returns fail-closed HTTP 503 and GUI explicitly states that the protected application was not opened and that system-browser fallback is forbidden.
- Integration commit `cd02d1a52a82217523e3846944a4329b5990c3f4` was produced only after `cargo check`, 24 `webgate-app` unit tests, the existing app full-stack E2E and `clippy -D warnings` passed in workflow run `33681665866`.
- GUI commit `62793b83be546b9f7af00ba7d4c8ba578f16dcfa` was produced only after source truth-contract tests, real headless Chromium session-state scenarios and Rust fmt/check/test/clippy passed in workflow run `33681860262`.
- T-080 remains **IN_PROGRESS** because current code intentionally cannot reach `Open`; T-041R is the blocking proof.
'''
replace_once(qualification_anchor, qualification_new)

path.write_text(s)
