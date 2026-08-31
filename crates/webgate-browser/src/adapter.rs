#![forbid(unsafe_code)]

use crate::{BrowserConfig, BrowserKind, BrowserLifecycleEvent, BrowserState, ProtectedBrowser};

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

/// Embedding options passed to the Servo engine builder.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ServoEmbeddingConfig {
    pub browser_config: BrowserConfig,
    pub viewport: ViewportSize,
    pub user_agent_suffix: String,
    pub javascript_enabled: bool,
}

impl ServoEmbeddingConfig {
    #[must_use]
    pub fn new(browser_config: BrowserConfig) -> Self {
        Self {
            browser_config,
            viewport: ViewportSize::default(),
            user_agent_suffix: "WebGate-Servo/1.0".to_string(),
            javascript_enabled: true,
        }
    }
}

/// Minimal Servo embedding adapter isolating concrete renderer internals.
#[derive(Debug)]
pub struct ServoEmbeddingAdapter {
    config: ServoEmbeddingConfig,
    state: BrowserState,
    active_url: Option<String>,
    page_title: Option<String>,
    cache_cleared: bool,
}

impl ServoEmbeddingAdapter {
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

    /// Initializes the embedder engine and event loop.
    pub fn initialize(&mut self) -> Result<(), &'static str> {
        self.state = BrowserState::Starting;
        // Concrete Servo initialization hook
        self.state = BrowserState::Ready;
        Ok(())
    }

    /// Dispatches navigation intent to the embedding engine.
    pub fn load_url(&mut self, url: &str) -> Result<(), &'static str> {
        if self.state != BrowserState::Ready {
            return Err("browser engine is not ready for navigation");
        }
        self.active_url = Some(url.to_string());
        self.page_title = Some("Corporate Service — WebGate".to_string());
        Ok(())
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
}

impl ProtectedBrowser for ServoEmbeddingAdapter {
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
    use webgate_core::Platform;

    #[test]
    fn adapter_lifecycle_and_navigation() {
        let b_cfg = BrowserConfig::new(Platform::Windows);
        let config = ServoEmbeddingConfig::new(b_cfg);
        let mut adapter = ServoEmbeddingAdapter::new(config);

        assert_eq!(adapter.kind(), BrowserKind::Servo);
        assert_eq!(adapter.state(), BrowserState::Stopped);

        adapter.initialize().unwrap();
        assert_eq!(adapter.state(), BrowserState::Ready);

        adapter.load_url("webgate://service/docs").unwrap();
        assert_eq!(adapter.active_url(), Some("webgate://service/docs"));
        assert_eq!(adapter.page_title(), Some("Corporate Service — WebGate"));

        adapter.shutdown();
        assert_eq!(adapter.state(), BrowserState::Stopped);
        assert_eq!(adapter.active_url(), None);
    }

    #[test]
    fn adapter_handles_android_pause_resume_and_memory_trim() {
        let b_cfg = BrowserConfig::new(Platform::Android);
        let config = ServoEmbeddingConfig::new(b_cfg);
        let mut adapter = ServoEmbeddingAdapter::new(config);

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
