#![forbid(unsafe_code)]

pub mod dual_failover;
pub mod failover;
pub mod relay;
pub mod restricted_http_connect;
pub mod restricted_socks5;
pub(crate) mod socks5_proto;

use std::net::{IpAddr, SocketAddr};

/// Construction failure for a local protected proxy endpoint.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EndpointError {
    NotLoopback,
    UnboundPort,
}

/// A proxy endpoint that is guaranteed to be loopback-only and already bound.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct LocalProxyEndpoint(SocketAddr);

impl LocalProxyEndpoint {
    pub fn new(address: IpAddr, port: u16) -> Result<Self, EndpointError> {
        if !address.is_loopback() {
            return Err(EndpointError::NotLoopback);
        }
        if port == 0 {
            return Err(EndpointError::UnboundPort);
        }
        Ok(Self(SocketAddr::new(address, port)))
    }

    #[must_use]
    pub const fn socket_addr(self) -> SocketAddr {
        self.0
    }
}

/// Loopback-only HTTP CONNECT endpoint intended for renderer proxy configuration.
/// Kept distinct from `LocalProxyEndpoint` so SOCKS5 and HTTP proxy protocols
/// cannot be accidentally interchanged at the browser boundary.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct HttpConnectProxyEndpoint(SocketAddr);

impl HttpConnectProxyEndpoint {
    pub fn new(address: IpAddr, port: u16) -> Result<Self, EndpointError> {
        if !address.is_loopback() {
            return Err(EndpointError::NotLoopback);
        }
        if port == 0 {
            return Err(EndpointError::UnboundPort);
        }
        Ok(Self(SocketAddr::new(address, port)))
    }

    #[must_use]
    pub const fn socket_addr(self) -> SocketAddr {
        self.0
    }

    #[must_use]
    pub fn proxy_uri(self) -> String {
        format!("http://{}", self.0)
    }
}

/// High-level transport state; detailed failover policy is introduced later.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TransportState {
    Stopped,
    Starting,
    Ready,
    Degraded,
    Offline,
}

/// Protocol-independent boundary presented to the application/browser layer.
pub trait TransportProvider {
    fn name(&self) -> &str;
    fn state(&self) -> TransportState;
    fn local_proxy(&self) -> Option<LocalProxyEndpoint>;
    fn stop(&mut self);
}

#[cfg(test)]
mod tests {
    use super::{EndpointError, HttpConnectProxyEndpoint, LocalProxyEndpoint};
    use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr};

    #[test]
    fn endpoint_accepts_ipv4_loopback() {
        let address = IpAddr::V4(Ipv4Addr::LOCALHOST);
        let expected = Ok(SocketAddr::new(address, 43117));
        let actual = LocalProxyEndpoint::new(address, 43117).map(LocalProxyEndpoint::socket_addr);
        assert_eq!(actual, expected);
    }

    #[test]
    fn endpoint_accepts_ipv6_loopback() {
        let address = IpAddr::V6(Ipv6Addr::LOCALHOST);
        let expected = Ok(SocketAddr::new(address, 43118));
        let actual = LocalProxyEndpoint::new(address, 43118).map(LocalProxyEndpoint::socket_addr);
        assert_eq!(actual, expected);
    }

    #[test]
    fn endpoint_rejects_non_loopback_address() {
        let result = LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::new(10, 0, 0, 7)), 43119);
        assert_eq!(result, Err(EndpointError::NotLoopback));
    }

    #[test]
    fn http_connect_endpoint_is_typed_and_loopback_only() {
        let address = IpAddr::V4(Ipv4Addr::LOCALHOST);
        let result = HttpConnectProxyEndpoint::new(address, 43120)
            .map(|endpoint| (endpoint.socket_addr(), endpoint.proxy_uri()));
        assert_eq!(
            result,
            Ok((
                SocketAddr::new(address, 43120),
                "http://127.0.0.1:43120".to_string()
            ))
        );
        assert_eq!(
            HttpConnectProxyEndpoint::new(IpAddr::V4(Ipv4Addr::new(10, 0, 0, 8)), 43120),
            Err(EndpointError::NotLoopback)
        );
    }

    #[test]
    fn endpoint_rejects_zero_port() {
        let result = LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 0);
        assert_eq!(result, Err(EndpointError::UnboundPort));
    }
}
