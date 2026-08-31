#![forbid(unsafe_code)]

use std::env;
use std::io::{Read, Write};
use std::net::{IpAddr, Ipv4Addr, TcpListener, TcpStream};
use std::process::Command;
use std::sync::{Arc, RwLock};
use std::thread;

use webgate_browser::BrowserKind;
use webgate_browser::capsule::BrowserCapsule;
use webgate_core::broker::{
    BrokerCapability, BrokerRequest, BrokerRequestPayload, BrokerSecurityGate,
};
use webgate_core::config::ClientConfigProfile;
use webgate_platform::current_platform;
use webgate_platform::keystore::{DeviceKeyStore, InMemoryDeviceKeyStore};
use webgate_transport::failover::{FailoverConfig, TransportFailoverController};
use webgate_transport::{LocalProxyEndpoint, TransportProvider, TransportState};

const CLIENT_UI_HTML: &str = include_str!("client_ui.html");

#[derive(Debug)]
struct DynamicRelayTransport {
    name: String,
    port: u16,
}

impl TransportProvider for DynamicRelayTransport {
    fn name(&self) -> &str {
        &self.name
    }

    fn state(&self) -> TransportState {
        TransportState::Ready
    }

    fn local_proxy(&self) -> Option<LocalProxyEndpoint> {
        LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), self.port).ok()
    }

    fn stop(&mut self) {}
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
    primary_port: u16,
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
            r#"{{"status":"ready","device_id":"{}","platform":"{:?}","primary_relay_port":{}}}"#,
            keystore_id,
            current_platform(),
            primary_port
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

        // Verify Broker Security Gate
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

        let is_ok = gate.verify_request(&nav_req).is_ok();
        let json_body = format!(
            r#"{{"ok":{},"target":"{}","proxy":"127.0.0.1:{}"}}"#,
            is_ok, target_url, primary_port
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
        // Try Microsoft Edge in standalone app window mode
        let edge_res = Command::new("msedge.exe")
            .arg(format!("--app={}", url))
            .arg("--window-size=1200,820")
            .spawn();

        if edge_res.is_err() {
            // Try Chrome in standalone app mode
            let chrome_res = Command::new("chrome.exe")
                .arg(format!("--app={}", url))
                .arg("--window-size=1200,820")
                .spawn();

            if chrome_res.is_err() {
                // Fallback to default browser
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
    println!(" УСТРОЙСТВО    : {}", device_id);
    println!(
        " ОСНОВНОЙ РЕЛЕЙ: {}:{}",
        profile.primary_relay.address, profile.primary_relay.port
    );
    if let Some(ref fb) = profile.fallback_relay {
        println!(" РЕЗЕРВНЫЙ УЗЕЛ: {}:{}", fb.address, fb.port);
    }
    println!(" ЦЕЛЕВОЙ РЕСУРС: {}", active_dest);
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

    // 1. Load or bind configuration profile
    let profile = if let Some(path) = config_path_opt {
        ClientConfigProfile::load_from_file(&path).unwrap_or_default()
    } else {
        ClientConfigProfile::default()
    };

    // 2. Resolve target destination
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

    // 3. Initialize Keystore (T-009, T-010)
    let mut keystore = InMemoryDeviceKeyStore::new();
    let device_id = match keystore.generate_key(profile.key_algorithm, &profile.device_label) {
        Ok(ident) => ident.id,
        Err(e) => {
            eprintln!("[Хранилище ключей] Ошибка генерации ключа устройства: {e:?}");
            return;
        }
    };

    // 4. Initialize Failover Transport Controller with profile endpoints (T-008)
    let primary = DynamicRelayTransport {
        name: profile.primary_relay.name.clone(),
        port: profile.primary_relay.port,
    };
    let fallback = DynamicRelayTransport {
        name: profile
            .fallback_relay
            .as_ref()
            .map(|r| r.name.clone())
            .unwrap_or_else(|| "Relay-Beta (Резервный)".to_string()),
        port: profile
            .fallback_relay
            .as_ref()
            .map(|r| r.port)
            .unwrap_or(43112),
    };

    let mut transport_ctrl =
        TransportFailoverController::new(primary, fallback, FailoverConfig::default());
    transport_ctrl.start();
    let proxy_ep = transport_ctrl.active_proxy_endpoint();

    // 5. If pure CLI mode was explicitly requested
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
                    println!("  [Капсула] Соединение установлено: {}", target_destination);
                    println!(
                        "  [Капсула] Граница изоляции активна. Сетевые маршруты ОС не затронуты."
                    );
                }
            }
        }
        println!(
            "\nКлиент WebGate активен [Платформа: {:?} | Устройство: {}]",
            platform, device_id
        );
        return;
    }

    // 6. Default User Launch: Lightweight GUI Application Window
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
    println!(" WebGate Окно Клиента запущено: {}", ui_url);
    println!(" Выбор сервисов и привязка конфигураций доступны в открывшемся окне.");
    println!("───────────────────────────────────────────────────────────────────────────");

    let profile_arc = Arc::new(RwLock::new(profile));
    let profile_clone = Arc::clone(&profile_arc);
    let dev_id_clone = device_id.clone();
    let port = bound_addr.port();

    // Spawn server thread
    let server_handle = thread::spawn(move || {
        for s in listener.incoming().flatten() {
            handle_client_stream(s, &profile_clone, &dev_id_clone, port);
        }
    });

    // Launch Native Desktop Application Window
    launch_app_window(&ui_url);

    // Keep client alive for the UI session
    let _ = server_handle.join();
}
