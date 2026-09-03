#!/usr/bin/env python3
from pathlib import Path

# Browser integration tests.
path = Path('crates/webgate-browser/tests/proxy_enforcement.rs')
text = path.read_text()
text = text.replace(
    'use webgate_browser::{BrowserKind, BrowserLifecycleEvent, BrowserState};',
    'use webgate_browser::{\n    BrowserKind, BrowserLifecycleEvent, BrowserState, HttpProxyEndpoint, HttpProxyEndpointError,\n};',
    1,
)
text = text.replace(
    '''fn spawn_test_loopback_listener() -> (TcpListener, SocketAddr) {\n    let listener = TcpListener::bind("127.0.0.1:0").unwrap();\n    let addr = listener.local_addr().unwrap();\n    (listener, addr)\n}\n''',
    '''fn spawn_test_loopback_listener() -> (TcpListener, HttpProxyEndpoint) {\n    let listener = TcpListener::bind("127.0.0.1:0").unwrap();\n    let addr = listener.local_addr().unwrap();\n    let endpoint = HttpProxyEndpoint::new(addr.ip(), addr.port()).unwrap();\n    (listener, endpoint)\n}\n''',
    1,
)
old_negative = '''#[test]\nfn browser_capsule_rejects_non_loopback_proxy_and_never_falls_back() {\n    let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());\n    let public_proxy = SocketAddr::new(IpAddr::V4(Ipv4Addr::new(203, 0, 113, 195)), 8080);\n\n    assert_eq!(\n        capsule.attach_proxy(public_proxy),\n        Err(CapsuleError::DirectEgressForbidden)\n    );\n    assert_eq!(capsule.start(), Err(CapsuleError::ProxyMissingFailClosed));\n    assert_eq!(capsule.state(), BrowserState::Failed);\n}\n'''
new_negative = '''#[test]\nfn browser_capsule_rejects_non_loopback_proxy_and_never_falls_back() {\n    let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());\n    assert_eq!(\n        HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::new(203, 0, 113, 195)), 8080),\n        Err(HttpProxyEndpointError::NotLoopback)\n    );\n    assert_eq!(capsule.start(), Err(CapsuleError::ProxyMissingFailClosed));\n    assert_eq!(capsule.state(), BrowserState::Failed);\n}\n'''
assert text.count(old_negative) == 1
text = text.replace(old_negative, new_negative, 1)
text = text.replace('capsule.attach_proxy(proxy_addr).unwrap();', 'capsule.attach_proxy(proxy_addr);')
text = text.replace('new_capsule.attach_proxy(proxy_addr).unwrap();', 'new_capsule.attach_proxy(proxy_addr);')
# Dual-relay test: add a real protocol adapter rather than lying about SOCKS being HTTP.
text = text.replace(
    '    use webgate_transport::failover::{FailoverConfig, TransportRole};\n',
    '    use webgate_transport::failover::{FailoverConfig, TransportRole};\n    use webgate_transport::restricted_http_connect::{\n        RestrictedHttpConnectConfig, RestrictedHttpConnectTransport,\n    };\n',
    1,
)
old_attach = '''    let mut capsule = BrowserCapsule::new(\n        BrowserKind::Servo,\n        NavigationPolicy::new(vec!["service".to_string()]),\n    );\n    capsule.attach_proxy(proxy_ep.socket_addr()).unwrap();\n    capsule.start().unwrap();\n'''
new_attach = '''    let mut browser_bridge = RestrictedHttpConnectTransport::new(RestrictedHttpConnectConfig {\n        name: "BrowserCapsule-HTTP-Bridge".to_string(),\n        upstream_socks5: proxy_ep,\n        local_listen_port: 0,\n        allowed_domains: vec!["service".to_string()],\n        allowed_ports: vec![443],\n        connect_timeout: Duration::from_millis(300),\n        max_header_bytes: 4096,\n    })\n    .unwrap();\n    let http_proxy = browser_bridge.start_proxy().unwrap();\n    let http_addr = http_proxy.socket_addr();\n    let browser_proxy = HttpProxyEndpoint::new(http_addr.ip(), http_addr.port()).unwrap();\n\n    let mut capsule = BrowserCapsule::new(\n        BrowserKind::Servo,\n        NavigationPolicy::new(vec!["service".to_string()]),\n    );\n    capsule.attach_proxy(browser_proxy);\n    capsule.start().unwrap();\n'''
assert text.count(old_attach) == 1
text = text.replace(old_attach, new_attach, 1)
text = text.replace(
    '''    transport.stop();\n    shut_b.store(true, Ordering::Release);''',
    '''    browser_bridge.stop();\n    transport.stop();\n    shut_b.store(true, Ordering::Release);''',
    1,
)
path.write_text(text)

# App E2E: compose restricted HTTP bridge above the real dual SOCKS proxy.
path = Path('crates/webgate-app/tests/e2e_full_stack.rs')
text = path.read_text()
text = text.replace(
    'use webgate_browser::{BrowserKind, BrowserLifecycleEvent, BrowserState};',
    'use webgate_browser::{BrowserKind, BrowserLifecycleEvent, BrowserState, HttpProxyEndpoint};',
    1,
)
text = text.replace(
    'use webgate_transport::failover::{FailoverConfig, TransportRole};\n',
    'use webgate_transport::failover::{FailoverConfig, TransportRole};\nuse webgate_transport::restricted_http_connect::{\n    RestrictedHttpConnectConfig, RestrictedHttpConnectTransport,\n};\n',
    1,
)
old = '''    // 5. Configure & Start BrowserCapsule attached to Transport SOCKS5 Proxy\n    let mut capsule = BrowserCapsule::new(\n        BrowserKind::Servo,\n        NavigationPolicy::new(vec!["service".to_string(), "corp.internal".to_string()]),\n    );\n    capsule.attach_proxy(proxy_endpoint.socket_addr()).unwrap();\n    capsule.start().unwrap();\n'''
new = '''    // 5. Adapt the restricted SOCKS5 transport into Servo-compatible HTTP CONNECT.\n    let mut browser_bridge = RestrictedHttpConnectTransport::new(RestrictedHttpConnectConfig {\n        name: "E2E-Browser-HTTP-Bridge".to_string(),\n        upstream_socks5: proxy_endpoint,\n        local_listen_port: 0,\n        allowed_domains: vec!["service".to_string(), "corp.internal".to_string()],\n        allowed_ports: vec![backend_addr.port(), 443],\n        connect_timeout: Duration::from_millis(300),\n        max_header_bytes: 4096,\n    })\n    .unwrap();\n    let http_proxy = browser_bridge.start_proxy().unwrap();\n    let http_addr = http_proxy.socket_addr();\n    let browser_proxy = HttpProxyEndpoint::new(http_addr.ip(), http_addr.port()).unwrap();\n\n    let mut capsule = BrowserCapsule::new(\n        BrowserKind::Servo,\n        NavigationPolicy::new(vec!["service".to_string(), "corp.internal".to_string()]),\n    );\n    capsule.attach_proxy(browser_proxy);\n    capsule.start().unwrap();\n'''
assert text.count(old) == 1
text = text.replace(old, new, 1)
text = text.replace(
    '''    capsule.shutdown();\n    transport.stop();''',
    '''    capsule.shutdown();\n    browser_bridge.stop();\n    transport.stop();''',
    1,
)
path.write_text(text)
