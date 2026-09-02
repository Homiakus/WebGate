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
    let entry = capsule
        .navigate("webgate://service/portal/index")
        .unwrap();
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
