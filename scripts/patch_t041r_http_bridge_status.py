#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-transport/src/restricted_http_connect.rs')
text = path.read_text()

anchor = '''#[derive(Debug, Clone)]
pub struct RestrictedHttpConnectConfig {
    pub name: String,
    pub upstream_socks5: LocalProxyEndpoint,
    pub local_listen_port: u16,
    pub allowed_domains: Vec<String>,
    pub allowed_ports: Vec<u16>,
    pub connect_timeout: Duration,
    pub max_header_bytes: usize,
}
'''
addition = anchor + '''
/// Read-only live view of a started restricted HTTP CONNECT bridge.
#[derive(Debug, Clone)]
pub struct RestrictedHttpConnectStatusHandle {
    state: Arc<AtomicU8>,
    endpoint: HttpConnectProxyEndpoint,
}

impl RestrictedHttpConnectStatusHandle {
    #[must_use]
    pub fn snapshot(&self) -> (TransportState, Option<HttpConnectProxyEndpoint>) {
        let state = decode_state(self.state.load(Ordering::Acquire));
        let endpoint = if matches!(state, TransportState::Ready | TransportState::Degraded) {
            Some(self.endpoint)
        } else {
            None
        };
        (state, endpoint)
    }
}
'''
assert text.count(anchor) == 1
text = text.replace(anchor, addition, 1)

anchor2 = '''    #[must_use]
    pub fn local_proxy(&self) -> Option<HttpConnectProxyEndpoint> {
        if matches!(
            self.state(),
            TransportState::Ready | TransportState::Degraded
        ) {
            self.local_endpoint
        } else {
            None
        }
    }

    pub fn stop(&mut self) {
'''
replacement2 = '''    #[must_use]
    pub fn local_proxy(&self) -> Option<HttpConnectProxyEndpoint> {
        if matches!(
            self.state(),
            TransportState::Ready | TransportState::Degraded
        ) {
            self.local_endpoint
        } else {
            None
        }
    }

    #[must_use]
    pub fn status_handle(&self) -> Option<RestrictedHttpConnectStatusHandle> {
        self.local_endpoint
            .map(|endpoint| RestrictedHttpConnectStatusHandle {
                state: Arc::clone(&self.state),
                endpoint,
            })
    }

    pub fn stop(&mut self) {
'''
assert text.count(anchor2) == 1
text = text.replace(anchor2, replacement2, 1)

needle = '''    #[test]
    fn unavailable_upstream_never_exposes_ready_endpoint() {'''
regression = '''    #[test]
    fn live_status_revokes_http_endpoint_after_owner_stop() {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let upstream_port = listener.local_addr().unwrap().port();
        let sidecar = thread::spawn(move || {
            let (mut probe, _) = listener.accept().unwrap();
            let mut greeting = [0_u8; 3];
            probe.read_exact(&mut greeting).unwrap();
            assert_eq!(greeting, [0x05, 0x01, 0x00]);
            probe.write_all(&[0x05, 0x00]).unwrap();
        });

        let mut bridge = RestrictedHttpConnectTransport::new(base_config(upstream_port)).unwrap();
        let endpoint = bridge.start_proxy().unwrap();
        sidecar.join().unwrap();
        let status = bridge.status_handle().unwrap();
        assert_eq!(
            status.snapshot(),
            (TransportState::Ready, Some(endpoint))
        );

        bridge.stop();
        assert_eq!(status.snapshot(), (TransportState::Stopped, None));
    }

''' + needle
assert text.count(needle) == 1
text = text.replace(needle, regression, 1)

path.write_text(text)
