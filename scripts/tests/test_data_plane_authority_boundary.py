from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
SERVER = ROOT / "server"


class DataPlaneAuthorityBoundaryTests(unittest.TestCase):
    def test_gateway_depends_on_narrow_service_authorizer_spi(self):
        source = (SERVER / "pkg" / "gateway" / "gateway.go").read_text(encoding="utf-8")
        self.assertIn("authorizer auth.ServiceAuthorizer", source)
        self.assertNotIn("*auth.SecureAccessAuthorizer", source)
        self.assertIn("AuthorizeServiceAccess(r.Context(),", source)

    def test_production_data_plane_does_not_construct_surrogate_authority(self):
        source = (SERVER / "cmd" / "webgate-server" / "main.go").read_text(encoding="utf-8")
        self.assertNotIn("authorizer := auth.NewSecureAccessAuthorizer()", source)
        self.assertIn("serviceAuthorizer := auth.NewUnavailableServiceAuthorizer()", source)
        self.assertIn("gateway.NewServerGateway(svcReg, devReg, serviceAuthorizer", source)

    def test_fail_closed_authority_spi_exists(self):
        source_path = SERVER / "pkg" / "auth" / "service_authorizer.go"
        self.assertTrue(source_path.is_file(), "service authorization SPI is missing")
        source = source_path.read_text(encoding="utf-8")
        self.assertIn("type ServiceAuthorizer interface", source)
        self.assertIn("ErrAuthorizationAuthorityUnavailable", source)
        self.assertIn("context.Context", source)


if __name__ == "__main__":
    unittest.main()
