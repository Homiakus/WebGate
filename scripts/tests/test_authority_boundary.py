from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
SERVER = ROOT / "server"


class AuthorityBoundaryContractTests(unittest.TestCase):
    def test_gateway_depends_on_narrow_service_authorizer_spi(self):
        source = (SERVER / "pkg" / "gateway" / "gateway.go").read_text(encoding="utf-8")
        self.assertIn("authorizer auth.ServiceAuthorizer", source)
        self.assertNotIn("*auth.SecureAccessAuthorizer", source)
        self.assertIn("AuthorizeServiceAccess(r.Context(),", source)

    def test_production_bootstrap_does_not_construct_webgate_surrogate_authority(self):
        source = (SERVER / "cmd" / "webgate-server" / "main.go").read_text(encoding="utf-8")
        self.assertNotIn("NewSecureAccessAuthorizer", source)
        self.assertIn("NewUnavailableServiceAuthorizer", source)

    def test_legacy_webgate_session_membership_authority_is_removed(self):
        legacy = SERVER / "pkg" / "auth" / "secureaccess.go"
        self.assertFalse(legacy.exists(), "WebGate-owned session/membership authority remains in production source")

    def test_admin_plane_does_not_depend_on_data_plane_surrogate(self):
        source = (SERVER / "pkg" / "admin" / "api.go").read_text(encoding="utf-8")
        self.assertNotIn("*auth.SecureAccessAuthorizer", source)


if __name__ == "__main__":
    unittest.main()
