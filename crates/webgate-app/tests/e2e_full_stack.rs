#![forbid(unsafe_code)]
#![allow(clippy::expect_used, clippy::panic, clippy::unwrap_used)]

use std::fs;
use std::io::{Read, Write};
use std::net::{Ipv4Addr, SocketAddr, TcpListener, TcpStream};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread::{self, JoinHandle};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use webgate_browser::capsule::BrowserCapsule;
use webgate_browser::{BrowserKind, BrowserLifecycleEvent, BrowserState, HttpProxyEndpoint};
use webgate_core::device::KeyAlgorithm;
use webgate_core::policy::NavigationPolicy;
use webgate_platform::keystore::{DeviceKeyStore, PersistentFileDeviceKeyStore};
use webgate_transport::dual_failover::{DualRelayConfig, DualRelayFailoverTransport};
use webgate_transport::failover::{FailoverConfig, TransportRole};
use webgate_transport::restricted_http_connect::{
    RestrictedHttpConnectConfig, RestrictedHttpConnectTransport,
};
use webgate_transport::{TransportProvider, TransportState};

struct MockSocks5Relay {
    pub addr: SocketAddr,
    shutdown: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
}

impl MockSocks5Relay {
    fn spawn(target_upstream: SocketAddr) -> Self {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let addr = listener.local_addr().unwrap();
        let shutdown = Arc::new(AtomicBool::new(false));
        let shutdown_clone = Arc::clone(&shutdown);

        let handle = thread::spawn(move || {
            while !shutdown_clone.load(Ordering::Acquire) {
                let (mut client_stream, _) = match listener.accept() {
                    Ok(res) => res,
                    Err(_) => break,
                };

                let _ = client_stream.set_read_timeout(Some(Duration::from_millis(500)));
                let _ = client_stream.set_write_timeout(Some(Duration::from_millis(500)));

                // SOCKS5 Greeting
                let mut greeting = [0_u8; 2];
                if client_stream.read_exact(&mut greeting).is_err() || greeting[0] != 0x05 {
                    continue;
                }
                let mut methods = vec![0_u8; usize::from(greeting[1])];
                if client_stream.read_exact(&mut methods).is_err() || !methods.contains(&0x00) {
                    continue;
                }
                if client_stream.write_all(&[0x05, 0x00]).is_err() {
                    continue;
                }

                // SOCKS5 Request
                let mut request = [0_u8; 4];
                match client_stream.read_exact(&mut request) {
                    Ok(()) => {}
                    Err(_) => continue, // probe-only connection
                }

                if request[0] != 0x05 || request[1] != 0x01 {
                    continue;
                }

                // Read target address
                match request[3] {
                    0x01 => {
                        let mut ip = [0_u8; 4];
                        let _ = client_stream.read_exact(&mut ip);
                    }
                    0x03 => {
                        let mut len = [0_u8; 1];
                        let _ = client_stream.read_exact(&mut len);
                        let mut domain = vec![0_u8; usize::from(len[0])];
                        let _ = client_stream.read_exact(&mut domain);
                    }
                    _ => continue,
                }
                let mut port_bytes = [0_u8; 2];
                let _ = client_stream.read_exact(&mut port_bytes);

                // Forward to target upstream
                let mut upstream_stream = match TcpStream::connect_timeout(
                    &target_upstream,
                    Duration::from_millis(500),
                ) {
                    Ok(conn) => conn,
                    Err(_) => {
                        let _ =
                            client_stream.write_all(&[0x05, 0x05, 0x00, 0x01, 127, 0, 0, 1, 0, 0]);
                        continue;
                    }
                };

                if client_stream
                    .write_all(&[0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0])
                    .is_err()
                {
                    continue;
                }

                // Bidirectional forward
                let mut client_clone = client_stream.try_clone().unwrap();
                let mut upstream_clone = upstream_stream.try_clone().unwrap();

                let t1 = thread::spawn(move || {
                    let mut buf = [0_u8; 4096];
                    loop {
                        match client_clone.read(&mut buf) {
                            Ok(0) | Err(_) => break,
                            Ok(n) => {
                                if upstream_clone.write_all(&buf[..n]).is_err() {
                                    break;
                                }
                            }
                        }
                    }
                });

                let mut buf = [0_u8; 4096];
                loop {
                    match upstream_stream.read(&mut buf) {
                        Ok(0) | Err(_) => break,
                        Ok(n) => {
                            if client_stream.write_all(&buf[..n]).is_err() {
                                break;
                            }
                        }
                    }
                }
                let _ = t1.join();
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

impl Drop for MockSocks5Relay {
    fn drop(&mut self) {
        self.stop();
    }
}

// Helper to perform HTTP request through SOCKS5 proxy
fn socks5_http_get(
    proxy_addr: SocketAddr,
    target_domain: &str,
    target_port: u16,
    path: &str,
) -> Result<(u16, String), String> {
    let mut stream = TcpStream::connect_timeout(&proxy_addr, Duration::from_secs(2))
        .map_err(|e| format!("connect proxy: {e}"))?;
    stream
        .set_read_timeout(Some(Duration::from_secs(2)))
        .unwrap();
    stream
        .set_write_timeout(Some(Duration::from_secs(2)))
        .unwrap();

    // SOCKS5 handshake
    stream.write_all(&[0x05, 0x01, 0x00]).unwrap();
    let mut greeting_resp = [0_u8; 2];
    stream.read_exact(&mut greeting_resp).unwrap();
    if greeting_resp != [0x05, 0x00] {
        return Err("socks5 no-auth rejected".to_string());
    }

    // Connect request
    let domain_bytes = target_domain.as_bytes();
    let mut req = vec![0x05, 0x01, 0x00, 0x03, domain_bytes.len() as u8];
    req.extend_from_slice(domain_bytes);
    req.extend_from_slice(&target_port.to_be_bytes());
    stream.write_all(&req).unwrap();

    let mut resp_hdr = [0_u8; 4];
    stream.read_exact(&mut resp_hdr).unwrap();
    if resp_hdr[1] != 0x00 {
        return Err(format!("socks5 connect error code: {}", resp_hdr[1]));
    }
    // Read bound addr
    match resp_hdr[3] {
        0x01 => {
            let mut buf = [0_u8; 6];
            stream.read_exact(&mut buf).unwrap();
        }
        0x03 => {
            let mut len = [0_u8; 1];
            stream.read_exact(&mut len).unwrap();
            let mut buf = vec![0_u8; usize::from(len[0]) + 2];
            stream.read_exact(&mut buf).unwrap();
        }
        _ => return Err("unsupported atyp".to_string()),
    }

    // Send HTTP GET request
    let http_req = format!(
        "GET {} HTTP/1.1\r\nHost: {}:{}\r\nConnection: close\r\n\r\n",
        path, target_domain, target_port
    );
    stream.write_all(http_req.as_bytes()).unwrap();

    let mut resp_str = String::new();
    stream.read_to_string(&mut resp_str).unwrap();

    let first_line = resp_str.lines().next().unwrap_or("");
    let status_code = if first_line.contains("200") {
        200
    } else if first_line.contains("404") {
        404
    } else if first_line.contains("403") {
        403
    } else if first_line.contains("401") {
        401
    } else {
        500
    };

    Ok((status_code, resp_str))
}

#[test]
fn test_real_end_to_end_full_stack_qualification() {
    // 1. Spawn Mock HTTP Backend Service
    let backend_listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let backend_addr = backend_listener.local_addr().unwrap();
    let backend_shutdown = Arc::new(AtomicBool::new(false));
    let backend_shutdown_clone = Arc::clone(&backend_shutdown);

    let backend_handle = thread::spawn(move || {
        while !backend_shutdown_clone.load(Ordering::Acquire) {
            let (mut stream, _) = match backend_listener.accept() {
                Ok(res) => res,
                Err(_) => break,
            };
            let mut buf = [0_u8; 1024];
            let n = stream.read(&mut buf).unwrap_or(0);
            let req = String::from_utf8_lossy(&buf[..n]);

            if req.contains("GET /reports/quarterly") {
                let body = r#"{"report":"Q3-Financials","status":"certified"}"#;
                let resp = format!(
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                    body.len(),
                    body
                );
                let _ = stream.write_all(resp.as_bytes());
            } else {
                let _ = stream.write_all(
                    b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
                );
            }
        }
    });

    // 2. Spawn Mock Relay A (Primary) and Relay B (Fallback)
    let mut relay_a = MockSocks5Relay::spawn(backend_addr);
    let relay_b = MockSocks5Relay::spawn(backend_addr);

    // 3. Platform Key Store (Ed25519 Device Key Qualification)
    let unique_name = format!(
        "qualified_device_{}.key",
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_nanos()
    );
    let key_path = std::env::temp_dir().join(unique_name);
    let mut key_store = PersistentFileDeviceKeyStore::open(&key_path).unwrap();
    let ident = key_store
        .generate_key(KeyAlgorithm::Ed25519, "e2e-qualified-device")
        .unwrap();
    assert!(
        !ident.public_key_hex.is_empty(),
        "device key must be generated"
    );

    let signature = key_store
        .sign_payload(b"test-pop-challenge-nonce-001")
        .unwrap();
    assert_eq!(
        signature.len(),
        64,
        "Ed25519 raw signature must be 64 bytes"
    );

    // 4. Configure Dual Relay Failover Transport with Destination Policy
    let transport_config = DualRelayConfig {
        name: "E2E-Qualified-Transport".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: relay_a.addr.port(),
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: relay_b.addr.port(),
        local_listen_port: 0,
        allowed_domains: vec!["service".to_string(), "corp.internal".to_string()],
        allowed_ports: vec![backend_addr.port(), 443],
        connect_timeout: Duration::from_millis(300),
        failover_config: FailoverConfig {
            max_consecutive_failures: 1,
            high_latency_threshold_ms: 1000,
            switchback_cooldown_sec: 1,
        },
    };

    let mut transport = DualRelayFailoverTransport::new(transport_config).unwrap();
    let proxy_endpoint = transport.start_proxy().unwrap();
    assert_eq!(transport.state(), TransportState::Ready);
    assert_eq!(transport.active_role(), Some(TransportRole::Primary));

    // 5. Adapt the restricted SOCKS5 transport into Servo-compatible HTTP CONNECT.
    let mut browser_bridge = RestrictedHttpConnectTransport::new(RestrictedHttpConnectConfig {
        name: "E2E-Browser-HTTP-Bridge".to_string(),
        upstream_socks5: proxy_endpoint,
        local_listen_port: 0,
        allowed_domains: vec!["service".to_string(), "corp.internal".to_string()],
        allowed_ports: vec![backend_addr.port(), 443],
        connect_timeout: Duration::from_millis(300),
        max_header_bytes: 4096,
    })
    .unwrap();
    let http_proxy = browser_bridge.start_proxy().unwrap();
    let http_addr = http_proxy.socket_addr();
    let browser_proxy = HttpProxyEndpoint::new(http_addr.ip(), http_addr.port()).unwrap();

    let mut capsule = BrowserCapsule::new(
        BrowserKind::Servo,
        NavigationPolicy::new(vec!["service".to_string(), "corp.internal".to_string()]),
    );
    capsule.attach_proxy(browser_proxy);
    capsule.start().unwrap();
    assert_eq!(capsule.state(), BrowserState::Ready);

    // 6. Test Browser Navigation & Subresource Fetch
    let nav_url = capsule
        .navigate("webgate://service/reports/quarterly")
        .unwrap();
    assert_eq!(nav_url.target_service_slug(), Some("reports"));

    let subresource = capsule
        .dispatch_subresource_fetch("webgate://service/reports/style.css")
        .unwrap();
    assert!(subresource.contains("proxied_response"));

    // 7. Test Real HTTP Request Roundtrip via SOCKS5 Proxy over Primary Relay
    let (status, resp_body) = socks5_http_get(
        proxy_endpoint.socket_addr(),
        "service",
        backend_addr.port(),
        "/reports/quarterly",
    )
    .unwrap();
    assert_eq!(status, 200);
    assert!(resp_body.contains("Q3-Financials"));

    // 8. Test Live Primary Relay Crash -> Failover to Fallback Relay
    relay_a.stop();

    // Next request to proxy triggers failover to Relay B
    let (status_fb, resp_body_fb) = socks5_http_get(
        proxy_endpoint.socket_addr(),
        "service",
        backend_addr.port(),
        "/reports/quarterly",
    )
    .unwrap();
    assert_eq!(status_fb, 200);
    assert!(resp_body_fb.contains("Q3-Financials"));
    assert_eq!(transport.active_role(), Some(TransportRole::Fallback));

    // 9. Test Destination Policy Rejection Fail-Closed (Disallowed Domain & Port)
    let disallowed_domain_res = socks5_http_get(
        proxy_endpoint.socket_addr(),
        "malicious.attacker.com",
        backend_addr.port(),
        "/leak",
    );
    assert!(
        disallowed_domain_res.is_err(),
        "disallowed domain must fail closed at proxy layer"
    );

    let disallowed_port_res = socks5_http_get(
        proxy_endpoint.socket_addr(),
        "service",
        9999, // port not in allowed_ports
        "/reports/quarterly",
    );
    assert!(
        disallowed_port_res.is_err(),
        "disallowed port must fail closed at proxy layer"
    );

    // 10. Platform Lifecycle Transitions
    capsule
        .handle_lifecycle_event(BrowserLifecycleEvent::Pause)
        .unwrap();
    assert_eq!(capsule.state(), BrowserState::Paused);
    capsule
        .handle_lifecycle_event(BrowserLifecycleEvent::Resume)
        .unwrap();
    assert_eq!(capsule.state(), BrowserState::Ready);

    // Cleanup
    let _ = fs::remove_file(&key_path);
    capsule.shutdown();
    browser_bridge.stop();
    transport.stop();
    backend_shutdown.store(true, Ordering::Release);
    let _ = TcpStream::connect_timeout(&backend_addr, Duration::from_millis(50));
    let _ = backend_handle.join();
}
