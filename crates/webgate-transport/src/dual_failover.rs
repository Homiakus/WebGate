#![forbid(unsafe_code)]

use crate::failover::{FailoverConfig, TransportHealth, TransportRole};
use crate::socks5_proto::{
    DestinationPolicy, PolicyValidationError, STATE_DEGRADED, STATE_OFFLINE, STATE_STOPPED,
    STREAM_POLL_TIMEOUT, SocksTarget, configure_handshake_timeouts, decode_state, encode_state,
    negotiate_local_client, probe_socks5_sidecar, read_target, read_upstream_reply,
    relay_bidirectional, socks5_negotiate_no_auth, write_connect_request, write_reply,
};
use crate::{LocalProxyEndpoint, TransportProvider, TransportState};
use std::fmt;
use std::io::{self, Read, Write};
use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU8, AtomicU64, Ordering};
use std::sync::{Arc, Mutex, RwLock};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

const ROLE_NONE: u8 = 0;
const ROLE_PRIMARY: u8 = 1;
const ROLE_FALLBACK: u8 = 2;

fn encode_role(role: Option<TransportRole>) -> u8 {
    match role {
        None => ROLE_NONE,
        Some(TransportRole::Primary) => ROLE_PRIMARY,
        Some(TransportRole::Fallback) => ROLE_FALLBACK,
    }
}

fn decode_role(role: u8) -> Option<TransportRole> {
    match role {
        ROLE_PRIMARY => Some(TransportRole::Primary),
        ROLE_FALLBACK => Some(TransportRole::Fallback),
        _ => None,
    }
}

fn current_epoch_sec() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or(Duration::ZERO)
        .as_secs()
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DualRelayError {
    EmptyAllowedDomains,
    EmptyAllowedPorts,
    InvalidAllowedDomain,
    InvalidAllowedPort,
    InvalidConnectTimeout,
    PrimaryUpstreamNotLoopback,
    FallbackUpstreamNotLoopback,
    AllUpstreamsUnavailable,
    ListenerBind,
    AlreadyStarted,
}

impl From<PolicyValidationError> for DualRelayError {
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
pub struct DualRelayConfig {
    pub name: String,
    pub primary_upstream_host: String,
    pub primary_upstream_port: u16,
    pub fallback_upstream_host: String,
    pub fallback_upstream_port: u16,
    pub local_listen_port: u16,
    pub allowed_domains: Vec<String>,
    pub allowed_ports: Vec<u16>,
    pub connect_timeout: Duration,
    pub failover_config: FailoverConfig,
}

/// Read-only snapshot of dynamic failover status.
#[derive(Debug, Clone)]
pub struct DualRelayStatusHandle {
    state: Arc<AtomicU8>,
    active_role: Arc<AtomicU8>,
    endpoint: LocalProxyEndpoint,
    primary_health: Arc<RwLock<TransportHealth>>,
    fallback_health: Arc<RwLock<TransportHealth>>,
}

impl DualRelayStatusHandle {
    #[must_use]
    pub fn snapshot(
        &self,
    ) -> (
        TransportState,
        Option<TransportRole>,
        Option<LocalProxyEndpoint>,
        TransportHealth,
        TransportHealth,
    ) {
        let state = decode_state(self.state.load(Ordering::Acquire));
        let role = decode_role(self.active_role.load(Ordering::Acquire));
        let endpoint = if matches!(state, TransportState::Ready | TransportState::Degraded) {
            Some(self.endpoint)
        } else {
            None
        };
        let p_health = self.primary_health.read().map(|h| *h).unwrap_or_default();
        let f_health = self.fallback_health.read().map(|h| *h).unwrap_or_default();
        (state, role, endpoint, p_health, f_health)
    }
}

/// High-availability dual-relay transport managing seamless failover between
/// independent primary and fallback relays over a unified loopback proxy.
pub struct DualRelayFailoverTransport {
    config: DualRelayConfig,
    policy: DestinationPolicy,
    primary_upstream: SocketAddr,
    fallback_upstream: SocketAddr,
    state: Arc<AtomicU8>,
    active_role: Arc<AtomicU8>,
    primary_health: Arc<RwLock<TransportHealth>>,
    fallback_health: Arc<RwLock<TransportHealth>>,
    last_failover_epoch_sec: Arc<AtomicU64>,
    local_endpoint: Option<LocalProxyEndpoint>,
    shutdown: Arc<AtomicBool>,
    listener_handle: Option<JoinHandle<()>>,
    workers: Arc<Mutex<Vec<JoinHandle<()>>>>,
}

impl fmt::Debug for DualRelayFailoverTransport {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("DualRelayFailoverTransport")
            .field("name", &self.config.name)
            .field("state", &self.state())
            .field("active_role", &self.active_role())
            .field("local_endpoint", &self.local_endpoint)
            .finish_non_exhaustive()
    }
}

impl DualRelayFailoverTransport {
    pub fn new(config: DualRelayConfig) -> Result<Self, DualRelayError> {
        let policy = DestinationPolicy::new(&config.allowed_domains, &config.allowed_ports)
            .map_err(DualRelayError::from)?;
        if config.connect_timeout.is_zero() {
            return Err(DualRelayError::InvalidConnectTimeout);
        }

        let p_ip = config
            .primary_upstream_host
            .parse::<IpAddr>()
            .map_err(|_| DualRelayError::PrimaryUpstreamNotLoopback)?;
        if !p_ip.is_loopback() || config.primary_upstream_port == 0 {
            return Err(DualRelayError::PrimaryUpstreamNotLoopback);
        }
        let primary_upstream = SocketAddr::new(p_ip, config.primary_upstream_port);

        let f_ip = config
            .fallback_upstream_host
            .parse::<IpAddr>()
            .map_err(|_| DualRelayError::FallbackUpstreamNotLoopback)?;
        if !f_ip.is_loopback() || config.fallback_upstream_port == 0 {
            return Err(DualRelayError::FallbackUpstreamNotLoopback);
        }
        let fallback_upstream = SocketAddr::new(f_ip, config.fallback_upstream_port);

        Ok(Self {
            config,
            policy,
            primary_upstream,
            fallback_upstream,
            state: Arc::new(AtomicU8::new(STATE_STOPPED)),
            active_role: Arc::new(AtomicU8::new(ROLE_NONE)),
            primary_health: Arc::new(RwLock::new(TransportHealth::default())),
            fallback_health: Arc::new(RwLock::new(TransportHealth::default())),
            last_failover_epoch_sec: Arc::new(AtomicU64::new(0)),
            local_endpoint: None,
            shutdown: Arc::new(AtomicBool::new(false)),
            listener_handle: None,
            workers: Arc::new(Mutex::new(Vec::new())),
        })
    }

    pub fn start_proxy(&mut self) -> Result<LocalProxyEndpoint, DualRelayError> {
        if self.listener_handle.is_some()
            || matches!(
                self.state(),
                TransportState::Starting | TransportState::Ready | TransportState::Degraded
            )
        {
            return Err(DualRelayError::AlreadyStarted);
        }

        self.local_endpoint = None;
        self.shutdown.store(false, Ordering::Release);
        self.set_state(TransportState::Starting);

        let primary_ok =
            probe_socks5_sidecar(self.primary_upstream, self.config.connect_timeout).is_ok();
        let fallback_ok =
            probe_socks5_sidecar(self.fallback_upstream, self.config.connect_timeout).is_ok();

        let initial_role = if primary_ok {
            if let Ok(mut lock) = self.primary_health.write() {
                lock.is_responsive = true;
                lock.consecutive_failures = 0;
            }
            Some(TransportRole::Primary)
        } else if fallback_ok {
            if let Ok(mut lock) = self.fallback_health.write() {
                lock.is_responsive = true;
                lock.consecutive_failures = 0;
            }
            if let Ok(mut lock) = self.primary_health.write() {
                lock.is_responsive = false;
                lock.consecutive_failures = self.config.failover_config.max_consecutive_failures;
            }
            Some(TransportRole::Fallback)
        } else {
            self.set_state(TransportState::Offline);
            self.active_role.store(ROLE_NONE, Ordering::Release);
            return Err(DualRelayError::AllUpstreamsUnavailable);
        };

        let initial_state = match initial_role {
            Some(TransportRole::Primary) => TransportState::Ready,
            Some(TransportRole::Fallback) => TransportState::Degraded,
            None => TransportState::Offline,
        };

        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, self.config.local_listen_port))
            .map_err(|_| {
                self.set_state(TransportState::Offline);
                DualRelayError::ListenerBind
            })?;
        let bound = listener.local_addr().map_err(|_| {
            self.set_state(TransportState::Offline);
            DualRelayError::ListenerBind
        })?;
        let endpoint = LocalProxyEndpoint::new(bound.ip(), bound.port()).map_err(|_| {
            self.set_state(TransportState::Offline);
            DualRelayError::ListenerBind
        })?;

        self.active_role
            .store(encode_role(initial_role), Ordering::Release);
        self.set_state(initial_state);

        let worker_context = FailoverWorkerContext {
            primary_addr: self.primary_upstream,
            fallback_addr: self.fallback_upstream,
            connect_timeout: self.config.connect_timeout,
            failover_config: self.config.failover_config,
            policy: self.policy.clone(),
            shutdown: Arc::clone(&self.shutdown),
            state: Arc::clone(&self.state),
            active_role: Arc::clone(&self.active_role),
            primary_health: Arc::clone(&self.primary_health),
            fallback_health: Arc::clone(&self.fallback_health),
            last_failover_epoch_sec: Arc::clone(&self.last_failover_epoch_sec),
        };
        let shutdown = Arc::clone(&self.shutdown);
        let state = Arc::clone(&self.state);
        let workers = Arc::clone(&self.workers);

        let handle = thread::spawn(move || {
            loop {
                let accepted = listener.accept();
                match accepted {
                    Ok((stream, _)) => {
                        if shutdown.load(Ordering::Acquire) {
                            break;
                        }
                        let ctx = worker_context.clone();
                        let worker = thread::spawn(move || {
                            let _ = handle_failover_client(stream, &ctx);
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
        Ok(endpoint)
    }

    /// Actively probes standby Primary when running on Fallback and initiates switchback
    /// once the cooldown period has elapsed.
    pub fn probe_and_maybe_switchback(&self, now_epoch_sec: u64) -> bool {
        if self.active_role() != Some(TransportRole::Fallback) {
            return false;
        }

        let last_fail = self.last_failover_epoch_sec.load(Ordering::Acquire);
        if now_epoch_sec.saturating_sub(last_fail)
            < self.config.failover_config.switchback_cooldown_sec
        {
            return false;
        }

        let start = Instant::now();
        if probe_socks5_sidecar(self.primary_upstream, self.config.connect_timeout).is_ok() {
            let latency_ms = start.elapsed().as_millis() as u64;
            if let Ok(mut lock) = self.primary_health.write() {
                lock.consecutive_failures = 0;
                lock.is_responsive = true;
                lock.last_latency_ms = latency_ms;
                lock.last_probe_epoch_sec = now_epoch_sec;
            }
            self.active_role.store(ROLE_PRIMARY, Ordering::Release);
            self.set_state(TransportState::Ready);
            true
        } else {
            if let Ok(mut lock) = self.primary_health.write() {
                lock.consecutive_failures = lock.consecutive_failures.saturating_add(1);
                lock.is_responsive = false;
                lock.last_probe_epoch_sec = now_epoch_sec;
            }
            false
        }
    }

    #[must_use]
    pub fn active_role(&self) -> Option<TransportRole> {
        decode_role(self.active_role.load(Ordering::Acquire))
    }

    #[must_use]
    pub fn primary_health(&self) -> TransportHealth {
        self.primary_health.read().map(|h| *h).unwrap_or_default()
    }

    #[must_use]
    pub fn fallback_health(&self) -> TransportHealth {
        self.fallback_health.read().map(|h| *h).unwrap_or_default()
    }

    #[must_use]
    pub fn status_handle(&self) -> Option<DualRelayStatusHandle> {
        self.local_endpoint.map(|endpoint| DualRelayStatusHandle {
            state: Arc::clone(&self.state),
            active_role: Arc::clone(&self.active_role),
            endpoint,
            primary_health: Arc::clone(&self.primary_health),
            fallback_health: Arc::clone(&self.fallback_health),
        })
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
        self.active_role.store(ROLE_NONE, Ordering::Release);
        self.set_state(TransportState::Stopped);
    }
}

impl TransportProvider for DualRelayFailoverTransport {
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

impl Drop for DualRelayFailoverTransport {
    fn drop(&mut self) {
        self.stop_internal();
    }
}

fn connect_and_request_upstream(
    upstream_addr: SocketAddr,
    target: &SocksTarget,
    port: u16,
    timeout: Duration,
) -> io::Result<(TcpStream, u8, Vec<u8>, u64)> {
    let start = Instant::now();
    let mut upstream = TcpStream::connect_timeout(&upstream_addr, timeout)?;
    configure_handshake_timeouts(&upstream, timeout)?;
    socks5_negotiate_no_auth(&mut upstream)?;
    write_connect_request(&mut upstream, target, port)?;
    let (code, reply) = read_upstream_reply(&mut upstream)?;
    let elapsed = start.elapsed().as_millis() as u64;
    Ok((upstream, code, reply, elapsed))
}

#[derive(Clone)]
struct FailoverWorkerContext {
    primary_addr: SocketAddr,
    fallback_addr: SocketAddr,
    connect_timeout: Duration,
    failover_config: FailoverConfig,
    policy: DestinationPolicy,
    shutdown: Arc<AtomicBool>,
    state: Arc<AtomicU8>,
    active_role: Arc<AtomicU8>,
    primary_health: Arc<RwLock<TransportHealth>>,
    fallback_health: Arc<RwLock<TransportHealth>>,
    last_failover_epoch_sec: Arc<AtomicU64>,
}

fn handle_failover_client(mut client: TcpStream, ctx: &FailoverWorkerContext) -> io::Result<()> {
    configure_handshake_timeouts(&client, ctx.connect_timeout)?;
    if !negotiate_local_client(&mut client)? {
        return Ok(());
    }

    let current_state = decode_state(ctx.state.load(Ordering::Acquire));
    if !matches!(
        current_state,
        TransportState::Ready | TransportState::Degraded
    ) {
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
    let mut port_bytes = [0_u8; 2];
    client.read_exact(&mut port_bytes)?;
    let port = u16::from_be_bytes(port_bytes);

    if !ctx.policy.allows(&target, port) {
        write_reply(&mut client, 0x02)?;
        return Ok(());
    }

    let current_role = decode_role(ctx.active_role.load(Ordering::Acquire));
    let now = current_epoch_sec();

    match current_role {
        Some(TransportRole::Primary) => {
            match connect_and_request_upstream(ctx.primary_addr, &target, port, ctx.connect_timeout)
            {
                Ok((upstream, code, reply, latency_ms)) => {
                    let high_latency = ctx.failover_config.high_latency_threshold_ms > 0
                        && latency_ms > ctx.failover_config.high_latency_threshold_ms;

                    if let Ok(mut lock) = ctx.primary_health.write() {
                        lock.last_latency_ms = latency_ms;
                        lock.last_probe_epoch_sec = now;
                        if high_latency {
                            lock.consecutive_failures = lock.consecutive_failures.saturating_add(1);
                        } else {
                            lock.consecutive_failures = 0;
                            lock.is_responsive = true;
                        }
                    }

                    if code == 0x00 {
                        client.write_all(&reply)?;
                        client.set_read_timeout(Some(STREAM_POLL_TIMEOUT))?;
                        client.set_write_timeout(Some(STREAM_POLL_TIMEOUT))?;
                        relay_bidirectional(client, upstream, &ctx.shutdown)
                    } else {
                        client.write_all(&reply)?;
                        Ok(())
                    }
                }
                Err(_) => {
                    // Primary failed; trigger failover to Fallback
                    let max_fails = ctx.failover_config.max_consecutive_failures.max(1);
                    let should_failover = if let Ok(mut lock) = ctx.primary_health.write() {
                        lock.consecutive_failures = lock.consecutive_failures.saturating_add(1);
                        lock.is_responsive = false;
                        lock.last_probe_epoch_sec = now;
                        lock.consecutive_failures >= max_fails
                    } else {
                        true
                    };

                    if should_failover {
                        ctx.active_role.store(ROLE_FALLBACK, Ordering::Release);
                        ctx.state.store(STATE_DEGRADED, Ordering::Release);
                        ctx.last_failover_epoch_sec.store(now, Ordering::Release);
                    }

                    // Attempt fallback immediately
                    match connect_and_request_upstream(
                        ctx.fallback_addr,
                        &target,
                        port,
                        ctx.connect_timeout,
                    ) {
                        Ok((upstream, code, reply, latency_ms)) => {
                            if let Ok(mut lock) = ctx.fallback_health.write() {
                                lock.consecutive_failures = 0;
                                lock.is_responsive = true;
                                lock.last_latency_ms = latency_ms;
                                lock.last_probe_epoch_sec = now;
                            }
                            client.write_all(&reply)?;
                            if code == 0x00 {
                                client.set_read_timeout(Some(STREAM_POLL_TIMEOUT))?;
                                client.set_write_timeout(Some(STREAM_POLL_TIMEOUT))?;
                                relay_bidirectional(client, upstream, &ctx.shutdown)
                            } else {
                                Ok(())
                            }
                        }
                        Err(_) => {
                            if let Ok(mut lock) = ctx.fallback_health.write() {
                                lock.consecutive_failures =
                                    lock.consecutive_failures.saturating_add(1);
                                lock.is_responsive = false;
                                lock.last_probe_epoch_sec = now;
                            }
                            ctx.state.store(STATE_OFFLINE, Ordering::Release);
                            ctx.active_role.store(ROLE_NONE, Ordering::Release);
                            write_reply(&mut client, 0x05)?;
                            Ok(())
                        }
                    }
                }
            }
        }
        Some(TransportRole::Fallback) => {
            match connect_and_request_upstream(
                ctx.fallback_addr,
                &target,
                port,
                ctx.connect_timeout,
            ) {
                Ok((upstream, code, reply, latency_ms)) => {
                    if let Ok(mut lock) = ctx.fallback_health.write() {
                        lock.consecutive_failures = 0;
                        lock.is_responsive = true;
                        lock.last_latency_ms = latency_ms;
                        lock.last_probe_epoch_sec = now;
                    }
                    client.write_all(&reply)?;
                    if code == 0x00 {
                        client.set_read_timeout(Some(STREAM_POLL_TIMEOUT))?;
                        client.set_write_timeout(Some(STREAM_POLL_TIMEOUT))?;
                        return relay_bidirectional(client, upstream, &ctx.shutdown);
                    }
                    Ok(())
                }
                Err(_) => {
                    if let Ok(mut lock) = ctx.fallback_health.write() {
                        lock.consecutive_failures = lock.consecutive_failures.saturating_add(1);
                        lock.is_responsive = false;
                        lock.last_probe_epoch_sec = now;
                    }
                    ctx.state.store(STATE_OFFLINE, Ordering::Release);
                    ctx.active_role.store(ROLE_NONE, Ordering::Release);
                    write_reply(&mut client, 0x05)?;
                    Ok(())
                }
            }
        }
        None => {
            write_reply(&mut client, 0x01)?;
            Ok(())
        }
    }
}
