#!/usr/bin/env python3
from pathlib import Path

cargo = Path('crates/webgate-browser/Cargo.toml')
text = cargo.read_text()
if '[features]' not in text:
    text = text.replace(
        '[dependencies]\n',
        '[features]\ndefault = []\nservo-runtime = ["dep:servo"]\n\n[dependencies]\n',
        1,
    )
if 'servo = {' not in text:
    text = text.replace(
        'webgate-core = { version = "0.1.0", path = "../webgate-core" }\n',
        'webgate-core = { version = "0.1.0", path = "../webgate-core" }\n'
        'servo = { version = "=0.5.0", optional = true, default-features = false, '
        'features = ["baked-in-resources", "bundled_freetype", "js_jit"] }\n',
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
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use servo::{
    EventLoopWaker, Preferences, RenderingContext, Servo, ServoBuilder, WebView, WebViewBuilder,
    WebViewDelegate,
};

use crate::HttpProxyEndpoint;

/// Minimal Servo event-loop wake bridge. The production runtime owns the Servo
/// instance on its renderer thread and consumes this signal by calling
/// `Servo::spin_event_loop`; wake itself never performs re-entrant Servo work.
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

/// Build Servo with both HTTP and HTTPS traffic bound to WebGate's explicitly
/// typed loopback HTTP CONNECT endpoint. This only establishes the engine/network
/// configuration seam; it is not renderer qualification evidence.
#[must_use]
pub fn build_servo(proxy: HttpProxyEndpoint, waker: ServoWakeSignal) -> Servo {
    let proxy_uri = proxy.proxy_uri();
    let mut preferences = Preferences::default();
    preferences.network_http_proxy_uri = proxy_uri.clone();
    preferences.network_https_proxy_uri = proxy_uri;
    // Never inherit an environment/bypass list into the protected renderer.
    preferences.network_http_no_proxy = String::new();

    ServoBuilder::default()
        .event_loop_waker(Box::new(waker))
        .preferences(preferences)
        .build()
}

/// Compile-time API seam for the next phase: a real WebView still requires a
/// renderer-owned RenderingContext and delegate. Creating the WebView is not
/// sufficient for `ApplicationSessionState::Open`; observable load/frame proof
/// remains mandatory.
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
#[allow(clippy::unwrap_used)]
mod tests {
    use super::ServoWakeSignal;
    use crate::HttpProxyEndpoint;
    use servo::EventLoopWaker;
    use std::net::{IpAddr, Ipv4Addr};

    #[test]
    fn wake_signal_is_edge_observable() {
        let signal = ServoWakeSignal::default();
        assert!(!signal.take());
        signal.wake();
        assert!(signal.take());
        assert!(!signal.take());
    }

    #[test]
    fn renderer_proxy_uri_is_explicit_http_loopback() {
        let endpoint =
            HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43120).unwrap();
        assert_eq!(endpoint.proxy_uri(), "http://127.0.0.1:43120");
    }
}
''')
