#![forbid(unsafe_code)]
#![allow(clippy::unwrap_used, clippy::panic)]

use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpListener};
use webgate_browser::capsule::{BrowserCapsule, CapsuleError};
use webgate_browser::qualification::{
    QualificationRunner, QualificationScenario, RenderingModel, SubresourceRequest,
};
use webgate_browser::{BrowserKind, BrowserLifecycleEvent, BrowserState};
use webgate_core::policy::{NavigationPolicy, PolicyError};

fn spawn_test_loopback_listener() -> (TcpListener, SocketAddr) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let addr = listener.local_addr().unwrap();
    (listener, addr)
}

#[test]
fn browser_capsule_fails_closed_without_proxy() {
    let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
    assert_eq!(capsule.state(), BrowserState::Stopped);

    let err = capsule.start();
    assert_eq!(err, Err(CapsuleError::ProxyMissingFailClosed));
    assert_eq!(capsule.state(), BrowserState::Failed);

    assert_eq!(
        capsule.navigate("webgate://service/factory"),
        Err(CapsuleError::BrowserNotReady(BrowserState::Failed))
    );
}

#[test]
fn browser_capsule_rejects_non_loopback_proxy_and_never_falls_back() {
    let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
    let public_proxy = SocketAddr::new(IpAddr::V4(Ipv4Addr::new(203, 0, 113, 195)), 8080);

    assert_eq!(
        capsule.attach_proxy(public_proxy),
        Err(CapsuleError::DirectEgressForbidden)
    );
    assert_eq!(capsule.start(), Err(CapsuleError::ProxyMissingFailClosed));
    assert_eq!(capsule.state(), BrowserState::Failed);
}

#[test]
fn browser_capsule_enforces_proxy_and_subresource_containment() {
    let (_listener, proxy_addr) = spawn_test_loopback_listener();
    let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());

    capsule.attach_proxy(proxy_addr).unwrap();
    capsule.start().unwrap();
    assert_eq!(capsule.state(), BrowserState::Ready);
    assert!(capsule.adapter().is_some());

    // Navigation to allowed destination
    let entry = capsule.navigate("webgate://service/portal/index").unwrap();
    assert_eq!(entry.target_service_slug(), Some("portal"));

    // Subresource fetch inside allowed policy
    let script = capsule
        .dispatch_subresource_fetch("webgate://service/portal/app.js")
        .unwrap();
    assert!(script.contains("proxied_response"));

    // Subresource fetch with path traversal / disallowed scheme rejected
    assert!(matches!(
        capsule.dispatch_subresource_fetch("file:///etc/passwd"),
        Err(CapsuleError::NavigationPolicyViolation(
            PolicyError::DisallowedScheme(_)
        ))
    ));

    capsule.shutdown();
    assert_eq!(capsule.state(), BrowserState::Stopped);
}

#[test]
fn browser_capsule_preserves_proxy_across_lifecycle_recreation() {
    let (_listener, proxy_addr) = spawn_test_loopback_listener();
    let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());

    capsule.attach_proxy(proxy_addr).unwrap();
    capsule.start().unwrap();

    let entry = capsule
        .navigate("webgate://service/inventory/overview")
        .unwrap();
    let saved_url = entry.as_url_string();

    // Android Pause -> LowMemory -> Resume
    capsule
        .handle_lifecycle_event(BrowserLifecycleEvent::Pause)
        .unwrap();
    assert_eq!(capsule.state(), BrowserState::Paused);

    capsule
        .handle_lifecycle_event(BrowserLifecycleEvent::LowMemory)
        .unwrap();
    assert!(capsule.is_cache_purged());

    capsule
        .handle_lifecycle_event(BrowserLifecycleEvent::Resume)
        .unwrap();
    assert_eq!(capsule.state(), BrowserState::Ready);

    // Shutdown / Recreate
    capsule.shutdown();
    assert_eq!(capsule.state(), BrowserState::Stopped);

    let mut new_capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
    new_capsule.attach_proxy(proxy_addr).unwrap();
    new_capsule.start().unwrap();
    new_capsule
        .handle_lifecycle_event(BrowserLifecycleEvent::RestoreState(saved_url))
        .unwrap();
    assert_eq!(new_capsule.state(), BrowserState::Ready);
    assert_eq!(
        new_capsule.current_url().unwrap().as_url_string(),
        "webgate://service/inventory/overview"
    );
}

#[test]
fn qualification_suite_runs_spa_csr_ssr_scenarios() {
    let (_listener, proxy_addr) = spawn_test_loopback_listener();
    let runner = QualificationRunner::new(proxy_addr);

    // Scenario 1: SPA
    let spa_scenario = QualificationScenario {
        name: "Admin Console SPA".to_string(),
        model: RenderingModel::Spa,
        entry_url: "webgate://service/admin/console".to_string(),
        subresources: vec![
            SubresourceRequest {
                url: "webgate://service/admin/bundle.js".to_string(),
                resource_type: "script",
            },
            SubresourceRequest {
                url: "webgate://service/admin/theme.css".to_string(),
                resource_type: "stylesheet",
            },
        ],
        expected_title: "Admin Console".to_string(),
        expected_signature: "div#admin-root".to_string(),
    };
    let rep1 = runner.run_scenario(&spa_scenario).unwrap();
    assert!(rep1.passed);
    assert!(rep1.proxy_enforced);

    // Scenario 2: CSR
    let csr_scenario = QualificationScenario {
        name: "Telemetry CSR".to_string(),
        model: RenderingModel::Csr,
        entry_url: "webgate://service/telemetry/live".to_string(),
        subresources: vec![SubresourceRequest {
            url: "webgate://service/telemetry/api/data".to_string(),
            resource_type: "fetch_api",
        }],
        expected_title: "Live Telemetry".to_string(),
        expected_signature: "div.chart".to_string(),
    };
    let rep2 = runner.run_scenario(&csr_scenario).unwrap();
    assert!(rep2.passed);
    assert!(rep2.proxy_enforced);

    // Scenario 3: SSR
    let ssr_scenario = QualificationScenario {
        name: "Reports SSR".to_string(),
        model: RenderingModel::Ssr,
        entry_url: "webgate://service/reports/annual".to_string(),
        subresources: vec![SubresourceRequest {
            url: "webgate://service/reports/style.css".to_string(),
            resource_type: "stylesheet",
        }],
        expected_title: "Annual Report".to_string(),
        expected_signature: "article.report".to_string(),
    };
    let rep3 = runner.run_scenario(&ssr_scenario).unwrap();
    assert!(rep3.passed);
    assert!(rep3.proxy_enforced);
}

#[test]
fn browser_capsule_with_dual_relay_failover_proxy_handles_relay_failure() {
    use std::io::{Read, Write};
    use std::sync::Arc;
    use std::sync::atomic::{AtomicBool, Ordering};
    use std::thread;
    use std::time::Duration;
    use webgate_transport::TransportProvider;
    use webgate_transport::dual_failover::{DualRelayConfig, DualRelayFailoverTransport};
    use webgate_transport::failover::{FailoverConfig, TransportRole};

    // Spawn Fake Relay A
    let ln_a = TcpListener::bind("127.0.0.1:0").unwrap();
    let addr_a = ln_a.local_addr().unwrap();
    let shut_a = Arc::new(AtomicBool::new(false));
    let shut_a_clone = Arc::clone(&shut_a);
    let handle_a = thread::spawn(move || {
        while !shut_a_clone.load(Ordering::Acquire) {
            let (mut s, _) = match ln_a.accept() {
                Ok(r) => r,
                Err(_) => break,
            };
            let mut buf = [0_u8; 128];
            let _ = s.read(&mut buf);
            let _ = s.write_all(&[0x05, 0x00]);
            let _ = s.read(&mut buf);
            let _ = s.write_all(&[0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0]);
        }
    });

    // Spawn Fake Relay B
    let ln_b = TcpListener::bind("127.0.0.1:0").unwrap();
    let addr_b = ln_b.local_addr().unwrap();
    let shut_b = Arc::new(AtomicBool::new(false));
    let shut_b_clone = Arc::clone(&shut_b);
    let handle_b = thread::spawn(move || {
        while !shut_b_clone.load(Ordering::Acquire) {
            let (mut s, _) = match ln_b.accept() {
                Ok(r) => r,
                Err(_) => break,
            };
            let mut buf = [0_u8; 128];
            let _ = s.read(&mut buf);
            let _ = s.write_all(&[0x05, 0x00]);
            let _ = s.read(&mut buf);
            let _ = s.write_all(&[0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0]);
        }
    });

    let config = DualRelayConfig {
        name: "BrowserCapsule-Failover".to_string(),
        primary_upstream_host: "127.0.0.1".to_string(),
        primary_upstream_port: addr_a.port(),
        fallback_upstream_host: "127.0.0.1".to_string(),
        fallback_upstream_port: addr_b.port(),
        local_listen_port: 0,
        allowed_domains: vec!["service".to_string()],
        allowed_ports: vec![443],
        connect_timeout: Duration::from_millis(300),
        failover_config: FailoverConfig {
            max_consecutive_failures: 1,
            high_latency_threshold_ms: 1000,
            switchback_cooldown_sec: 10,
        },
    };

    let mut transport = DualRelayFailoverTransport::new(config).unwrap();
    let proxy_ep = transport.start_proxy().unwrap();

    let mut capsule = BrowserCapsule::new(
        BrowserKind::Servo,
        NavigationPolicy::new(vec!["service".to_string()]),
    );
    capsule.attach_proxy(proxy_ep.socket_addr()).unwrap();
    capsule.start().unwrap();
    assert_eq!(capsule.state(), BrowserState::Ready);

    // Initial navigation succeeds through Relay A
    let nav = capsule
        .navigate("webgate://service/finance/overview")
        .unwrap();
    assert_eq!(nav.target_service_slug(), Some("finance"));
    assert_eq!(transport.active_role(), Some(TransportRole::Primary));

    // Relay A crashes
    shut_a.store(true, Ordering::Release);
    let _ = std::net::TcpStream::connect_timeout(&addr_a, Duration::from_millis(50));
    let _ = handle_a.join();

    // Subresource fetch through same proxy automatically triggers failover to Relay B
    let sub = capsule
        .dispatch_subresource_fetch("webgate://service/finance/chart.js")
        .unwrap();
    assert!(sub.contains("proxied_response"));

    transport.stop();
    shut_b.store(true, Ordering::Release);
    let _ = std::net::TcpStream::connect_timeout(&addr_b, Duration::from_millis(50));
    let _ = handle_b.join();
}
