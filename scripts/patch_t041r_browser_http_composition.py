#!/usr/bin/env python3
from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    assert count == 1, (label, count)
    return text.replace(old, new, 1)

# ---------------------------------------------------------------------------
# webgate-browser: own a protocol-specific HTTP proxy type without depending on
# webgate-transport, preserving the architecture boundary.
# ---------------------------------------------------------------------------
lib = Path('crates/webgate-browser/src/lib.rs')
text = lib.read_text()
text = replace_once(
    text,
    'use webgate_core::Platform;\n',
    'use std::net::{IpAddr, SocketAddr};\nuse webgate_core::Platform;\n',
    'browser imports',
)
anchor = '''/// Browser engines recognized by the WebGate browser boundary.
'''
proxy_type = '''/// Construction failure for the renderer-facing HTTP proxy endpoint.
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

''' + anchor
text = replace_once(text, anchor, proxy_type, 'browser HTTP proxy type')
# Add focused type tests to existing test module.
test_anchor = '''    #[test]
    fn browser_config_preserves_platform_without_platform_api_calls() {'''
test_add = '''    #[test]
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

''' + test_anchor
text = replace_once(text, test_anchor, test_add, 'browser proxy tests')
lib.write_text(text)

# Adapter now consumes the browser-owned HTTP proxy type.
adapter = Path('crates/webgate-browser/src/adapter.rs')
text = adapter.read_text()
text = replace_once(
    text,
    'use crate::{BrowserConfig, BrowserKind, BrowserLifecycleEvent, BrowserState, ProtectedBrowser};\nuse std::net::SocketAddr;\n',
    'use crate::{\n    BrowserConfig, BrowserKind, BrowserLifecycleEvent, BrowserState, HttpProxyEndpoint,\n    ProtectedBrowser,\n};\n',
    'adapter imports',
)
text = text.replace('pub proxy_endpoint: Option<SocketAddr>,', 'pub proxy_endpoint: Option<HttpProxyEndpoint>,')
text = text.replace(
    'pub fn with_proxy(mut self, proxy_endpoint: SocketAddr) -> Self {',
    'pub fn with_proxy(mut self, proxy_endpoint: HttpProxyEndpoint) -> Self {',
)
text = text.replace('pub fn proxy_endpoint(&self) -> Option<SocketAddr> {', 'pub fn proxy_endpoint(&self) -> Option<HttpProxyEndpoint> {')
text = replace_once(
    text,
    '''    fn test_loopback_proxy() -> SocketAddr {
        SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41080)
    }
''',
    '''    fn test_loopback_proxy() -> HttpProxyEndpoint {
        HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41080).unwrap()
    }
''',
    'adapter test proxy',
)
# Invalid raw addresses can no longer reach the adapter; construction is tested in lib.rs.
invalid_test = '''    #[test]
    fn adapter_rejects_non_loopback_proxy() {
        let b_cfg = BrowserConfig::new(Platform::Windows);
        let public_proxy = SocketAddr::new(IpAddr::V4(Ipv4Addr::new(198, 51, 100, 1)), 8080);
        let config = ServoEmbeddingConfig::new(b_cfg).with_proxy(public_proxy);
        let mut adapter = ServoContractAdapter::new(config);

        assert!(adapter.initialize().is_err());
        assert_eq!(adapter.state(), BrowserState::Failed);
    }

'''
assert text.count(invalid_test) == 1
text = text.replace(invalid_test, '', 1)
adapter.write_text(text)

# BrowserCapsule attachment is type-safe: no raw SocketAddr API remains.
capsule = Path('crates/webgate-browser/src/capsule.rs')
text = capsule.read_text()
text = replace_once(
    text,
    'use crate::{BrowserConfig, BrowserKind, BrowserLifecycleEvent, BrowserState, ProtectedBrowser};\nuse std::net::SocketAddr;\n',
    'use crate::{\n    BrowserConfig, BrowserKind, BrowserLifecycleEvent, BrowserState, HttpProxyEndpoint,\n    ProtectedBrowser,\n};\n',
    'capsule imports',
)
text = text.replace('    DirectEgressForbidden,\n', '')
text = text.replace('    pub proxy_endpoint: SocketAddr,', '    pub proxy_endpoint: HttpProxyEndpoint,')
old_ctor = '''impl CapsuleProxyConfig {
    pub fn new(proxy_endpoint: SocketAddr) -> Result<Self, CapsuleError> {
        if !proxy_endpoint.ip().is_loopback() {
            return Err(CapsuleError::DirectEgressForbidden);
        }
        if proxy_endpoint.port() == 0 {
            return Err(CapsuleError::InvalidProxyAddress(
                "port cannot be zero".to_string(),
            ));
        }
        Ok(Self { proxy_endpoint })
    }
}
'''
new_ctor = '''impl CapsuleProxyConfig {
    #[must_use]
    pub const fn new(proxy_endpoint: HttpProxyEndpoint) -> Self {
        Self { proxy_endpoint }
    }
}
'''
text = replace_once(text, old_ctor, new_ctor, 'capsule proxy config')
text = replace_once(
    text,
    '''    pub fn attach_proxy(&mut self, endpoint: SocketAddr) -> Result<(), CapsuleError> {
        let config = CapsuleProxyConfig::new(endpoint)?;
        self.proxy_config = Some(config);
        Ok(())
    }
''',
    '''    pub fn attach_proxy(&mut self, endpoint: HttpProxyEndpoint) {
        self.proxy_config = Some(CapsuleProxyConfig::new(endpoint));
    }
''',
    'capsule attach proxy',
)
# Tests: construct only valid typed endpoints; invalid construction never reaches capsule.
text = text.replace(
    'let loopback = SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41000);',
    'let loopback = HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41000).unwrap();',
)
text = text.replace('assert_eq!(capsule.attach_proxy(loopback), Ok(()));', 'capsule.attach_proxy(loopback);')
text = text.replace('capsule.attach_proxy(loopback).unwrap();', 'capsule.attach_proxy(loopback);')
old_negative = '''    #[test]
    fn capsule_rejects_non_loopback_proxy_egress() {
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
        let public_ip = SocketAddr::new(IpAddr::V4(Ipv4Addr::new(8, 8, 8, 8)), 8080);

        assert_eq!(
            capsule.attach_proxy(public_ip),
            Err(CapsuleError::DirectEgressForbidden)
        );
        assert_eq!(capsule.start(), Err(CapsuleError::ProxyMissingFailClosed));
    }

'''
assert text.count(old_negative) == 1
text = text.replace(old_negative, '', 1)
capsule.write_text(text)

# ---------------------------------------------------------------------------
# webgate-app: compose base SOCKS transport and restricted HTTP CONNECT bridge.
# ---------------------------------------------------------------------------
main = Path('crates/webgate-app/src/main.rs')
text = main.read_text()
text = replace_once(
    text,
    '''use webgate_transport::failover::FailoverConfig;
use webgate_transport::restricted_socks5::{
''',
    '''use webgate_transport::failover::FailoverConfig;
use webgate_transport::restricted_http_connect::{
    RestrictedHttpConnectConfig, RestrictedHttpConnectError, RestrictedHttpConnectStatusHandle,
    RestrictedHttpConnectTransport,
};
use webgate_transport::restricted_socks5::{
''',
    'main HTTP bridge imports',
)
text = replace_once(
    text,
    'use webgate_transport::{LocalProxyEndpoint, TransportProvider, TransportState};\n',
    'use webgate_transport::{HttpConnectProxyEndpoint, LocalProxyEndpoint, TransportProvider, TransportState};\n',
    'main transport imports',
)
start = text.index('/// Owns the running transport listener/workers')
end = text.index('const fn transport_state_label')
new_runtime = r'''/// Base protected SOCKS transport owner. It is intentionally private to the
/// application composition layer and never crosses into the browser crate.
#[derive(Debug)]
enum ClientBaseTransportOwner {
    Primary(RestrictedSocks5Transport),
    Dual(DualRelayFailoverTransport),
}

impl ClientBaseTransportOwner {
    #[must_use]
    fn state(&self) -> TransportState {
        match self {
            Self::Primary(transport) => transport.state(),
            Self::Dual(transport) => transport.state(),
        }
    }
}

#[derive(Debug, Clone)]
enum ClientBaseTransportStatus {
    Primary(RestrictedProxyStatusHandle),
    Dual(DualRelayStatusHandle),
}

impl ClientBaseTransportStatus {
    #[must_use]
    fn snapshot(&self) -> (TransportState, Option<LocalProxyEndpoint>) {
        match self {
            Self::Primary(status) => status.snapshot(),
            Self::Dual(status) => {
                let (state, _role, endpoint, _primary_health, _fallback_health) = status.snapshot();
                (state, endpoint)
            }
        }
    }
}

/// Owns both protocol layers. The bridge field is declared first so it is
/// revoked before the underlying SOCKS listener when this owner is dropped.
#[derive(Debug)]
struct ClientTransportOwner {
    browser_bridge: RestrictedHttpConnectTransport,
    base: ClientBaseTransportOwner,
}

impl ClientTransportOwner {
    #[must_use]
    fn state(&self) -> TransportState {
        combine_transport_states(self.base.state(), self.browser_bridge.state())
    }
}

/// Live transport truth source used by GUI, CLI and the session orchestrator.
/// Successful runtime state is the conjunction of the base SOCKS layer and the
/// renderer-facing restricted HTTP CONNECT bridge.
#[derive(Debug, Clone)]
enum ClientTransportStatus {
    Live {
        base: ClientBaseTransportStatus,
        browser_bridge: RestrictedHttpConnectStatusHandle,
    },
    Fixed {
        state: TransportState,
        endpoint: Option<HttpConnectProxyEndpoint>,
    },
}

impl ClientTransportStatus {
    #[must_use]
    fn snapshot(&self) -> (TransportState, Option<HttpConnectProxyEndpoint>) {
        match self {
            Self::Live {
                base,
                browser_bridge,
            } => {
                let (base_state, base_endpoint) = base.snapshot();
                let (bridge_state, bridge_endpoint) = browser_bridge.snapshot();
                let combined = combine_transport_states(base_state, bridge_state);
                let endpoint = if matches!(combined, TransportState::Ready | TransportState::Degraded)
                    && base_endpoint.is_some()
                {
                    bridge_endpoint
                } else {
                    None
                };
                (combined, endpoint)
            }
            Self::Fixed { state, endpoint } => (*state, *endpoint),
        }
    }

    #[must_use]
    const fn offline() -> Self {
        Self::Fixed {
            state: TransportState::Offline,
            endpoint: None,
        }
    }

    #[cfg(test)]
    #[must_use]
    const fn fixed(state: TransportState, endpoint: Option<HttpConnectProxyEndpoint>) -> Self {
        Self::Fixed { state, endpoint }
    }
}

const fn combine_transport_states(base: TransportState, bridge: TransportState) -> TransportState {
    if matches!(base, TransportState::Offline) || matches!(bridge, TransportState::Offline) {
        TransportState::Offline
    } else if matches!(base, TransportState::Stopped) || matches!(bridge, TransportState::Stopped) {
        TransportState::Stopped
    } else if matches!(base, TransportState::Starting) || matches!(bridge, TransportState::Starting) {
        TransportState::Starting
    } else if matches!(base, TransportState::Degraded) || matches!(bridge, TransportState::Degraded) {
        TransportState::Degraded
    } else {
        TransportState::Ready
    }
}

#[derive(Debug)]
enum ClientTransportStartError {
    Primary(RestrictedProxyError),
    Dual(DualRelayError),
    BrowserBridge(RestrictedHttpConnectError),
    BaseStatusHandleUnavailable,
    BridgeStatusHandleUnavailable,
}

impl std::fmt::Display for ClientTransportStartError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Primary(error) => write!(formatter, "primary transport: {error:?}"),
            Self::Dual(error) => write!(formatter, "dual transport: {error:?}"),
            Self::BrowserBridge(error) => write!(formatter, "browser HTTP bridge: {error:?}"),
            Self::BaseStatusHandleUnavailable => formatter.write_str("base live status handle unavailable"),
            Self::BridgeStatusHandleUnavailable => formatter.write_str("browser bridge live status handle unavailable"),
        }
    }
}

fn start_client_transport(
    profile: &ClientConfigProfile,
) -> Result<(ClientTransportOwner, ClientTransportStatus), ClientTransportStartError> {
    let (base_owner, base_status, base_endpoint) = if let Some(fallback) = &profile.fallback_relay {
        let mut transport = build_dual_relay_transport(profile, fallback)
            .map_err(ClientTransportStartError::Dual)?;
        let endpoint = transport
            .start_proxy()
            .map_err(ClientTransportStartError::Dual)?;
        let status = transport
            .status_handle()
            .ok_or(ClientTransportStartError::BaseStatusHandleUnavailable)?;
        (
            ClientBaseTransportOwner::Dual(transport),
            ClientBaseTransportStatus::Dual(status),
            endpoint,
        )
    } else {
        let mut transport =
            build_primary_transport(profile).map_err(ClientTransportStartError::Primary)?;
        let endpoint = transport
            .start_proxy()
            .map_err(ClientTransportStartError::Primary)?;
        let status = transport
            .status_handle()
            .ok_or(ClientTransportStartError::BaseStatusHandleUnavailable)?;
        (
            ClientBaseTransportOwner::Primary(transport),
            ClientBaseTransportStatus::Primary(status),
            endpoint,
        )
    };

    let mut browser_bridge = RestrictedHttpConnectTransport::new(RestrictedHttpConnectConfig {
        name: "webgate-browser-http-connect".to_string(),
        upstream_socks5: base_endpoint,
        local_listen_port: 0,
        allowed_domains: profile.allowed_domains.clone(),
        allowed_ports: vec![443],
        connect_timeout: PRIMARY_PROXY_CONNECT_TIMEOUT,
        max_header_bytes: 16 * 1024,
    })
    .map_err(ClientTransportStartError::BrowserBridge)?;
    browser_bridge
        .start_proxy()
        .map_err(ClientTransportStartError::BrowserBridge)?;
    let bridge_status = browser_bridge
        .status_handle()
        .ok_or(ClientTransportStartError::BridgeStatusHandleUnavailable)?;

    let owner = ClientTransportOwner {
        browser_bridge,
        base: base_owner,
    };
    let status = ClientTransportStatus::Live {
        base: base_status,
        browser_bridge: bridge_status,
    };
    debug_assert!(matches!(
        owner.state(),
        TransportState::Ready | TransportState::Degraded
    ));
    debug_assert!(status.snapshot().1.is_some());
    Ok((owner, status))
}

'''
text = text[:start] + new_runtime + text[end:]
text = text.replace(
    'fn proxy_json(endpoint: Option<LocalProxyEndpoint>) -> String {',
    'fn proxy_json(endpoint: Option<HttpConnectProxyEndpoint>) -> String {',
)
# Test helper and synthetic endpoints use renderer-compatible HTTP type.
text = text.replace(
    'proxy: Option<LocalProxyEndpoint>,',
    'proxy: Option<HttpConnectProxyEndpoint>,',
)
text = text.replace(
    'LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43117).unwrap()',
    'HttpConnectProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43117).unwrap()',
)
main.write_text(text)

# Session manager receives only transport's HTTP CONNECT endpoint and validates
# conversion to the browser-owned HTTP type at the crate boundary.
session = Path('crates/webgate-app/src/session.rs')
text = session.read_text()
text = text.replace(
    'use webgate_browser::BrowserKind;\n',
    'use webgate_browser::{BrowserKind, HttpProxyEndpoint};\n',
)
text = text.replace(
    'use webgate_transport::{LocalProxyEndpoint, TransportState};',
    'use webgate_transport::{HttpConnectProxyEndpoint, TransportState};',
)
text = text.replace(
    'protected_proxy: Option<LocalProxyEndpoint>,',
    'protected_proxy: Option<HttpConnectProxyEndpoint>,',
)
old_attach = '''        transitions.push(ApplicationSessionState::StartingProtectedBrowser);
        if let Err(error) = capsule.attach_proxy(proxy.socket_addr()) {
            return self.insert_terminal(
                session_id,
                target_url,
                ApplicationSessionState::Failed,
                format!("protected browser proxy attachment failed: {error:?}"),
                transitions,
            );
        }

        if let Err(error) = capsule.start() {'''
new_attach = '''        transitions.push(ApplicationSessionState::StartingProtectedBrowser);
        let proxy_address = proxy.socket_addr();
        let browser_proxy = match HttpProxyEndpoint::new(proxy_address.ip(), proxy_address.port()) {
            Ok(endpoint) => endpoint,
            Err(error) => {
                return self.insert_terminal(
                    session_id,
                    target_url,
                    ApplicationSessionState::Failed,
                    format!("protected browser HTTP proxy conversion failed: {error:?}"),
                    transitions,
                );
            }
        };
        capsule.attach_proxy(browser_proxy);

        if let Err(error) = capsule.start() {'''
text = replace_once(text, old_attach, new_attach, 'session typed proxy attach')
text = text.replace(
    'fn test_proxy() -> LocalProxyEndpoint {\n        LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43117).unwrap()\n    }',
    'fn test_proxy() -> HttpConnectProxyEndpoint {\n        HttpConnectProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43117).unwrap()\n    }',
)
session.write_text(text)
