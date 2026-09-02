#!/usr/bin/env python3
import json
import tempfile
import unittest
from pathlib import Path

from scripts.build_distribution import (
    MANIFEST_SCHEMA,
    compute_sha256,
    create_signed_manifest,
    verify_manifest,
)


class BuildDistributionTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.dir_path = Path(self.temp_dir.name)

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_computes_sha256_accurately(self):
        sample_file = self.dir_path / "WebGate-sample.exe"
        sample_file.write_bytes(b"WebGate Secure Access Binary Payload v1.0.0")
        digest = compute_sha256(sample_file)
        self.assertEqual(len(digest), 64)

    def test_creates_and_verifies_valid_signed_manifest_windows(self):
        artifact = self.dir_path / "WebGate-1.0.0-windows-x86_64.exe"
        artifact.write_bytes(b"windows_x86_64_installer_mock_payload")

        secret = "super_secure_release_signing_key"
        manifest = create_signed_manifest(
            version="1.0.0",
            channel="stable",
            source_commit="d0c8199756fd204caa335f59a83e41a4787c7bc8",
            platform="windows",
            arch="x86_64",
            artifact_path=artifact,
            signing_key_id="release-2026-prod",
            signing_secret=secret,
        )

        self.assertEqual(manifest.schema, MANIFEST_SCHEMA)
        self.assertEqual(manifest.version, "1.0.0")
        self.assertEqual(manifest.platform, "windows")
        self.assertEqual(manifest.arch, "x86_64")
        self.assertEqual(manifest.artifact, "WebGate-1.0.0-windows-x86_64.exe")

        manifest_file = self.dir_path / "manifest.json"
        manifest_file.write_text(manifest.to_json(), encoding="utf-8")

        ok, msg = verify_manifest(manifest_file, self.dir_path, signing_secret=secret)
        self.assertTrue(ok, f"Verification failed: {msg}")

    def test_creates_and_verifies_valid_signed_manifest_android(self):
        artifact = self.dir_path / "WebGate-1.0.0-android-arm64.apk"
        artifact.write_bytes(b"android_arm64_apk_mock_payload")

        secret = "android_signing_key_secret"
        manifest = create_signed_manifest(
            version="1.0.0",
            channel="stable",
            source_commit="d0c8199756fd204caa335f59a83e41a4787c7bc8",
            platform="android",
            arch="arm64",
            artifact_path=artifact,
            signing_key_id="release-2026-android",
            signing_secret=secret,
        )

        manifest_file = self.dir_path / "manifest_android.json"
        manifest_file.write_text(manifest.to_json(), encoding="utf-8")

        ok, msg = verify_manifest(manifest_file, self.dir_path, signing_secret=secret)
        self.assertTrue(ok, f"Android verification failed: {msg}")

    def test_verification_fails_on_tampered_artifact(self):
        artifact = self.dir_path / "WebGate-tampered.exe"
        artifact.write_bytes(b"original_payload")

        secret = "release_secret"
        manifest = create_signed_manifest(
            version="1.0.0",
            channel="stable",
            source_commit="abcdef123456",
            platform="windows",
            arch="x86_64",
            artifact_path=artifact,
            signing_key_id="key-1",
            signing_secret=secret,
        )

        manifest_file = self.dir_path / "manifest.json"
        manifest_file.write_text(manifest.to_json(), encoding="utf-8")

        # Tamper with the artifact
        artifact.write_bytes(b"tampered_malicious_payload")

        ok, msg = verify_manifest(manifest_file, self.dir_path, signing_secret=secret)
        self.assertFalse(ok)
        self.assertIn("SHA256 mismatch", msg)

    def test_verification_fails_on_tampered_signature(self):
        artifact = self.dir_path / "WebGate-valid.exe"
        artifact.write_bytes(b"clean_payload")

        secret = "release_secret"
        manifest = create_signed_manifest(
            version="1.0.0",
            channel="stable",
            source_commit="abcdef123456",
            platform="windows",
            arch="x86_64",
            artifact_path=artifact,
            signing_key_id="key-1",
            signing_secret=secret,
        )

        manifest_file = self.dir_path / "manifest.json"
        manifest_file.write_text(manifest.to_json(), encoding="utf-8")

        # Wrong secret verification
        ok, msg = verify_manifest(manifest_file, self.dir_path, signing_secret="wrong_secret")
        self.assertFalse(ok)
        self.assertIn("Signature verification failed", msg)

    def test_creates_and_verifies_valid_signed_manifest_relay_and_server(self):
        relay_art = self.dir_path / "webgate-relay.exe"
        relay_art.write_bytes(b"webgate_relay_executable_bytes_2026")
        server_art = self.dir_path / "webgate-server.exe"
        server_art.write_bytes(b"webgate_server_gateway_bytes_2026")

        secret = "corp_release_signing_secret"

        # Relay manifest
        manifest_relay = create_signed_manifest(
            version="1.0.0",
            channel="stable",
            source_commit="commit123456",
            platform="linux",
            arch="x86_64",
            artifact_path=relay_art,
            signing_key_id="release-relay-key",
            signing_secret=secret,
        )
        relay_manifest_file = self.dir_path / "manifest-relay.json"
        relay_manifest_file.write_text(manifest_relay.to_json(), encoding="utf-8")
        ok, msg = verify_manifest(relay_manifest_file, self.dir_path, signing_secret=secret)
        self.assertTrue(ok, f"Relay verification failed: {msg}")

        # Server manifest
        manifest_server = create_signed_manifest(
            version="1.0.0",
            channel="stable",
            source_commit="commit123456",
            platform="linux",
            arch="x86_64",
            artifact_path=server_art,
            signing_key_id="release-server-key",
            signing_secret=secret,
        )
        server_manifest_file = self.dir_path / "manifest-server.json"
        server_manifest_file.write_text(manifest_server.to_json(), encoding="utf-8")
        ok, msg = verify_manifest(server_manifest_file, self.dir_path, signing_secret=secret)
        self.assertTrue(ok, f"Server verification failed: {msg}")

    def test_create_manifest_rejects_unsupported_platforms_and_channels(self):
        art = self.dir_path / "dummy.bin"
        art.write_bytes(b"dummy")

        with self.assertRaises(ValueError):
            create_signed_manifest(
                version="1.0.0",
                channel="stable",
                source_commit="commit",
                platform="solaris",
                arch="x86_64",
                artifact_path=art,
                signing_key_id="k1",
                signing_secret="s1",
            )

        with self.assertRaises(ValueError):
            create_signed_manifest(
                version="1.0.0",
                channel="experimental-alpha",
                source_commit="commit",
                platform="windows",
                arch="x86_64",
                artifact_path=art,
                signing_key_id="k1",
                signing_secret="s1",
            )

    def test_create_manifest_rejects_nonexistent_artifact(self):
        nonexistent = self.dir_path / "does_not_exist.exe"
        with self.assertRaises(FileNotFoundError):
            create_signed_manifest(
                version="1.0.0",
                channel="stable",
                source_commit="commit",
                platform="windows",
                arch="x86_64",
                artifact_path=nonexistent,
                signing_key_id="k1",
                signing_secret="s1",
            )


if __name__ == "__main__":
    unittest.main()
