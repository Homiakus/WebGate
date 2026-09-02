from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
TRUTH_JS = ROOT / "crates" / "webgate-app" / "src" / "client_ui_truth_patch.js"
CLIENT_MAIN = ROOT / "crates" / "webgate-app" / "src" / "main.rs"


class ClientUiTruthContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.truth_js = TRUTH_JS.read_text(encoding="utf-8")
        cls.client_main = CLIENT_MAIN.read_text(encoding="utf-8")

    def test_navigation_success_requires_authoritative_payload(self):
        self.assertIn("!response.ok", self.truth_js)
        self.assertIn("data.ok !== true", self.truth_js)
        self.assertIn("data.state !== 'transport_ready'", self.truth_js)
        self.assertIn("!data.protected_proxy", self.truth_js)

    def test_truth_patch_does_not_manufacture_open_session_success(self):
        forbidden = (
            "Сессия успешно установлена",
            "Капсула запущена для:",
            "Сессия установлена с",
        )
        for phrase in forbidden:
            self.assertNotIn(phrase, self.truth_js)
        self.assertIn(
            "Реальный запуск браузера будет подтверждаться отдельно",
            self.truth_js,
        )

    def test_core_failure_clears_unverified_profile_and_blocks_launch(self):
        self.assertIn("clearUnverifiedProfile();", self.truth_js)
        self.assertIn("coreOnline = false", self.truth_js)
        self.assertIn("ДЕМО-ДАННЫЕ НЕ ИСПОЛЬЗУЮТСЯ", self.truth_js)
        self.assertIn("button.disabled = busy || !coreOnline", self.truth_js)

    def test_config_success_requires_backend_commit_and_reconciliation(self):
        self.assertIn("data.status !== 'ok'", self.truth_js)
        self.assertIn("await truthfulLoadLiveBackend();", self.truth_js)
        self.assertIn("Активный профиль не изменён", self.truth_js)
        self.assertNotIn("parseConfigFile(file.name, content)", self.truth_js)

    def test_navigation_http_contract_is_semantic_not_always_200(self):
        self.assertIn('"400 Bad Request"', self.client_main)
        self.assertIn('"403 Forbidden"', self.client_main)
        self.assertIn('"503 Service Unavailable"', self.client_main)
        self.assertIn('r#"{{\"ok\":true,\"state\":\"transport_ready\"', self.client_main)
        self.assertIn("browser session is not yet opened", self.client_main)

    def test_truth_controller_is_injected_after_legacy_document_body(self):
        self.assertIn('include_str!("client_ui_truth_patch.js")', self.client_main)
        self.assertIn('CLIENT_UI_HTML.replacen("</body>", &patch, 1)', self.client_main)
        self.assertIn("WEBGATE_TRUTH_PATCH_ACTIVE", self.truth_js)


if __name__ == "__main__":
    unittest.main()
