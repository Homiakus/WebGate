#![forbid(unsafe_code)]

use std::env;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::process::Command;
use std::sync::{Arc, RwLock};
use std::thread;
use std::time::Duration;

use webgate_browser::BrowserKind;
use webgate_browser::capsule::BrowserCapsule;
use webgate_core::broker::{
    BrokerCapability, BrokerRequest, BrokerRequestPayload, BrokerSecurityGate,
};
use webgate_core::config::{ClientConfigProfile, ConfigError, RelayEndpointConfig};
use webgate_platform::current_platform;
use webgate_platform::keystore::{DeviceKeyStore, PersistentFileDeviceKeyStore};
use webgate_transport::dual_failover::{
    DualRelayConfig, DualRelayError, DualRelayFailoverTransport,
};
use webgate_transport::failover::FailoverConfig;
use webgate_transport::restricted_socks5::{
    RestrictedProxyError, RestrictedSocks5Config, RestrictedSocks5Transport,
};
use webgate_transport::{LocalProxyEndpoint, TransportProvider, TransportState};

const CLIENT_UI_HTML: &str = include_str!("client_ui.html");
const CLIENT_UI_TRUTH_PATCH_JS: &str = include_str!("client_ui_truth_patch.js");
const PRIMARY_PROXY_CONNECT_TIMEOUT: Duration = Duration::from_secs(2);

fn load_client_profile(config_path: Option<&str>) -> Result<ClientConfigProfile, ConfigError> {
    match config_path {
        Some(path) => ClientConfigProfile::load_from_file(path),
        None => Ok(ClientConfigProfile::default()),
    }
}

fn build_primary_transport(
    profile: &ClientConfigProfile,
) -> Result<RestrictedSocks5Transport, RestrictedProxyError> {
    RestrictedSocks5Transport::new(RestrictedSocks5Config {
        name: profile.primary_relay.name.clone(),
        upstream_host: profile.primary_relay.address.clone(),
        upstream_port: profile.primary_relay.port,
        local_listen_port: 0,
        allowed_domains: profile.allowed_domains.clone(),
        allowed_ports: vec![443],
        connect_timeout: PRIMARY_PROXY_CONNECT_TIMEOUT,
    })
}

fn build_dual_relay_transport(
    profile: &ClientConfigProfile,
    fallback: &RelayEndpointConfig,
) -> Result<DualRelayFailoverTransport, DualRelayError> {
    DualRelayFailoverTransport::new(DualRelayConfig {
        name: format!("{} / {}", profile.primary_relay.name, fallback.name),
        primary_upstream_host: profile.primary_relay.address.clone(),
        primary_upstream_port: profile.primary_relay.port,
        fallback_upstream_host: fallback.address.clone(),
        fallback_upstream_port: fallback.port,
        local_listen_port: 0,
        allowed_domains: profile.allowed_domains.clone(),
        allowed_ports: vec![443],
        connect_timeout: PRIMARY_PROXY_CONNECT_TIMEOUT,
        failover_config: FailoverConfig::default(),
    })
}

const fn transport_state_label(state: TransportState) -> &'static str {
    match state {
        TransportState::Stopped => "stopped",
        TransportState::Starting => "starting",
        TransportState::Ready => "ready",
        TransportState::Degraded => "degraded",
        TransportState::Offline => "offline",
    }
}

fn proxy_json(endpoint: Option<LocalProxyEndpoint>) -> String {
    match endpoint {
        Some(endpoint) => format!("\"{}\"", endpoint.socket_addr()),
        None => "null".to_string(),
    }
}

fn profile_to_json(profile: &ClientConfigProfile) -> String {
    let mut dests = Vec::new();
    for d in &profile.destinations {
        dests.push(format!(
            r#"{{"id":"{}","name":"{}","url":"{}","category":"{}","description":"{}"}}"#,
            escape_json(&d.id),
            escape_json(&d.name),
            escape_json(&d.url),
            escape_json(&d.category),
            escape_json(&d.description)
        ));
    }
    let fallback = match &profile.fallback_relay {
        Some(fb) => format!(
            r#"{{"name":"{}","address":"{}","port":{}}}"#,
            escape_json(&fb.name),
            escape_json(&fb.address),
            fb.port
        ),
        None => "null".to_string(),
    };
    format!(
        r#"{{"profile_id":"{}","profile_name":"{}","version":"{}","primary_relay":{{"name":"{}","address":"{}","port":{}}},"fallback_relay":{},"destinations":[{}]}}"#,
        escape_json(&profile.profile_id),
        escape_json(&profile.profile_name),
        escape_json(&profile.version),
        escape_json(&profile.primary_relay.name),
        escape_json(&profile.primary_relay.address),
        profile.primary_relay.port,
        fallback,
        dests.join(",")
    )
}

fn escape_json(s: &str) -> String {
    s.replace('\\', "\\\\")
        .replace('"', "\\\"")
        .replace('\n', "\\n")
        .replace('\r', "\\r")
        .replace('\t', "\\t")
}

fn extract_json_string_field(json: &str, field_name: &str) -> Option<String> {
    let key_pattern = format!("\"{}\"", field_name);
    let key_pos = json.find(&key_pattern)?;
    let after_key = &json[key_pos + key_pattern.len()..];
    let colon_pos = after_key.find(':')?;
    let after_colon = after_key[colon_pos + 1..].trim_start();
    if !after_colon.starts_with('"') {
        return None;
    }
    let string_content = &after_colon[1..];
    let mut result = String::new();
    let mut chars = string_content.chars();
    while let Some(ch) = chars.next() {
        if ch == '"' {
            return Some(result);
        }
        if ch == '\\' {
            match chars.next() {
                Some('"') => result.push('"'),
                Some('\\') => result.push('\\'),
                Some('/') => result.push('/'),
                Some('b') => result.push('\x08'),
                Some('f') => result.push('\x0c'),
                Some('n') => result.push('\n'),
                Some('r') => result.push('\r'),
                Some('t') => result.push('\t'),
                Some(other) => {
                    result.push('\\');
                    result.push(other);
                }
                None => return None,
            }
        } else {
            result.push(ch);
        }
    }
    None
}

fn client_ui_document() -> String {
    let patch = format!("<script>\n{}\n</script>\n</body>", CLIENT_UI_TRUTH_PATCH_JS);
    CLIENT_UI_HTML.replacen("</body>", &patch, 1)
}

fn write_http_response(
    stream: &mut TcpStream,
    status: &str,
    content_type: &str,
    body: &str,
) {
    let response = format!(
        "HTTP/1.1 {}\r\nContent-Type: {}\r\nCache-Control: no-store\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        status,
        content_type,
        body.len(),
        body
    );
    let _ = stream.write_all(response.as_bytes());
}

fn write_json_response(stream: &mut TcpStream, status: &str, body: &str) {
    write_http_response(stream, status, "application/json; charset=utf-8", body);
}

/// Atomically binds and validates runtime client configuration.
/// Ensures fail-closed semantics: any parsing, validation, or lock failure leaves current configuration unchanged.
pub fn transactional_bind_config(
    profile_arc: &Arc<RwLock<ClientConfigProfile>>,
    raw_body: &str,
) -> Result<ClientConfigProfile, ConfigError> {
    if raw_body.trim().is_empty() {
        return Err(ConfigError::ValidationError(
            "Request body is empty".to_string(),
        ));
    }

    let content = if let Some(content_val) = extract_json_string_field(raw_body, "content") {
        content_val
    } else if raw_body.trim_start().starts_with('{') {
        return Err(ConfigError::ParseError(
            "Malformed JSON payload: missing or invalid 'content' field".to_string(),
        ));
    } else {
        raw_body.to_string()
    };

    let new_profile = ClientConfigProfile::from_toml_str(&content)?;

    let mut lock = profile_arc.write().map_err(|_| ConfigError::LockPoisoned)?;
    *lock = new_profile.clone();
    Ok(new_profile)
}

fn handle_client_stream(
    mut stream: TcpStream,
    profile_arc: &Arc<RwLock<ClientConfigProfile>>,
    keystore_id: &str,
    transport_state: TransportState,
    protected_proxy: Option<LocalProxyEndpoint>,
) {
    let mut buf = [0u8; 8192];
    let bytes_read = match stream.read(&mut buf) {
        Ok(n) if n > 0 => n,
        _ => return,
    };

    let req_str = match std::str::from_utf8(&buf[..bytes_read]) {
        Ok(s) => s,
        Err(_) => return,
    };

    let first_line = req_str.lines().next().unwrap_or("");
    let parts: Vec<&str> = first_line.split_whitespace().collect();
    if parts.len() < 2 {
        return;
    }
    let method = parts[0];
    let path = parts[1];

    if method == "GET" && (path == "/" || path == "/index.html") {
        let body = client_ui_document();
        write_http_response(&mut stream, "200 OK", "text/html; charset=utf-8", &body);
        return;
    }

    if method == "GET" && path == "/api/profile" {
        match profile_arc.read() {
            Ok(profile) => write_json_response(&mut stream, "200 OK", &profile_to_json(&profile)),
            Err(_) => write_json_response(
                &mut stream,
                "500 Internal Server Error",
                r#"{"status":"error","message":"profile state unavailable"}"#,
            ),
        }
        return;
    }

    if method == "GET" && path == "/api/status" {
        let json_body = format!(
            r#"{{"status":"{}","device_id":"{}","platform":"{:?}","protected_proxy":{}}}"#,
            transport_state_label(transport_state),
            escape_json(keystore_id),
            current_platform(),
            proxy_json(protected_proxy)
        );
        write_json_response(&mut stream, "200 OK", &json_body);
        return;
    }

    if method == "POST" && path == "/api/bind_config" {
        let body = match req_str.find("\r\n\r\n") {
            Some(idx) => &req_str[idx + 4..],
            None => "",
        };

        match transactional_bind_config(profile_arc, body) {
            Ok(new_profile) => {
                let json_body = format!(
                    r#"{{"status":"ok","profile_id":"{}","profile_name":"{}","version":"{}"}}"#,
                    escape_json(&new_profile.profile_id),
                    escape_json(&new_profile.profile_name),
                    escape_json(&new_profile.version)
                );
                write_json_response(&mut stream, "200 OK", &json_body);
            }
            Err(err) => {
                let status_code = match err {
                    ConfigError::LockPoisoned => "500 Internal Server Error",
                    _ => "400 Bad Request",
                };
                let json_body = format!(
                    r#"{{"status":"error","message":"{}"}}"#,
                    escape_json(&err.to_string())
                );
                write_json_response(&mut stream, status_code, &json_body);
            }
        }
        return;
    }

    if method == "POST" && path == "/api/navigate" {
        let body = match req_str.find("\r\n\r\n") {
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

        let gate = BrokerSecurityGate::new(
            vec![
                BrokerCapability::NavigateService,
                BrokerCapability::QueryDeviceStatus,
            ],
            "sess_editorial_gui".to_string(),
        );
        let nav_req = BrokerRequest {
            request_id: "req_nav_gui".to_string(),
            session_token: "sess_editorial_gui".to_string(),
            payload: BrokerRequestPayload::Navigate {
                target_url: target_url.clone(),
            },
        };

        if let Err(error) = gate.verify_request(&nav_req) {
            let json_body = format!(
                r#"{{"ok":false,"state":"denied","message":"{}","target":"{}","transport_status":"{}","protected_proxy":{}}}"#,
                escape_json(&format!("navigation policy denied: {error:?}")),
                escape_json(&target_url),
                transport_state_label(transport_state),
                proxy_json(protected_proxy)
            );
            write_json_response(&mut stream, "403 Forbidden", &json_body);
            return;
        }

        let transport_usable = matches!(
            transport_state,
            TransportState::Ready | TransportState::Degraded
        ) && protected_proxy.is_some();
        if !transport_usable {
            let json_body = format!(
                r#"{{"ok":false,"state":"offline","message":"protected transport is not ready","target":"{}","transport_status":"{}","protected_proxy":null}}"#,
                escape_json(&target_url),
                transport_state_label(transport_state)
            );
            write_json_response(&mut stream, "503 Service Unavailable", &json_body);
            return;
        }

        let json_body = format!(
            r#"{{"ok":true,"state":"transport_ready","message":"protected transport and policy are ready; browser session is not yet opened","target":"{}","transport_status":"{}","protected_proxy":{}}}"#,
            escape_json(&target_url),
            transport_state_label(transport_state),
            proxy_json(protected_proxy)
        );
        write_json_response(&mut stream, "200 OK", &json_body);
        return;
    }

    write_http_response(&mut stream, "404 Not Found", "text/plain; charset=utf-8", "");
}

fn launch_app_window(url: &str) {
    #[cfg(target_os = "windows")]
    {
        // This browser opens only the local WebGate control UI. Protected resources
        // must not be opened here; T-041 owns the real protected browser runtime.
        let edge_res = Command::new("msedge.exe")
            .arg(format!("--app={url}"))
            .arg("--window-size=1200,820")
            .spawn();

        if edge_res.is_err() {
            let chrome_res = Command::new("chrome.exe")
                .arg(format!("--app={url}"))
                .arg("--window-size=1200,820")
                .spawn();

            if chrome_res.is_err() {
                let _ = Command::new("cmd").args(["/c", "start", "", url]).spawn();
            }
        }
    }

    #[cfg(target_os = "macos")]
    {
        let _ = Command::new("open").arg(url).spawn();
    }

    #[cfg(target_os = "linux")]
    {
        let _ = Command::new("xdg-open").arg(url).spawn();
    }
}

fn print_editorial_banner(profile: &ClientConfigProfile, active_dest: &str, device_id: &str) {
    println!("───────────────────────────────────────────────────────────────────────────");
    println!(
        " 01 / WEBGATE КЛИЕНТ       [ СРЕДА ИЗОЛЯЦИИ КАПСУЛЫ v{} ]",
        profile.version
    );
    println!("───────────────────────────────────────────────────────────────────────────");
    println!(
        " ПРОФИЛЬ       : {} ({})",
        profile.profile_name, profile.profile_id
    );
    println!(" УСТРОЙСТВО    : {device_id}");
    println!(
        " ОСНОВНОЙ РЕЛЕЙ: {}:{}",
        profile.primary_relay.address, profile.primary_relay.port
    );
    if let Some(ref fb) = profile.fallback_relay {
        println!(" РЕЗЕРВНЫЙ УЗЕЛ: {}:{}", fb.address, fb.port);
    }
    println!(" ЦЕЛЕВОЙ РЕСУРС: {active_dest}");
    println!(" ДОСТУПНЫЕ МАРШРУТЫ И СЕРВИСЫ:");
    for (idx, dest) in profile.destinations.iter().enumerate() {
        let mark = if dest.url == active_dest {
            "▶ [АКТИВЕН]"
        } else {
            "  "
        };
        println!(
            "   {:02} {} {:<28} → {}",
            idx + 1,
            mark,
            dest.name,
            dest.url
        );
    }
    println!("───────────────────────────────────────────────────────────────────────────");
}

fn main() {
    let platform = current_platform();
    let args: Vec<String> = env::args().collect();

    let mut config_path_opt: Option<String> = None;
    let mut destination_opt: Option<String> = None;
    let mut list_only = false;
    let mut cli_only = false;

    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "--config" | "-c" if i + 1 < args.len() => {
                config_path_opt = Some(args[i + 1].clone());
                i += 1;
            }
            "--destination" | "-d" | "--target" | "-t" if i + 1 < args.len() => {
                destination_opt = Some(args[i + 1].clone());
                i += 1;
            }
            "--list" | "-l" => list_only = true,
            "--cli" | "--terminal" => cli_only = true,
            "--help" | "-h" => {
                println!("WebGate Клиент — Защищенный браузерный клиент доступа к приватным ресурсам");
                println!();
                println!("ИСПОЛЬЗОВАНИЕ:");
                println!("    webgate-app [ПАРАМЕТРЫ]");
                println!();
                println!("ПАРАМЕТРЫ:");
                println!("    -c, --config <ПУТЬ>         Привязать файл конфигурационного профиля (.toml/.json)");
                println!("    -d, --destination <URL|ID>  Выбрать целевой сервис или URL для перехода");
                println!("    -l, --list                  Показать список доступных сервисов в профиле и выйти");
                println!("        --cli                   Запуск только в терминальном режиме без открытия окна GUI");
                println!("    -h, --help                  Показать справочную информацию");
                return;
            }
            _ => {}
        }
        i += 1;
    }

    let profile = match load_client_profile(config_path_opt.as_deref()) {
        Ok(profile) => profile,
        Err(error) => {
            eprintln!("[Конфигурация] Не удалось загрузить явно указанный профиль: {error:?}");
            std::process::exit(2);
        }
    };

    let target_destination = if let Some(req_dest) = destination_opt {
        if let Some(matched) = profile.find_destination(&req_dest) {
            matched.url.clone()
        } else {
            req_dest
        }
    } else {
        profile
            .default_destination_url
            .clone()
            .unwrap_or_else(|| "webgate://service/docs/overview".to_string())
    };

    if list_only {
        println!("Настроенные сервисы и маршруты в профиле '{}':", profile.profile_name);
        for (idx, dest) in profile.destinations.iter().enumerate() {
            println!(" {:02}. [{}] {} → {}", idx + 1, dest.id, dest.name, dest.url);
            println!("     {}", dest.description);
        }
        return;
    }

    let key_path = std::env::var("WEBGATE_DEVICE_KEY_PATH")
        .map(std::path::PathBuf::from)
        .unwrap_or_else(|_| {
            std::env::current_dir()
                .unwrap_or_else(|_| std::path::PathBuf::from("."))
                .join(".webgate")
                .join("device.key")
        });

    let keystore = match PersistentFileDeviceKeyStore::open(&key_path) {
        Ok(mut ks) => {
            if ks.get_device_identity().ok().flatten().is_none()
                && ks.generate_key(profile.key_algorithm, &profile.device_label).is_err()
            {
                eprintln!("[Хранилище ключей] Ошибка генерации ключа устройства");
                return;
            }
            ks
        }
        Err(e) => {
            eprintln!("[Хранилище ключей] Ошибка открытия хранилища ключей: {e:?}");
            return;
        }
    };
    let device_id = match keystore.get_device_identity() {
        Ok(Some(ident)) => ident.id,
        _ => {
            eprintln!("[Хранилище ключей] Идентификатор устройства не найден");
            return;
        }
    };

    let (transport_state, proxy_ep) = if let Some(ref fallback) = profile.fallback_relay {
        match build_dual_relay_transport(&profile, fallback) {
            Ok(mut dual_transport) => {
                let ep = dual_transport.start_proxy().ok();
                let state = dual_transport.state();
                if ep.is_none() {
                    eprintln!("[Транспорт] Dual-relay upstreams не подтверждены; protected proxy остаётся OFFLINE.");
                }
                (state, ep)
            }
            Err(error) => {
                eprintln!("[Транспорт] Некорректная конфигурация dual relay: {error:?}");
                (TransportState::Offline, None)
            }
        }
    } else {
        match build_primary_transport(&profile) {
            Ok(mut primary) => {
                let ep = primary.start_proxy().ok();
                let state = primary.state();
                if ep.is_none() {
                    eprintln!("[Транспорт] Primary sidecar не подтверждён; protected proxy остаётся OFFLINE.");
                }
                (state, ep)
            }
            Err(error) => {
                eprintln!("[Транспорт] Некорректная политика primary proxy: {error:?}");
                (TransportState::Offline, None)
            }
        }
    };

    if cli_only {
        print_editorial_banner(&profile, &target_destination, &device_id);
        let session_token = "sess_editorial_bound".to_string();
        let gate = BrokerSecurityGate::new(
            vec![
                BrokerCapability::NavigateService,
                BrokerCapability::QueryDeviceStatus,
            ],
            session_token.clone(),
        );
        let nav_req = BrokerRequest {
            request_id: "req_nav_editorial".to_string(),
            session_token,
            payload: BrokerRequestPayload::Navigate {
                target_url: target_destination.clone(),
            },
        };

        if gate.verify_request(&nav_req).is_ok() {
            println!("  [Брокер IPC] Запрос авторизован с привилегией: NavigateService");
            let policy = profile.build_navigation_policy();
            let mut capsule = BrowserCapsule::new(BrowserKind::Servo, policy);
            if let Some(ep) = proxy_ep {
                let proxy_attached = capsule.attach_proxy(ep.socket_addr()).is_ok();
                let capsule_started = capsule.start().is_ok();
                let navigated = capsule.navigate(&target_destination).is_ok();
                if proxy_attached && capsule_started && navigated {
                    println!("  [Капсула] Соединение установлено: {target_destination}");
                    println!("  [Капсула] Граница изоляции активна. Сетевые маршруты ОС не затронуты.");
                }
            } else {
                println!("  [Транспорт] OFFLINE: реальный защищённый proxy/tunnel не подтверждён; навигация запрещена.");
            }
        }
        println!(
            "\nКлиент WebGate запущен [Платформа: {:?} | Устройство: {} | Транспорт: {}]",
            platform,
            device_id,
            transport_state_label(transport_state)
        );
        return;
    }

    let listener = match TcpListener::bind("127.0.0.1:43110") {
        Ok(l) => l,
        Err(_) => match TcpListener::bind("127.0.0.1:0") {
            Ok(l) => l,
            Err(e) => {
                eprintln!("Не удалось запустить локальный шлюз интерфейса: {e:?}");
                return;
            }
        },
    };

    let bound_addr = match listener.local_addr() {
        Ok(addr) => addr,
        Err(_) => return,
    };

    let ui_url = format!("http://127.0.0.1:{}", bound_addr.port());
    println!("───────────────────────────────────────────────────────────────────────────");
    println!(" WebGate локальная панель клиента: {ui_url}");
    println!(
        " Защищённый транспорт: {} (proxy: {})",
        transport_state_label(transport_state),
        proxy_json(proxy_ep)
    );
    println!("───────────────────────────────────────────────────────────────────────────");

    let profile_arc = Arc::new(RwLock::new(profile));
    let profile_clone = Arc::clone(&profile_arc);
    let dev_id_clone = device_id.clone();

    let server_handle = thread::spawn(move || {
        for s in listener.incoming().flatten() {
            handle_client_stream(s, &profile_clone, &dev_id_clone, transport_state, proxy_ep);
        }
    });

    launch_app_window(&ui_url);
    let _ = server_handle.join();
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use std::net::{IpAddr, Ipv4Addr};

    fn http_roundtrip(
        request: &str,
        transport_state: TransportState,
        proxy: Option<LocalProxyEndpoint>,
    ) -> String {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let profile_arc = Arc::new(RwLock::new(ClientConfigProfile::default()));
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

        let mut client = TcpStream::connect(addr).unwrap();
        client.write_all(request.as_bytes()).unwrap();
        let mut response = String::new();
        client.read_to_string(&mut response).unwrap();
        server_thread.join().unwrap();
        response
    }

    #[test]
    fn default_profile_is_used_only_when_no_config_was_requested() {
        let profile = load_client_profile(None).unwrap();
        assert_eq!(profile, ClientConfigProfile::default());
    }

    #[test]
    fn explicit_missing_config_returns_error_instead_of_defaults() {
        let missing = std::env::temp_dir().join(format!(
            "webgate-config-missing-{}-{}.toml",
            std::process::id(),
            "t035"
        ));
        let _ = std::fs::remove_file(&missing);
        let result = load_client_profile(missing.to_str());
        assert!(matches!(result, Err(ConfigError::FileNotFound(_))));
    }

    #[test]
    fn dual_relay_transport_rejects_nonloopback_fallback() {
        let profile = ClientConfigProfile::default();
        let fallback = RelayEndpointConfig {
            name: "non-loopback-fallback".to_string(),
            address: "192.0.2.20".to_string(),
            port: 4443,
        };
        let result = build_dual_relay_transport(&profile, &fallback);
        assert!(matches!(result, Err(DualRelayError::FallbackUpstreamNotLoopback)));
    }

    #[test]
    fn primary_transport_rejects_nonloopback_plaintext_sidecar() {
        let mut profile = ClientConfigProfile::default();
        profile.primary_relay.address = "192.0.2.10".to_string();
        let mut transport = build_primary_transport(&profile).unwrap();
        assert!(matches!(
            transport.start_proxy(),
            Err(RestrictedProxyError::UpstreamNotLoopback)
        ));
        assert_eq!(transport.state(), TransportState::Offline);
        assert_eq!(transport.local_proxy(), None);
    }

    #[test]
    fn primary_transport_rejects_empty_destination_policy() {
        let mut profile = ClientConfigProfile::default();
        profile.allowed_domains.clear();
        assert!(matches!(
            build_primary_transport(&profile),
            Err(RestrictedProxyError::EmptyAllowedDomains)
        ));
    }

    #[test]
    fn offline_transport_never_serializes_a_protected_proxy() {
        assert_eq!(proxy_json(None), "null");
        assert_eq!(transport_state_label(TransportState::Offline), "offline");
    }

    #[test]
    fn rendered_client_ui_includes_truth_controller() {
        let document = client_ui_document();
        assert!(document.contains("WEBGATE_TRUTH_PATCH_ACTIVE"));
        assert!(document.contains("ДЕМО-ДАННЫЕ НЕ ИСПОЛЬЗУЮТСЯ"));
    }

    #[test]
    fn navigate_offline_returns_503_and_never_success() {
        let body = r#"{"target_url":"webgate://service/docs/overview"}"#;
        let request = format!(
            "POST /api/navigate HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            body.len(), body
        );
        let response = http_roundtrip(&request, TransportState::Offline, None);
        assert!(response.starts_with("HTTP/1.1 503 Service Unavailable"));
        assert!(response.contains("\"ok\":false"));
        assert!(response.contains("\"state\":\"offline\""));
        assert!(!response.contains("\"ok\":true"));
    }

    #[test]
    fn navigate_requires_target_url() {
        let body = r#"{}"#;
        let request = format!(
            "POST /api/navigate HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            body.len(), body
        );
        let response = http_roundtrip(&request, TransportState::Ready, None);
        assert!(response.starts_with("HTTP/1.1 400 Bad Request"));
        assert!(response.contains("\"state\":\"invalid_request\""));
    }

    #[test]
    fn navigate_ready_reports_transport_ready_not_open_session() {
        let proxy = LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43117).unwrap();
        let body = r#"{"target_url":"webgate://service/docs/overview"}"#;
        let request = format!(
            "POST /api/navigate HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            body.len(), body
        );
        let response = http_roundtrip(&request, TransportState::Ready, Some(proxy));
        assert!(response.starts_with("HTTP/1.1 200 OK"));
        assert!(response.contains("\"ok\":true"));
        assert!(response.contains("\"state\":\"transport_ready\""));
        assert!(response.contains("browser session is not yet opened"));
    }

    #[test]
    fn transactional_bind_config_updates_profile_on_valid_payload() {
        let initial = ClientConfigProfile::default();
        let profile_arc = Arc::new(RwLock::new(initial));
        let payload = r#"{"content": "profile_id = \"custom-fleet\"\nprofile_name = \"Custom Fleet Mesh\"\nprimary_relay_addr = \"127.0.0.1\"\nprimary_relay_port = 52000\nallowed_domains = \"service, internal.mesh\"\ndestination = \"node1|Node One|webgate://service/node1|Infra|Node Telemetry\"\n"}"#;
        let res = transactional_bind_config(&profile_arc, payload);
        assert!(res.is_ok(), "Expected Ok but got {:?}", res);
        let updated = res.unwrap();
        assert_eq!(updated.profile_id, "custom-fleet");
        assert_eq!(updated.profile_name, "Custom Fleet Mesh");
        assert_eq!(updated.primary_relay.port, 52000);
        assert_eq!(updated.allowed_domains, vec!["service", "internal.mesh"]);
        assert_eq!(updated.destinations.len(), 1);
        assert_eq!(profile_arc.read().unwrap().profile_id, "custom-fleet");
    }

    #[test]
    fn transactional_bind_config_fails_closed_on_invalid_syntax() {
        let initial = ClientConfigProfile::default();
        let original_id = initial.profile_id.clone();
        let profile_arc = Arc::new(RwLock::new(initial));
        let payload = r#"{"content": "not a valid key value pair without equals"}"#;
        let res = transactional_bind_config(&profile_arc, payload);
        assert!(res.is_err());
        assert_eq!(profile_arc.read().unwrap().profile_id, original_id);
    }

    #[test]
    fn transactional_bind_config_fails_closed_on_validation_failure() {
        let initial = ClientConfigProfile::default();
        let original_id = initial.profile_id.clone();
        let profile_arc = Arc::new(RwLock::new(initial));
        let payload = r#"{"content": "profile_id = \"valid-id\"\nprimary_relay_addr = \"127.0.0.1\"\nprimary_relay_port = 0\ndestination = \"d1|D1|webgate://service/d1|Cat|Desc\"\n"}"#;
        let res = transactional_bind_config(&profile_arc, payload);
        assert!(res.is_err());
        assert_eq!(profile_arc.read().unwrap().profile_id, original_id);
    }

    #[test]
    fn transactional_bind_config_fails_closed_on_disallowed_destination_scheme() {
        let initial = ClientConfigProfile::default();
        let original_id = initial.profile_id.clone();
        let profile_arc = Arc::new(RwLock::new(initial));
        let payload = r#"{"content": "profile_id = \"valid-id\"\nprimary_relay_addr = \"127.0.0.1\"\nprimary_relay_port = 43111\ndestination = \"d1|D1|file:///etc/passwd|Cat|Desc\"\n"}"#;
        let res = transactional_bind_config(&profile_arc, payload);
        assert!(res.is_err());
        assert_eq!(profile_arc.read().unwrap().profile_id, original_id);
    }

    #[test]
    fn transactional_bind_config_fails_closed_on_malformed_json() {
        let initial = ClientConfigProfile::default();
        let original_id = initial.profile_id.clone();
        let profile_arc = Arc::new(RwLock::new(initial));
        let payload = r#"{"content": "unclosed string without end"#;
        let res = transactional_bind_config(&profile_arc, payload);
        assert!(res.is_err());
        assert_eq!(profile_arc.read().unwrap().profile_id, original_id);
    }

    #[test]
    fn handle_client_stream_bind_config_http_transactional_roundtrip() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let profile_arc = Arc::new(RwLock::new(ClientConfigProfile::default()));
        let profile_clone = Arc::clone(&profile_arc);

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(stream, &profile_clone, "dev_test_123", TransportState::Ready, None);
        });

        let mut client = TcpStream::connect(addr).unwrap();
        let valid_body = r#"{"content":"profile_id = \"http-fleet\"\nprofile_name = \"HTTP Fleet\"\nprimary_relay_addr = \"127.0.0.1\"\nprimary_relay_port = 48000\ndestination = \"srv|Srv|webgate://service/s|Cat|Desc\"\n"}"#;
        let req = format!(
            "POST /api/bind_config HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            valid_body.len(), valid_body
        );
        client.write_all(req.as_bytes()).unwrap();
        let mut resp = String::new();
        client.read_to_string(&mut resp).unwrap();
        server_thread.join().unwrap();
        assert!(resp.starts_with("HTTP/1.1 200 OK"));
        assert!(resp.contains("\"status\":\"ok\""));
        assert!(resp.contains("\"profile_id\":\"http-fleet\""));
        assert_eq!(profile_arc.read().unwrap().profile_id, "http-fleet");
    }

    #[test]
    fn handle_client_stream_bind_config_http_bad_request_fails_closed() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let profile_arc = Arc::new(RwLock::new(ClientConfigProfile::default()));
        let profile_clone = Arc::clone(&profile_arc);

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(stream, &profile_clone, "dev_test_123", TransportState::Ready, None);
        });

        let mut client = TcpStream::connect(addr).unwrap();
        let invalid_body = r#"{"content":"primary_relay_port = not_a_number\n"}"#;
        let req = format!(
            "POST /api/bind_config HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            invalid_body.len(), invalid_body
        );
        client.write_all(req.as_bytes()).unwrap();
        let mut resp = String::new();
        client.read_to_string(&mut resp).unwrap();
        server_thread.join().unwrap();
        assert!(resp.starts_with("HTTP/1.1 400 Bad Request"));
        assert!(resp.contains("\"status\":\"error\""));
        assert_eq!(profile_arc.read().unwrap().profile_id, "default-fleet");
    }
}
