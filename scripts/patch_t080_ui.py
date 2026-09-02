#!/usr/bin/env python3
from pathlib import Path

truth = Path('crates/webgate-app/src/client_ui_truth_patch.js')
s = truth.read_text()


def replace_once(old: str, new: str) -> None:
    global s
    count = s.count(old)
    assert count == 1, (old[:140], count)
    s = s.replace(old, new, 1)


replace_once("setLaunchBusy(false, 'ПРОВЕРИТЬ ДОСТУП');", "setLaunchBusy(false, 'ОТКРЫТЬ ПРИЛОЖЕНИЕ');")

old_failure = '''    function navigationFailureMessage(responseStatus, data) {
        if (responseStatus === 403) return 'Доступ к приложению запрещён политикой WebGate.';
        if (responseStatus === 503 || data?.transport_status === 'offline') {
            return 'Не удалось установить защищённый маршрут. Прямое подключение заблокировано.';
        }
        if (responseStatus === 400) return 'Запрос приложения некорректен или не поддерживается.';
        return data?.message || 'Ядро WebGate не подтвердило защищённый маршрут.';
    }
'''
new_failure = '''    function navigationFailureMessage(responseStatus, data) {
        if (data?.state === 'renderer_unqualified') {
            return 'Защищённый браузер ещё не квалифицирован. Приложение не открыто; системный браузерный fallback запрещён.';
        }
        if (responseStatus === 403 || data?.state === 'denied') {
            return 'Доступ к приложению запрещён политикой WebGate.';
        }
        if (responseStatus === 503 || data?.state === 'offline') {
            return 'Не удалось установить защищённую сессию. Прямое подключение заблокировано.';
        }
        if (responseStatus === 400) return 'Запрос приложения некорректен или не поддерживается.';
        return data?.message || 'Ядро WebGate не подтвердило защищённое открытие приложения.';
    }
'''
replace_once(old_failure, new_failure)

old_launch = '''    window.launchNavigation = async function launchNavigationTruthfully() {
        const url = document.getElementById('target-url-input').value.trim();
        if (!coreOnline) {
            showToast('Ядро WebGate недоступно. Защищённый запуск невозможен.', 'danger', 5000);
            setTruthState('offline', 'ЯДРО WEBGATE НЕДОСТУПНО · ЗАПУСК ЗАПРЕЩЁН');
            return;
        }
        if (!url.startsWith('webgate://') && !url.startsWith('https://')) {
            showToast('Разрешены только webgate:// и https:// назначения.', 'danger', 4500);
            logMessage('ОШИБКА_ПОЛИТИКИ', `Отклонён неподдерживаемый URL: ${url}`, 'err');
            return;
        }

        setLaunchBusy(true, 'ПРОВЕРКА...');
        logMessage('ДОСТУП', `Запрошена authoritative-проверка защищённого маршрута: ${url}`, 'normal');

        try {
            const response = await fetch('/api/navigate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ target_url: url })
            });
            const data = await readJsonSafely(response);

            if (!response.ok || !data || data.ok !== true || data.state !== 'transport_ready' || !data.protected_proxy) {
                const message = navigationFailureMessage(response.status, data);
                logMessage('ДОСТУП_ОТКЛОНЁН', message, 'err');
                showToast(message, 'danger', 5500);
                setTruthState(response.status === 403 ? 'degraded' : 'offline', message.toUpperCase());
                return;
            }

            transportStatus = data.transport_status || transportStatus;
            const message = 'Защищённый маршрут подтверждён. Реальный запуск браузера будет подтверждаться отдельно.';
            logMessage('МАРШРУТ', `${message} Цель: ${data.target}`, 'ok');
            showToast(message, 'success', 4500);
            setTruthState(transportStatus === 'degraded' ? 'degraded' : 'ready',
                transportStatus === 'degraded'
                    ? 'ЗАЩИЩЁННЫЙ МАРШРУТ ПОДТВЕРЖДЁН · РЕЖИМ DEGRADED'
                    : 'ЗАЩИЩЁННЫЙ МАРШРУТ ПОДТВЕРЖДЁН');
        } catch (error) {
            coreOnline = false;
            const message = 'Связь с ядром WebGate потеряна. Защищённый запуск не подтверждён.';
            logMessage('ЯДРО', `${message} ${error.message || error}`, 'err');
            showToast(message, 'danger', 5500);
            setTruthState('offline', 'ЯДРО WEBGATE НЕДОСТУПНО · ЗАЩИЩЁННЫЙ ЗАПУСК НЕ ПОДТВЕРЖДЁН');
        } finally {
            setLaunchBusy(false, coreOnline ? 'ПРОВЕРИТЬ ДОСТУП' : 'ЯДРО НЕДОСТУПНО');
        }
    };
'''
new_launch = '''    window.launchNavigation = async function launchNavigationTruthfully() {
        const url = document.getElementById('target-url-input').value.trim();
        if (!coreOnline) {
            showToast('Ядро WebGate недоступно. Защищённый запуск невозможен.', 'danger', 5000);
            setTruthState('offline', 'ЯДРО WEBGATE НЕДОСТУПНО · ЗАПУСК ЗАПРЕЩЁН');
            return;
        }
        if (!url.startsWith('webgate://') && !url.startsWith('https://')) {
            showToast('Разрешены только webgate:// и https:// назначения.', 'danger', 4500);
            logMessage('ОШИБКА_ПОЛИТИКИ', `Отклонён неподдерживаемый URL: ${url}`, 'err');
            return;
        }

        setLaunchBusy(true, 'ЗАПУСК...');
        logMessage('СЕССИЯ', `Запрошен защищённый запуск приложения: ${url}`, 'normal');

        try {
            const response = await fetch('/api/session/open', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ target_url: url })
            });
            const data = await readJsonSafely(response);

            if (!response.ok || !data || data.ok !== true || data.state !== 'open' || !data.session_id) {
                const message = navigationFailureMessage(response.status, data);
                const rendererBlocked = data?.state === 'renderer_unqualified';
                logMessage(rendererBlocked ? 'БРАУЗЕР_НЕ_КВАЛИФИЦИРОВАН' : 'ДОСТУП_ОТКЛОНЁН',
                    `${message}${data?.session_id ? ` Сессия: ${data.session_id}` : ''}`, 'err');
                showToast(message, rendererBlocked ? 'warning' : 'danger', 6500);
                setTruthState(rendererBlocked || response.status === 403 ? 'degraded' : 'offline', message.toUpperCase());
                return;
            }

            const message = `Защищённое приложение открыто. Сессия: ${data.session_id}`;
            logMessage('СЕССИЯ_OPEN', `${message} Цель: ${data.target}`, 'ok');
            showToast(message, 'success', 4500);
            setTruthState('ready', `ЗАЩИЩЁННОЕ ПРИЛОЖЕНИЕ ОТКРЫТО · СЕССИЯ ${data.session_id}`);
        } catch (error) {
            coreOnline = false;
            const message = 'Связь с ядром WebGate потеряна. Защищённое приложение не открыто.';
            logMessage('ЯДРО', `${message} ${error.message || error}`, 'err');
            showToast(message, 'danger', 5500);
            setTruthState('offline', 'ЯДРО WEBGATE НЕДОСТУПНО · ЗАЩИЩЁННОЕ ПРИЛОЖЕНИЕ НЕ ОТКРЫТО');
        } finally {
            setLaunchBusy(false, coreOnline ? 'ОТКРЫТЬ ПРИЛОЖЕНИЕ' : 'ЯДРО НЕДОСТУПНО');
        }
    };
'''
replace_once(old_launch, new_launch)
truth.write_text(s)

# Update the source-level truth contract for T-080 session semantics.
test_path = Path('scripts/tests/test_client_ui_truth.py')
t = test_path.read_text()
t = t.replace(
    '''        self.assertIn("data.state !== 'transport_ready'", self.truth_js)\n        self.assertIn("!data.protected_proxy", self.truth_js)''',
    '''        self.assertIn("fetch('/api/session/open'", self.truth_js)\n        self.assertIn("data.state !== 'open'", self.truth_js)\n        self.assertIn("!data.session_id", self.truth_js)''',
)
t = t.replace(
    '''        self.assertIn(\n            "Реальный запуск браузера будет подтверждаться отдельно",\n            self.truth_js,\n        )''',
    '''        self.assertIn("renderer_unqualified", self.truth_js)\n        self.assertIn("системный браузерный fallback запрещён", self.truth_js)\n        self.assertIn("Защищённое приложение открыто. Сессия:", self.truth_js)''',
)
t = t.replace(
    '''        self.assertIn('r#"{{\\\"ok\\\":true,\\\"state\\\":\\\"transport_ready\\\"', self.client_main)\n        self.assertIn("browser session is not yet opened", self.client_main)''',
    '''        self.assertIn('path == "/api/session/open"', self.client_main)\n        self.assertIn("ApplicationSessionState::RendererUnqualified", self.client_main)\n        self.assertIn("session_http_status(snapshot.state)", self.client_main)\n        self.assertIn("browser session is not yet opened", self.client_main)''',
)
test_path.write_text(t)

# Evolve the real-browser harness from route-preflight truth to session-open truth.
browser_path = Path('scripts/test_client_ui_browser.py')
b = browser_path.read_text()
replacements = {
    "scenario === 'navigate_success' || scenario === 'navigate_ok_false' ||\n          scenario === 'navigate_503' || scenario === 'navigate_malformed' ||\n          scenario === 'navigate_disconnect'": "scenario === 'session_open_success' || scenario === 'session_ok_false' ||\n          scenario === 'session_503' || scenario === 'session_malformed' ||\n          scenario === 'session_disconnect' || scenario === 'renderer_unqualified'",
    "if (scenario === 'navigate_success') {": "if (scenario === 'session_open_success') {",
    "body.includes('ЗАЩИЩЁННЫЙ МАРШРУТ ПОДТВЕРЖДЁН') &&\n            body.includes('Реальный запуск браузера будет подтверждаться отдельно')": "body.includes('ЗАЩИЩЁННОЕ ПРИЛОЖЕНИЕ ОТКРЫТО') &&\n            body.includes('wgs-browser-test')",
    "finish(ok, 'success is transport_ready, never fabricated browser-open state');": "finish(ok, 'Open is rendered only from authoritative session-open proof');",
    "const failureVisible = body.includes('не подтвердило') ||\n          body.includes('недоступ') || body.includes('потеряна') ||\n          body.includes('ОТКЛОНЁН') || body.includes('ошибк');": "const failureVisible = body.includes('не подтвердило') ||\n          body.includes('недоступ') || body.includes('потеряна') ||\n          body.includes('ОТКЛОНЁН') || body.includes('ошибк') ||\n          body.includes('НЕ КВАЛИФИЦИРОВАН') || body.includes('не квалифицирован');",
    "if path == \"/api/navigate\":": "if path == \"/api/session/open\":",
    "if scenario == \"navigate_success\":": "if scenario == \"session_open_success\":",
    "\"state\": \"transport_ready\",\n                        \"target\": \"webgate://service/docs/overview\",\n                        \"transport_status\": \"ready\",\n                        \"protected_proxy\": \"127.0.0.1:43117\",": "\"state\": \"open\",\n                        \"session_id\": \"wgs-browser-test\",\n                        \"target\": \"webgate://service/docs/overview\",\n                        \"message\": \"protected application open\",",
    "elif scenario == \"navigate_ok_false\":": "elif scenario == \"session_ok_false\":",
    "elif scenario == \"navigate_503\":": "elif scenario == \"session_503\":",
    "elif scenario == \"navigate_malformed\":": "elif scenario == \"session_malformed\":",
    "elif scenario == \"navigate_disconnect\":": "elif scenario == \"session_disconnect\":",
    '        "navigate_success",\n        "navigate_ok_false",\n        "navigate_503",\n        "navigate_malformed",\n        "navigate_disconnect",': '        "session_open_success",\n        "session_ok_false",\n        "session_503",\n        "session_malformed",\n        "session_disconnect",\n        "renderer_unqualified",',
}
for old, new in replacements.items():
    count = b.count(old)
    assert count == 1, (old[:100], count)
    b = b.replace(old, new, 1)

# Insert the explicit current-build blocker response before the generic unexpected branch.
anchor = '''            elif scenario == "session_disconnect":
                with contextlib.suppress(Exception):
                    self.connection.shutdown(socket.SHUT_RDWR)
                self.connection.close()
            else:
'''
replacement = '''            elif scenario == "session_disconnect":
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
'''
assert b.count(anchor) == 1, b.count(anchor)
b = b.replace(anchor, replacement, 1)
browser_path.write_text(b)
