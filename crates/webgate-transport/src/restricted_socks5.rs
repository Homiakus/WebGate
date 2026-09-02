#![forbid(unsafe_code)]

use crate::socks5_proto::{
    DestinationPolicy, PolicyValidationError, STATE_OFFLINE, STATE_STOPPED, STREAM_POLL_TIMEOUT,
    configure_handshake_timeouts, decode_state, encode_state, negotiate_local_client,
    probe_socks5_sidecar, read_target, read_upstream_reply, relay_bidirectional,
    socks5_negotiate_no_auth, write_connect_request, write_reply,
};
use crate::{LocalProxyEndpoint, TransportProvider, TransportState};
use std::fmt;
use std::io::{self, Read, Write};
use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

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

impl From<PolicyValidationError> for RestrictedProxyError {
    fn from(err: PolicyValidationError) -> Self {
        match err {
            PolicyValidationError::EmptyAllowedDomains => Self::EmptyAllowedDomains,
            PolicyValidationError::EmptyAllowedPorts => Self::EmptyAllowedPorts,
            PolicyValidationError::InvalidAllowedDomain => Self::InvalidAllowedDomain,
            PolicyValidationError::InvalidAllowedPort => Self::InvalidAllowedPort,
        }
    }
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

/// Read-only live view of a started restricted proxy.
#[derive(Debug, Clone)]
pub struct RestrictedProxyStatusHandle {
    state: Arc<AtomicU8>,
    endpoint: LocalProxyEndpoint,
}

impl RestrictedProxyStatusHandle {
    #[must_use]
    pub fn snapshot(&self) -> (TransportState, Option<LocalProxyEndpoint>) {
        let state = decode_state(self.state.load(Ordering::Acquire));
        let endpoint = if matches!(state, TransportState::Ready | TransportState::Degraded) {
            Some(self.endpoint)
        } else {
            None
        };
        (state, endpoint)
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
        let policy = DestinationPolicy::new(&config.allowed_domains, &config.allowed_ports)
            .map_err(RestrictedProxyError::from)?;
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
            let mapped = match error.kind() {
                io::ErrorKind::InvalidData => RestrictedProxyError::UpstreamProtocol,
                _ => RestrictedProxyError::UpstreamUnavailable,
            };
            return Err(mapped);
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

        let handle = thread::spawn(move || {
            loop {
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
            }
        });

        self.listener_handle = Some(handle);
        self.local_endpoint = Some(endpoint);
        self.set_state(TransportState::Ready);
        Ok(endpoint)
    }

    #[must_use]
    pub fn status_handle(&self) -> Option<RestrictedProxyStatusHandle> {
        self.local_endpoint
            .map(|endpoint| RestrictedProxyStatusHandle {
                state: Arc::clone(&self.state),
                endpoint,
            })
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
