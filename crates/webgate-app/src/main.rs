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
use webgate_core::config::{ClientConfigProfile, ConfigError};
use webgate_platform::current_platform;
use webgate_platform::keystore::{DeviceKeyStore, PersistentFileDeviceKeyStore};
use webgate_transport::failover::{FailoverConfig, TransportFailoverController};
use webgate_transport::restricted_socks5::{
    RestrictedProxyError, RestrictedSocks5Config, RestrictedSocks5Transport,
};
use webgate_transport::{LocalProxyEndpoint, TransportProvider, TransportState};

const CLIENT_UI_HTML: &str = include_str!("client_ui.html");
const PRIMARY_PROXY_CONNECT_TIMEOUT: Duration = Duration::from_secs(2);

/// Configuration-only fallback placeholder.
///
/// T-036 supplies a real primary provider. A configured fallback address/port is
/// still not proof that an independent protected transport exists; T-042 owns it.
#[derive(Debug)]
struct ConfiguredRelayTransport {
    name: String,
}

impl TransportProvider for ConfiguredRelayTransport {
    fn name(&self) -> &str {
        &self.name
    }

    fn state(&self) -> TransportState {
        TransportState::Offline
    }

    fn local_proxy(&self) -> Option<LocalProxyEndpoint> {
        None
    }

    fn stop(&mut self) {}
}

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
            d.id, d.name, d.url, d.category, d.description
        ));
    }
    let fallback = match &profile.fallback_relay {
        Some(fb) => format!(
            r#"{{"name":"{}","address":"{}","port":{}}}"#,
            fb.name, fb.address, fb.port
        ),
        None => "null".to_string(),
    };
    format!(
        r#"{{"profile_id":"{}","profile_name":"{}","version":"{}","primary_relay":{{"name":"{}","address":"{}","port":{}}},"fallback_relay":{},"destinations":[{}]}}"#,
        profile.profile_id,
        profile.profile_name,
        profile.version,
        profile.primary_relay.name,
        profile.primary_relay.address,
        profile.primary_relay.port,
        fallback,
        dests.join(",")
    )
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
        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            CLIENT_UI_HTML.len(),
            CLIENT_UI_HTML
        );
        let _ = stream.write_all(response.as_bytes());
        return;
    }

    if method == "GET" && path == "/api/profile" {
        let json_body = match profile_arc.read() {
            Ok(p) => profile_to_json(&p),
            Err(_) => "{}".to_string(),
        };
        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            json_body.len(),
            json_body
        );
        let _ = stream.write_all(response.as_bytes());
        return;
    }

    if method == "GET" && path == "/api/status" {
        let json_body = format!(
            r#"{{"status":"{}","device_id":"{}","platform":"{:?}","protected_proxy":{}}}"#,
            transport_state_label(transport_state),
            keystore_id,
            current_platform(),
            proxy_json(protected_proxy)
        );
        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            json_body.len(),
            json_body
        );
        let _ = stream.write_all(response.as_bytes());
        return;
    }

    if method == "POST" && path == "/api/bind_config" {
        if let Some(body_start) = req_str.find("\r\n\r\n") {
            let body = &req_str[body_start + 4..];
            if let Some(c_start) = body.find("\"content\":") {
                let content_sub = &body[c_start + 10..];
                if let (Some(first_quote), Some(second_quote)) = (
                    content_sub.find('"'),
                    content_sub
                        .find('"')
                        .and_then(|q| content_sub[q + 1..].find('"')),
                ) {
                    let raw_content = &content_sub[first_quote + 1..first_quote + 1 + second_quote];
                    let clean_content = raw_content.replace("\\n", "\n").replace("\\\"", "\"");
                    if let (Ok(new_profile), Ok(mut lock)) = (
                        ClientConfigProfile::from_toml_str(&clean_content),
                        profile_arc.write(),
                    ) {
                        *lock = new_profile;
                    }
                }
            }
        }
        let response = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 15\r\nConnection: close\r\n\r\n{\"status\":\"ok\"}";
        let _ = stream.write_all(response.as_bytes());
        return;
    }

    if method == "POST" && path == "/api/navigate" {
        let mut target_url = "webgate://service/docs/overview".to_string();
        if let Some(body_start) = req_str.find("\r\n\r\n") {
            let body = &req_str[body_start + 4..];
            if let Some(t_start) = body.find("\"target_url\":") {
                let url_sub = &body[t_start + 13..];
                if let (Some(first_quote), Some(second_quote)) = (
                    url_sub.find('"'),
                    url_sub.find('"').and_then(|q| url_sub[q + 1..].find('"')),
                ) {
                    target_url =
                        url_sub[first_quote + 1..first_quote + 1 + second_quote].to_string();
                }
            }
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

        let transport_usable = matches!(
            transport_state,
            TransportState::Ready | TransportState::Degraded
        ) && protected_proxy.is_some();
        let is_ok = gate.verify_request(&nav_req).is_ok() && transport_usable;
        let json_body = format!(
            r#"{{"ok":{},"target":"{}","transport_status":"{}","protected_proxy":{}}}"#,
            is_ok,
            target_url,
            transport_state_label(transport_state),
            proxy_json(protected_proxy)
        );
        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            json_body.len(),
            json_body
        );
        let _ = stream.write_all(response.as_bytes());
        return;
    }

    let response = "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n";
    let _ = stream.write_all(response.as_bytes());
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
            "--list" | "-l" => {
                list_only = true;
            }
            "--cli" | "--terminal" => {
                cli_only = true;
            }
            "--help" | "-h" => {
                println!(
                    "WebGate Клиент — Защищенный браузерный клиент доступа к приватным ресурсам"
                );
                println!();
                println!("ИСПОЛЬЗОВАНИЕ:");
                println!("    webgate-app [ПАРАМЕТРЫ]");
                println!();
                println!("ПАРАМЕТРЫ:");
                println!(
                    "    -c, --config <ПУТЬ>         Привязать файл конфигурационного профиля (.toml/.json)"
                );
                println!(
                    "    -d, --destination <URL|ID>  Выбрать целевой сервис или URL для перехода"
                );
                println!(
                    "    -l, --list                  Показать список доступных сервисов в профиле и выйти"
                );
                println!(
                    "        --cli                   Запуск только в терминальном режиме без открытия окна GUI"
                );
                println!("    -h, --help                  Показать справочную информацию");
                return;
            }
            _ => {}
        }
        i += 1;
    }

    // An explicitly requested configuration is authoritative input. If it cannot
    // be read or validated, fail closed instead of silently changing to defaults.
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
        println!(
            "Настроенные сервисы и маршруты в профиле '{}':",
            profile.profile_name
        );
        for (idx, dest) in profile.destinations.iter().enumerate() {
            println!(
                " {:02}. [{}] {} → {}",
                idx + 1,
                dest.id,
                dest.name,
                dest.url
            );
            println!("     {}", dest.description);
        }
        return;
    }

    // T-040: Production platform key storage with persistence on disk.
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
                && ks
                    .generate_key(profile.key_algorithm, &profile.device_label)
                    .is_err()
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

    // T-036 primary path: the browser-facing listener is real, loopback-only and
    // destination restricted. It never direct-dials protected destinations; every
    // allowed CONNECT is delegated to the explicitly configured local SOCKS5 sidecar.
    let mut primary = match build_primary_transport(&profile) {
        Ok(primary) => primary,
        Err(error) => {
            eprintln!("[Транспорт] Некорректная политика primary proxy: {error:?}");
            return;
        }
    };
    if let Err(error) = primary.start_proxy() {
        eprintln!(
            "[Транспорт] Primary sidecar не подтверждён ({error:?}); protected proxy остаётся OFFLINE."
        );
    }

    // T-042 owns a materially independent fallback transport. Configuration alone
    // is not readiness, so the fallback deliberately remains Offline here.
    let fallback = ConfiguredRelayTransport {
        name: profile
            .fallback_relay
            .as_ref()
            .map(|r| r.name.clone())
            .unwrap_or_else(|| "Relay-Beta (Резервный)".to_string()),
    };

    let mut transport_ctrl =
        TransportFailoverController::new(primary, fallback, FailoverConfig::default());
    let transport_state = transport_ctrl.start();
    let proxy_ep = transport_ctrl.active_proxy_endpoint();

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
        println!(
            "\nКлиент WebGate запущен [Платформа: {:?} | Устройство: {} | Транспорт: {}]",
            platform,
            device_id,
            transport_state_label(transport_state)
        );
        return;
    }

    // Default user launch is a local control UI only. It is not the protected
    // browser runtime and cannot claim protected connectivity while transport is Offline.
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
    fn configured_fallback_is_offline_without_backend() {
        let transport = ConfiguredRelayTransport {
            name: "configured-only".to_string(),
        };
        assert_eq!(transport.state(), TransportState::Offline);
        assert_eq!(transport.local_proxy(), None);
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
}
