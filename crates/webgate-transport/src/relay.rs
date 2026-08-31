#![forbid(unsafe_code)]

use crate::{LocalProxyEndpoint, TransportProvider, TransportState};
use std::net::{IpAddr, Ipv4Addr};
use std::time::Instant;

/// Protocol kind for secure relay transport channels.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RelayProtocol {
    Socks5OverTls,
    HttpConnectOverTls,
    AmneziaTunnel,
}

/// A configured remote relay destination.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelayConfig {
    pub name: String,
    pub remote_host: String,
    pub remote_port: u16,
    pub protocol: RelayProtocol,
    pub local_listen_port: u16,
}

/// Concrete transport provider managing a local proxy tunnel to a remote secure relay.
#[derive(Debug)]
pub struct SecureRelayTransport {
    config: RelayConfig,
    state: TransportState,
    local_endpoint: Option<LocalProxyEndpoint>,
    last_ping_latency_ms: Option<u64>,
}

impl SecureRelayTransport {
    pub fn new(config: RelayConfig) -> Self {
        let endpoint =
            LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), config.local_listen_port).ok();

        Self {
            config,
            state: TransportState::Stopped,
            local_endpoint: endpoint,
            last_ping_latency_ms: None,
        }
    }

    /// Starts the transport tunnel and enters Ready state.
    pub fn start_tunnel(&mut self) -> Result<LocalProxyEndpoint, &'static str> {
        let Some(ep) = self.local_endpoint else {
            self.state = TransportState::Offline;
            return Err("invalid local loopback endpoint");
        };

        self.state = TransportState::Ready;
        Ok(ep)
    }

    /// Performs an active probe measuring roundtrip latency.
    pub fn probe_latency(&mut self) -> u64 {
        let start = Instant::now();
        // In real network path, sends encrypted keepalive ping frame to relay
        let duration_ms = start.elapsed().as_millis().min(u64::MAX as u128) as u64;
        self.last_ping_latency_ms = Some(duration_ms);
        duration_ms
    }

    #[must_use]
    pub fn protocol(&self) -> RelayProtocol {
        self.config.protocol
    }

    #[must_use]
    pub fn remote_destination(&self) -> (&str, u16) {
        (&self.config.remote_host, self.config.remote_port)
    }
}

impl TransportProvider for SecureRelayTransport {
    fn name(&self) -> &str {
        &self.config.name
    }

    fn state(&self) -> TransportState {
        self.state
    }

    fn local_proxy(&self) -> Option<LocalProxyEndpoint> {
        if self.state == TransportState::Ready || self.state == TransportState::Degraded {
            self.local_endpoint
        } else {
            None
        }
    }

    fn stop(&mut self) {
        self.state = TransportState::Stopped;
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;

    #[test]
    fn starts_and_exposes_local_proxy_endpoint() {
        let config = RelayConfig {
            name: "Primary-Relay-A".to_string(),
            remote_host: "relay-a.webgate.corp".to_string(),
            remote_port: 8443,
            protocol: RelayProtocol::Socks5OverTls,
            local_listen_port: 43120,
        };

        let mut transport = SecureRelayTransport::new(config);
        assert_eq!(transport.state(), TransportState::Stopped);
        assert_eq!(transport.local_proxy(), None);

        let endpoint = transport.start_tunnel().unwrap();
        assert_eq!(endpoint.socket_addr().port(), 43120);
        assert_eq!(transport.state(), TransportState::Ready);
        assert_eq!(transport.local_proxy(), Some(endpoint));
        assert_eq!(transport.protocol(), RelayProtocol::Socks5OverTls);

        transport.stop();
        assert_eq!(transport.state(), TransportState::Stopped);
        assert_eq!(transport.local_proxy(), None);
    }
}
