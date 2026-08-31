from pathlib import Path
import hashlib
import unittest


ROOT = Path(__file__).resolve().parents[2]
SERVER = ROOT / "server"
SNAPSHOT = SERVER / "third_party" / "secureaccess"
UPSTREAM_SHA = "827abb1add11a9fcbd0a9944e65efbd20c675739"
EXPECTED_BLOBS = {
    "go.mod": "594a9c44026deef24815096e8b750db3291d09f3",
    "LICENSE": "094569d25a7322deca171590dabdc253bb5a452b",
    "doc.go": "9ada938d38d5786559cb772d92aa4fc25b53eca9",
    "secureaccess/doc.go": "eeaa2c064abf01b8fdd21e76ce6fb3ec6cde2ccc",
    "secureaccess/version.go": "de3bf071e5228d94fe1df531a2c359b0046e39be",
}


def git_blob_sha(data: bytes) -> str:
    header = f"blob {len(data)}\0".encode("ascii")
    return hashlib.sha1(header + data).hexdigest()


class SecureAccessDependencyContractTests(unittest.TestCase):
    def test_server_uses_secureaccess_supported_go_line(self):
        go_mod = (SERVER / "go.mod").read_text(encoding="utf-8")
        self.assertIn("\ngo 1.26\n", go_mod)
        self.assertIn("\ntoolchain go1.26.6\n", go_mod)

    def test_secureaccess_snapshot_is_pinned_and_local(self):
        go_mod = (SERVER / "go.mod").read_text(encoding="utf-8")
        self.assertIn("require github.com/Homiakus/secureaccess v0.4.0", go_mod)
        self.assertIn("replace github.com/Homiakus/secureaccess => ./third_party/secureaccess", go_mod)

        provenance = SNAPSHOT / "UPSTREAM.md"
        self.assertTrue(provenance.is_file(), "pinned SecureAcces provenance is missing")
        text = provenance.read_text(encoding="utf-8")
        self.assertIn(UPSTREAM_SHA, text)
        self.assertIn("v0.4.0", text)
        self.assertIn("does **not** claim that SecureAccess authorization is integrated yet", text)

    def test_secureaccess_anchor_files_are_byte_identical_to_upstream_blobs(self):
        for relative, expected_sha in EXPECTED_BLOBS.items():
            path = SNAPSHOT / relative
            self.assertTrue(path.is_file(), f"missing pinned upstream file: {relative}")
            self.assertEqual(
                expected_sha,
                git_blob_sha(path.read_bytes()),
                f"vendored SecureAccess file drifted from upstream: {relative}",
            )


if __name__ == "__main__":
    unittest.main()
