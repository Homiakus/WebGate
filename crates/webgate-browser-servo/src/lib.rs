#![forbid(unsafe_code)]

use servo::{Servo, ServoBuilder};
use webgate_browser::{BrowserKind, BrowserState, ProtectedBrowser};

/// Minimal Servo adapter used to prove the browser-engine dependency boundary.
///
/// Rendering/window integration is intentionally deferred to later tasks. The
/// Servo handle remains private so Servo-specific types cannot leak into the
/// portable browser contract.
pub struct ServoBrowser {
    engine: Option<Servo>,
    state: BrowserState,
}

impl ServoBrowser {
    #[must_use]
    pub fn start() -> Self {
        Self {
            engine: Some(ServoBuilder::default().build()),
            state: BrowserState::Ready,
        }
    }

    pub fn spin_event_loop(&self) {
        if let Some(engine) = self.engine.as_ref() {
            engine.spin_event_loop();
        }
    }
}

impl ProtectedBrowser for ServoBrowser {
    fn kind(&self) -> BrowserKind {
        BrowserKind::Servo
    }

    fn state(&self) -> BrowserState {
        self.state
    }

    fn shutdown(&mut self) {
        self.engine.take();
        self.state = BrowserState::Stopped;
    }
}

#[cfg(test)]
mod tests {
    use super::ServoBrowser;
    use webgate_browser::ProtectedBrowser;

    #[test]
    fn servo_adapter_implements_protected_browser_boundary() {
        fn assert_browser<T: ProtectedBrowser>() {}
        assert_browser::<ServoBrowser>();
    }
}
