#!/usr/bin/env python3
from pathlib import Path

module = r'''#![forbid(unsafe_code)]

use crate::socks5_proto::{
    DestinationPolicy, PolicyValidationError, STATE_OFFLINE, STATE_STOPPED, STREAM_POLL_TIMEOUT,
    SocksTarget, configure_handshake_timeouts, decode_state, encode_state, normalize_target_domain,
    probe_socks5_sidecar, read_upstream_reply, relay_bidirectional, socks5_negotiate_no_auth,
    write_connect_request,
};
use crate::{HttpConnectProxyEndpoint, LocalProxyEndpoint, TransportState};
use std::fmt;
use std::io::{self, ErrorKind, Read, Write};
use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

const MIN_HEADER_BYTES: usize = 256;
const MAX_HEADER_BYTES: usize = 64 * 1024;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RestrictedHttpConnectError {
    EmptyAllowedDomains,
    EmptyAllowedPorts,
    InvalidAllowedDomain,
    InvalidAllowedPort,
    InvalidConnectTimeout,
    InvalidMaxHeaderBytes,
    UpstreamNotLoopback,
    UpstreamUnavailable,
    UpstreamProtocol,
    ListenerBind,
    AlreadyStarted,
}

impl From<PolicyValidationError> for RestrictedHttpConnectError {
    fn from(error: PolicyValidationError) -> Self {
        match error {
            PolicyValidationError::EmptyAllowedDomains => Self::EmptyAllowedDomains,
            PolicyValidationError::EmptyAllowedPorts => Self::EmptyAllowedPorts,
            PolicyValidationError::InvalidAllowedDomain => Self::InvalidAllowedDomain,
            PolicyValidationError::InvalidAllowedPort => Self::InvalidAllowedPort,
        }
    }
}

#[derive(Debug, Clone)]
pub struct RestrictedHttpConnectConfig {
    pub name: String,
    pub upstream_socks5: LocalProxyEndpoint,
    pub local_listen_port: u16,
    pub allowed_domains: Vec<String>,
    pub allowed_ports: Vec<u16>,
    pub connect_timeout: Duration,
    pub max_header_bytes: usize,
}

pub struct RestrictedHttpConnectTransport {
    config: RestrictedHttpConnectConfig,
    policy: DestinationPolicy,
    state: Arc<AtomicU8>,
    local_endpoint: Option<HttpConnectProxyEndpoint>,
    shutdown: Arc<AtomicBool>,
    listener_handle: Option<JoinHandle<()>>,
    workers: Arc<Mutex<Vec<JoinHandle<()>>>>,
}

impl fmt::Debug for RestrictedHttpConnectTransport {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("RestrictedHttpConnectTransport")
            .field("name", &self.config.name)
            .field("state", &self.state())
            .field("local_endpoint", &self.local_endpoint)
            .finish_non_exhaustive()
    }
}

impl RestrictedHttpConnectTransport {
    pub fn new(config: RestrictedHttpConnectConfig) -> Result<Self, RestrictedHttpConnectError> {
        let policy = DestinationPolicy::new(&config.allowed_domains, &config.allowed_ports)
            .map_err(RestrictedHttpConnectError::from)?;
        if config.connect_timeout.is_zero() {
            return Err(RestrictedHttpConnectError::InvalidConnectTimeout);
        }
        if !(MIN_HEADER_BYTES..=MAX_HEADER_BYTES).contains(&config.max_header_bytes) {
            return Err(RestrictedHttpConnectError::InvalidMaxHeaderBytes);
        }
        let upstream = config.upstream_socks5.socket_addr();
        if !upstream.ip().is_loopback() || upstream.port() == 0 {
            return Err(RestrictedHttpConnectError::UpstreamNotLoopback);
        }

        Ok(Self {
            config,
            policy,
            state: Arc::new(AtomicU8::new(STATE_STOPPED)),
            local_endpoint: None,
            shutdown: Arc::new(AtomicBool::new(false)),
            listener_handle: None,
            workers: Arc::new(Mutex::new(Vec::new())),
        })
    }

    pub fn start_proxy(
        &mut self,
    ) -> Result<HttpConnectProxyEndpoint, RestrictedHttpConnectError> {
        if self.listener_handle.is_some()
            || matches!(self.state(), TransportState::Starting | TransportState::Ready)
        {
            return Err(RestrictedHttpConnectError::AlreadyStarted);
        }

        self.local_endpoint = None;
        self.shutdown.store(false, Ordering::Release);
        self.set_state(TransportState::Starting);
        let upstream = self.config.upstream_socks5.socket_addr();
        if let Err(error) = probe_socks5_sidecar(upstream, self.config.connect_timeout) {
            self.set_state(TransportState::Offline);
            return Err(match error.kind() {
                ErrorKind::InvalidData => RestrictedHttpConnectError::UpstreamProtocol,
                _ => RestrictedHttpConnectError::UpstreamUnavailable,
            });
        }

        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, self.config.local_listen_port))
            .map_err(|_| {
                self.set_state(TransportState::Offline);
                RestrictedHttpConnectError::ListenerBind
            })?;
        let bound = listener.local_addr().map_err(|_| {
            self.set_state(TransportState::Offline);
            RestrictedHttpConnectError::ListenerBind
        })?;
        let endpoint = HttpConnectProxyEndpoint::new(bound.ip(), bound.port()).map_err(|_| {
            self.set_state(TransportState::Offline);
            RestrictedHttpConnectError::ListenerBind
        })?;

        let shutdown = Arc::clone(&self.shutdown);
        let state = Arc::clone(&self.state);
        let workers = Arc::clone(&self.workers);
        let policy = self.policy.clone();
        let timeout = self.config.connect_timeout;
        let max_header_bytes = self.config.max_header_bytes;

        let handle = thread::spawn(move || loop {
            match listener.accept() {
                Ok((stream, _)) => {
                    if shutdown.load(Ordering::Acquire) {
                        break;
                    }
                    let worker_shutdown = Arc::clone(&shutdown);
                    let worker_state = Arc::clone(&state);
                    let worker_policy = policy.clone();
                    let worker = thread::spawn(move || {
                        let _ = handle_client(
                            stream,
                            upstream,
                            timeout,
                            max_header_bytes,
                            &worker_policy,
                            &worker_shutdown,
                            &worker_state,
                        );
                    });
                    match workers.lock() {
                        Ok(mut handles) => handles.push(worker),
                        Err(poisoned) => {
                            state.store(STATE_OFFLINE, Ordering::Release);
                            let mut handles = poisoned.into_inner();
                            handles.push(worker);
                            break;
                        }
                    }
                }
                Err(_) => {
                    if !shutdown.load(Ordering::Acquire) {
                        state.store(STATE_OFFLINE, Ordering::Release);
                    }
                    break;
                }
            }
        });

        self.listener_handle = Some(handle);
        self.local_endpoint = Some(endpoint);
        self.set_state(TransportState::Ready);
        Ok(endpoint)
    }

    #[must_use]
    pub fn state(&self) -> TransportState {
        decode_state(self.state.load(Ordering::Acquire))
    }

    #[must_use]
    pub fn local_proxy(&self) -> Option<HttpConnectProxyEndpoint> {
        if matches!(self.state(), TransportState::Ready | TransportState::Degraded) {
            self.local_endpoint
        } else {
            None
        }
    }

    pub fn stop(&mut self) {
        self.stop_internal();
    }

    fn set_state(&self, state: TransportState) {
        self.state.store(encode_state(state), Ordering::Release);
    }

    fn stop_internal(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(endpoint) = self.local_endpoint {
            let _ = TcpStream::connect_timeout(&endpoint.socket_addr(), Duration::from_millis(100));
        }
        if let Some(handle) = self.listener_handle.take() {
            let _ = handle.join();
        }
        let mut workers = match self.workers.lock() {
            Ok(workers) => workers,
            Err(poisoned) => poisoned.into_inner(),
        };
        for worker in workers.drain(..) {
            let _ = worker.join();
        }
        drop(workers);
        self.local_endpoint = None;
        self.set_state(TransportState::Stopped);
    }
}

impl Drop for RestrictedHttpConnectTransport {
    fn drop(&mut self) {
        self.stop_internal();
    }
}

#[derive(Debug, PartialEq, Eq)]
enum RequestError {
    TooLarge,
    Malformed,
    MethodNotAllowed,
}

fn handle_client(
    mut client: TcpStream,
    upstream_address: SocketAddr,
    connect_timeout: Duration,
    max_header_bytes: usize,
    policy: &DestinationPolicy,
    shutdown: &Arc<AtomicBool>,
    state: &Arc<AtomicU8>,
) -> io::Result<()> {
    configure_handshake_timeouts(&client, connect_timeout)?;
    if decode_state(state.load(Ordering::Acquire)) != TransportState::Ready {
        write_http_error(&mut client, 503, "Service Unavailable")?;
        return Ok(());
    }

    let (head, buffered_tunnel_bytes) = match read_http_head(&mut client, max_header_bytes) {
        Ok(value) => value,
        Err(RequestError::TooLarge) => {
            write_http_error(&mut client, 431, "Request Header Fields Too Large")?;
            return Ok(());
        }
        Err(RequestError::MethodNotAllowed) => {
            write_http_error(&mut client, 405, "Method Not Allowed")?;
            return Ok(());
        }
        Err(RequestError::Malformed) => {
            write_http_error(&mut client, 400, "Bad Request")?;
            return Ok(());
        }
    };

    let (target, port) = match parse_connect_request(&head) {
        Ok(value) => value,
        Err(RequestError::MethodNotAllowed) => {
            write_http_error(&mut client, 405, "Method Not Allowed")?;
            return Ok(());
        }
        Err(_) => {
            write_http_error(&mut client, 400, "Bad Request")?;
            return Ok(());
        }
    };
    if !policy.allows(&target, port) {
        write_http_error(&mut client, 403, "Forbidden")?;
        return Ok(());
    }

    let mut upstream = match TcpStream::connect_timeout(&upstream_address, connect_timeout) {
        Ok(stream) => stream,
        Err(_) => {
            state.store(STATE_OFFLINE, Ordering::Release);
            write_http_error(&mut client, 502, "Bad Gateway")?;
            return Ok(());
        }
    };
    if configure_handshake_timeouts(&upstream, connect_timeout).is_err()
        || socks5_negotiate_no_auth(&mut upstream).is_err()
    {
        state.store(STATE_OFFLINE, Ordering::Release);
        write_http_error(&mut client, 502, "Bad Gateway")?;
        return Ok(());
    }
    if write_connect_request(&mut upstream, &target, port).is_err() {
        write_http_error(&mut client, 502, "Bad Gateway")?;
        return Ok(());
    }
    let (reply_code, _) = match read_upstream_reply(&mut upstream) {
        Ok(reply) => reply,
        Err(_) => {
            state.store(STATE_OFFLINE, Ordering::Release);
            write_http_error(&mut client, 502, "Bad Gateway")?;
            return Ok(());
        }
    };
    if reply_code != 0x00 {
        write_http_error(&mut client, 502, "Bad Gateway")?;
        return Ok(());
    }

    client.write_all(b"HTTP/1.1 200 Connection Established\r\n\r\n")?;
    if !buffered_tunnel_bytes.is_empty() {
        upstream.write_all(&buffered_tunnel_bytes)?;
    }

    client.set_read_timeout(Some(STREAM_POLL_TIMEOUT))?;
    client.set_write_timeout(Some(STREAM_POLL_TIMEOUT))?;
    upstream.set_read_timeout(Some(STREAM_POLL_TIMEOUT))?;
    upstream.set_write_timeout(Some(STREAM_POLL_TIMEOUT))?;
    relay_bidirectional(client, upstream, shutdown)
}

fn read_http_head(
    stream: &mut TcpStream,
    max_header_bytes: usize,
) -> Result<(Vec<u8>, Vec<u8>), RequestError> {
    let mut buffer = Vec::with_capacity(max_header_bytes.min(4096));
    let mut chunk = [0_u8; 1024];
    loop {
        if let Some(end) = find_header_end(&buffer) {
            let tail = buffer.split_off(end);
            return Ok((buffer, tail));
        }
        if buffer.len() >= max_header_bytes {
            return Err(RequestError::TooLarge);
        }
        let remaining = max_header_bytes.saturating_sub(buffer.len());
        let read_len = remaining.min(chunk.len());
        let count = stream
            .read(&mut chunk[..read_len])
            .map_err(|_| RequestError::Malformed)?;
        if count == 0 {
            return Err(RequestError::Malformed);
        }
        buffer.extend_from_slice(&chunk[..count]);
    }
}

fn find_header_end(bytes: &[u8]) -> Option<usize> {
    bytes.windows(4).position(|window| window == b"\r\n\r\n").map(|index| index + 4)
}

fn parse_connect_request(head: &[u8]) -> Result<(SocksTarget, u16), RequestError> {
    let text = std::str::from_utf8(head).map_err(|_| RequestError::Malformed)?;
    if text.contains("\r\n ") || text.contains("\r\n\t") {
        return Err(RequestError::Malformed);
    }
    let request_line = text.split("\r\n").next().ok_or(RequestError::Malformed)?;
    let mut parts = request_line.split_ascii_whitespace();
    let method = parts.next().ok_or(RequestError::Malformed)?;
    if method != "CONNECT" {
        return Err(RequestError::MethodNotAllowed);
    }
    let authority = parts.next().ok_or(RequestError::Malformed)?;
    let version = parts.next().ok_or(RequestError::Malformed)?;
    if parts.next().is_some() || version != "HTTP/1.1" {
        return Err(RequestError::Malformed);
    }
    parse_authority(authority)
}

fn parse_authority(authority: &str) -> Result<(SocksTarget, u16), RequestError> {
    if authority.is_empty()
        || authority.contains(['/', '?', '#', '@'])
        || authority.chars().any(char::is_control)
    {
        return Err(RequestError::Malformed);
    }

    let (host, port_text) = if let Some(rest) = authority.strip_prefix('[') {
        let close = rest.find(']').ok_or(RequestError::Malformed)?;
        let host = &rest[..close];
        let suffix = &rest[close + 1..];
        let port = suffix.strip_prefix(':').ok_or(RequestError::Malformed)?;
        if host.is_empty() || port.is_empty() || port.contains(':') {
            return Err(RequestError::Malformed);
        }
        (host, port)
    } else {
        let (host, port) = authority.rsplit_once(':').ok_or(RequestError::Malformed)?;
        if host.is_empty() || host.contains(':') || port.is_empty() {
            return Err(RequestError::Malformed);
        }
        (host, port)
    };

    let port = port_text.parse::<u16>().map_err(|_| RequestError::Malformed)?;
    if port == 0 {
        return Err(RequestError::Malformed);
    }
    if let Ok(ip) = host.parse::<IpAddr>() {
        return Ok((SocksTarget::Ip(ip), port));
    }
    let normalized = normalize_target_domain(host).ok_or(RequestError::Malformed)?;
    Ok((SocksTarget::Domain(normalized), port))
}

fn write_http_error(stream: &mut TcpStream, code: u16, reason: &str) -> io::Result<()> {
    write!(
        stream,
        "HTTP/1.1 {code} {reason}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
    )
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use std::sync::mpsc;

    fn loopback_endpoint(port: u16) -> LocalProxyEndpoint {
        LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), port).unwrap()
    }

    fn base_config(upstream_port: u16) -> RestrictedHttpConnectConfig {
        RestrictedHttpConnectConfig {
            name: "servo-http-bridge".to_string(),
            upstream_socks5: loopback_endpoint(upstream_port),
            local_listen_port: 0,
            allowed_domains: vec!["app.internal".to_string()],
            allowed_ports: vec![443],
            connect_timeout: Duration::from_secs(1),
            max_header_bytes: 4096,
        }
    }

    fn spawn_mock_socks5() -> (u16, mpsc::Receiver<(String, u16)>, JoinHandle<()>) {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let port = listener.local_addr().unwrap().port();
        let (tx, rx) = mpsc::channel();
        let handle = thread::spawn(move || {
            // startup probe
            let (mut probe, _) = listener.accept().unwrap();
            let mut greeting = [0_u8; 3];
            probe.read_exact(&mut greeting).unwrap();
            assert_eq!(greeting, [0x05, 0x01, 0x00]);
            probe.write_all(&[0x05, 0x00]).unwrap();
            drop(probe);

            // CONNECT tunnel
            let (mut stream, _) = listener.accept().unwrap();
            stream.read_exact(&mut greeting).unwrap();
            stream.write_all(&[0x05, 0x00]).unwrap();
            let mut head = [0_u8; 4];
            stream.read_exact(&mut head).unwrap();
            assert_eq!(head, [0x05, 0x01, 0x00, 0x03]);
            let mut length = [0_u8; 1];
            stream.read_exact(&mut length).unwrap();
            let mut domain = vec![0_u8; usize::from(length[0])];
            stream.read_exact(&mut domain).unwrap();
            let mut port_bytes = [0_u8; 2];
            stream.read_exact(&mut port_bytes).unwrap();
            let target_port = u16::from_be_bytes(port_bytes);
            tx.send((String::from_utf8(domain).unwrap(), target_port)).unwrap();
            stream
                .write_all(&[0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 1])
                .unwrap();

            let mut payload = [0_u8; 4];
            stream.read_exact(&mut payload).unwrap();
            assert_eq!(&payload, b"ping");
            stream.write_all(b"pong").unwrap();
        });
        (port, rx, handle)
    }

    #[test]
    fn constructor_rejects_invalid_policy_and_header_limit() {
        let mut config = base_config(9);
        config.allowed_domains.clear();
        assert!(matches!(
            RestrictedHttpConnectTransport::new(config),
            Err(RestrictedHttpConnectError::EmptyAllowedDomains)
        ));

        let mut config = base_config(9);
        config.max_header_bytes = 64;
        assert!(matches!(
            RestrictedHttpConnectTransport::new(config),
            Err(RestrictedHttpConnectError::InvalidMaxHeaderBytes)
        ));
    }

    #[test]
    fn parser_rejects_non_connect_malformed_authority_and_obs_fold() {
        assert_eq!(
            parse_connect_request(b"GET https://app.internal/ HTTP/1.1\r\n\r\n"),
            Err(RequestError::MethodNotAllowed)
        );
        assert_eq!(
            parse_connect_request(b"CONNECT user@app.internal:443 HTTP/1.1\r\n\r\n"),
            Err(RequestError::Malformed)
        );
        assert_eq!(
            parse_connect_request(b"CONNECT app.internal:443 HTTP/1.1\r\n folded\r\n\r\n"),
            Err(RequestError::Malformed)
        );
    }

    #[test]
    fn policy_rejects_unlisted_target_and_port() {
        let policy = DestinationPolicy::new(&["app.internal".to_string()], &[443]).unwrap();
        let (target, port) = parse_authority("evil.internal:443").unwrap();
        assert!(!policy.allows(&target, port));
        let (target, port) = parse_authority("app.internal:8443").unwrap();
        assert!(!policy.allows(&target, port));
    }

    #[test]
    fn allowed_connect_preserves_domain_and_tunnels_bytes() {
        let (upstream_port, observed, upstream_handle) = spawn_mock_socks5();
        let mut bridge = RestrictedHttpConnectTransport::new(base_config(upstream_port)).unwrap();
        let endpoint = bridge.start_proxy().unwrap();
        assert_eq!(bridge.state(), TransportState::Ready);
        assert!(endpoint.socket_addr().ip().is_loopback());
        assert!(endpoint.proxy_uri().starts_with("http://127.0.0.1:"));

        let mut client = TcpStream::connect(endpoint.socket_addr()).unwrap();
        client
            .write_all(b"CONNECT app.internal:443 HTTP/1.1\r\nHost: app.internal:443\r\n\r\nping")
            .unwrap();
        let mut response = Vec::new();
        let mut byte = [0_u8; 1];
        while !response.ends_with(b"\r\n\r\n") {
            client.read_exact(&mut byte).unwrap();
            response.push(byte[0]);
        }
        assert!(response.starts_with(b"HTTP/1.1 200 Connection Established"));
        let mut pong = [0_u8; 4];
        client.read_exact(&mut pong).unwrap();
        assert_eq!(&pong, b"pong");
        assert_eq!(observed.recv_timeout(Duration::from_secs(1)).unwrap(), ("app.internal".to_string(), 443));

        bridge.stop();
        assert_eq!(bridge.state(), TransportState::Stopped);
        assert!(bridge.local_proxy().is_none());
        upstream_handle.join().unwrap();
    }

    #[test]
    fn malformed_and_denied_requests_do_not_poison_global_ready_state() {
        let (upstream_port, _observed, upstream_handle) = spawn_mock_socks5();
        let mut bridge = RestrictedHttpConnectTransport::new(base_config(upstream_port)).unwrap();
        let endpoint = bridge.start_proxy().unwrap();

        let mut denied = TcpStream::connect(endpoint.socket_addr()).unwrap();
        denied
            .write_all(b"CONNECT evil.internal:443 HTTP/1.1\r\nHost: evil.internal\r\n\r\n")
            .unwrap();
        let mut denied_response = [0_u8; 64];
        let count = denied.read(&mut denied_response).unwrap();
        assert!(std::str::from_utf8(&denied_response[..count]).unwrap().contains("403 Forbidden"));
        assert_eq!(bridge.state(), TransportState::Ready);

        let mut malformed = TcpStream::connect(endpoint.socket_addr()).unwrap();
        malformed.write_all(b"GET / HTTP/1.1\r\n\r\n").unwrap();
        let count = malformed.read(&mut denied_response).unwrap();
        assert!(std::str::from_utf8(&denied_response[..count]).unwrap().contains("405 Method Not Allowed"));
        assert_eq!(bridge.state(), TransportState::Ready);

        // Complete the mock's second expected connection so the helper thread can exit.
        let mut allowed = TcpStream::connect(endpoint.socket_addr()).unwrap();
        allowed.write_all(b"CONNECT app.internal:443 HTTP/1.1\r\n\r\nping").unwrap();
        let mut all = Vec::new();
        allowed.read_to_end(&mut all).unwrap();
        assert!(all.windows(4).any(|window| window == b"pong"));

        bridge.stop();
        upstream_handle.join().unwrap();
    }

    #[test]
    fn unavailable_upstream_never_exposes_ready_endpoint() {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let port = listener.local_addr().unwrap().port();
        drop(listener);
        let mut bridge = RestrictedHttpConnectTransport::new(base_config(port)).unwrap();
        assert!(matches!(
            bridge.start_proxy(),
            Err(RestrictedHttpConnectError::UpstreamUnavailable)
        ));
        assert_eq!(bridge.state(), TransportState::Offline);
        assert!(bridge.local_proxy().is_none());
    }
}
'''

Path('crates/webgate-transport/src/restricted_http_connect.rs').write_text(module)

lib_path = Path('crates/webgate-transport/src/lib.rs')
text = lib_path.read_text()
old = 'pub mod relay;\npub mod restricted_socks5;\npub(crate) mod socks5_proto;\n'
new = 'pub mod relay;\npub mod restricted_http_connect;\npub mod restricted_socks5;\npub(crate) mod socks5_proto;\n'
assert text.count(old) == 1
text = text.replace(old, new, 1)

anchor = '''impl LocalProxyEndpoint {
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
'''
addition = anchor + '''
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
'''
assert text.count(anchor) == 1
text = text.replace(anchor, addition, 1)

text = text.replace(
    '    use super::{EndpointError, LocalProxyEndpoint};',
    '    use super::{EndpointError, HttpConnectProxyEndpoint, LocalProxyEndpoint};',
    1,
)
insert_test = '''
    #[test]
    fn http_connect_endpoint_is_typed_and_loopback_only() {
        let address = IpAddr::V4(Ipv4Addr::LOCALHOST);
        let endpoint = HttpConnectProxyEndpoint::new(address, 43120).unwrap();
        assert_eq!(endpoint.socket_addr(), SocketAddr::new(address, 43120));
        assert_eq!(endpoint.proxy_uri(), "http://127.0.0.1:43120");
        assert_eq!(
            HttpConnectProxyEndpoint::new(IpAddr::V4(Ipv4Addr::new(10, 0, 0, 8)), 43120),
            Err(EndpointError::NotLoopback)
        );
    }
'''
marker = '\n    #[test]\n    fn endpoint_rejects_zero_port() {'
assert text.count(marker) == 1
text = text.replace(marker, insert_test + marker, 1)
lib_path.write_text(text)
