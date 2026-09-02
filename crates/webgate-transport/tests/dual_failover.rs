#![forbid(unsafe_code)]
#![allow(clippy::expect_used, clippy::panic, clippy::unwrap_used)]

use std::io::{ErrorKind, Read, Write};
use std::net::{Ipv4Addr, SocketAddr, TcpListener, TcpStream};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use webgate_transport::dual_failover::{
    DualRelayConfig, DualRelayError, DualRelayFailoverTransport,
};
use webgate_transport::failover::{FailoverConfig, TransportRole};
use webgate_transport::{TransportProvider, TransportState};

#[derive(Debug, Clone, PartialEq, Eq)]
struct ObservedRelayConnect {
    relay_name: &'static str,
    target: String,
    port: u16,
}

fn read_target_address(stream: &mut TcpStream, atyp: u8) -> std::io::Result<String> {
    match atyp {
        0x01 => {
            let mut octets = [0_u8; 4];
            stream.read_exact(&mut octets)?;
            Ok(std::net::Ipv4Addr::from(octets).to_string())
        }
        0x03 => {
            let mut len = [0_u8; 1];
            stream.read_exact(&mut len)?;
            let mut name = vec![0_u8; usize::from(len[0])];
            stream.read_exact(&mut name)?;
            String::from_utf8(name).map_err(|_| {
                std::io::Error::new(ErrorKind::InvalidData, "invalid UTF-8 SOCKS5 domain")
            })
        }
        0x04 => {
            let mut octets = [0_u8; 16];
            stream.read_exact(&mut octets)?;
            Ok(std::net::Ipv6Addr::from(octets).to_string())
        }
        _ => Err(std::io::Error::new(
            ErrorKind::InvalidData,
            "unsupported SOCKS5 address type",
        )),
    }
}

struct FakeRelayServer {
    pub addr: SocketAddr,
    pub shutdown: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
}

impl FakeRelayServer {
    fn spawn(name: &'static str, tx: mpsc::Sender<ObservedRelayConnect>) -> Self {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let addr = listener.local_addr().unwrap();
        let shutdown = Arc::new(AtomicBool::new(false));
        let shutdown_clone = Arc::clone(&shutdown);

        let handle = thread::spawn(move || {
            while !shutdown_clone.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(res) => res,
                    Err(_) => break,
                };

                let _ = stream.set_read_timeout(Some(Duration::from_millis(500)));
                let _ = stream.set_write_timeout(Some(Duration::from_millis(500)));

                let mut greeting = [0_u8; 2];
                if stream.read_exact(&mut greeting).is_err() {
                    continue;
                }
                if greeting[0] != 0x05 {
                    continue;
                }
                let mut methods = vec![0_u8; usize::from(greeting[1])];
                if stream.read_exact(&mut methods).is_err() || !methods.contains(&0x00) {
                    continue;
                }
                if stream.write_all(&[0x05, 0x00]).is_err() {
                    continue;
                }

                let mut request = [0_u8; 4];
                match stream.read_exact(&mut request) {
                    Ok(()) => {}
                    Err(_) => continue, // probe-only handshake
                }

                if request[0] != 0x05 || request[1] != 0x01 {
                    continue;
                }

                let target = match read_target_address(&mut stream, request[3]) {
                    Ok(t) => t,
                    Err(_) => continue,
                };
                let mut port_bytes = [0_u8; 2];
                if stream.read_exact(&mut port_bytes).is_err() {
                    continue;
                }
                let port = u16::from_be_bytes(port_bytes);

                let _ = tx.send(ObservedRelayConnect {
                    relay_name: name,
                    target,
                    port,
                });

                if stream
                    .write_all(&[0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0])
                    .is_err()
                {
                    continue;
                }

                // Echo payload with relay header prefix
                let mut buf = [0_u8; 512];
                if let Ok(n @ 1..) = stream.read(&mut buf) {
                    let mut resp = format!("ACK-FROM-{}:", name).into_bytes();
                    resp.extend_from_slice(&buf[..n]);
                    let _ = stream.write_all(&resp);
                }
            }
        });

        Self {
            addr,
            shutdown,
            handle: Some(handle),
        }
    }

    fn stop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        let _ = TcpStream::connect_timeout(&self.addr, Duration::from_millis(50));
        if let Some(h) = self.handle.take() {
            let _ = h.join();
        }
    }
}

impl Drop for FakeRelayServer {
    fn drop(&mut self) {
        self.stop();
    }
}

fn client_socks5_connect(
    proxy_addr: SocketAddr,
    target_domain: &str,
    target_port: u16,
) -> std::io::Result<(TcpStream, u8)> {
    let mut stream = TcpStream::connect_timeout(&proxy_addr, Duration::from_secs(2))?;
    stream.set_read_timeout(Some(Duration::from_secs(2)))?;
    stream.set_write_timeout(Some(Duration::from_secs(2)))?;

    // SOCKS5 Handshake
    stream.write_all(&[0x05, 0x01, 0x00])?;
    let mut greeting_ack = [0_u8; 2];
    stream.read_exact(&mut greeting_ack)?;
    if greeting_ack != [0x05, 0x00] {
        return Err(std::io::Error::new(
            ErrorKind::InvalidData,
            "socks5 handshake rejected",
        ));
    }

    // CONNECT request
    let mut req = vec![0x05, 0x01, 0x00, 0x03, target_domain.len() as u8];
    req.extend_from_slice(target_domain.as_bytes());
    req.extend_from_slice(&target_port.to_be_bytes());
    stream.write_all(&req)?;

    let mut resp_header = [0_u8; 4];
    stream.read_exact(&mut resp_header)?;
    let rep_code = resp_header[1];

    match resp_header[3] {
        0x01 => {
            let mut rest = [0_u8; 6];
            stream.read_exact(&mut rest)?;
        }
        0x03 => {
            let mut len = [0_u8; 1];
            stream.read_exact(&mut len)?;
            let mut rest = vec![0_u8; usize::from(len[0]) + 2];
            stream.read_exact(&mut rest)?;
        }
        0x04 => {
            let mut rest = [0_u8; 18];
            stream.read_exact(&mut rest)?;
        }
        _ => {}
    }

    Ok((stream, rep_code))
}

#[test]
fn dual_relay_starts_on_primary_and_routes_traffic() {
    let (tx, rx) = mpsc::channel();
    let mut relay_a = FakeRelayServer::spawn("RELAY-A", tx.clone());
    let mut relay_b = FakeRelayServer::spawn("RELAY-B", tx);

    let config = DualRelayConfig {
        name: "DualRelay-Test".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: relay_a.addr.port(),
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: relay_b.addr.port(),
        local_listen_port: 0,
        allowed_domains: vec!["service.webgate.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_secs(1),
        failover_config: FailoverConfig::default(),
    };

    let mut transport = DualRelayFailoverTransport::new(config).unwrap();
    let endpoint = transport.start_proxy().unwrap();

    assert_eq!(transport.state(), TransportState::Ready);
    assert_eq!(transport.active_role(), Some(TransportRole::Primary));
    assert_eq!(transport.local_proxy(), Some(endpoint));

    // Client connects via proxy
    let (mut stream, rep) =
        client_socks5_connect(endpoint.socket_addr(), "service.webgate.internal", 443).unwrap();
    assert_eq!(rep, 0x00);

    // Send payload & verify echo from RELAY-A
    stream.write_all(b"HelloPrimary").unwrap();
    let mut buf = [0_u8; 64];
    let n = stream.read(&mut buf).unwrap();
    let echo = String::from_utf8_lossy(&buf[..n]);
    assert_eq!(echo, "ACK-FROM-RELAY-A:HelloPrimary");

    let observed = rx.recv_timeout(Duration::from_secs(1)).unwrap();
    assert_eq!(observed.relay_name, "RELAY-A");
    assert_eq!(observed.target, "service.webgate.internal");
    assert_eq!(observed.port, 443);

    transport.stop();
    relay_a.stop();
    relay_b.stop();
}

#[test]
fn dual_relay_starts_on_fallback_when_primary_offline() {
    let (tx, rx) = mpsc::channel();
    let mut relay_b = FakeRelayServer::spawn("RELAY-B", tx);

    let config = DualRelayConfig {
        name: "DualRelay-FallbackStart".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: 43999, // Offline primary
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: relay_b.addr.port(),
        local_listen_port: 0,
        allowed_domains: vec!["service.webgate.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_millis(200),
        failover_config: FailoverConfig::default(),
    };

    let mut transport = DualRelayFailoverTransport::new(config).unwrap();
    let endpoint = transport.start_proxy().unwrap();

    assert_eq!(transport.state(), TransportState::Degraded);
    assert_eq!(transport.active_role(), Some(TransportRole::Fallback));

    let (mut stream, rep) =
        client_socks5_connect(endpoint.socket_addr(), "service.webgate.internal", 443).unwrap();
    assert_eq!(rep, 0x00);

    stream.write_all(b"HelloFallback").unwrap();
    let mut buf = [0_u8; 64];
    let n = stream.read(&mut buf).unwrap();
    let echo = String::from_utf8_lossy(&buf[..n]);
    assert_eq!(echo, "ACK-FROM-RELAY-B:HelloFallback");

    let observed = rx.recv_timeout(Duration::from_secs(1)).unwrap();
    assert_eq!(observed.relay_name, "RELAY-B");

    transport.stop();
    relay_b.stop();
}

#[test]
fn dual_relay_live_failover_during_primary_crash() {
    let (tx, rx) = mpsc::channel();
    let mut relay_a = FakeRelayServer::spawn("RELAY-A", tx.clone());
    let mut relay_b = FakeRelayServer::spawn("RELAY-B", tx);

    let config = DualRelayConfig {
        name: "DualRelay-LiveCrash".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: relay_a.addr.port(),
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: relay_b.addr.port(),
        local_listen_port: 0,
        allowed_domains: vec!["service.webgate.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_millis(300),
        failover_config: FailoverConfig {
            max_consecutive_failures: 1,
            high_latency_threshold_ms: 1000,
            switchback_cooldown_sec: 10,
        },
    };

    let mut transport = DualRelayFailoverTransport::new(config).unwrap();
    let endpoint = transport.start_proxy().unwrap();

    assert_eq!(transport.active_role(), Some(TransportRole::Primary));

    // 1st request succeeds on Primary
    let (_, rep1) =
        client_socks5_connect(endpoint.socket_addr(), "service.webgate.internal", 443).unwrap();
    assert_eq!(rep1, 0x00);
    let obs1 = rx.recv_timeout(Duration::from_secs(1)).unwrap();
    assert_eq!(obs1.relay_name, "RELAY-A");

    // Primary crashes
    relay_a.stop();

    // 2nd request detects primary failure, fails over immediately to Fallback, and succeeds!
    let (mut stream2, rep2) =
        client_socks5_connect(endpoint.socket_addr(), "service.webgate.internal", 443).unwrap();
    assert_eq!(rep2, 0x00);

    stream2.write_all(b"FailoverPayload").unwrap();
    let mut buf = [0_u8; 64];
    let n = stream2.read(&mut buf).unwrap();
    let echo = String::from_utf8_lossy(&buf[..n]);
    assert_eq!(echo, "ACK-FROM-RELAY-B:FailoverPayload");

    assert_eq!(transport.active_role(), Some(TransportRole::Fallback));
    assert_eq!(transport.state(), TransportState::Degraded);

    let obs2 = rx.recv_timeout(Duration::from_secs(1)).unwrap();
    assert_eq!(obs2.relay_name, "RELAY-B");

    transport.stop();
    relay_b.stop();
}

#[test]
fn dual_relay_standby_primary_recovery_and_switchback() {
    let (tx, rx) = mpsc::channel();
    let mut relay_b = FakeRelayServer::spawn("RELAY-B", tx.clone());

    // Start a listener to obtain a fixed port for Relay A
    let dummy_a = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
    let primary_port = dummy_a.local_addr().unwrap().port();
    drop(dummy_a); // currently offline

    let config = DualRelayConfig {
        name: "DualRelay-Switchback".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: primary_port,
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: relay_b.addr.port(),
        local_listen_port: 0,
        allowed_domains: vec!["service.webgate.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_millis(200),
        failover_config: FailoverConfig {
            max_consecutive_failures: 1,
            high_latency_threshold_ms: 1000,
            switchback_cooldown_sec: 5,
        },
    };

    let mut transport = DualRelayFailoverTransport::new(config).unwrap();
    let endpoint = transport.start_proxy().unwrap();

    assert_eq!(transport.active_role(), Some(TransportRole::Fallback));

    // Request goes to Relay B
    let (_, rep1) =
        client_socks5_connect(endpoint.socket_addr(), "service.webgate.internal", 443).unwrap();
    assert_eq!(rep1, 0x00);
    assert_eq!(rx.recv().unwrap().relay_name, "RELAY-B");

    // Relay A comes back up on the designated port
    let listener_a = TcpListener::bind((Ipv4Addr::LOCALHOST, primary_port)).unwrap();
    let shutdown_a = Arc::new(AtomicBool::new(false));
    let shutdown_a_clone = Arc::clone(&shutdown_a);
    let tx_a = tx;

    let handle_a = thread::spawn(move || {
        while !shutdown_a_clone.load(Ordering::Acquire) {
            let (mut stream, _) = match listener_a.accept() {
                Ok(res) => res,
                Err(_) => break,
            };
            let mut greeting = [0_u8; 2];
            if stream.read_exact(&mut greeting).is_err() {
                continue;
            }
            let mut methods = vec![0_u8; usize::from(greeting[1])];
            if stream.read_exact(&mut methods).is_err() || stream.write_all(&[0x05, 0x00]).is_err()
            {
                continue;
            }
            let mut req = [0_u8; 4];
            if stream.read_exact(&mut req).is_ok() && req[0] == 0x05 && req[1] == 0x01 {
                let _ = read_target_address(&mut stream, req[3]);
                let mut port = [0_u8; 2];
                let _ = stream.read_exact(&mut port);
                let _ = tx_a.send(ObservedRelayConnect {
                    relay_name: "RELAY-A-RECOVERED",
                    target: "service.webgate.internal".to_string(),
                    port: 443,
                });
                let _ = stream.write_all(&[0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0]);
            }
        }
    });

    // Before cooldown: probe returns false
    assert!(!transport.probe_and_maybe_switchback(0));
    assert_eq!(transport.active_role(), Some(TransportRole::Fallback));

    // After cooldown: probe succeeds and switches back to Primary
    assert!(transport.probe_and_maybe_switchback(100));
    assert_eq!(transport.active_role(), Some(TransportRole::Primary));
    assert_eq!(transport.state(), TransportState::Ready);

    // Subsequent traffic routes to recovered Relay A
    let (_, rep2) =
        client_socks5_connect(endpoint.socket_addr(), "service.webgate.internal", 443).unwrap();
    assert_eq!(rep2, 0x00);
    assert_eq!(rx.recv().unwrap().relay_name, "RELAY-A-RECOVERED");

    shutdown_a.store(true, Ordering::Release);
    let _ = TcpStream::connect_timeout(
        &SocketAddr::from(([127, 0, 0, 1], primary_port)),
        Duration::from_millis(50),
    );
    let _ = handle_a.join();
    transport.stop();
    relay_b.stop();
}

#[test]
fn dual_relay_fails_closed_when_both_relays_down() {
    let config = DualRelayConfig {
        name: "DualRelay-AllDown".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: 43991,
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: 43992,
        local_listen_port: 0,
        allowed_domains: vec!["service.webgate.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_millis(100),
        failover_config: FailoverConfig::default(),
    };

    let mut transport = DualRelayFailoverTransport::new(config).unwrap();
    let res = transport.start_proxy();

    assert_eq!(res, Err(DualRelayError::AllUpstreamsUnavailable));
    assert_eq!(transport.state(), TransportState::Offline);
    assert_eq!(transport.active_role(), None);
    assert_eq!(transport.local_proxy(), None);
}

#[test]
fn dual_relay_enforces_destination_policy() {
    let (tx, _) = mpsc::channel();
    let mut relay_a = FakeRelayServer::spawn("RELAY-A", tx.clone());
    let mut relay_b = FakeRelayServer::spawn("RELAY-B", tx);

    let config = DualRelayConfig {
        name: "DualRelay-Policy".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: relay_a.addr.port(),
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: relay_b.addr.port(),
        local_listen_port: 0,
        allowed_domains: vec!["authorized.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_secs(1),
        failover_config: FailoverConfig::default(),
    };

    let mut transport = DualRelayFailoverTransport::new(config).unwrap();
    let endpoint = transport.start_proxy().unwrap();

    // Disallowed domain -> rejected by proxy ruleset (0x02)
    let (_, rep_domain) =
        client_socks5_connect(endpoint.socket_addr(), "forbidden.evil.com", 443).unwrap();
    assert_eq!(rep_domain, 0x02);

    // Disallowed port -> rejected by proxy ruleset (0x02)
    let (_, rep_port) =
        client_socks5_connect(endpoint.socket_addr(), "authorized.internal", 80).unwrap();
    assert_eq!(rep_port, 0x02);

    transport.stop();
    relay_a.stop();
    relay_b.stop();
}

#[test]
fn dual_relay_rejects_non_loopback_upstreams() {
    let bad_primary = DualRelayConfig {
        name: "DualRelay-BadPrimary".to_string(),
        primary_upstream_host: "192.0.2.10".to_string(),
        primary_upstream_port: 8443,
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: 8444,
        local_listen_port: 0,
        allowed_domains: vec!["service.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_secs(1),
        failover_config: FailoverConfig::default(),
    };
    assert_eq!(
        DualRelayFailoverTransport::new(bad_primary).err(),
        Some(DualRelayError::PrimaryUpstreamNotLoopback)
    );

    let bad_fallback = DualRelayConfig {
        name: "DualRelay-BadFallback".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: 8443,
        fallback_upstream_host: "10.0.0.1".to_string(),
        fallback_upstream_port: 8444,
        local_listen_port: 0,
        allowed_domains: vec!["service.internal".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_secs(1),
        failover_config: FailoverConfig::default(),
    };
    assert_eq!(
        DualRelayFailoverTransport::new(bad_fallback).err(),
        Some(DualRelayError::FallbackUpstreamNotLoopback)
    );
}
