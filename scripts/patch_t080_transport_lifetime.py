#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-app/src/main.rs')
text = path.read_text()

def replace_once(old: str, new: str) -> None:
    global text
    count = text.count(old)
    assert count == 1, (old[:140], count)
    text = text.replace(old, new, 1)

replace_once(
'''use webgate_transport::dual_failover::{
    DualRelayConfig, DualRelayError, DualRelayFailoverTransport,
};''',
'''use webgate_transport::dual_failover::{
    DualRelayConfig, DualRelayError, DualRelayFailoverTransport, DualRelayStatusHandle,
};''')
replace_once(
'''use webgate_transport::restricted_socks5::{
    RestrictedProxyError, RestrictedSocks5Config, RestrictedSocks5Transport,
};''',
'''use webgate_transport::restricted_socks5::{
    RestrictedProxyError, RestrictedProxyStatusHandle, RestrictedSocks5Config,
    RestrictedSocks5Transport,
};''')

anchor = '''fn build_dual_relay_transport(
    profile: &ClientConfigProfile,
    fallback: &RelayEndpointConfig,
) -> Result<DualRelayFailoverTransport, DualRelayError> {
    DualRelayFailoverTransport::new(DualRelayConfig {
        name: format!("{} / {}", profile.primary_relay.name, fallback.name),
        primary_upstream_host: profile.primary_relay.address.clone(),
        primary_upstream_port: profile.primary_relay.port,
        fallback_upstream_host: fallback.address.clone(),
        fallback_upstream_port: fallback.port,
        local_listen_port: 0,
        allowed_domains: profile.allowed_domains.clone(),
        allowed_ports: vec![443],
        connect_timeout: PRIMARY_PROXY_CONNECT_TIMEOUT,
        failover_config: FailoverConfig::default(),
    })
}
'''
addition = anchor + '''
/// Owns the running transport listener/workers for exactly as long as the client
/// process needs the protected endpoint. Dropping this value revokes the endpoint.
#[derive(Debug)]
enum ClientTransportOwner {
    Primary(RestrictedSocks5Transport),
    Dual(DualRelayFailoverTransport),
}

impl ClientTransportOwner {
    #[must_use]
    fn state(&self) -> TransportState {
        match self {
            Self::Primary(transport) => transport.state(),
            Self::Dual(transport) => transport.state(),
        }
    }
}

/// Live transport truth source used by GUI, CLI and the session orchestrator.
/// A fixed snapshot is permitted only for fail-closed bootstrap failure/tests;
/// successful transports always use a live status handle tied to the owner state.
#[derive(Debug, Clone)]
enum ClientTransportStatus {
    Primary(RestrictedProxyStatusHandle),
    Dual(DualRelayStatusHandle),
    Fixed {
        state: TransportState,
        endpoint: Option<LocalProxyEndpoint>,
    },
}

impl ClientTransportStatus {
    #[must_use]
    fn snapshot(&self) -> (TransportState, Option<LocalProxyEndpoint>) {
        match self {
            Self::Primary(status) => status.snapshot(),
            Self::Dual(status) => {
                let (state, _role, endpoint, _primary_health, _fallback_health) = status.snapshot();
                (state, endpoint)
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
    const fn fixed(state: TransportState, endpoint: Option<LocalProxyEndpoint>) -> Self {
        Self::Fixed { state, endpoint }
    }
}

#[derive(Debug)]
enum ClientTransportStartError {
    Primary(RestrictedProxyError),
    Dual(DualRelayError),
    StatusHandleUnavailable,
}

fn start_client_transport(
    profile: &ClientConfigProfile,
) -> Result<(ClientTransportOwner, ClientTransportStatus), ClientTransportStartError> {
    if let Some(fallback) = &profile.fallback_relay {
        let mut transport =
            build_dual_relay_transport(profile, fallback).map_err(ClientTransportStartError::Dual)?;
        transport
            .start_proxy()
            .map_err(ClientTransportStartError::Dual)?;
        let status = transport
            .status_handle()
            .ok_or(ClientTransportStartError::StatusHandleUnavailable)?;
        let owner = ClientTransportOwner::Dual(transport);
        debug_assert!(matches!(
            owner.state(),
            TransportState::Ready | TransportState::Degraded
        ));
        Ok((owner, ClientTransportStatus::Dual(status)))
    } else {
        let mut transport =
            build_primary_transport(profile).map_err(ClientTransportStartError::Primary)?;
        transport
            .start_proxy()
            .map_err(ClientTransportStartError::Primary)?;
        let status = transport
            .status_handle()
            .ok_or(ClientTransportStartError::StatusHandleUnavailable)?;
        let owner = ClientTransportOwner::Primary(transport);
        debug_assert_eq!(owner.state(), TransportState::Ready);
        Ok((owner, ClientTransportStatus::Primary(status)))
    }
}
'''
replace_once(anchor, addition)

replace_once(
'''fn handle_client_stream(
    mut stream: TcpStream,
    profile_arc: &Arc<RwLock<ClientConfigProfile>>,
    session_manager: &Arc<Mutex<ApplicationSessionManager>>,
    keystore_id: &str,
    transport_state: TransportState,
    protected_proxy: Option<LocalProxyEndpoint>,
) {''',
'''fn handle_client_stream(
    mut stream: TcpStream,
    profile_arc: &Arc<RwLock<ClientConfigProfile>>,
    session_manager: &Arc<Mutex<ApplicationSessionManager>>,
    keystore_id: &str,
    transport_status: &ClientTransportStatus,
) {''')
replace_once(
'''    let method = parts[0];
    let path = parts[1];
''',
'''    let method = parts[0];
    let path = parts[1];
    // Never trust bootstrap-time readiness. Every request observes the current
    // transport state and endpoint from the live status handle.
    let (transport_state, protected_proxy) = transport_status.snapshot();
''')

old_bootstrap = '''    let (transport_state, proxy_ep) = if let Some(ref fallback) = profile.fallback_relay {
        match build_dual_relay_transport(&profile, fallback) {
            Ok(mut dual_transport) => {
                let ep = dual_transport.start_proxy().ok();
                let state = dual_transport.state();
                if ep.is_none() {
                    eprintln!(
                        "[Транспорт] Dual-relay upstreams не подтверждены; protected proxy остаётся OFFLINE."
                    );
                }
                (state, ep)
            }
            Err(error) => {
                eprintln!("[Транспорт] Некорректная конфигурация dual relay: {error:?}");
                (TransportState::Offline, None)
            }
        }
    } else {
        match build_primary_transport(&profile) {
            Ok(mut primary) => {
                let ep = primary.start_proxy().ok();
                let state = primary.state();
                if ep.is_none() {
                    eprintln!(
                        "[Транспорт] Primary sidecar не подтверждён; protected proxy остаётся OFFLINE."
                    );
                }
                (state, ep)
            }
            Err(error) => {
                eprintln!("[Транспорт] Некорректная политика primary proxy: {error:?}");
                (TransportState::Offline, None)
            }
        }
    };
'''
new_bootstrap = '''    // The owner is intentionally retained for the whole client lifetime. In the
    // previous implementation it was dropped inside this bootstrap expression,
    // which immediately stopped the listener while leaving a copied Ready state.
    let (_transport_owner, transport_status) = match start_client_transport(&profile) {
        Ok((owner, status)) => (Some(owner), status),
        Err(error) => {
            eprintln!(
                "[Транспорт] Protected transport failed to start and remains OFFLINE: {error:?}"
            );
            (None, ClientTransportStatus::offline())
        }
    };
    let (transport_state, proxy_ep) = transport_status.snapshot();
'''
replace_once(old_bootstrap, new_bootstrap)

# CLI must refresh immediately before application open rather than reuse bootstrap copy.
replace_once(
'''            let mut session_manager = ApplicationSessionManager::new();
            let snapshot = session_manager.open_application(
                &profile,
                &target_destination,
                transport_state,
                proxy_ep,
            );''',
'''            let mut session_manager = ApplicationSessionManager::new();
            let (live_transport_state, live_proxy_ep) = transport_status.snapshot();
            let snapshot = session_manager.open_application(
                &profile,
                &target_destination,
                live_transport_state,
                live_proxy_ep,
            );''')

# Server thread gets a cloneable live status handle, not frozen state/endpoint.
replace_once(
'''    let session_manager = Arc::new(Mutex::new(ApplicationSessionManager::new()));
    let session_manager_clone = Arc::clone(&session_manager);
    let dev_id_clone = device_id.clone();

    let server_handle = thread::spawn(move || {''',
'''    let session_manager = Arc::new(Mutex::new(ApplicationSessionManager::new()));
    let session_manager_clone = Arc::clone(&session_manager);
    let transport_status_clone = transport_status.clone();
    let dev_id_clone = device_id.clone();

    let server_handle = thread::spawn(move || {''')
replace_once(
'''                &session_manager_clone,
                &dev_id_clone,
                transport_state,
                proxy_ep,
            );''',
'''                &session_manager_clone,
                &dev_id_clone,
                &transport_status_clone,
            );''')

# Unit HTTP helper keeps existing convenient input but wraps it in a fixed test status.
replace_once(
'''        let session_manager_clone = Arc::clone(&session_manager);
        let request = request.to_string();

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                &session_manager_clone,
                "dev_test_123",
                transport_state,
                proxy,
            );
        });''',
'''        let session_manager_clone = Arc::clone(&session_manager);
        let transport_status = ClientTransportStatus::fixed(transport_state, proxy);
        let request = request.to_string();

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                &session_manager_clone,
                "dev_test_123",
                &transport_status,
            );
        });''')

# Add regression helpers/tests inside tests module.
test_anchor = '''    #[test]
    fn default_profile_is_used_only_when_no_config_was_requested() {'''
new_tests = r'''    fn spawn_probe_only_socks5_sidecar() -> (u16, thread::JoinHandle<()>) {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let port = listener.local_addr().unwrap().port();
        let handle = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut greeting = [0_u8; 3];
            stream.read_exact(&mut greeting).unwrap();
            assert_eq!(greeting, [0x05, 0x01, 0x00]);
            stream.write_all(&[0x05, 0x00]).unwrap();
        });
        (port, handle)
    }

    #[test]
    fn started_client_transport_owner_keeps_proxy_live_until_drop() {
        let (upstream_port, sidecar) = spawn_probe_only_socks5_sidecar();
        let mut profile = ClientConfigProfile::default();
        profile.primary_relay.address = "127.0.0.1".to_string();
        profile.primary_relay.port = upstream_port;
        profile.fallback_relay = None;

        let (owner, status) = start_client_transport(&profile).unwrap();
        let (state, endpoint) = status.snapshot();
        assert_eq!(state, TransportState::Ready);
        let endpoint = endpoint.unwrap();
        let live_connection = TcpStream::connect(endpoint.socket_addr());
        assert!(live_connection.is_ok());
        drop(live_connection);
        sidecar.join().unwrap();

        drop(owner);
        let (state_after_drop, endpoint_after_drop) = status.snapshot();
        assert_eq!(state_after_drop, TransportState::Stopped);
        assert_eq!(endpoint_after_drop, None);
    }

    #[test]
    fn fixed_offline_status_never_exposes_endpoint() {
        assert_eq!(
            ClientTransportStatus::offline().snapshot(),
            (TransportState::Offline, None)
        );
    }

''' + test_anchor
replace_once(test_anchor, new_tests)

path.write_text(text)
