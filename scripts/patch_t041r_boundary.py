#!/usr/bin/env python3
from pathlib import Path

adapter_path = Path('crates/webgate-browser/src/adapter.rs')
a = adapter_path.read_text()


def ar(old: str, new: str) -> None:
    global a
    count = a.count(old)
    assert count == 1, (old[:120], count)
    a = a.replace(old, new, 1)


viewport_anchor = '''impl Default for ViewportSize {
    fn default() -> Self {
        Self {
            width: 1280,
            height: 800,
        }
    }
}
'''
qualification_types = viewport_anchor + '''
/// Renderer evidence used by the application session state machine.
///
/// This is deliberately renderer-agnostic. A request method returning `Ok(())`
/// is not enough to qualify `Open`; the adapter must report evidence observed
/// from the actual renderer runtime.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct RendererQualificationSnapshot {
    pub engine_instance_created: bool,
    pub webview_created: bool,
    pub requested_url: Option<String>,
    pub observed_url: Option<String>,
    pub load_terminal_or_usable: bool,
    pub frame_ready_count: u64,
    pub crashed: bool,
    pub closed: bool,
    pub proxy_boundary_verified: bool,
}

impl RendererQualificationSnapshot {
    /// Whether the snapshot contains enough positive runtime evidence to expose
    /// a protected application session as `Open`.
    #[must_use]
    pub fn qualifies_open(&self) -> bool {
        self.engine_instance_created
            && self.webview_created
            && self.proxy_boundary_verified
            && self.load_terminal_or_usable
            && self.frame_ready_count > 0
            && !self.crashed
            && !self.closed
            && self.requested_url.is_some()
            && self.requested_url == self.observed_url
    }
}

/// Stable WebGate-side boundary implemented by either a real renderer or an
/// explicitly contract-only test/prototype adapter.
pub trait RendererQualificationEvidence {
    fn qualification_snapshot(&self) -> RendererQualificationSnapshot;
}
'''
ar(viewport_anchor, qualification_types)

ar('/// Minimal Servo embedding adapter isolating concrete renderer internals.\n#[derive(Debug)]\npub struct ServoEmbeddingAdapter {', '/// Contract-only Servo-shaped adapter. It enforces proxy/lifecycle invariants\n/// but does NOT own a real Servo engine or WebView and can never qualify `Open`.\n#[derive(Debug)]\npub struct ServoContractAdapter {')
ar('impl ServoEmbeddingAdapter {', 'impl ServoContractAdapter {')
ar('impl ProtectedBrowser for ServoEmbeddingAdapter {', 'impl ProtectedBrowser for ServoContractAdapter {')

# Make the simulated nature explicit in API names/comments while retaining the
# existing method for compatibility with BrowserCapsule tests until real Servo exists.
ar('    /// Initializes the embedder engine and event loop.\n', '    /// Initializes only the contract-state adapter. No Servo engine/WebView is created.\n')
ar('        // Concrete Servo initialization hook bounded strictly by loopback proxy\n', '        // Contract-only readiness: this is NOT renderer qualification evidence.\n')
ar('    /// Dispatches navigation intent to the embedding engine.\n', '    /// Records navigation intent for contract tests; it does not call `WebView::load`.\n')
ar('    /// Dispatches a subresource fetch through the engine\'s proxy pipeline.\n', '    /// Simulates a subresource result for legacy contract tests only.\n')
ar('        // Return simulated verified proxied response\n', '        // Explicitly simulated: never usable as production renderer/network evidence.\n')

# Add qualification evidence implementation before ProtectedBrowser impl.
protected_anchor = 'impl ProtectedBrowser for ServoContractAdapter {'
evidence_impl = '''impl RendererQualificationEvidence for ServoContractAdapter {
    fn qualification_snapshot(&self) -> RendererQualificationSnapshot {
        let proxy_boundary_verified = self
            .config
            .proxy_endpoint
            .is_some_and(|proxy| proxy.ip().is_loopback() && proxy.port() != 0);
        RendererQualificationSnapshot {
            engine_instance_created: false,
            webview_created: false,
            requested_url: self.active_url.clone(),
            observed_url: None,
            load_terminal_or_usable: false,
            frame_ready_count: 0,
            crashed: self.state == BrowserState::Failed,
            closed: self.state == BrowserState::Stopped,
            proxy_boundary_verified,
        }
    }
}

'''
assert a.count(protected_anchor) == 1
a = a.replace(protected_anchor, evidence_impl + protected_anchor, 1)

# Rename tests and construction sites.
a = a.replace('ServoEmbeddingAdapter::new', 'ServoContractAdapter::new')

# Add explicit proof test before lifecycle test.
test_anchor = '    #[test]\n    fn adapter_handles_android_pause_resume_and_memory_trim() {'
proof_test = '''    #[test]
    fn contract_adapter_can_never_qualify_protected_open() {
        let b_cfg = BrowserConfig::new(Platform::Windows);
        let config = ServoEmbeddingConfig::new(b_cfg).with_proxy(test_loopback_proxy());
        let mut adapter = ServoContractAdapter::new(config);
        adapter.initialize().unwrap();
        adapter.load_url("webgate://service/docs").unwrap();

        let proof = adapter.qualification_snapshot();
        assert!(proof.proxy_boundary_verified);
        assert_eq!(proof.requested_url.as_deref(), Some("webgate://service/docs"));
        assert!(!proof.engine_instance_created);
        assert!(!proof.webview_created);
        assert_eq!(proof.observed_url, None);
        assert_eq!(proof.frame_ready_count, 0);
        assert!(!proof.qualifies_open());
    }

'''
assert a.count(test_anchor) == 1
a = a.replace(test_anchor, proof_test + test_anchor, 1)
adapter_path.write_text(a)

capsule_path = Path('crates/webgate-browser/src/capsule.rs')
c = capsule_path.read_text()


def cr(old: str, new: str) -> None:
    global c
    count = c.count(old)
    assert count == 1, (old[:120], count)
    c = c.replace(old, new, 1)

cr(
    'use crate::adapter::{ServoEmbeddingAdapter, ServoEmbeddingConfig};',
    'use crate::adapter::{\n    RendererQualificationEvidence, RendererQualificationSnapshot, ServoContractAdapter,\n    ServoEmbeddingConfig,\n};',
)
cr('    adapter: Option<ServoEmbeddingAdapter>,', '    adapter: Option<ServoContractAdapter>,')
cr('    pub fn adapter(&self) -> Option<&ServoEmbeddingAdapter> {', '    pub fn adapter(&self) -> Option<&ServoContractAdapter> {')
cr('        let mut adapter = ServoEmbeddingAdapter::new(servo_cfg);', '        let mut adapter = ServoContractAdapter::new(servo_cfg);')

adapter_method = '''    #[must_use]
    pub fn adapter(&self) -> Option<&ServoContractAdapter> {
        self.adapter.as_ref()
    }
'''
qualification_method = adapter_method + '''
    /// Returns renderer-observed proof. Contract-only adapters intentionally
    /// return an unqualified snapshot even when BrowserCapsule policy/proxy setup
    /// itself is ready.
    #[must_use]
    pub fn renderer_qualification(&self) -> RendererQualificationSnapshot {
        self.adapter
            .as_ref()
            .map(RendererQualificationEvidence::qualification_snapshot)
            .unwrap_or_default()
    }
'''
cr(adapter_method, qualification_method)

# Extend existing positive contract test: capsule ready is not application Open proof.
cr(
    '        assert_eq!(capsule.state(), BrowserState::Ready);\n        assert!(capsule.adapter().is_some());',
    '        assert_eq!(capsule.state(), BrowserState::Ready);\n        assert!(capsule.adapter().is_some());\n        assert!(!capsule.renderer_qualification().qualifies_open());',
)
capsule_path.write_text(c)

session_path = Path('crates/webgate-app/src/session.rs')
s = session_path.read_text()

# Rename documentation references.
s = s.replace('ServoEmbeddingAdapter', 'ServoContractAdapter')

old_final = '''        // IMPORTANT: current ServoContractAdapter is a contract stub. Its
        // initialize/load_url methods do not yet own a real Servo engine/webview
        // or provide a renderer/navigation-commit proof. Retain the capsule for
        // lifecycle ownership, but never claim Open.
        transitions.push(ApplicationSessionState::RendererUnqualified);
        let snapshot = ApplicationSessionSnapshot {
            session_id: session_id.clone(),
            target_url,
            state: ApplicationSessionState::RendererUnqualified,
            message: "BrowserCapsule accepted proxy and navigation intent, but the embedded renderer is not production-qualified; protected Open is blocked".to_string(),
            transitions,
        };
'''
new_final = '''        let renderer_proof = capsule.renderer_qualification();
        let (state, message) = if renderer_proof.qualifies_open() {
            transitions.push(ApplicationSessionState::Open);
            (
                ApplicationSessionState::Open,
                "protected renderer produced qualified URL/load/frame evidence".to_string(),
            )
        } else {
            transitions.push(ApplicationSessionState::RendererUnqualified);
            (
                ApplicationSessionState::RendererUnqualified,
                "BrowserCapsule accepted proxy and navigation intent, but the renderer did not produce production-qualified engine/WebView/URL/load/frame evidence; protected Open is blocked".to_string(),
            )
        };
        let snapshot = ApplicationSessionSnapshot {
            session_id: session_id.clone(),
            target_url,
            state,
            message,
            transitions,
        };
'''
count = s.count(old_final)
assert count == 1, count
s = s.replace(old_final, new_final, 1)
session_path.write_text(s)
