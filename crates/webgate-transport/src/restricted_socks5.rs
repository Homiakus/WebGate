#![forbid(unsafe_code)]

use crate::{LocalProxyEndpoint, TransportProvider, TransportState};
use std::fmt;
use std::io::{self, ErrorKind, Read, Write};
use std::net::{IpAddr, Ipv4Addr, Shutdown, SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

const STATE_STOPPED: u8 = 0;
const STATE_STARTING: u8 = 1;
const STATE_READY: u8 = 2;
const STATE_DEGRADED: u8 = 3;
const STATE_OFFLINE: u8 = 4;
const STREAM_POLL_TIMEOUT: Duration = Duration::from_millis(200);

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RestrictedProxyError {
    EmptyAllowedDomains,
    EmptyAllowedPorts,
    InvalidAllowedDomain,
    InvalidAllowedPort,
    InvalidConnectTimeout,
    UpstreamNotLoopback,
    UpstreamUnavailable,
    UpstreamProtocol,
    ListenerBind,
    AlreadyStarted,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RestrictedSocks5Config {
    pub name: String,
    pub upstream_host: String,
    pub upstream_port: u16,
    pub local_listen_port: u16,
    pub allowed_domains: Vec<String>,
    pub allowed_ports: Vec<u16>,
    pub connect_timeout: Duration,
}

#[derive(Debug, Clone)]
struct DestinationPolicy {
    allowed_domains: Vec<String>,
    allowed_ports: Vec<u16>,
}

impl DestinationPolicy {
    fn from_config(config: &RestrictedSocks5Config) -> Result<Self, RestrictedProxyError> {
        if config.allowed_domains.is_empty() {
            return Err(RestrictedProxyError::EmptyAllowedDomains);
        }
        if config.allowed_ports.is_empty() {
            return Err(RestrictedProxyError::EmptyAllowedPorts);
        }
        if config.allowed_ports.contains(&0) {
            return Err(RestrictedProxyError::InvalidAllowedPort);
        }

        let mut allowed_domains = Vec::with_capacity(config.allowed_domains.len());
        for raw in &config.allowed_domains {
            let normalized = raw.trim().to_ascii_lowercase();
            let invalid = normalized.is_empty()
                || normalized == "*"
                || normalized.chars().any(char::is_whitespace)
                || normalized.chars().any(char::is_control)
                || normalized
                    .strip_prefix("*.")
                    .is_some_and(|suffix| suffix.is_empty() || suffix.contains('*'))
                || (!normalized.starts_with("*.") && normalized.contains('*'));
            if invalid {
                return Err(RestrictedProxyError::InvalidAllowedDomain);
            }
            allowed_domains.push(normalized);
        }

        let mut allowed_ports = config.allowed_ports.clone();
        allowed_ports.sort_unstable();
        allowed_ports.dedup();

        Ok(Self {
            allowed_domains,
            allowed_ports,
        })
    }

    fn allows(&self, target: &SocksTarget, port: u16) -> bool {
        if self.allowed_ports.binary_search(&port).is_err() {
            return false;
        }

        match target {
            SocksTarget::Domain(host) => {
                let normalized = normalize_target_domain(host);
                let Some(host) = normalized else {
                    return false;
                };
                let domain_is_ip_literal = host.parse::<IpAddr>().is_ok();
                self.allowed_domains.iter().any(|allowed| {
                    if let Some(suffix) = allowed.strip_prefix("*.") {
                        !domain_is_ip_literal
                            && host.len() > suffix.len() + 1
                            && host.ends_with(suffix)
                            && host.as_bytes().get(host.len() - suffix.len() - 1) == Some(&b'.')
                    } else {
                        host == *allowed
                    }
                })
            }
            SocksTarget::Ip(ip) => {
                let target = ip.to_string().to_ascii_lowercase();
                self.allowed_domains
                    .iter()
                    .any(|allowed| allowed == &target)
            }
        }
    }
}

#[derive(Debug, Clone)]
enum SocksTarget {
    Domain(String),
    Ip(IpAddr),
}

fn normalize_target_domain(raw: &str) -> Option<String> {
    if raw.is_empty() || !raw.is_ascii() || raw.chars().any(char::is_control) {
        return None;
    }
    let normalized = raw.trim_end_matches('.').to_ascii_lowercase();
    if normalized.is_empty()
        || normalized.len() > 253
        || normalized.split('.').any(|label| {
            label.is_empty()
                || label.len() > 63
                || label.starts_with('-')
                || label.ends_with('-')
                || !label
                    .bytes()
                    .all(|byte| byte.is_ascii_alphanumeric() || byte == b'-')
        })
    {
        return None;
    }
    Some(normalized)
}

fn encode_state(state: TransportState) -> u8 {
    match state {
        TransportState::Stopped => STATE_STOPPED,
        TransportState::Starting => STATE_STARTING,
        TransportState::Ready => STATE_READY,
        TransportState::Degraded => STATE_DEGRADED,
        TransportState::Offline => STATE_OFFLINE,
    }
}

fn decode_state(state: u8) -> TransportState {
    match state {
        STATE_STOPPED => TransportState::Stopped,
        STATE_STARTING => TransportState::Starting,
        STATE_READY => TransportState::Ready,
        STATE_DEGRADED => TransportState::Degraded,
        _ => TransportState::Offline,
    }
}

pub struct RestrictedSocks5Transport {
    config: RestrictedSocks5Config,
    policy: DestinationPolicy,
    state: Arc<AtomicU8>,
    local_endpoint: Option<LocalProxyEndpoint>,
    shutdown: Arc<AtomicBool>,
    listener_handle: Option<JoinHandle<()>>,
    workers: Arc<Mutex<Vec<JoinHandle<()>>>>,
}

impl fmt::Debug for RestrictedSocks5Transport {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("RestrictedSocks5Transport")
            .field("name", &self.config.name)
            .field("state", &self.state())
            .field("local_endpoint", &self.local_endpoint)
            .finish_non_exhaustive()
    }
}

impl RestrictedSocks5Transport {
    pub fn new(config: RestrictedSocks5Config) -> Result<Self, RestrictedProxyError> {
        let policy = DestinationPolicy::from_config(&config)?;
        if config.connect_timeout.is_zero() {
            return Err(RestrictedProxyError::InvalidConnectTimeout);
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

    pub fn start_proxy(&mut self) -> Result<LocalProxyEndpoint, RestrictedProxyError> {
        if self.listener_handle.is_some()
            || matches!(
                self.state(),
                TransportState::Starting | TransportState::Ready
            )
        {
            return Err(RestrictedProxyError::AlreadyStarted);
        }

        self.local_endpoint = None;
        self.shutdown.store(false, Ordering::Release);
        self.set_state(TransportState::Starting);

        let upstream = match self.literal_loopback_upstream() {
            Ok(upstream) => upstream,
            Err(error) => {
                self.set_state(TransportState::Offline);
                return Err(error);
            }
        };
        if let Err(error) = probe_socks5_sidecar(upstream, self.config.connect_timeout) {
            self.set_state(TransportState::Offline);
            return Err(error);
        }

        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, self.config.local_listen_port))
            .map_err(|_| {
                self.set_state(TransportState::Offline);
                RestrictedProxyError::ListenerBind
            })?;
        let bound = listener.local_addr().map_err(|_| {
            self.set_state(TransportState::Offline);
            RestrictedProxyError::ListenerBind
        })?;
        let endpoint = LocalProxyEndpoint::new(bound.ip(), bound.port()).map_err(|_| {
            self.set_state(TransportState::Offline);
            RestrictedProxyError::ListenerBind
        })?;

        let shutdown = Arc::clone(&self.shutdown);
        let state = Arc::clone(&self.state);
        let workers = Arc::clone(&self.workers);
        let policy = self.policy.clone();
        let connect_timeout = self.config.connect_timeout;

        let handle = thread::spawn(move || loop {
            let accepted = listener.accept();
            match accepted {
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
                            connect_timeout,
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

    fn literal_loopback_upstream(&self) -> Result<SocketAddr, RestrictedProxyError> {
        let ip = self
            .config
            .upstream_host
            .parse::<IpAddr>()
            .map_err(|_| RestrictedProxyError::UpstreamNotLoopback)?;
        if !ip.is_loopback() || self.config.upstream_port == 0 {
            return Err(RestrictedProxyError::UpstreamNotLoopback);
        }
        Ok(SocketAddr::new(ip, self.config.upstream_port))
    }

    fn set_state(&self, state: TransportState) {
        self.state.store(encode_state(state), Ordering::Release);
    }

    fn stop_internal(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(endpoint) = self.local_endpoint {
            let _ =
                TcpStream::connect_timeout(&endpoint.socket_addr(), Duration::from_millis(100));
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

impl TransportProvider for RestrictedSocks5Transport {
    fn name(&self) -> &str {
        &self.config.name
    }

    fn state(&self) -> TransportState {
        decode_state(self.state.load(Ordering::Acquire))
    }

    fn local_proxy(&self) -> Option<LocalProxyEndpoint> {
        if matches!(
            self.state(),
            TransportState::Ready | TransportState::Degraded
        ) {
            self.local_endpoint
        } else {
            None
        }
    }

    fn stop(&mut self) {
        self.stop_internal();
    }
}

impl Drop for RestrictedSocks5Transport {
    fn drop(&mut self) {
        self.stop_internal();
    }
}

fn probe_socks5_sidecar(
    upstream: SocketAddr,
    timeout: Duration,
) -> Result<(), RestrictedProxyError> {
    let mut stream = TcpStream::connect_timeout(&upstream, timeout)
        .map_err(|_| RestrictedProxyError::UpstreamUnavailable)?;
    configure_handshake_timeouts(&stream, timeout)
        .map_err(|_| RestrictedProxyError::UpstreamUnavailable)?;
    socks5_negotiate_no_auth(&mut stream).map_err(|_| RestrictedProxyError::UpstreamProtocol)
}

fn configure_handshake_timeouts(stream: &TcpStream, timeout: Duration) -> io::Result<()> {
    stream.set_read_timeout(Some(timeout))?;
    stream.set_write_timeout(Some(timeout))
}

fn socks5_negotiate_no_auth(stream: &mut TcpStream) -> io::Result<()> {
    stream.write_all(&[0x05, 0x01, 0x00])?;
    let mut response = [0_u8; 2];
    stream.read_exact(&mut response)?;
    if response != [0x05, 0x00] {
        return Err(io::Error::new(
            ErrorKind::InvalidData,
            "SOCKS5 sidecar did not accept no-authentication method",
        ));
    }
    Ok(())
}

fn handle_client(
    mut client: TcpStream,
    upstream_address: SocketAddr,
    connect_timeout: Duration,
    policy: &DestinationPolicy,
    shutdown: &Arc<AtomicBool>,
    state: &Arc<AtomicU8>,
) -> io::Result<()> {
    configure_handshake_timeouts(&client, connect_timeout)?;
    if !negotiate_local_client(&mut client)? {
        return Ok(());
    }

    if decode_state(state.load(Ordering::Acquire)) != TransportState::Ready {
        write_reply(&mut client, 0x01)?;
        return Ok(());
    }

    let mut header = [0_u8; 4];
    client.read_exact(&mut header)?;
    if header[0] != 0x05 || header[2] != 0x00 {
        write_reply(&mut client, 0x01)?;
        return Ok(());
    }
    if header[1] != 0x01 {
        write_reply(&mut client, 0x07)?;
        return Ok(());
    }

    let target = match read_target(&mut client, header[3]) {
        Ok(target) => target,
        Err(_) => {
            write_reply(&mut client, 0x08)?;
            return Ok(());
        }
    };
    let mut port = [0_u8; 2];
    client.read_exact(&mut port)?;
    let port = u16::from_be_bytes(port);

    if !policy.allows(&target, port) {
        write_reply(&mut client, 0x02)?;
        return Ok(());
    }

    let mut upstream = match TcpStream::connect_timeout(&upstream_address, connect_timeout) {
        Ok(stream) => stream,
        Err(_) => {
            state.store(STATE_OFFLINE, Ordering::Release);
            write_reply(&mut client, 0x01)?;
            return Ok(());
        }
    };
    if configure_handshake_timeouts(&upstream, connect_timeout).is_err()
        || socks5_negotiate_no_auth(&mut upstream).is_err()
    {
        state.store(STATE_OFFLINE, Ordering::Release);
        write_reply(&mut client, 0x01)?;
        return Ok(());
    }

    write_connect_request(&mut upstream, &target, port)?;
    let (reply_code, reply) = match read_upstream_reply(&mut upstream) {
        Ok(reply) => reply,
        Err(_) => {
            state.store(STATE_OFFLINE, Ordering::Release);
            write_reply(&mut client, 0x01)?;
            return Ok(());
        }
    };
    client.write_all(&reply)?;
    if reply_code != 0x00 {
        return Ok(());
    }

    client.set_read_timeout(Some(STREAM_POLL_TIMEOUT))?;
    client.set_write_timeout(Some(STREAM_POLL_TIMEOUT))?;
    upstream.set_read_timeout(Some(STREAM_POLL_TIMEOUT))?;
    upstream.set_write_timeout(Some(STREAM_POLL_TIMEOUT))?;
    relay_bidirectional(client, upstream, shutdown)
}

fn negotiate_local_client(client: &mut TcpStream) -> io::Result<bool> {
    let mut greeting = [0_u8; 2];
    client.read_exact(&mut greeting)?;
    if greeting[0] != 0x05 || greeting[1] == 0 {
        client.write_all(&[0x05, 0xff])?;
        return Ok(false);
    }
    let mut methods = vec![0_u8; usize::from(greeting[1])];
    client.read_exact(&mut methods)?;
    if !methods.contains(&0x00) {
        client.write_all(&[0x05, 0xff])?;
        return Ok(false);
    }
    client.write_all(&[0x05, 0x00])?;
    Ok(true)
}

fn read_target(stream: &mut TcpStream, atyp: u8) -> io::Result<SocksTarget> {
    match atyp {
        0x01 => {
            let mut octets = [0_u8; 4];
            stream.read_exact(&mut octets)?;
            Ok(SocksTarget::Ip(IpAddr::V4(octets.into())))
        }
        0x03 => {
            let mut length = [0_u8; 1];
            stream.read_exact(&mut length)?;
            if length[0] == 0 {
                return Err(io::Error::new(ErrorKind::InvalidData, "empty domain"));
            }
            let mut bytes = vec![0_u8; usize::from(length[0])];
            stream.read_exact(&mut bytes)?;
            let domain = String::from_utf8(bytes)
                .map_err(|_| io::Error::new(ErrorKind::InvalidData, "invalid domain encoding"))?;
            Ok(SocksTarget::Domain(domain))
        }
        0x04 => {
            let mut octets = [0_u8; 16];
            stream.read_exact(&mut octets)?;
            Ok(SocksTarget::Ip(IpAddr::V6(octets.into())))
        }
        _ => Err(io::Error::new(
            ErrorKind::InvalidData,
            "unsupported SOCKS5 address type",
        )),
    }
}

fn write_connect_request(
    stream: &mut TcpStream,
    target: &SocksTarget,
    port: u16,
) -> io::Result<()> {
    let mut request = vec![0x05, 0x01, 0x00];
    match target {
        SocksTarget::Domain(domain) => {
            let normalized = normalize_target_domain(domain)
                .ok_or_else(|| io::Error::new(ErrorKind::InvalidData, "invalid domain"))?;
            let length = u8::try_from(normalized.len())
                .map_err(|_| io::Error::new(ErrorKind::InvalidData, "domain too long"))?;
            request.push(0x03);
            request.push(length);
            request.extend_from_slice(normalized.as_bytes());
        }
        SocksTarget::Ip(IpAddr::V4(ip)) => {
            request.push(0x01);
            request.extend_from_slice(&ip.octets());
        }
        SocksTarget::Ip(IpAddr::V6(ip)) => {
            request.push(0x04);
            request.extend_from_slice(&ip.octets());
        }
    }
    request.extend_from_slice(&port.to_be_bytes());
    stream.write_all(&request)
}

fn read_upstream_reply(stream: &mut TcpStream) -> io::Result<(u8, Vec<u8>)> {
    let mut header = [0_u8; 4];
    stream.read_exact(&mut header)?;
    if header[0] != 0x05 || header[2] != 0x00 {
        return Err(io::Error::new(
            ErrorKind::InvalidData,
            "invalid SOCKS5 upstream reply",
        ));
    }

    let mut reply = header.to_vec();
    match header[3] {
        0x01 => {
            let mut rest = [0_u8; 6];
            stream.read_exact(&mut rest)?;
            reply.extend_from_slice(&rest);
        }
        0x03 => {
            let mut length = [0_u8; 1];
            stream.read_exact(&mut length)?;
            reply.push(length[0]);
            let mut rest = vec![0_u8; usize::from(length[0]) + 2];
            stream.read_exact(&mut rest)?;
            reply.extend_from_slice(&rest);
        }
        0x04 => {
            let mut rest = [0_u8; 18];
            stream.read_exact(&mut rest)?;
            reply.extend_from_slice(&rest);
        }
        _ => {
            return Err(io::Error::new(
                ErrorKind::InvalidData,
                "invalid SOCKS5 upstream address type",
            ));
        }
    }
    Ok((header[1], reply))
}

fn write_reply(stream: &mut TcpStream, code: u8) -> io::Result<()> {
    stream.write_all(&[0x05, code, 0x00, 0x01, 0, 0, 0, 0, 0, 0])
}

fn relay_bidirectional(
    client: TcpStream,
    upstream: TcpStream,
    shutdown: &Arc<AtomicBool>,
) -> io::Result<()> {
    let client_reader = client.try_clone()?;
    let upstream_reader = upstream.try_clone()?;
    let first_shutdown = Arc::clone(shutdown);
    let second_shutdown = Arc::clone(shutdown);

    thread::scope(|scope| {
        let upstream_writer = upstream;
        let client_writer = client;
        scope.spawn(move || {
            copy_until_shutdown(client_reader, upstream_writer, &first_shutdown);
        });
        scope.spawn(move || {
            copy_until_shutdown(upstream_reader, client_writer, &second_shutdown);
        });
    });
    Ok(())
}

fn copy_until_shutdown(mut reader: TcpStream, mut writer: TcpStream, shutdown: &AtomicBool) {
    let mut buffer = [0_u8; 16 * 1024];
    while !shutdown.load(Ordering::Acquire) {
        match reader.read(&mut buffer) {
            Ok(0) => break,
            Ok(count) => {
                if writer.write_all(&buffer[..count]).is_err() {
                    break;
                }
            }
            Err(error) if matches!(error.kind(), ErrorKind::WouldBlock | ErrorKind::TimedOut) => {}
            Err(_) => break,
        }
    }
    let _ = writer.shutdown(Shutdown::Write);
    let _ = reader.shutdown(Shutdown::Read);
}
