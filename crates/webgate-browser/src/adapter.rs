#![forbid(unsafe_code)]

use crate::{
    BrowserConfig, BrowserKind, BrowserLifecycleEvent, BrowserState, HttpProxyEndpoint,
    ProtectedBrowser,
};

/// Rendering viewport dimensions.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ViewportSize {
    pub width: u32,
    pub height: u32,
}

impl Default for ViewportSize {
    fn default() -> Self {
        Self {
            width: 1280,
            height: 800,
        }
    }
}

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

/// Embedding options passed to the Servo engine builder.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ServoEmbeddingConfig {
    pub browser_config: BrowserConfig,
    pub viewport: ViewportSize,
    pub user_agent_suffix: String,
    pub javascript_enabled: bool,
    pub proxy_endpoint: Option<HttpProxyEndpoint>,
}

impl ServoEmbeddingConfig {
    #[must_use]
    pub fn new(browser_config: BrowserConfig) -> Self {
        Self {
            browser_config,
            viewport: ViewportSize::default(),
            user_agent_suffix: "WebGate-Servo/1.0".to_string(),
            javascript_enabled: true,
            proxy_endpoint: None,
        }
    }

    #[must_use]
    pub fn with_proxy(mut self, proxy_endpoint: HttpProxyEndpoint) -> Self {
        self.proxy_endpoint = Some(proxy_endpoint);
        self
    }
}

/// Contract-only Servo-shaped adapter. It enforces proxy/lifecycle invariants
/// but does NOT own a real Servo engine or WebView and can never qualify `Open`.
#[derive(Debug)]
pub struct ServoContractAdapter {
    config: ServoEmbeddingConfig,
    state: BrowserState,
    active_url: Option<String>,
    page_title: Option<String>,
    cache_cleared: bool,
}

impl ServoContractAdapter {
    #[must_use]
    pub fn new(config: ServoEmbeddingConfig) -> Self {
        Self {
            config,
            state: BrowserState::Stopped,
            active_url: None,
            page_title: None,
            cache_cleared: false,
        }
    }

    /// Initializes only the contract-state adapter. No Servo engine/WebView is created.
    /// Fails closed if loopback proxy endpoint is missing or non-loopback.
    pub fn initialize(&mut self) -> Result<(), &'static str> {
        let Some(proxy) = self.config.proxy_endpoint else {
            self.state = BrowserState::Failed;
            return Err(
                "proxy configuration required: fail-closed without verified loopback proxy",
            );
        };
        if !proxy.ip().is_loopback() {
            self.state = BrowserState::Failed;
            return Err("direct egress forbidden: proxy endpoint must be loopback");
        }
        if proxy.port() == 0 {
            self.state = BrowserState::Failed;
            return Err("invalid proxy port: port cannot be zero");
        }

        self.state = BrowserState::Starting;
        // Contract-only readiness: this is NOT renderer qualification evidence.
        self.state = BrowserState::Ready;
        Ok(())
    }

    /// Records navigation intent for contract tests; it does not call `WebView::load`.
    pub fn load_url(&mut self, url: &str) -> Result<(), &'static str> {
        if self.state != BrowserState::Ready {
            return Err("browser engine is not ready for navigation");
        }
        self.active_url = Some(url.to_string());
        self.page_title = Some("Corporate Service — WebGate".to_string());
        Ok(())
    }

    /// Simulates a subresource result for legacy contract tests only.
    pub fn execute_proxied_fetch(&self, target_url: &str) -> Result<String, &'static str> {
        if self.state != BrowserState::Ready {
            return Err("cannot fetch subresource: engine not in ready state");
        }
        let Some(proxy) = self.config.proxy_endpoint else {
            return Err("proxy configuration missing during subresource fetch");
        };
        if !proxy.ip().is_loopback() || proxy.port() == 0 {
            return Err("invalid proxy during subresource fetch");
        }

        // Explicitly simulated: never usable as production renderer/network evidence.
        Ok(format!("proxied_response:{target_url}"))
    }

    /// Handles platform lifecycle events (Android pause/resume/memory trim).
    pub fn handle_lifecycle_event(
        &mut self,
        event: BrowserLifecycleEvent,
    ) -> Result<(), &'static str> {
        match event {
            BrowserLifecycleEvent::Pause => {
                if self.state == BrowserState::Ready {
                    self.state = BrowserState::Paused;
                    Ok(())
                } else {
                    Err("cannot pause browser when not in ready state")
                }
            }
            BrowserLifecycleEvent::Resume => {
                if self.state == BrowserState::Paused {
                    let Some(proxy) = self.config.proxy_endpoint else {
                        self.state = BrowserState::Failed;
                        return Err("proxy configuration required on resume");
                    };
                    if !proxy.ip().is_loopback() || proxy.port() == 0 {
                        self.state = BrowserState::Failed;
                        return Err("invalid proxy on resume");
                    }
                    self.state = BrowserState::Ready;
                    Ok(())
                } else {
                    Err("cannot resume browser when not paused")
                }
            }
            BrowserLifecycleEvent::SaveState => Ok(()),
            BrowserLifecycleEvent::RestoreState(url) => {
                self.active_url = Some(url);
                self.page_title = Some("Corporate Service — WebGate".to_string());
                Ok(())
            }
            BrowserLifecycleEvent::LowMemory => {
                self.cache_cleared = true;
                Ok(())
            }
        }
    }

    #[must_use]
    pub fn active_url(&self) -> Option<&str> {
        self.active_url.as_deref()
    }

    #[must_use]
    pub fn page_title(&self) -> Option<&str> {
        self.page_title.as_deref()
    }

    #[must_use]
    pub fn config(&self) -> &ServoEmbeddingConfig {
        &self.config
    }

    #[must_use]
    pub const fn is_cache_cleared(&self) -> bool {
        self.cache_cleared
    }

    #[must_use]
    pub fn proxy_endpoint(&self) -> Option<HttpProxyEndpoint> {
        self.config.proxy_endpoint
    }
}

impl RendererQualificationEvidence for ServoContractAdapter {
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

impl ProtectedBrowser for ServoContractAdapter {
    fn kind(&self) -> BrowserKind {
        BrowserKind::Servo
    }

    fn state(&self) -> BrowserState {
        self.state
    }

    fn shutdown(&mut self) {
        self.state = BrowserState::Stopped;
        self.active_url = None;
        self.page_title = None;
        self.cache_cleared = false;
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use std::net::{IpAddr, Ipv4Addr};
    use webgate_core::Platform;

    fn test_loopback_proxy() -> HttpProxyEndpoint {
        HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41080).unwrap()
    }

    #[test]
    fn adapter_fails_closed_without_proxy() {
        let b_cfg = BrowserConfig::new(Platform::Windows);
        let config = ServoEmbeddingConfig::new(b_cfg);
        let mut adapter = ServoContractAdapter::new(config);

        assert_eq!(adapter.state(), BrowserState::Stopped);
        assert!(adapter.initialize().is_err());
        assert_eq!(adapter.state(), BrowserState::Failed);
    }

    #[test]
    fn adapter_lifecycle_and_navigation() {
        let b_cfg = BrowserConfig::new(Platform::Windows);
        let config = ServoEmbeddingConfig::new(b_cfg).with_proxy(test_loopback_proxy());
        let mut adapter = ServoContractAdapter::new(config);

        assert_eq!(adapter.kind(), BrowserKind::Servo);
        assert_eq!(adapter.state(), BrowserState::Stopped);

        adapter.initialize().unwrap();
        assert_eq!(adapter.state(), BrowserState::Ready);

        adapter.load_url("webgate://service/docs").unwrap();
        assert_eq!(adapter.active_url(), Some("webgate://service/docs"));
        assert_eq!(adapter.page_title(), Some("Corporate Service — WebGate"));

        let fetch_res = adapter
            .execute_proxied_fetch("https://docs.webgate.local/api/v1")
            .unwrap();
        assert_eq!(
            fetch_res,
            "proxied_response:https://docs.webgate.local/api/v1"
        );

        adapter.shutdown();
        assert_eq!(adapter.state(), BrowserState::Stopped);
        assert_eq!(adapter.active_url(), None);
    }

    #[test]
    fn contract_adapter_can_never_qualify_protected_open() {
        let b_cfg = BrowserConfig::new(Platform::Windows);
        let config = ServoEmbeddingConfig::new(b_cfg).with_proxy(test_loopback_proxy());
        let mut adapter = ServoContractAdapter::new(config);
        adapter.initialize().unwrap();
        adapter.load_url("webgate://service/docs").unwrap();

        let proof = adapter.qualification_snapshot();
        assert!(proof.proxy_boundary_verified);
        assert_eq!(
            proof.requested_url.as_deref(),
            Some("webgate://service/docs")
        );
        assert!(!proof.engine_instance_created);
        assert!(!proof.webview_created);
        assert_eq!(proof.observed_url, None);
        assert_eq!(proof.frame_ready_count, 0);
        assert!(!proof.qualifies_open());
    }

    #[test]
    fn adapter_handles_android_pause_resume_and_memory_trim() {
        let b_cfg = BrowserConfig::new(Platform::Android);
        let config = ServoEmbeddingConfig::new(b_cfg).with_proxy(test_loopback_proxy());
        let mut adapter = ServoContractAdapter::new(config);

        adapter.initialize().unwrap();
        adapter.load_url("webgate://service/factory").unwrap();

        // Pause
        adapter
            .handle_lifecycle_event(BrowserLifecycleEvent::Pause)
            .unwrap();
        assert_eq!(adapter.state(), BrowserState::Paused);

        // Memory trim while paused
        adapter
            .handle_lifecycle_event(BrowserLifecycleEvent::LowMemory)
            .unwrap();
        assert!(adapter.is_cache_cleared());

        // Resume
        adapter
            .handle_lifecycle_event(BrowserLifecycleEvent::Resume)
            .unwrap();
        assert_eq!(adapter.state(), BrowserState::Ready);
        assert_eq!(adapter.active_url(), Some("webgate://service/factory"));
    }
}
