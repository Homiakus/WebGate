#!/usr/bin/env python3
from pathlib import Path

cargo = Path('crates/webgate-browser/Cargo.toml')
text = cargo.read_text()
if '[features]' not in text:
    text = text.replace('[dependencies]\n', '[features]\ndefault = []\nservo-runtime = ["dep:servo"]\n\n[dependencies]\n', 1)
if 'servo = {' not in text:
    text = text.replace(
        'webgate-core = { version = "0.1.0", path = "../webgate-core" }\n',
        'webgate-core = { version = "0.1.0", path = "../webgate-core" }\nservo = { version = "=0.6.0", optional = true, default-features = false, features = ["bundled"] }\n',
        1,
    )
cargo.write_text(text)

lib = Path('crates/webgate-browser/src/lib.rs')
lib_text = lib.read_text()
anchor = 'pub mod qualification;\n'
addition = anchor + '#[cfg(feature = "servo-runtime")]\npub mod servo_runtime;\n'
if 'pub mod servo_runtime;' not in lib_text:
    assert lib_text.count(anchor) == 1
    lib_text = lib_text.replace(anchor, addition, 1)
lib.write_text(lib_text)

runtime = Path('crates/webgate-browser/src/servo_runtime.rs')
runtime.write_text(r'''#![forbid(unsafe_code)]

use std::rc::Rc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

use servo::{
    EventLoopWaker, Preferences, RenderingContext, Servo, ServoBuilder, WebView, WebViewBuilder,
    WebViewDelegate,
};

/// Minimal Servo 0.6 event-loop wake bridge. The production runtime will use
/// this signal to schedule `Servo::spin_event_loop` on its owning thread.
#[derive(Debug, Clone, Default)]
pub struct ServoWakeSignal(Arc<AtomicBool>);

impl ServoWakeSignal {
    #[must_use]
    pub fn take(&self) -> bool {
        self.0.swap(false, Ordering::AcqRel)
    }
}

impl EventLoopWaker for ServoWakeSignal {
    fn clone_box(&self) -> Box<dyn EventLoopWaker> {
        Box::new(self.clone())
    }

    fn wake(&self) {
        self.0.store(true, Ordering::Release);
    }
}

/// Compile-time qualified construction path for Servo with WebGate's HTTP
/// CONNECT endpoint configured as both HTTP and HTTPS proxy authority.
///
/// This function deliberately does not claim renderer readiness. A caller still
/// needs to create a real RenderingContext/WebView, pump Servo's event loop and
/// collect URL/load/frame/crash evidence before `Open` can be qualified.
#[must_use]
pub fn build_servo(proxy_uri: &str, waker: ServoWakeSignal) -> Servo {
    let mut preferences = Preferences::default();
    preferences.network_http_proxy_uri = proxy_uri.to_string();
    preferences.network_https_proxy_uri = proxy_uri.to_string();

    ServoBuilder::default()
        .event_loop_waker(Box::new(waker))
        .preferences(preferences)
        .build()
}

/// Servo API seam used by the next runtime phase. Keeping this function here
/// ensures WebViewBuilder/RenderingContext/WebViewDelegate API drift is caught
/// by the feature qualification job while Servo-specific types remain private
/// to the browser crate.
#[must_use]
pub fn build_webview(
    servo: &Servo,
    rendering_context: Rc<dyn RenderingContext>,
    delegate: Rc<dyn WebViewDelegate>,
) -> WebView {
    WebViewBuilder::new(servo, rendering_context)
        .delegate(delegate)
        .build()
}

#[cfg(test)]
mod tests {
    use super::ServoWakeSignal;
    use servo::EventLoopWaker;

    #[test]
    fn wake_signal_is_edge_observable() {
        let signal = ServoWakeSignal::default();
        assert!(!signal.take());
        signal.wake();
        assert!(signal.take());
        assert!(!signal.take());
    }
}
''')
