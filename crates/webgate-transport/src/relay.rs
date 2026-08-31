#![forbid(unsafe_code)]

use crate::{LocalProxyEndpoint, TransportProvider, TransportState};
use std::net::{IpAddr, Ipv4Addr};

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

/// Concrete transport provider configuration for a secure relay.
///
/// The protocol backend is intentionally not implemented yet. This type therefore
/// stays fail-closed instead of advertising a configured endpoint as a live tunnel.
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

    pub fn start_tunnel(&mut self) -> Result<LocalProxyEndpoint, &'static str> {
        self.state = TransportState::Offline;
        self.last_ping_latency_ms = None;
        Err("secure relay transport backend is not implemented")
    }

    #[must_use]
    pub fn last_probe_latency_ms(&self) -> Option<u64> {
        self.last_ping_latency_ms
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
        self.local_endpoint
    }

    fn stop(&mut self) {
        self.state = TransportState::Stopped;
        self.last_ping_latency_ms = None;
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;

    fn relay_config() -> RelayConfig {
        RelayConfig {
            name: "Primary-Relay-A".to_string(),
            remote_host: "relay-a.webgate.corp".to_string(),
            remote_port: 8443,
            protocol: RelayProtocol::Socks5OverTls,
            local_listen_port: 43120,
        }
    }

    #[test]
    fn configured_relay_does_not_claim_ready_without_backend() {
        let mut transport = SecureRelayTransport::new(relay_config());
        assert_eq!(transport.state(), TransportState::Stopped);
        assert_eq!(transport.local_proxy(), None);

        assert_eq!(
            transport.start_tunnel(),
            Err("secure relay transport backend is not implemented")
        );
        assert_eq!(transport.state(), TransportState::Offline);
        assert_eq!(transport.local_proxy(), None);
        assert_eq!(transport.last_probe_latency_ms(), None);
        assert_eq!(transport.protocol(), RelayProtocol::Socks5OverTls);
        assert_eq!(
            transport.remote_destination(),
            ("relay-a.webgate.corp", 8443)
        );

        transport.stop();
        assert_eq!(transport.state(), TransportState::Stopped);
        assert_eq!(transport.local_proxy(), None);
    }
}
