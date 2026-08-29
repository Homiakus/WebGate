#![forbid(unsafe_code)]

use webgate_core::Platform;

/// Browser engines recognized by the WebGate browser boundary.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BrowserKind {
    Servo,
    Compatibility,
}

/// Observable lifecycle state of the protected browser capsule.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BrowserState {
    Stopped,
    Starting,
    Ready,
    Failed,
}

/// Platform-neutral configuration passed to a protected-browser adapter.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BrowserConfig {
    platform: Platform,
}

impl BrowserConfig {
    #[must_use]
    pub const fn new(platform: Platform) -> Self {
        Self { platform }
    }

    #[must_use]
    pub const fn platform(self) -> Platform {
        self.platform
    }
}

/// Minimal engine boundary. Concrete Servo types must not leak through it.
pub trait ProtectedBrowser {
    fn kind(&self) -> BrowserKind;
    fn state(&self) -> BrowserState;
    fn shutdown(&mut self);
}

#[cfg(test)]
mod tests {
    use super::BrowserConfig;
    use webgate_core::Platform;

    #[test]
    fn browser_config_preserves_platform_without_platform_api_calls() {
        let config = BrowserConfig::new(Platform::Android);
        assert_eq!(config.platform(), Platform::Android);
    }
}
