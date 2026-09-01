#![forbid(unsafe_code)]

pub mod failover;
pub mod relay;
pub mod restricted_socks5;

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
    use super::{EndpointError, LocalProxyEndpoint};
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
    fn endpoint_rejects_zero_port() {
        let result = LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 0);
        assert_eq!(result, Err(EndpointError::UnboundPort));
    }
}
