#!/usr/bin/env python3
"""Headless-browser qualification for the WebGate client truth controller.

This runs the real client_ui.html plus client_ui_truth_patch.js in Chromium against
controlled local /api/* responses. It intentionally uses only the Python standard
library and a preinstalled Chromium/Chrome binary so no JavaScript test framework
becomes a production dependency.
"""

from __future__ import annotations

import contextlib
import html
import http.server
import json
import shutil
import socket
import subprocess
import sys
import threading
import time
from pathlib import Path
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parents[1]
CLIENT_HTML = ROOT / "crates" / "webgate-app" / "src" / "client_ui.html"
TRUTH_JS = ROOT / "crates" / "webgate-app" / "src" / "client_ui_truth_patch.js"

PROFILE = {
    "profile_id": "browser-test",
    "profile_name": "Browser Test Workspace",
    "version": "test",
    "primary_relay": {"name": "test", "address": "127.0.0.1", "port": 43111},
    "fallback_relay": None,
    "destinations": [
        {
            "id": "docs",
            "name": "Docs",
            "url": "webgate://service/docs/overview",
            "category": "Test",
            "description": "Browser truth test",
        }
    ],
}


def find_chrome() -> str:
    candidates = (
        "google-chrome",
        "google-chrome-stable",
        "chromium",
        "chromium-browser",
    )
    for candidate in candidates:
        found = shutil.which(candidate)
        if found:
            return found
    raise RuntimeError(
        "Chromium/Chrome is required for client UI browser qualification; none found"
    )


def inject_before_body(document: str, script: str) -> str:
    marker = "</body>"
    if marker not in document:
        raise RuntimeError("client_ui.html has no closing body tag")
    return document.replace(marker, f"<script>\n{script}\n</script>\n{marker}", 1)


def scenario_probe_script(name: str) -> str:
    quoted = json.dumps(name)
    return f"""
<script>
(() => {{
  const scenario = {quoted};
  const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));
  const text = () => document.body.innerText || '';
  const finish = (ok, detail) => {{
    const pre = document.createElement('pre');
    pre.id = 'browser-truth-result';
    pre.textContent = (ok ? 'WEBGATE_BROWSER_TRUTH_PASS:' : 'WEBGATE_BROWSER_TRUTH_FAIL:') + scenario + ':' + detail;
    document.body.appendChild(pre);
  }};

  window.addEventListener('load', async () => {{
    await sleep(350);
    try {{
      if (scenario === 'core_offline') {{
        const launch = document.getElementById('btn-launch');
        const ok = text().includes('ДЕМО-ДАННЫЕ НЕ ИСПОЛЬЗУЮТСЯ') &&
          launch && launch.disabled &&
          !text().includes('FactoryOS Production Terminal');
        finish(ok, 'offline core is explicit and launch is blocked');
        return;
      }}

      if (scenario === 'navigate_success' || scenario === 'navigate_ok_false' ||
          scenario === 'navigate_503' || scenario === 'navigate_malformed' ||
          scenario === 'navigate_disconnect') {{
        const input = document.getElementById('target-url-input');
        input.value = 'webgate://service/docs/overview';
        await window.launchNavigation();
        await sleep(150);
        const body = text();
        const oldFalseSuccess = body.includes('Сессия успешно установлена') ||
          body.includes('Капсула запущена для:') ||
          body.includes('Сессия установлена с');

        if (scenario === 'navigate_success') {{
          const ok = body.includes('ЗАЩИЩЁННЫЙ МАРШРУТ ПОДТВЕРЖДЁН') &&
            body.includes('Реальный запуск браузера будет подтверждаться отдельно') &&
            !oldFalseSuccess;
          finish(ok, 'success is transport_ready, never fabricated browser-open state');
          return;
        }}

        const failureVisible = body.includes('не подтвердило') ||
          body.includes('недоступ') || body.includes('потеряна') ||
          body.includes('ОТКЛОНЁН');
        finish(failureVisible && !oldFalseSuccess, 'negative navigation cannot render synthetic success');
        return;
      }}

      if (scenario === 'config_rejected') {{
        const input = document.getElementById('file-input');
        const before = document.getElementById('cfg-profile-name').innerText;
        const file = new File([
          'profile_id = "rejected"\\nprimary_relay_addr = "127.0.0.1"\\nprimary_relay_port = 43111\\n'
        ], 'rejected.toml', {{type: 'text/plain'}});
        const transfer = new DataTransfer();
        transfer.items.add(file);
        input.files = transfer.files;
        input.dispatchEvent(new Event('change', {{bubbles: true}}));
        await sleep(500);
        const after = document.getElementById('cfg-profile-name').innerText;
        const body = text();
        const ok = before === 'Browser Test Workspace' &&
          after === 'Browser Test Workspace' &&
          body.includes('Активный профиль не изменён');
        finish(ok, 'rejected config leaves authoritative profile unchanged');
        return;
      }}

      finish(false, 'unknown scenario');
    }} catch (error) {{
      finish(false, String(error && error.stack || error));
    }}
  }});
}})();
</script>
"""


class ScenarioServer(http.server.ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, address: tuple[str, int], scenario: str, document: str):
        self.scenario = scenario
        self.document = document
        super().__init__(address, ScenarioHandler)


class ScenarioHandler(http.server.BaseHTTPRequestHandler):
    server: ScenarioServer

    def log_message(self, _format: str, *_args: object) -> None:
        return

    def _send_json(self, status: int, payload: object) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler API
        path = urlparse(self.path).path
        if path in ("/", "/index.html"):
            body = self.server.document.encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if path == "/api/profile":
            if self.server.scenario == "core_offline":
                self._send_json(503, {"status": "offline"})
            else:
                self._send_json(200, PROFILE)
            return
        if path == "/api/status":
            if self.server.scenario == "core_offline":
                self._send_json(503, {"status": "offline"})
            else:
                self._send_json(
                    200,
                    {
                        "status": "ready",
                        "device_id": "dev_browser_test",
                        "platform": "Linux",
                        "protected_proxy": "127.0.0.1:43117",
                    },
                )
            return
        self.send_error(404)

    def do_POST(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler API
        path = urlparse(self.path).path
        length = int(self.headers.get("Content-Length", "0"))
        if length:
            self.rfile.read(length)

        if path == "/api/navigate":
            scenario = self.server.scenario
            if scenario == "navigate_success":
                self._send_json(
                    200,
                    {
                        "ok": True,
                        "state": "transport_ready",
                        "target": "webgate://service/docs/overview",
                        "transport_status": "ready",
                        "protected_proxy": "127.0.0.1:43117",
                    },
                )
            elif scenario == "navigate_ok_false":
                self._send_json(
                    200,
                    {
                        "ok": False,
                        "state": "offline",
                        "message": "route not confirmed",
                        "transport_status": "offline",
                        "protected_proxy": None,
                    },
                )
            elif scenario == "navigate_503":
                self._send_json(
                    503,
                    {
                        "ok": False,
                        "state": "offline",
                        "message": "protected transport unavailable",
                        "transport_status": "offline",
                        "protected_proxy": None,
                    },
                )
            elif scenario == "navigate_malformed":
                body = b"{not-json"
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            elif scenario == "navigate_disconnect":
                with contextlib.suppress(Exception):
                    self.connection.shutdown(socket.SHUT_RDWR)
                self.connection.close()
            else:
                self._send_json(500, {"ok": False, "message": "unexpected scenario"})
            return

        if path == "/api/bind_config":
            if self.server.scenario == "config_rejected":
                self._send_json(
                    400,
                    {"status": "error", "message": "candidate rejected by authority"},
                )
            else:
                self._send_json(500, {"status": "error", "message": "not configured"})
            return

        self.send_error(404)


def run_scenario(chrome: str, scenario: str, base_document: str) -> None:
    document = inject_before_body(
        inject_before_body(base_document, TRUTH_JS.read_text(encoding="utf-8")),
        scenario_probe_script(scenario),
    )
    server = ScenarioServer(("127.0.0.1", 0), scenario, document)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        url = f"http://127.0.0.1:{server.server_port}/"
        command = [
            chrome,
            "--headless=new",
            "--disable-gpu",
            "--no-sandbox",
            "--disable-dev-shm-usage",
            "--disable-background-networking",
            "--virtual-time-budget=2500",
            "--dump-dom",
            url,
        ]
        result = subprocess.run(
            command,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=30,
            check=False,
        )
        marker = f"WEBGATE_BROWSER_TRUTH_PASS:{scenario}:"
        if result.returncode != 0 or marker not in result.stdout:
            fail_fragment = "WEBGATE_BROWSER_TRUTH_FAIL:"
            detail = "browser produced no result marker"
            if fail_fragment in result.stdout:
                tail = result.stdout.split(fail_fragment, 1)[1].split("<", 1)[0]
                detail = html.unescape(tail)
            raise AssertionError(
                f"scenario {scenario!r} failed: {detail}\n"
                f"chrome exit={result.returncode}\n"
                f"stderr tail={result.stderr[-1500:]}\n"
            )
        print(f"PASS {scenario}")
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


def main() -> int:
    chrome = find_chrome()
    base_document = CLIENT_HTML.read_text(encoding="utf-8")
    scenarios = (
        "core_offline",
        "navigate_success",
        "navigate_ok_false",
        "navigate_503",
        "navigate_malformed",
        "navigate_disconnect",
        "config_rejected",
    )
    print(f"Browser: {chrome}")
    for scenario in scenarios:
        run_scenario(chrome, scenario, base_document)
    print(f"Qualified {len(scenarios)} client truth scenarios in a real headless browser")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"CLIENT UI BROWSER QUALIFICATION FAILED: {error}", file=sys.stderr)
        raise
