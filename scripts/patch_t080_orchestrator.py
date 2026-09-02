#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-app/src/main.rs')
s = path.read_text()


def replace_once(old: str, new: str) -> None:
    global s
    count = s.count(old)
    assert count == 1, (old[:120], count)
    s = s.replace(old, new, 1)


replace_once(
    '#![forbid(unsafe_code)]\n\n',
    '#![forbid(unsafe_code)]\n\nmod session;\n\n',
)
replace_once(
    'use std::sync::{Arc, RwLock};',
    'use std::sync::{Arc, Mutex, RwLock};',
)
replace_once(
    'use webgate_browser::BrowserKind;\nuse webgate_browser::capsule::BrowserCapsule;\n',
    'use session::{ApplicationSessionManager, ApplicationSessionSnapshot, ApplicationSessionState};\n',
)

proxy_anchor = '''fn proxy_json(endpoint: Option<LocalProxyEndpoint>) -> String {
    match endpoint {
        Some(endpoint) => format!("\\\"{}\\\"", endpoint.socket_addr()),
        None => "null".to_string(),
    }
}
'''
proxy_extension = proxy_anchor + '''
fn session_http_status(state: ApplicationSessionState) -> &'static str {
    match state {
        ApplicationSessionState::Open | ApplicationSessionState::Closed => "200 OK",
        ApplicationSessionState::Denied => "403 Forbidden",
        ApplicationSessionState::Offline | ApplicationSessionState::RendererUnqualified => {
            "503 Service Unavailable"
        }
        ApplicationSessionState::Failed => "500 Internal Server Error",
        ApplicationSessionState::Requested
        | ApplicationSessionState::Authorizing
        | ApplicationSessionState::TransportReady
        | ApplicationSessionState::StartingProtectedBrowser
        | ApplicationSessionState::Navigating => "202 Accepted",
    }
}

fn session_to_json(snapshot: &ApplicationSessionSnapshot) -> String {
    let ok = snapshot.state == ApplicationSessionState::Open;
    format!(
        r#"{{"ok":{},"state":"{}","session_id":"{}","target":"{}","message":"{}"}}"#,
        ok,
        snapshot.state.as_str(),
        escape_json(&snapshot.session_id),
        escape_json(&snapshot.target_url),
        escape_json(&snapshot.message)
    )
}
'''
replace_once(proxy_anchor, proxy_extension)

sig_old = '''fn handle_client_stream(
    mut stream: TcpStream,
    profile_arc: &Arc<RwLock<ClientConfigProfile>>,
    keystore_id: &str,
    transport_state: TransportState,
    protected_proxy: Option<LocalProxyEndpoint>,
) {'''
sig_new = '''fn handle_client_stream(
    mut stream: TcpStream,
    profile_arc: &Arc<RwLock<ClientConfigProfile>>,
    session_manager: &Arc<Mutex<ApplicationSessionManager>>,
    keystore_id: &str,
    transport_state: TransportState,
    protected_proxy: Option<LocalProxyEndpoint>,
) {'''
replace_once(sig_old, sig_new)

navigate_anchor = '''    if method == "POST" && path == "/api/navigate" {
'''
session_routes = '''    if method == "POST" && path == "/api/session/open" {
        let body = match req_str.find("\\r\\n\\r\\n") {
            Some(idx) => &req_str[idx + 4..],
            None => "",
        };
        let Some(target_url) = extract_json_string_field(body, "target_url") else {
            write_json_response(
                &mut stream,
                "400 Bad Request",
                r#"{"ok":false,"state":"invalid_request","message":"target_url is required"}"#,
            );
            return;
        };
        if target_url.trim().is_empty() {
            write_json_response(
                &mut stream,
                "400 Bad Request",
                r#"{"ok":false,"state":"invalid_request","message":"target_url is empty"}"#,
            );
            return;
        }

        let profile = match profile_arc.read() {
            Ok(profile) => profile.clone(),
            Err(_) => {
                write_json_response(
                    &mut stream,
                    "500 Internal Server Error",
                    r#"{"ok":false,"state":"failed","message":"profile state unavailable"}"#,
                );
                return;
            }
        };
        let mut manager = match session_manager.lock() {
            Ok(manager) => manager,
            Err(_) => {
                write_json_response(
                    &mut stream,
                    "500 Internal Server Error",
                    r#"{"ok":false,"state":"failed","message":"session manager unavailable"}"#,
                );
                return;
            }
        };
        let snapshot = manager.open_application(
            &profile,
            &target_url,
            transport_state,
            protected_proxy,
        );
        write_json_response(
            &mut stream,
            session_http_status(snapshot.state),
            &session_to_json(&snapshot),
        );
        return;
    }

    if method == "POST" && path == "/api/session/close" {
        let body = match req_str.find("\\r\\n\\r\\n") {
            Some(idx) => &req_str[idx + 4..],
            None => "",
        };
        let Some(session_id) = extract_json_string_field(body, "session_id") else {
            write_json_response(
                &mut stream,
                "400 Bad Request",
                r#"{"ok":false,"state":"invalid_request","message":"session_id is required"}"#,
            );
            return;
        };
        let mut manager = match session_manager.lock() {
            Ok(manager) => manager,
            Err(_) => {
                write_json_response(
                    &mut stream,
                    "500 Internal Server Error",
                    r#"{"ok":false,"state":"failed","message":"session manager unavailable"}"#,
                );
                return;
            }
        };
        match manager.close(&session_id) {
            Some(snapshot) => write_json_response(
                &mut stream,
                session_http_status(snapshot.state),
                &session_to_json(&snapshot),
            ),
            None => write_json_response(
                &mut stream,
                "404 Not Found",
                r#"{"ok":false,"state":"not_found","message":"session not found"}"#,
            ),
        }
        return;
    }

'''
replace_once(navigate_anchor, session_routes + navigate_anchor)

cli_old = '''        if gate.verify_request(&nav_req).is_ok() {
            println!("  [Брокер IPC] Запрос авторизован с привилегией: NavigateService");
            let policy = profile.build_navigation_policy();
            let mut capsule = BrowserCapsule::new(BrowserKind::Servo, policy);
            if let Some(ep) = proxy_ep {
                let proxy_attached = capsule.attach_proxy(ep.socket_addr()).is_ok();
                let capsule_started = capsule.start().is_ok();
                let navigated = capsule.navigate(&target_destination).is_ok();
                if proxy_attached && capsule_started && navigated {
                    println!("  [Капсула] Соединение установлено: {target_destination}");
                    println!(
                        "  [Капсула] Граница изоляции активна. Сетевые маршруты ОС не затронуты."
                    );
                }
            } else {
                println!(
                    "  [Транспорт] OFFLINE: реальный защищённый proxy/tunnel не подтверждён; навигация запрещена."
                );
            }
        }
'''
cli_new = '''        if gate.verify_request(&nav_req).is_ok() {
            println!("  [Брокер IPC] Запрос авторизован с привилегией: NavigateService");
            let mut session_manager = ApplicationSessionManager::new();
            let snapshot = session_manager.open_application(
                &profile,
                &target_destination,
                transport_state,
                proxy_ep,
            );
            println!(
                "  [Сессия {}] {}: {}",
                snapshot.session_id,
                snapshot.state.as_str(),
                snapshot.message
            );
            if snapshot.state != ApplicationSessionState::Open {
                eprintln!(
                    "  [Капсула] Защищённый Open НЕ подтверждён; небезопасный браузерный fallback запрещён."
                );
            }
        }
'''
replace_once(cli_old, cli_new)

server_old = '''    let profile_arc = Arc::new(RwLock::new(profile));
    let profile_clone = Arc::clone(&profile_arc);
    let dev_id_clone = device_id.clone();

    let server_handle = thread::spawn(move || {
        for s in listener.incoming().flatten() {
            handle_client_stream(s, &profile_clone, &dev_id_clone, transport_state, proxy_ep);
        }
    });
'''
server_new = '''    let profile_arc = Arc::new(RwLock::new(profile));
    let profile_clone = Arc::clone(&profile_arc);
    let session_manager = Arc::new(Mutex::new(ApplicationSessionManager::new()));
    let session_manager_clone = Arc::clone(&session_manager);
    let dev_id_clone = device_id.clone();

    let server_handle = thread::spawn(move || {
        for s in listener.incoming().flatten() {
            handle_client_stream(
                s,
                &profile_clone,
                &session_manager_clone,
                &dev_id_clone,
                transport_state,
                proxy_ep,
            );
        }
    });
'''
replace_once(server_old, server_new)

helper_old = '''        let profile_arc = Arc::new(RwLock::new(ClientConfigProfile::default()));
        let profile_clone = Arc::clone(&profile_arc);
        let request = request.to_string();

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                "dev_test_123",
                transport_state,
                proxy,
            );
        });
'''
helper_new = '''        let profile_arc = Arc::new(RwLock::new(ClientConfigProfile::default()));
        let profile_clone = Arc::clone(&profile_arc);
        let session_manager = Arc::new(Mutex::new(ApplicationSessionManager::new()));
        let session_manager_clone = Arc::clone(&session_manager);
        let request = request.to_string();

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                &session_manager_clone,
                "dev_test_123",
                transport_state,
                proxy,
            );
        });
'''
replace_once(helper_old, helper_new)

# Two direct bind-config tests create their own server threads. Add a manager to
# each local fixture and pass it into handle_client_stream.
needle = '''        let profile_arc = Arc::new(RwLock::new(ClientConfigProfile::default()));
        let profile_clone = Arc::clone(&profile_arc);

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                "dev_test_123",
                TransportState::Ready,
                None,
            );
        });
'''
replacement = '''        let profile_arc = Arc::new(RwLock::new(ClientConfigProfile::default()));
        let profile_clone = Arc::clone(&profile_arc);
        let session_manager = Arc::new(Mutex::new(ApplicationSessionManager::new()));
        let session_manager_clone = Arc::clone(&session_manager);

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                &session_manager_clone,
                "dev_test_123",
                TransportState::Ready,
                None,
            );
        });
'''
count = s.count(needle)
assert count == 2, count
s = s.replace(needle, replacement)

# Add API characterization next to the existing transport_ready route test.
api_test_anchor = '''    #[test]
    fn transactional_bind_config_updates_profile_on_valid_payload() {
'''
api_tests = '''    #[test]
    fn session_open_invokes_capsule_but_blocks_unqualified_renderer() {
        let proxy = LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43117).unwrap();
        let body = r#"{"target_url":"webgate://service/docs/overview"}"#;
        let request = format!(
            "POST /api/session/open HTTP/1.1\\r\\nHost: localhost\\r\\nContent-Type: application/json\\r\\nContent-Length: {}\\r\\nConnection: close\\r\\n\\r\\n{}",
            body.len(),
            body
        );
        let response = http_roundtrip(&request, TransportState::Ready, Some(proxy));
        assert!(response.starts_with("HTTP/1.1 503 Service Unavailable"));
        assert!(response.contains("\\\"ok\\\":false"));
        assert!(response.contains("\\\"state\\\":\\\"renderer_unqualified\\\""));
        assert!(response.contains("\\\"session_id\\\":\\\"wgs-"));
        assert!(!response.contains("\\\"state\\\":\\\"open\\\""));
    }

    #[test]
    fn session_open_offline_never_claims_open() {
        let body = r#"{"target_url":"webgate://service/docs/overview"}"#;
        let request = format!(
            "POST /api/session/open HTTP/1.1\\r\\nHost: localhost\\r\\nContent-Type: application/json\\r\\nContent-Length: {}\\r\\nConnection: close\\r\\n\\r\\n{}",
            body.len(),
            body
        );
        let response = http_roundtrip(&request, TransportState::Offline, None);
        assert!(response.starts_with("HTTP/1.1 503 Service Unavailable"));
        assert!(response.contains("\\\"state\\\":\\\"offline\\\""));
        assert!(!response.contains("\\\"state\\\":\\\"open\\\""));
    }

'''
replace_once(api_test_anchor, api_tests + api_test_anchor)

path.write_text(s)
