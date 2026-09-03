
---

# 19. WebGate Browser Runtime — Go/QuickJS-in-Wasm Migration Program

**Status:** ACTIVE EXECUTION EXTENSION  
**Accepted:** 2026-09-03  
**Target:** `STRICT` renderer built around Go + QuickJS-in-Wasm + OS sandbox + capability IPC + zero-network renderer + incremental Servo-style layout + WebRender-style DisplayList + WebGate ContinuitySession.

This section is part of the same living `MASTER_PLAN.md`. It does not create a parallel roadmap. It amends T-041R, T-064, T-080, T-061, T-078 and T-087 where stated below. Existing Servo work remains a measured comparison/fallback lane until the renderer bake-off proves that WebGate Browser Runtime (WGBR) can replace it for the claimed application corpus.

The migration uses a **strangler architecture**: keep the current Rust application/session boundary stable, add WGBR behind a renderer-agnostic process adapter, qualify it incrementally, switch applications explicitly, and retire Servo only after evidence. No task may silently fall back to the system browser or direct OS networking.

---

## 19.A Architecture decision

The browser runtime is not a general-purpose web browser. It is an **application-scoped capability-secure web runtime** for explicitly authorized WebGate applications.

Target trust model:

```text
                    TRUSTED WEBGATE HOST

SecureAcces / policy / device identity
                  │
          ApplicationSession
                  │
        ApplicationOriginGraph
                  │
          CapabilityBroker
          ┌───────┴────────┐
          │                │
    ResourceBroker    StorageBroker
          │                │
   ContinuitySession       │
          │                │
      PathManager          │
          │                │
       Carrier             │
          │                │
══════════╪════════ capability IPC ═══════════════════════
          │
          ▼
               UNTRUSTED WGBR RENDERER

               DocumentActor
                    │
          ┌─────────┼─────────┐
          │         │         │
        HTML       CSS       JS
                             │
                      QuickJS-in-Wasm
                             │
                           wazero
          │         │         │
          └─────────┴────┬────┘
                         ▼
                    Packed DOM
                         │
                    Style Engine
                         │
                      Box Tree
                         │
                   Fragment Tree
                         │
                    Display List
                         │
════════════════ capability IPC ═════════════════════════
                         ▼
                  TRUSTED COMPOSITOR
                    ├─ software
                    └─ GPU backend later
                         │
                         ▼
                       Frame
```

Core architectural rule:

> The renderer does not own networking, DNS, persistent storage, filesystem access, process creation, device APIs, authorization or transport selection. It can only request typed capabilities from the WebGate host.

The renderer may parse/render untrusted HTML/CSS/JS and therefore is treated as compromised by design.

---

## 19.B Renderer modes and migration policy

Canonical modes after this amendment:

```text
STRICT
  WGBR-Go
  zero-network renderer
  capability IPC
  application-origin graph
  strongest WebGate-owned runtime boundary

SERVO-TRANSITIONAL
  Servo
  retained while WGBR compatibility/performance is being proven
  never an automatic fallback from STRICT

COMPAT
  hardened platform WebView/Chromium-class engine where required
  same WebGate authorization/session policy
  explicit application/admin choice
  never the system browser
```

Rules:

- Renderer choice is explicit and observable in policy/session evidence.
- A failing STRICT launch never silently changes to SERVO or COMPAT.
- Applications may be assigned a qualified engine based on compatibility evidence.
- Servo removal is forbidden before T-126/T-128.
- Persistent security state and application authorization remain host-owned so renderer switching does not require trust-state migration.

### Amendments to existing tasks

- **T-041R:** remains a valid Servo qualification lane but is no longer the sole path to a production protected renderer. Status becomes `PARALLEL / TRANSITIONAL`. Servo may still be qualified for comparison or compatibility, but WGBR can independently satisfy the protected-renderer requirement through T-129.
- **T-080:** becomes fully renderer-agnostic. `ApplicationSessionManager` must accept any backend satisfying the stable renderer contract and runtime evidence gate.
- **T-064:** becomes a three-mode compatibility strategy: `STRICT=WGBR`, `SERVO-TRANSITIONAL`, `COMPAT=qualified platform engine` until T-128 decides whether Servo remains.
- **T-061/T-078:** release-binary/chaos qualification must execute the selected production renderer process, its sandbox, ResourceBroker and ContinuitySession integration.
- **T-087:** human journey tests must show engine selection/incompatibility truthfully and must not expose automatic insecure fallback.

---

## 19.C New WGBR invariants

Append conceptually after I-055:

- **I-056 Renderer network blindness:** production WGBR renderer code has no direct network capability. No TCP/UDP/QUIC/DNS socket is available to renderer code or its JS runtime.
- **I-057 Broker-owned resource authority:** every navigation, redirect, script, stylesheet, image, font, fetch/XHR, WebSocket/SSE and module resource crosses `ResourceBroker` policy before transport.
- **I-058 Renderer compromise containment:** a compromised renderer cannot create a new trust root, widen application origins, read another application session, access arbitrary host files, execute child processes or bypass WebGate authorization.
- **I-059 Capability IPC:** renderer↔host operations are typed, versioned, bounded and capability-scoped. Unknown messages, oversized messages, stale capabilities and cross-session capability reuse fail closed.
- **I-060 Application origin authority:** URL strings are presentation/standards objects, not authority. Host-side `ApplicationOriginGraph` decides which logical origins/resources exist and how they map to protected services.
- **I-061 Redirect reauthorization:** every redirect is re-evaluated by the broker. An allowed resource cannot redirect into an unauthorized origin.
- **I-062 Host-owned persistence:** cookies, local storage, session storage policy, downloads/uploads and durable browser state are mediated by trusted brokers. Renderer-local state is ephemeral unless explicitly committed by a broker.
- **I-063 Document single ownership:** DOM + JS observable state for one document is serialized through a `DocumentActor`; background work may be parallel but cannot race observable web semantics.
- **I-064 No pointer-authority DOM:** JS-visible DOM handles are generation-scoped IDs. Navigation/document destruction invalidates old handles deterministically.
- **I-065 Incremental rendering:** ordinary DOM/style mutations do not require unconditional full-document recascade/layout/paint; invalidation is explicit and bounded.
- **I-066 DisplayList boundary:** renderer produces a typed bounded DisplayList; renderer does not issue arbitrary GPU/native drawing commands.
- **I-067 No JIT requirement in STRICT:** baseline WGBR JavaScript execution uses QuickJS-in-Wasm without native JIT/RWX memory. Any future JIT is a separate security decision and cannot silently replace this baseline.
- **I-068 JS resource budgets:** JS execution, Wasm memory, timers, task queues, DOM size, layout work and resource requests have per-document/session/global budgets with deterministic failure behavior.
- **I-069 Decoder budgets:** images/fonts/SVG/canvas resources are size/time/pixel/frame bounded before expensive decode/render work.
- **I-070 Profile-bound compatibility:** WGBR claims support only for an explicit versioned `WGWeb` profile backed by WPT/Test262 and WebGate corpus evidence.
- **I-071 Renderer evidence:** `Open` requires actual process+sandbox+document+layout+DisplayList+presented-frame evidence, not successful method return or synthetic state.
- **I-072 Renderer crash isolation:** renderer crash/hang terminates or degrades only its owning application session and cannot terminate the WebGate host or silently bypass to another engine.
- **I-073 Continuity opacity:** renderer resource streams survive qualified carrier/path migration through `ContinuitySession` without renderer awareness; transport identity is not exposed as application authority.
- **I-074 Reversible renderer migration:** switching default renderer is a policy/release change with rollback. User/security data does not become trapped in a renderer-specific storage format.
- **I-075 No premature Servo retirement:** Servo is removed only after the bake-off, compatibility corpus, security gate and production pilot meet the explicit T-128 exit criteria.

---

## 19.D Findings opened by the WGBR transition

- **F-065 — BrowserCapsule is still Servo-shaped internally:** OPEN / Critical architecture → T-097.
- **F-066 — No production renderer process/capability boundary exists:** OPEN / Critical → T-098/T-099.
- **F-067 — Current proxy model is weaker than zero-network renderer:** OPEN / Critical → T-100/T-101.
- **F-068 — Renderer qualification does not yet prove OS sandbox/network blindness:** OPEN / High → T-102.
- **F-069 — No persistent interactive DOM/event-loop engine exists in Go:** OPEN / Critical runtime → T-103/T-104.
- **F-070 — JavaScript compatibility/runtime isolation is unqualified:** OPEN / Critical → T-105/T-106.
- **F-071 — CSS/layout path lacks incremental interactive semantics:** OPEN / High → T-108/T-109/T-110.
- **F-072 — No stable DisplayList/compositor contract exists:** OPEN / High → T-112/T-113.
- **F-073 — Browser APIs/storage/resource semantics are incomplete:** OPEN / High → T-115/T-116/T-117.
- **F-074 — WGBR compatibility claims lack profile/WPT/application-corpus evidence:** OPEN / Critical product risk → T-118/T-119/T-120.
- **F-075 — Renderer automation interface is not standardized:** OPEN / Medium → T-121.
- **F-076 — Browser resource streams are not yet bound to ContinuitySession:** OPEN / High → T-122.
- **F-077 — Tier-1 process/sandbox integration is missing:** OPEN / High → T-123.
- **F-078 — Browser attack surface has no dedicated fuzz/sandbox qualification:** OPEN / Critical security → T-124.
- **F-079 — Performance/memory advantage over Servo is only a hypothesis:** OPEN / High → T-125/T-126.
- **F-080 — Renderer default/Servo-retirement decision lacks evidence:** OPEN / High → T-126/T-128/T-129.

---

## 19.E Proposed code/package boundaries

Do not rewrite the current Rust WebGate application. Add an independent Go module/process behind the existing renderer abstraction.

```text
browser-runtime/
  go.mod
  cmd/webgate-browser-runtime/
    main.go

  internal/hostproto/       # generated/versioned IPC messages only
  internal/runtime/         # process role/bootstrap
  internal/renderer/        # untrusted renderer composition root
  internal/document/        # DocumentActor/event loop
  internal/dom/             # packed DOM + generation handles
  internal/html/            # parser/tree builder integration
  internal/css/             # tokenizer/parser/cascade/selectors
  internal/style/           # computed style + invalidation
  internal/layout/          # box tree / fragment tree
  internal/displaylist/     # renderer output IR
  internal/js/              # backend abstraction
  internal/jsquick/         # QuickJS.wasm + wazero adapter
  internal/webapi/          # DOM bindings/timers/fetch facade
  internal/input/           # hit test/focus/forms/selection
  internal/text/            # shaping/font abstraction
  internal/media/           # bounded image/SVG/canvas paths
  internal/metrics/         # content-free runtime evidence

  testdata/
    wpt/
    corpus/
    reftests/
```

Trusted Rust side adds renderer-agnostic process/broker boundaries rather than importing Go internals:

```text
crates/webgate-browser/
  renderer_contract.rs
  process_adapter.rs
  qualification.rs

crates/webgate-app/
  resource_broker.rs
  storage_broker.rs
  capability_broker.rs
  application_origin_graph.rs
```

Exact crate ownership may be adjusted to preserve existing architecture boundaries, but the trust split is invariant.

### IPC transport baseline

Use local-only OS IPC, not TCP loopback, so the renderer cannot repurpose a network stack:

```text
Linux/macOS: Unix-domain socket or equivalent local channel
Windows:     named pipe / qualified local IPC
Android:     same-app multiprocess local IPC abstraction
```

Wire format must be generated/strictly decoded and versioned. Protobuf or an equivalently mature generated schema is preferred over an ad-hoc binary parser. Every envelope has protocol version, session/process generation, request ID, message type, size bound and capability context where required.

---

# 19.F Atomic execution plan

## Phase 0 — Preserve current product while creating the seam

### T-097 — Renderer Contract Decoupling
**Status:** READY · **Priority:** P0 · **Owns:** F-065 · **Protects:** I-020, I-071, I-074

Refactor `webgate-browser` so `BrowserCapsule` no longer stores or constructs `ServoContractAdapter` directly.

Deliverables:

- stable `RendererBackend`/equivalent trait;
- backend factory selected explicitly by policy/config;
- renderer-neutral lifecycle: create, navigate, input/lifecycle, qualification, shutdown;
- renderer-neutral `RendererQualificationSnapshotV2` compatibility path;
- current Servo adapter moved behind the same boundary with no behavior weakening;
- architecture test forbids direct Servo type dependency from `BrowserCapsule` and `ApplicationSessionManager`;
- existing fail-closed and no-system-browser tests remain green.

**Exit:** current behavior is unchanged, but a second renderer backend can be added without editing BrowserCapsule/session policy code.

### T-098 — WGBR Go Module + Multi-Role Process Skeleton
**Status:** TODO · **Priority:** P0 · **Depends on:** T-097 · **Owns:** part of F-066

Create `browser-runtime/` Go module and one signed executable `webgate-browser-runtime` with explicit roles:

```text
--role=renderer
--role=compositor
--role=selftest
```

Initial renderer does not parse HTML; it performs authenticated startup IPC, reports process/build identity and exits cleanly.

Requirements:

- `CGO_ENABLED=0` baseline where dependencies permit;
- deterministic version/build metadata;
- bounded startup timeout;
- parent-death detection;
- renderer cannot choose its own session/application identity;
- CI build/test on Tier-1 host runners available to the project.

**Exit:** Rust host launches/stops a real Go renderer process and observes its exact PID/build/generation without calling it `Open`.

### T-099 — Versioned Capability IPC Protocol
**Status:** TODO · **Priority:** P0 security · **Depends on:** T-098 · **Owns:** F-066 · **Protects:** I-058, I-059

Define generated IPC schema and state machine.

Minimum message families:

```text
Hello / HelloAck
CreateDocument / DocumentCreated
NavigateRequest / NavigationCommit / NavigationFail
ResourceRequest / ResourceChunk / ResourceEnd / ResourceFail
StorageRequest / StorageResult
FrameReady / DisplayListCommit
InputEvent
LifecycleEvent
RuntimeFault / Close
QualificationSnapshot
```

Security properties:

- maximum envelope and per-message size;
- protocol-version negotiation with explicit incompatibility;
- request IDs and session/process generation binding;
- per-capability opaque IDs with scope/expiry;
- no renderer-supplied authoritative AccountID/OriginID/service route;
- unknown enum/message => fail closed;
- bounded outstanding requests/queues;
- malformed-stream fuzz target.

**Exit:** hostile/malformed renderer messages cannot create host work outside explicit bounded schemas.

### T-100 — ResourceBroker + Zero-Network Renderer Contract
**Status:** TODO · **Priority:** P0 security · **Depends on:** T-099, T-088/T-090 interfaces as needed · **Owns:** F-067 · **Protects:** I-056, I-057, I-060, I-061, I-073

Create trusted `ResourceBroker` as the only browser-resource authority.

Request model uses logical application origins/resources rather than arbitrary network destinations:

```text
ApplicationResourceRef {
  application_session_id
  origin_slot
  path/query
  method
  request metadata allowed by policy
}
```

Requirements:

- host maps `origin_slot` through signed/authorized `ApplicationOriginGraph`;
- every redirect is re-authorized;
- all response/body/headers bounded;
- dangerous hop-by-hop/proxy headers stripped or owned by host;
- broker owns same-origin/CORS policy subset required by WGWeb profile;
- broker attaches to ContinuitySession/resource stream, not raw renderer socket;
- renderer never receives relay IP, carrier credentials or transport secrets unless explicitly safe metadata is required.

Static architecture gate forbids `net`, `net/http`, raw socket and DNS imports under untrusted renderer packages except specifically allowlisted test shims that never ship.

**Exit:** a real renderer process can receive an HTML resource only through ResourceBroker; adding `http.Get`/`net.Dial` to renderer production packages fails CI.

### T-101 — OS Sandbox and Syscall-Deny Baseline
**Status:** TODO · **Priority:** P0 security · **Depends on:** T-098/T-100 · **Owns:** F-067 · **Protects:** I-056, I-058, I-072

Implement platform sandbox adapter with Linux as executable CI reference and Windows/Android as Tier-1 deliverables.

Required effective policy for renderer:

```text
no network sockets
no arbitrary filesystem
no process creation
no ptrace/debug attach
no raw device access
no privilege escalation
bounded memory/process resources
only inherited/explicit IPC handles
```

Platform targets:

- Linux: namespaces + seccomp + Landlock or justified equivalents;
- Windows: restricted token/AppContainer-class isolation + Job Object/resource controls + named-pipe allowlist;
- Android: isolated/multiprocess app sandbox + explicit IPC boundary + no INTERNET permission for renderer process where architecture permits;
- macOS later: sandbox profile/entitlement boundary if claimed.

Tests must attempt forbidden socket/file/process syscalls from a purpose-built hostile renderer helper.

**Exit:** zero-network is proved at runtime, not only by Go import policy.

### T-102 — Renderer Qualification Evidence V2
**Status:** TODO · **Priority:** P0 correctness · **Depends on:** T-098..T-101 · **Owns:** F-068 · **Protects:** I-005, I-071

Replace Servo-centric evidence fields with renderer-neutral proof:

```text
renderer_process_created
renderer_build_verified
sandbox_verified
network_syscalls_denied
ipc_session_bound
resource_broker_verified
requested_application
committed_application
main_resource_committed
document_created
event_loop_live
first_layout_completed
display_list_committed
presented_frame_count
crashed
hung
closed
```

`Open` requires all mandatory positive evidence and zero terminal faults. No backend may synthesize these fields from method success.

---

## Phase 1 — Deterministic document/JS core

### T-103 — Packed DOM Store + Generation Handles
**Status:** TODO · **Priority:** P0 runtime · **Depends on:** T-098 · **Owns:** F-069 · **Protects:** I-064, I-068

Implement packed/document-owned storage rather than pointer-heavy object graphs.

Baseline:

```text
[]Node
[]Attribute
[]TextChunk / rope-backed text where measured useful
AtomTable for tag/attribute/property strings
NodeID + DocumentGeneration handles
```

Requirements:

- O(1) or bounded parent/child/sibling traversal;
- stale JS handles fail after document generation changes;
- subtree deletion does not leave live hidden references;
- DOM size/depth/attribute/text budgets;
- mutation journal/event for style/layout invalidation;
- fuzz parser/mutation operations against invariants.

### T-104 — DocumentActor + HTML Event Loop Semantics
**Status:** TODO · **Priority:** P0 correctness · **Depends on:** T-103 · **Owns:** F-069 · **Protects:** I-063, I-068

One `DocumentActor` owns observable DOM+JS state.

Queues:

```text
navigation/input tasks
timers
network completion tasks
animation-frame tasks
microtask checkpoints
style/layout commit scheduling
```

Rules:

- no goroutine may mutate DOM directly;
- background work returns immutable/versioned results to actor;
- microtasks drain at defined checkpoints;
- timer count/frequency bounded;
- long task/watchdog evidence recorded;
- navigation cancels/invalidates old document tasks deterministically.

**Exit:** deterministic tests prove task/microtask/timer/input order for the initial WGWeb subset.

### T-105 — QuickJS-in-Wasm + wazero Runtime
**Status:** TODO · **Priority:** P0 runtime/security · **Depends on:** T-098/T-104 · **Owns:** F-070 · **Protects:** I-067, I-068

Integrate a reproducibly built/pinned QuickJS Wasm module executed by wazero.

Requirements:

- no native JIT/RWX dependency;
- deterministic upstream/version/hash/license provenance;
- ES compatibility tracked with Test262 subset;
- hard memory ceiling per JS realm;
- interrupt/watchdog path for runaway JS;
- host calls are an explicit minimal import table;
- no WASI filesystem/network imports in renderer JS instance;
- Promise/job queue integrates with DocumentActor;
- JS exception/termination cannot corrupt host process state.

A startup snapshot may be evaluated only after correctness; snapshot must contain no application/session secrets or mutable page state.

### T-106 — JS↔DOM Binding Layer
**Status:** TODO · **Priority:** P0 · **Depends on:** T-103..T-105 · **Owns:** F-070

Implement generated/table-driven bindings rather than scattered handwritten host functions where practical.

Initial interfaces:

```text
Window
Document
Node / Element / HTMLElement
EventTarget / Event
querySelector/querySelectorAll subset
attributes/class/style
textContent
create/append/remove/replace
getBoundingClientRect after layout gate
Promise
console
setTimeout/clearTimeout
requestAnimationFrame
URL
```

Every JS object references generation-scoped handles; invalid handles throw deterministic DOM exceptions rather than dereferencing stale memory/state.

### T-107 — HTML Parser, Tree Builder and Navigation Commit
**Status:** TODO · **Priority:** P0 · **Depends on:** T-100/T-103/T-104 · **Owns:** document-load foundation

Use a standards-aligned parser abstraction; `x/net/html` may bootstrap but must be replaced/extended wherever WGWeb conformance requires tree-builder behavior not exposed by it.

Navigation lifecycle:

```text
Requested
ResourceBroker authorized
Main resource headers accepted
Parser started
Document created
DOM interactive/usable threshold
load terminal/qualified threshold
```

Scripts/styles discovered during parse become broker requests, never raw fetches.

---

## Phase 2 — CSS/layout/rendering pipeline

### T-108 — CSS Parser, Selectors and Cascade
**Status:** TODO · **Priority:** P0/P1 · **Depends on:** T-103/T-107 · **Owns:** F-071

Implement WGWeb/1 CSS baseline with explicit feature table.

Initial priority:

```text
type/class/id/attribute selectors
combinators and common pseudo-classes
specificity/cascade/inheritance
custom properties required by corpus
box model
font/text properties
colors/background/border
position/display/overflow
Flexbox
Grid
Tables
```

Parser errors recover per CSS rules where profile requires; pathological selectors/stylesheets have time/node budgets.

### T-109 — BoxTree → FragmentTree Layout Core
**Status:** TODO · **Priority:** P0/P1 · **Depends on:** T-108/T-111 · **Owns:** F-071

Adopt Servo-style separation:

```text
DOM + computed style
      ↓
BoxTree
      ↓
formatting contexts
      ↓
FragmentTree
```

Start serial/deterministic. Add parallel formatting-context jobs only after correctness and profiling prove benefit.

Required WGWeb/1 formatting contexts: block/inline, flex, grid, table, positioned elements, scroll containers and replaced elements required by corpus.

### T-110 — Incremental Style/Layout Invalidation
**Status:** TODO · **Priority:** P0 performance/correctness · **Depends on:** T-108/T-109 · **Owns:** F-071 · **Protects:** I-065

Introduce explicit damage classes:

```text
D0 NONE
D1 COMPOSITE
D2 PAINT
D3 LAYOUT_LOCAL
D4 LAYOUT_SUBTREE
D5 DOCUMENT
```

DOM/style mutations calculate damage and generation IDs. Stale async style/layout results are discarded.

Acceptance includes mutation workloads proving a color/opacity/local-size change does not force full-document reconstruction unless dependency propagation requires it.

### T-111 — Text Shaping + FontBroker
**Status:** TODO · **Priority:** P1 · **Depends on:** T-100/T-109 · **Protects:** I-057, I-069

Use `go-text/typesetting` or equivalently reviewed pure-Go shaping stack rather than implementing Unicode/OpenType shaping from scratch.

Trusted `FontBroker` owns font resource authorization/cache and returns bounded font data/handles.

Qualification covers Latin/Cyrillic first plus BiDi/script shaping required by supported corpus; unsupported scripts/features are explicit profile gaps.

### T-112 — Typed Retained DisplayList IR
**Status:** TODO · **Priority:** P0/P1 · **Depends on:** T-109/T-110 · **Owns:** F-072 · **Protects:** I-066

Renderer output is immutable/bounded DisplayList primitives, for example:

```text
Rect
Border
TextRun
ImageRef
Gradient
Clip
Transform
OpacityGroup
ScrollNode
CanvasSurfaceRef
```

Requirements:

- schema/version/generation;
- bounds/count/memory validation in trusted compositor;
- no arbitrary shader/native pointer/GPU command supplied by renderer;
- resource handles scoped to compositor/session;
- damage regions permit partial redraw.

### T-113 — Deterministic Software Compositor + Frame Evidence
**Status:** TODO · **Priority:** P0 qualification · **Depends on:** T-112 · **Owns:** F-072 · **Protects:** I-066, I-071

Implement software RGBA backend first.

It owns:

- DisplayList validation;
- rasterization;
- clipping/transforms/scroll offsets;
- presented-frame generation counter;
- deterministic screenshots/reftests.

GPU backend is explicitly deferred until the software path passes correctness/compatibility gates. GPU optimization may later consume the same DisplayList contract.

### T-114 — Input, Hit Testing, Focus, Forms and Scrolling
**Status:** TODO · **Priority:** P0/P1 · **Depends on:** T-104/T-109/T-113

Implement host→renderer input events and renderer-local hit testing/focus semantics.

Initial contracts:

```text
pointer/touch click
keyboard input
focus/blur/tab order
text inputs/textareas
buttons/checkbox/radio/select
form submit through ResourceBroker
scroll containers
selection/caret baseline
```

User-gesture capabilities are minted by the host/input path and cannot be forged by JS for clipboard/download/other privileged APIs.

---

## Phase 3 — Browser APIs and resource semantics

### T-115 — StorageBroker, Cookies, History and Session State
**Status:** TODO · **Priority:** P0/P1 security · **Depends on:** T-099/T-100/T-106 · **Owns:** part of F-073 · **Protects:** I-062, I-074

Host owns persistent state partitioned by organization/application/origin/device policy.

Initial support:

```text
cookies with scope/SameSite/Secure policy required by profile
localStorage
sessionStorage
history/navigation state
cache metadata as explicitly designed
```

Renderer receives values through scoped IPC, never direct database/filesystem access. Clearing/revocation/logout semantics are deterministic and auditable where security relevant.

### T-116 — Unified Fetch/XHR/WebSocket/SSE APIs
**Status:** TODO · **Priority:** P0 compatibility · **Depends on:** T-100/T-104..T-106/T-122 · **Owns:** F-073 · **Protects:** I-057, I-061, I-073

All APIs are facades over ResourceBroker streams:

```text
fetch
XMLHttpRequest
WebSocket
EventSource/SSE
module/script/style/image/font requests
```

No API owns a socket. Cancellation/backpressure map to broker/ContinuitySession streams. Redirects and cross-origin requests re-enter policy.

### T-117 — Bounded Image/SVG/Canvas2D Pipeline
**Status:** TODO · **Priority:** P1 · **Depends on:** T-100/T-112/T-113 · **Owns:** F-073 · **Protects:** I-069

Baseline support:

- common image formats already safely supported by reviewed Go libraries;
- SVG subset required by corpus;
- Canvas2D subset required by charts/internal apps.

Budgets:

```text
encoded bytes
pixel count
width/height
animation frames
decode time
canvas surface bytes
SVG node/path complexity
```

Decoder isolation into a separate process/Wasm sandbox is evaluated after profiling/threat review; dangerous native decoder dependencies are not introduced casually.

---

## Phase 4 — Compatibility contract and automation

### T-118 — `WGWeb/1` Versioned Compatibility Profile
**Status:** TODO · **Priority:** P0 product contract · **Depends on:** T-107..T-117 baseline · **Owns:** F-074 · **Protects:** I-070

Create machine-readable profile declaring implemented/unsupported web capabilities.

Baseline non-goals unless corpus evidence promotes them:

```text
ServiceWorker
WebRTC
WebBluetooth
WebUSB
WebSerial
WebGPU
WebXR
Push/Notifications
cross-origin iframe
arbitrary popup/window.open
browser extensions
```

Early must-have candidates:

```text
HTML/forms
DOM/events
CSS cascade/Flex/Grid/Table
ES modules/Promises
fetch/XHR
WebSocket/SSE
cookies/storage/history
SVG
Canvas2D subset
file upload/download through broker policy
MutationObserver/ResizeObserver where corpus requires
```

No marketing statement says “full web compatibility”; support is `WGWeb/<version>` plus measured application corpus.

### T-119 — WPT + Test262 Conformance Harness
**Status:** TODO · **Priority:** P0 quality · **Depends on:** T-105/T-118 · **Owns:** F-074

Build pinned/reproducible subsets:

- Test262 for QuickJS build/runtime sanity;
- WPT DOM/events/HTML/CSS/fetch/websocket/etc. directories corresponding to WGWeb/1;
- reftest runner against deterministic software compositor;
- expected-failure manifest tied to profile version;
- regression dashboard by subsystem/commit.

A feature cannot be marked supported only because one internal page works.

### T-120 — WebGate Application Corpus + Compatibility Scanner
**Status:** TODO · **Priority:** P0 product risk · **Depends on:** T-118/T-119 · **Owns:** F-074

Build legal/reproducible corpus of representative internal-app patterns:

```text
static/SSR
HTMX
React
Vue
Angular
Grafana-like dashboard
LIMS-like forms/tables
ERP-like workflow
WebSocket
SSE
OIDC redirect
large DOM/virtual lists
charts/SVG/Canvas2D
upload/download
responsive/mobile
```

For each test:

```text
functional result
visual/reftest result
JS console/runtime errors
unsupported API/features
startup time
steady RAM
peak RAM
CPU/frame time
network/resource behavior
```

Add compatibility scanner that reports required features and chooses only among explicitly qualified engines; it never auto-grants an insecure origin/capability.

### T-121 — WebDriver BiDi Automation Surface
**Status:** TODO · **Priority:** P1 DX/qualification · **Depends on:** T-104/T-114/T-120 · **Owns:** F-075

Define an internal automation model then expose WebDriver BiDi-compatible adapter for supported operations.

Required first operations:

```text
create/close context
navigate
query DOM
evaluate script
click/type
observe console/errors
network/resource events
screenshot
wait for lifecycle/frame
```

CDP compatibility may be added only where tooling value justifies it; internal semantics do not depend on CDP.

---

## Phase 5 — WebGate-specific integration

### T-122 — ContinuitySession-Native Browser Resource Streams
**Status:** TODO · **Priority:** P0 resilience · **Depends on:** T-088..T-091, T-100/T-116 · **Owns:** F-076 · **Protects:** I-073

Bind ResourceBroker streams to ContinuitySession logical streams.

Qualification:

```text
page open over QUIC/H3
kill carrier
H2/TLS reaches Q5
same fetch/WebSocket/SSE logical stream policy survives where protocol semantics allow
renderer process remains alive
no renderer-visible relay/IP authority change
no duplicate delivered bytes
no acknowledged-byte loss
no OS direct escape
```

WebSocket/SSE reconnect semantics must match web API behavior; continuity must not fabricate application messages.

### T-123 — Tier-1 Platform Process/Lifecycle Integration
**Status:** TODO · **Priority:** P0/P1 · **Depends on:** T-101/T-113/T-114 · **Owns:** F-077

Tier-1 first:

- Windows renderer/compositor IPC + sandbox + frame surface;
- Android renderer/compositor process lifecycle, app background/foreground, activity recreation, low memory, input/touch and keystore/session reconnection;
- Linux as CI/reference desktop environment.

Renderer state saved across lifecycle contains no bearer credentials or unvalidated URL authority. Process recreation requires fresh broker/capability binding.

---

## Phase 6 — Security, performance and migration decision

### T-124 — WGBR Security/Fuzz/Sandbox Qualification
**Status:** TODO · **Priority:** P0 security · **Depends on:** T-099..T-117, T-123 · **Owns:** F-078

Dedicated fuzz/mutation targets:

```text
IPC decoder/state machine
HTML parser/tree builder
CSS parser/selectors
DOM mutation sequences
JS binding handles
DisplayList validator
resource redirects/headers
storage/cookie scope
image/SVG bounds
input/event sequences
```

Adversarial renderer helper attempts:

```text
socket/DNS
filesystem read/write
process spawn
oversized IPC
capability replay
cross-session capability use
forged user gesture
arbitrary origin request
redirect escape
DisplayList resource forgery
infinite JS
DOM explosion
layout pathological case
image/SVG decompression/complexity bomb
```

Pass requires bounded memory/work and zero host privilege escape under the modeled platform sandbox.

### T-125 — Performance, Memory and Startup Budget Gate
**Status:** TODO · **Priority:** P0/P1 · **Depends on:** functional T-107..T-123 · **Owns:** F-079

Measure before optimizing.

Track at minimum:

```text
renderer cold start p50/p95
first qualified frame p50/p95
warm app open
steady/peak RSS
Go heap + GC pause
QuickJS/Wasm memory
DOM bytes/node
layout time
DisplayList build time
software raster time
input-to-frame latency
resource-broker overhead
IPC bytes/messages
```

Initial directional goals, revised only from evidence:

- materially lower cold-start/steady-memory than Servo for WGWeb corpus;
- no full-document layout for local paint-only mutations;
- bounded GC growth under long-lived dashboard workloads;
- no hidden GPU dependency for baseline qualification.

Only after profiling may add object arenas, worker pools, parallel layout contexts, startup snapshots or GPU backend.

### T-126 — Renderer Bake-Off: WGBR vs Servo vs COMPAT
**Status:** TODO · **Priority:** P0 architecture decision · **Depends on:** T-120/T-124/T-125 and qualified comparison backends · **Owns:** F-079/F-080 · **Protects:** I-075

Run identical corpus/journeys against:

```text
WGBR STRICT
Servo transitional
COMPAT platform engine
```

Score dimensions:

```text
functional compatibility
visual compatibility
startup
RAM/CPU
failure recovery
network-boundary strength
sandbox strength
attack surface/dependency count
cross-platform burden
maintenance complexity
binary/distribution burden
WebGate Continuity integration
```

No weighted score may hide a hard security failure. Document which application classes require COMPAT and whether Servo adds unique value after WGBR.

### T-127 — WGBR STRICT Controlled Pilot
**Status:** TODO · **Priority:** P0 pilot · **Depends on:** T-122..T-126

Enable WGBR only by explicit policy for a bounded pilot corpus/deployment ring.

Requirements:

- rollback to previous explicitly selected renderer is one signed config/release action;
- no state loss because persistent storage is broker-owned;
- crash/compatibility diagnostics identify WGBR truthfully;
- collect content-free compatibility/performance/crash metrics;
- run representative human journeys including network migration and renderer crash.

### T-128 — Servo Retention/Retirement Decision
**Status:** BLOCKED ON EVIDENCE · **Priority:** P0 architecture cleanup · **Depends on:** T-126/T-127 · **Owns:** F-080 · **Protects:** I-074, I-075

Decision outcomes:

```text
A. Retire Servo
   WGBR becomes STRICT default
   COMPAT remains explicit fallback

B. Retain Servo transitional/specialized
   only for measured application classes where it adds value

C. WGBR not yet sufficient
   Servo remains default; WGBR stays preview
```

Servo may be removed only if:

- WGBR passes all security gates;
- required WGWeb/1 WPT/corpus thresholds are accepted;
- pilot stability/performance meets release budgets;
- COMPAT covers remaining justified applications without insecure system-browser fallback;
- no required Tier-1 function uniquely depends on Servo;
- migration/rollback documentation is complete.

### T-129 — WGBR Production/STRICT Convergence Gate
**Status:** TODO · **Priority:** P0 final renderer gate · **Depends on:** T-097..T-128 as applicable, T-061/T-078/T-087 integration evidence · **Protects:** I-056..I-075

`WGBR STRICT Production Qualified` requires:

- renderer-agnostic BrowserCapsule/session orchestration;
- real Go renderer process and verified OS sandbox;
- runtime-proven zero-network renderer;
- typed bounded capability IPC;
- host-owned ResourceBroker/StorageBroker/ApplicationOriginGraph;
- QuickJS-in-Wasm+wazero supply-chain and execution budgets;
- generation-safe packed DOM + deterministic DocumentActor;
- WGWeb/1 HTML/CSS/layout/JS/API profile implemented to accepted WPT/Test262 thresholds;
- incremental invalidation and retained DisplayList;
- deterministic software compositor and real presented-frame qualification;
- input/forms/focus/scroll critical journeys;
- ContinuitySession resource-stream migration qualification;
- Tier-1 Windows/Android behavior claimed by release;
- security fuzz/sandbox negative matrix passes;
- performance budgets recorded and accepted;
- application corpus/pilot evidence exists;
- selected renderer decision under T-128 is reflected in README/product claims;
- exact release binaries pass T-061/T-078/T-087 paths with no direct/system-browser fallback.

---

## 19.G Dependency graph

```text
CURRENT SEAM
T-097
  ↓
T-098 ─→ T-099 ─→ T-100 ─→ T-101 ─→ T-102
  │          │        │
  │          │        └──────────────────────┐
  │          │                               │
DOCUMENT / JS                                │
  └→ T-103 → T-104 → T-105 → T-106 → T-107│
                                             │
STYLE / RENDER                               │
T-107 → T-108 → T-109 → T-110               │
                    └→ T-112 → T-113 → T-114│
               T-111 ───────┘                │
                                             │
WEB APIs                                     │
T-099/T-100/T-106 → T-115                    │
T-100/T-104..106 → T-116 ← T-122             │
T-100/T-112/113 → T-117                      │
                                             │
COMPATIBILITY                                │
T-107..117 → T-118 → T-119 → T-120 → T-121 │
                                             │
WEBGATE INTEGRATION                          │
T-088..091 + T-100/T-116 → T-122             │
T-101/T-113/T-114 → T-123                    │
                                             ▼
SECURITY / PERFORMANCE
T-099..123 → T-124
T-107..123 → T-125
T-120 + T-124 + T-125 → T-126
                              ↓
                            T-127
                              ↓
                            T-128
                              ↓
                            T-129
```

T-053/T-054/T-055 and T-088..T-096 remain parallel network/security prerequisites where referenced. WGBR work must not block fixes to existing production/security defects.

---

## 19.H Atomic execution order

Do not start by implementing “all HTML/CSS/JS”. Execute vertical slices that remain runnable after every task.

### Slice A — hostile blank renderer

```text
T-097 → T-098 → T-099 → T-100 → T-101 → T-102
```

Proof:

```text
real Go renderer process
sandbox verified
network denied
host sends one brokered document payload
renderer cannot fetch anything itself
no Open state yet
```

### Slice B — first static qualified frame

```text
T-103 → T-104 → T-107 → minimal T-108/T-109 → T-112 → T-113
```

Proof:

```text
brokered HTML + CSS
DOM created
layout completed
DisplayList validated
software frame presented
qualification evidence is real
```

### Slice C — first interactive JS page

```text
T-105 → T-106 → T-110 → T-114
```

Proof:

```text
button click
JS handler
DOM mutation
incremental style/layout
new DisplayList/frame
no network/file/process capability
```

### Slice D — first real WebGate application

```text
T-115 → T-116 → T-117 as needed → T-118
```

Proof:

```text
login/session cookies
fetch/XHR
WebSocket/SSE if app requires
forms
SVG/Canvas where required
all through brokers
```

### Slice E — resilient application session

```text
T-122 + T-090/T-091/T-096
```

Proof:

```text
application open
resource stream active
carrier A killed
carrier B Q5
renderer survives
same protected application remains usable
no duplicate/escape
```

### Slice F — evidence before default switch

```text
T-119 → T-120 → T-121 → T-123 → T-124 → T-125 → T-126 → T-127 → T-128 → T-129
```

No task may skip directly to Servo retirement.

---

## 19.I First 12 implementation commits/tasks recommended

The first implementation sequence should be deliberately boring and architecture-heavy:

1. **T-097A:** introduce `RendererBackend` trait without changing behavior.
2. **T-097B:** move Servo construction behind backend factory; add architecture test.
3. **T-098A:** add Go module + `--role=selftest` binary and CI build.
4. **T-098B:** Rust process adapter launches selftest renderer with timeout/parent binding.
5. **T-099A:** define `Hello/HelloAck/Close/Fault` generated IPC schema.
6. **T-099B:** bounded local IPC framing + malformed-message tests.
7. **T-100A:** define `ApplicationOriginGraph` + typed `ApplicationResourceRef` with no network implementation yet.
8. **T-100B:** ResourceBroker serves one fixed test HTML resource over IPC.
9. **T-101A:** Linux CI sandbox blocks network/files/processes in hostile helper.
10. **T-102A:** add WGBR qualification snapshot fields; `Open` still impossible.
11. **T-103A:** packed DOM node/attribute store + generation handles.
12. **T-107A:** parse a brokered static HTML document into DOM and prove document-created evidence.

Only after these are green should QuickJS/layout complexity enter the branch. This minimizes the risk of building a sophisticated renderer on a weak security/process boundary.

---

## 19.J Rollback and compatibility policy

Every migration phase must remain reversible.

- WGBR has a distinct backend identifier and feature/policy gate.
- Existing Servo/COMPAT backend is not modified to depend on WGBR internals.
- Application assignments record engine/profile explicitly.
- Host-owned storage format is renderer-neutral and versioned.
- No WGBR migration may rewrite device identity, SecureAcces grants, service registry or ContinuitySession trust roots.
- A bad WGBR release can be disabled by signed rollout policy while other WebGate components remain on the same release where compatibility permits.
- Renderer process protocol supports adjacent-version compatibility window or explicit clean refusal.
- If WGBR cannot parse/render an application, UI reports `BROWSER_INCOMPATIBLE`; it does not silently launch another engine.

---

## 19.K Permanent verification additions

Add to CI/release qualification as implementation lands:

```text
# Go browser runtime
cd browser-runtime && go test ./...
cd browser-runtime && go vet ./...
cd browser-runtime && CGO_ENABLED=0 go test ./...
cd browser-runtime && CGO_ENABLED=0 go build ./cmd/webgate-browser-runtime

# architecture
renderer package imports forbidden network/process/filesystem packages -> fail
BrowserCapsule direct Servo dependency -> fail after T-097
renderer IPC schema drift without version handling -> fail

# security runtime
hostile renderer socket attempt -> denied
hostile renderer file read -> denied
hostile renderer process spawn -> denied
capability replay/cross-session use -> denied
redirect outside ApplicationOriginGraph -> denied
renderer crash -> host survives; session state truthful
infinite JS -> bounded termination/degraded state
oversized DOM/style/image/DisplayList -> bounded rejection

# compatibility
Test262 pinned subset
WPT WGWeb/1 selected suites
software compositor reftests
WebGate corpus journeys

# continuity
kill active carrier during fetch/WebSocket/SSE scenarios
same renderer process survives eligible migration
all paths lost -> explicit degraded/offline; zero direct fallback
```

---

## 19.L Performance architecture rules

Do not optimize before measurements, but preserve the following design opportunities:

- packed stores/IDs instead of pointer forests to reduce GC pressure;
- atom/intern tables for repeated tag/property names;
- immutable/versioned worker results;
- dirty-style/layout flags and damage regions;
- retained DisplayList and compositor-owned scroll/transform where safe;
- worker pools for resource decode/style/layout only after correctness;
- bounded caches with explicit ownership/accounting;
- optional sterile QuickJS runtime snapshot after security review;
- GPU backend only behind the same DisplayList validator;
- no per-DOM-event goroutine model.

Performance wins never weaken sandbox, capability or broker boundaries.

---

## 19.M Research/donor policy

Existing projects are evidence/donor candidates, not architecture owners.

Evaluate/reuse only after license, supply-chain, conformance and maintenance review:

- `go-webengine/engine`: CSS/layout/paint/parser ideas and tests; do not inherit whole-page screenshot architecture blindly.
- `go-text/typesetting`: preferred pure-Go text shaping candidate.
- QuickJS: ECMAScript engine source compiled reproducibly to Wasm.
- wazero: pure-Go Wasm runtime.
- Servo: layout/invalidation architecture reference and comparison backend.
- WebRender/Firefox rendering docs: DisplayList/compositor architecture reference.
- WPE WebKit/WebKit2, Chromium, Ladybird: multiprocess trust-boundary references.
- Lightpanda/Sciter-class specialized runtimes: evidence for scope reduction rather than full-web reimplementation.
- WPT/Test262/WebDriver BiDi: compatibility and automation contract references.

Do not vendor a large donor subsystem merely because it accelerates a demo. Every adopted subsystem must fit I-056..I-075.

---

## 19.N Status truth and release claims

Until T-129 is DONE:

Allowed wording:

> WebGate is developing an experimental Go-based STRICT browser runtime with a zero-network renderer and capability-mediated resource model. Servo/COMPAT remain explicit comparison/compatibility paths while WGBR is qualified.

Forbidden wording unless the owning evidence exists:

```text
full browser replacement
Chrome-compatible
complete Web standards support
production WGBR STRICT
memory safer than every alternative
faster/lighter than Servo
sandbox escape-proof
zero attack surface
```

After T-126, performance/compatibility statements may quote the measured corpus and exact platforms/versions only.

---

## 19.O WGBR convergence amendment

Technical next-generation convergence may use WGBR as the protected renderer only when T-129 is DONE. `Enterprise Qualified` additionally still requires the existing security, resilience, product and human gates.

The intended final architecture is therefore:

```text
WebGate UI / ApplicationSession
          ↓
renderer-neutral BrowserCapsule
          ↓
WGBR STRICT process
  Go DocumentActor
  Packed DOM
  QuickJS-in-Wasm / wazero
  incremental style/layout
  DisplayList
          ↓ capability IPC
trusted Resource/Storage/Capability brokers
          ↓
ContinuitySession
          ↓
Adaptive Path Manager
          ↓
heterogeneous protected carriers
          ↓
Origin / authorized private application
```

The most important transition invariant is simple:

> **Do not replace Servo with another monolith. Replace the renderer dependency with a small untrusted rendering process whose every privileged action is mediated by WebGate.**
