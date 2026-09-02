#!/usr/bin/env python3
"""Headless-browser qualification for the WebGate client truth controller.

Runs the real client_ui.html plus client_ui_truth_patch.js in Chromium against
controlled local /api/* responses. The harness intentionally uses only the Python
standard library and a preinstalled Chromium/Chrome binary.
"""

from __future__ import annotations

import contextlib
import http.server
import json
import re
import shutil
import socket
import subprocess
import sys
import threading
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
    for candidate in (
        "google-chrome",
        "google-chrome-stable",
        "chromium",
        "chromium-browser",
    ):
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


def prepare_base_document(document: str) -> str:
    """Remove nonessential remote font requests from the deterministic CI fixture."""
    document = re.sub(
        r"\s*<link[^>]+href=\"https://fonts\.googleapis\.com[^>]*>\s*",
        "\n",
        document,
        flags=re.IGNORECASE,
    )
    document = re.sub(
        r"\s*<link[^>]+href=\"https://fonts\.gstatic\.com[^>]*>\s*",
        "\n",
        document,
        flags=re.IGNORECASE,
    )
    return document


def scenario_probe_script(name: str) -> str:
    quoted = json.dumps(name)
    return f"""
(() => {{
  const scenario = {quoted};
  const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));
  const text = () => document.body.innerText || '';
  const finish = (ok, detail) => {{
    const pre = document.createElement('pre');
    pre.id = 'browser-truth-result';
    pre.dataset.result = ok ? 'pass' : 'fail';
    pre.textContent = (ok ? 'WEBGATE_BROWSER_TRUTH_PASS:' : 'WEBGATE_BROWSER_TRUTH_FAIL:') + scenario + ':' + detail;
    document.body.appendChild(pre);
  }};

  setTimeout(async () => {{
    try {{
      await sleep(250);

      if (scenario === 'core_offline') {{
        const launch = document.getElementById('btn-launch');
        const body = text();
        const ok = body.includes('ДЕМО-ДАННЫЕ НЕ ИСПОЛЬЗУЮТСЯ') &&
          launch && launch.disabled &&
          !body.includes('FactoryOS Production Terminal');
        finish(ok, 'offline core is explicit and launch is blocked');
        return;
      }}

      if (scenario === 'session_open_success' || scenario === 'session_ok_false' ||
          scenario === 'session_503' || scenario === 'session_malformed' ||
          scenario === 'session_disconnect' || scenario === 'renderer_unqualified') {{
        const input = document.getElementById('target-url-input');
        if (!input || typeof window.launchNavigation !== 'function') {{
          finish(false, 'navigation controls are not initialized');
          return;
        }}
        input.value = 'webgate://service/docs/overview';
        await window.launchNavigation();
        await sleep(150);
        const body = text();
        const oldFalseSuccess = body.includes('Сессия успешно установлена') ||
          body.includes('Капсула запущена для:') ||
          body.includes('Сессия установлена с');

        if (scenario === 'session_open_success') {{
          const ok = body.includes('ЗАЩИЩЁННОЕ ПРИЛОЖЕНИЕ ОТКРЫТО') &&
            body.includes('wgs-browser-test') &&
            !oldFalseSuccess;
          finish(ok, 'Open is rendered only from authoritative session-open proof');
          return;
        }}

        const failureVisible = body.includes('не подтвердило') ||
          body.includes('недоступ') || body.includes('потеряна') ||
          body.includes('ОТКЛОНЁН') || body.includes('ошибк') ||
          body.includes('НЕ КВАЛИФИЦИРОВАН') || body.includes('не квалифицирован');
        finish(
          failureVisible && !oldFalseSuccess,
          'negative navigation cannot render synthetic success'
        );
        return;
      }}

      if (scenario === 'config_rejected') {{
        const input = document.getElementById('file-input');
        const profileName = document.getElementById('cfg-profile-name');
        if (!input || !profileName) {{
          finish(false, 'configuration controls are not initialized');
          return;
        }}
        const before = profileName.innerText;
        const file = new File([
          'profile_id = "rejected"\\nprimary_relay_addr = "127.0.0.1"\\nprimary_relay_port = 43111\\n'
        ], 'rejected.toml', {{type: 'text/plain'}});
        const transfer = new DataTransfer();
        transfer.items.add(file);
        input.files = transfer.files;
        input.dispatchEvent(new Event('change', {{bubbles: true}}));
        await sleep(500);
        const after = profileName.innerText;
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
  }}, 25);
}})();
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
            self.send_header("Cache-Control", "no-store")
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

        if path == "/api/session/open":
            scenario = self.server.scenario
            if scenario == "session_open_success":
                self._send_json(
                    200,
                    {
                        "ok": True,
                        "state": "open",
                        "session_id": "wgs-browser-test",
                        "target": "webgate://service/docs/overview",
                        "message": "protected application open",
                    },
                )
            elif scenario == "session_ok_false":
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
            elif scenario == "session_503":
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
            elif scenario == "session_malformed":
                body = b"{not-json"
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            elif scenario == "session_disconnect":
                with contextlib.suppress(Exception):
                    self.connection.shutdown(socket.SHUT_RDWR)
                self.connection.close()
            elif scenario == "renderer_unqualified":
                self._send_json(
                    503,
                    {
                        "ok": False,
                        "state": "renderer_unqualified",
                        "session_id": "wgs-browser-blocked",
                        "target": "webgate://service/docs/overview",
                        "message": "embedded renderer is not production-qualified",
                    },
                )
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
            "--disable-extensions",
            "--virtual-time-budget=3000",
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
        pass_marker = f"WEBGATE_BROWSER_TRUTH_PASS:{scenario}:"
        fail_marker = f"WEBGATE_BROWSER_TRUTH_FAIL:{scenario}:"
        if result.returncode != 0 or pass_marker not in result.stdout:
            detail = "browser produced no result marker"
            if fail_marker in result.stdout:
                detail = result.stdout.split(fail_marker, 1)[1].split("<", 1)[0]
            diagnostic = result.stdout[-4000:]
            raise AssertionError(
                f"scenario {scenario!r} failed: {detail}\n"
                f"chrome exit={result.returncode}\n"
                f"stdout tail={diagnostic}\n"
                f"stderr tail={result.stderr[-1500:]}\n"
            )
        print(f"PASS {scenario}")
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


def main() -> int:
    chrome = find_chrome()
    base_document = prepare_base_document(CLIENT_HTML.read_text(encoding="utf-8"))
    scenarios = (
        "core_offline",
        "session_open_success",
        "session_ok_false",
        "session_503",
        "session_malformed",
        "session_disconnect",
        "renderer_unqualified",
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
