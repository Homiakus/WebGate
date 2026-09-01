#![forbid(unsafe_code)]
#![allow(clippy::expect_used, clippy::panic, clippy::unwrap_used)]

use std::io::{ErrorKind, Read, Write};
use std::net::{Ipv4Addr, SocketAddr, TcpListener, TcpStream};
use std::sync::mpsc::{self, Receiver};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use webgate_transport::restricted_socks5::{
    RestrictedProxyError, RestrictedSocks5Config, RestrictedSocks5Transport,
};
use webgate_transport::{TransportProvider, TransportState};

#[derive(Debug, Clone, PartialEq, Eq)]
struct ObservedConnect {
    host: String,
    port: u16,
}

fn read_target(stream: &mut TcpStream, atyp: u8) -> std::io::Result<String> {
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

fn spawn_fake_sidecar(max_connections: usize) -> (SocketAddr, Receiver<ObservedConnect>, JoinHandle<()>) {
    let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
    let address = listener.local_addr().unwrap();
    let (tx, rx) = mpsc::channel();

    let handle = thread::spawn(move || {
        for _ in 0..max_connections {
            let (mut stream, _) = listener.accept().unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(1)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(1)))
                .unwrap();

            let mut greeting = [0_u8; 2];
            stream.read_exact(&mut greeting).unwrap();
            assert_eq!(greeting[0], 0x05);
            let mut methods = vec![0_u8; usize::from(greeting[1])];
            stream.read_exact(&mut methods).unwrap();
            assert!(methods.contains(&0x00));
            stream.write_all(&[0x05, 0x00]).unwrap();

            let mut request = [0_u8; 4];
            match stream.read_exact(&mut request) {
                Ok(()) => {}
                Err(error)
                    if matches!(
                        error.kind(),
                        ErrorKind::UnexpectedEof | ErrorKind::WouldBlock | ErrorKind::TimedOut
                    ) =>
                {
                    continue;
                }
                Err(error) => panic!("sidecar request read failed: {error}"),
            }

            assert_eq!(request[0], 0x05);
            assert_eq!(request[1], 0x01);
            let host = read_target(&mut stream, request[3]).unwrap();
            let mut port_bytes = [0_u8; 2];
            stream.read_exact(&mut port_bytes).unwrap();
            let port = u16::from_be_bytes(port_bytes);
            tx.send(ObservedConnect { host, port }).unwrap();

            stream
                .write_all(&[0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0])
                .unwrap();

            let mut ping = [0_u8; 4];
            if stream.read_exact(&mut ping).is_ok() {
                assert_eq!(&ping, b"ping");
                stream.write_all(b"pong").unwrap();
            }
        }
    });

    (address, rx, handle)
}

fn config(
    upstream: SocketAddr,
    allowed_domains: Vec<&str>,
    allowed_ports: Vec<u16>,
) -> RestrictedSocks5Config {
    RestrictedSocks5Config {
        name: "primary-sidecar".to_string(),
        upstream_host: upstream.ip().to_string(),
        upstream_port: upstream.port(),
        local_listen_port: 0,
        allowed_domains: allowed_domains.into_iter().map(str::to_string).collect(),
        allowed_ports,
        connect_timeout: Duration::from_millis(500),
    }
}

fn socks5_greeting(stream: &mut TcpStream) {
    stream.write_all(&[0x05, 0x01, 0x00]).unwrap();
    let mut response = [0_u8; 2];
    stream.read_exact(&mut response).unwrap();
    assert_eq!(response, [0x05, 0x00]);
}

fn request_domain(stream: &mut TcpStream, command: u8, host: &str, port: u16) -> u8 {
    let mut request = vec![0x05, command, 0x00, 0x03, u8::try_from(host.len()).unwrap()];
    request.extend_from_slice(host.as_bytes());
    request.extend_from_slice(&port.to_be_bytes());
    stream.write_all(&request).unwrap();

    let mut reply = [0_u8; 10];
    stream.read_exact(&mut reply).unwrap();
    assert_eq!(reply[0], 0x05);
    reply[1]
}

#[test]
fn starts_only_after_sidecar_handshake_and_forwards_allowed_bytes() {
    let (upstream, observed, sidecar) = spawn_fake_sidecar(2);
    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["docs.internal"],
        vec![443],
    ))
    .unwrap();

    assert_eq!(transport.state(), TransportState::Stopped);
    assert_eq!(transport.local_proxy(), None);

    let endpoint = transport.start_proxy().unwrap();
    assert_eq!(transport.state(), TransportState::Ready);
    assert_eq!(transport.local_proxy(), Some(endpoint));
    assert!(endpoint.socket_addr().ip().is_loopback());
    assert_ne!(endpoint.socket_addr().port(), 0);

    let mut client = TcpStream::connect(endpoint.socket_addr()).unwrap();
    client
        .set_read_timeout(Some(Duration::from_secs(1)))
        .unwrap();
    socks5_greeting(&mut client);
    assert_eq!(request_domain(&mut client, 0x01, "docs.internal", 443), 0x00);
    client.write_all(b"ping").unwrap();
    let mut pong = [0_u8; 4];
    client.read_exact(&mut pong).unwrap();
    assert_eq!(&pong, b"pong");

    assert_eq!(
        observed.recv_timeout(Duration::from_secs(1)).unwrap(),
        ObservedConnect {
            host: "docs.internal".to_string(),
            port: 443,
        }
    );

    transport.stop();
    assert_eq!(transport.state(), TransportState::Stopped);
    assert_eq!(transport.local_proxy(), None);
    sidecar.join().unwrap();
}

#[test]
fn disallowed_destination_is_rejected_before_upstream_connect() {
    let (upstream, _observed, sidecar) = spawn_fake_sidecar(1);
    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["docs.internal"],
        vec![443],
    ))
    .unwrap();
    let endpoint = transport.start_proxy().unwrap();

    let mut client = TcpStream::connect(endpoint.socket_addr()).unwrap();
    socks5_greeting(&mut client);
    assert_eq!(request_domain(&mut client, 0x01, "evil.example", 443), 0x02);

    transport.stop();
    sidecar.join().unwrap();
}

#[test]
fn wildcard_matching_requires_a_domain_label_boundary_and_preserves_domain_for_upstream_dns() {
    let (upstream, observed, sidecar) = spawn_fake_sidecar(2);
    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["*.internal"],
        vec![443],
    ))
    .unwrap();
    let endpoint = transport.start_proxy().unwrap();

    let mut denied = TcpStream::connect(endpoint.socket_addr()).unwrap();
    socks5_greeting(&mut denied);
    assert_eq!(request_domain(&mut denied, 0x01, "evilinternal", 443), 0x02);
    drop(denied);

    let mut allowed = TcpStream::connect(endpoint.socket_addr()).unwrap();
    socks5_greeting(&mut allowed);
    assert_eq!(request_domain(&mut allowed, 0x01, "app.internal", 443), 0x00);

    assert_eq!(
        observed.recv_timeout(Duration::from_secs(1)).unwrap(),
        ObservedConnect {
            host: "app.internal".to_string(),
            port: 443,
        }
    );

    transport.stop();
    sidecar.join().unwrap();
}

#[test]
fn destination_port_policy_is_enforced_locally() {
    let (upstream, _observed, sidecar) = spawn_fake_sidecar(1);
    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["docs.internal"],
        vec![443],
    ))
    .unwrap();
    let endpoint = transport.start_proxy().unwrap();

    let mut client = TcpStream::connect(endpoint.socket_addr()).unwrap();
    socks5_greeting(&mut client);
    assert_eq!(request_domain(&mut client, 0x01, "docs.internal", 80), 0x02);

    transport.stop();
    sidecar.join().unwrap();
}

#[test]
fn udp_and_bind_commands_are_rejected_locally() {
    let (upstream, _observed, sidecar) = spawn_fake_sidecar(1);
    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["docs.internal"],
        vec![443],
    ))
    .unwrap();
    let endpoint = transport.start_proxy().unwrap();

    for command in [0x02, 0x03] {
        let mut client = TcpStream::connect(endpoint.socket_addr()).unwrap();
        socks5_greeting(&mut client);
        assert_eq!(request_domain(&mut client, command, "docs.internal", 443), 0x07);
    }

    transport.stop();
    sidecar.join().unwrap();
}

#[test]
fn empty_destination_policy_is_invalid() {
    let upstream = SocketAddr::from((Ipv4Addr::LOCALHOST, 43111));
    assert!(matches!(
        RestrictedSocks5Transport::new(config(upstream, vec![], vec![443])),
        Err(RestrictedProxyError::EmptyAllowedDomains)
    ));
    assert!(matches!(
        RestrictedSocks5Transport::new(config(upstream, vec!["docs.internal"], vec![])),
        Err(RestrictedProxyError::EmptyAllowedPorts)
    ));
}

#[test]
fn plaintext_sidecar_upstream_must_be_loopback() {
    let upstream = SocketAddr::from(([192, 0, 2, 10], 43111));
    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["docs.internal"],
        vec![443],
    ))
    .unwrap();

    assert!(matches!(
        transport.start_proxy(),
        Err(RestrictedProxyError::UpstreamNotLoopback)
    ));
    assert_eq!(transport.state(), TransportState::Offline);
    assert_eq!(transport.local_proxy(), None);
}

#[test]
fn unavailable_sidecar_never_publishes_local_proxy() {
    let reserved = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
    let upstream = reserved.local_addr().unwrap();
    drop(reserved);

    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["docs.internal"],
        vec![443],
    ))
    .unwrap();

    assert!(matches!(
        transport.start_proxy(),
        Err(RestrictedProxyError::UpstreamUnavailable)
    ));
    assert_eq!(transport.state(), TransportState::Offline);
    assert_eq!(transport.local_proxy(), None);
}

#[test]
fn non_socks5_sidecar_never_becomes_ready() {
    let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
    let upstream = listener.local_addr().unwrap();
    let sidecar = thread::spawn(move || {
        let (mut stream, _) = listener.accept().unwrap();
        let mut greeting = [0_u8; 3];
        stream.read_exact(&mut greeting).unwrap();
        stream.write_all(&[0x05, 0xff]).unwrap();
    });

    let mut transport = RestrictedSocks5Transport::new(config(
        upstream,
        vec!["docs.internal"],
        vec![443],
    ))
    .unwrap();

    assert!(matches!(
        transport.start_proxy(),
        Err(RestrictedProxyError::UpstreamProtocol)
    ));
    assert_eq!(transport.state(), TransportState::Offline);
    assert_eq!(transport.local_proxy(), None);
    sidecar.join().unwrap();
}
