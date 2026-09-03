#![forbid(unsafe_code)]

pub mod adapter;
pub mod capsule;
pub mod qualification;

use std::net::{IpAddr, SocketAddr};
use webgate_core::Platform;

/// Construction failure for the renderer-facing HTTP proxy endpoint.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HttpProxyEndpointError {
    NotLoopback,
    UnboundPort,
}

/// Browser-owned, loopback-only HTTP proxy endpoint. This type deliberately
/// differs from the transport crate's SOCKS5 endpoint so protocol confusion is
/// rejected by the Rust type system before a renderer is started.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct HttpProxyEndpoint(SocketAddr);

impl HttpProxyEndpoint {
    pub fn new(address: IpAddr, port: u16) -> Result<Self, HttpProxyEndpointError> {
        if !address.is_loopback() {
            return Err(HttpProxyEndpointError::NotLoopback);
        }
        if port == 0 {
            return Err(HttpProxyEndpointError::UnboundPort);
        }
        Ok(Self(SocketAddr::new(address, port)))
    }

    #[must_use]
    pub const fn socket_addr(self) -> SocketAddr {
        self.0
    }

    #[must_use]
    pub const fn ip(self) -> IpAddr {
        self.0.ip()
    }

    #[must_use]
    pub const fn port(self) -> u16 {
        self.0.port()
    }

    #[must_use]
    pub fn proxy_uri(self) -> String {
        format!("http://{}", self.0)
    }
}

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
    Paused,
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

/// Platform lifecycle events received by the browser capsule (e.g. Android pause/resume/recreate).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BrowserLifecycleEvent {
    Pause,
    Resume,
    SaveState,
    RestoreState(String),
    LowMemory,
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
    fn renderer_http_proxy_is_loopback_only_and_protocol_explicit() {
        use super::{HttpProxyEndpoint, HttpProxyEndpointError};
        use std::net::{IpAddr, Ipv4Addr, SocketAddr};

        let loopback = IpAddr::V4(Ipv4Addr::LOCALHOST);
        assert_eq!(
            HttpProxyEndpoint::new(loopback, 43120).map(HttpProxyEndpoint::socket_addr),
            Ok(SocketAddr::new(loopback, 43120))
        );
        assert_eq!(
            HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::new(192, 0, 2, 8)), 43120),
            Err(HttpProxyEndpointError::NotLoopback)
        );
        assert_eq!(
            HttpProxyEndpoint::new(loopback, 0),
            Err(HttpProxyEndpointError::UnboundPort)
        );
    }

    #[test]
    fn browser_config_preserves_platform_without_platform_api_calls() {
        let config = BrowserConfig::new(Platform::Android);
        assert_eq!(config.platform(), Platform::Android);
    }
}
