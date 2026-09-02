#![forbid(unsafe_code)]

use crate::TransportState;
use std::io::{self, ErrorKind, Read, Write};
use std::net::{IpAddr, Shutdown, SocketAddr, TcpStream};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread;
use std::time::Duration;

pub const STATE_STOPPED: u8 = 0;
pub const STATE_STARTING: u8 = 1;
pub const STATE_READY: u8 = 2;
pub const STATE_DEGRADED: u8 = 3;
pub const STATE_OFFLINE: u8 = 4;
pub const STREAM_POLL_TIMEOUT: Duration = Duration::from_millis(200);

pub fn encode_state(state: TransportState) -> u8 {
    match state {
        TransportState::Stopped => STATE_STOPPED,
        TransportState::Starting => STATE_STARTING,
        TransportState::Ready => STATE_READY,
        TransportState::Degraded => STATE_DEGRADED,
        TransportState::Offline => STATE_OFFLINE,
    }
}

pub fn decode_state(state: u8) -> TransportState {
    match state {
        STATE_STOPPED => TransportState::Stopped,
        STATE_STARTING => TransportState::Starting,
        STATE_READY => TransportState::Ready,
        STATE_DEGRADED => TransportState::Degraded,
        _ => TransportState::Offline,
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PolicyValidationError {
    EmptyAllowedDomains,
    EmptyAllowedPorts,
    InvalidAllowedDomain,
    InvalidAllowedPort,
}

#[derive(Debug, Clone)]
pub struct DestinationPolicy {
    pub allowed_domains: Vec<String>,
    pub allowed_ports: Vec<u16>,
}

impl DestinationPolicy {
    pub fn new(domains: &[String], ports: &[u16]) -> Result<Self, PolicyValidationError> {
        if domains.is_empty() {
            return Err(PolicyValidationError::EmptyAllowedDomains);
        }
        if ports.is_empty() {
            return Err(PolicyValidationError::EmptyAllowedPorts);
        }
        if ports.contains(&0) {
            return Err(PolicyValidationError::InvalidAllowedPort);
        }

        let mut allowed_domains = Vec::with_capacity(domains.len());
        for raw in domains {
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
                return Err(PolicyValidationError::InvalidAllowedDomain);
            }
            allowed_domains.push(normalized);
        }

        let mut allowed_ports = ports.to_vec();
        allowed_ports.sort_unstable();
        allowed_ports.dedup();

        Ok(Self {
            allowed_domains,
            allowed_ports,
        })
    }

    #[must_use]
    pub fn allows(&self, target: &SocksTarget, port: u16) -> bool {
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
pub enum SocksTarget {
    Domain(String),
    Ip(IpAddr),
}

pub fn normalize_target_domain(raw: &str) -> Option<String> {
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

pub fn configure_handshake_timeouts(stream: &TcpStream, timeout: Duration) -> io::Result<()> {
    stream.set_read_timeout(Some(timeout))?;
    stream.set_write_timeout(Some(timeout))
}

pub fn probe_socks5_sidecar(upstream: SocketAddr, timeout: Duration) -> io::Result<()> {
    let mut stream = TcpStream::connect_timeout(&upstream, timeout)?;
    configure_handshake_timeouts(&stream, timeout)?;
    socks5_negotiate_no_auth(&mut stream)
}

pub fn socks5_negotiate_no_auth(stream: &mut TcpStream) -> io::Result<()> {
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

pub fn negotiate_local_client(client: &mut TcpStream) -> io::Result<bool> {
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

pub fn read_target(stream: &mut TcpStream, atyp: u8) -> io::Result<SocksTarget> {
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

pub fn write_connect_request(
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

pub fn read_upstream_reply(stream: &mut TcpStream) -> io::Result<(u8, Vec<u8>)> {
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

pub fn write_reply(stream: &mut TcpStream, code: u8) -> io::Result<()> {
    stream.write_all(&[0x05, code, 0x00, 0x01, 0, 0, 0, 0, 0, 0])
}

pub fn relay_bidirectional(
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

pub fn copy_until_shutdown(mut reader: TcpStream, mut writer: TcpStream, shutdown: &AtomicBool) {
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
