from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
SERVER = ROOT / "server"
UPSTREAM_SHA = "827abb1add11a9fcbd0a9944e65efbd20c675739"


class SecureAccessDependencyContractTests(unittest.TestCase):
    def test_server_uses_secureaccess_supported_go_line(self):
        go_mod = (SERVER / "go.mod").read_text(encoding="utf-8")
        self.assertIn("\ngo 1.26\n", go_mod)

    def test_secureaccess_snapshot_is_pinned_and_local(self):
        go_mod = (SERVER / "go.mod").read_text(encoding="utf-8")
        self.assertIn("github.com/Homiakus/secureaccess", go_mod)
        self.assertIn("replace github.com/Homiakus/secureaccess => ./third_party/secureaccess", go_mod)

        provenance = SERVER / "third_party" / "secureaccess" / "UPSTREAM.md"
        self.assertTrue(provenance.is_file(), "pinned SecureAcces provenance is missing")
        text = provenance.read_text(encoding="utf-8")
        self.assertIn(UPSTREAM_SHA, text)
        self.assertIn("v0.4.0", text)


if __name__ == "__main__":
    unittest.main()
