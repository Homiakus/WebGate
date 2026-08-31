from __future__ import annotations

import importlib.util
import json
import sys
import unittest
from pathlib import Path

MODULE_PATH = Path(__file__).resolve().parents[1] / "project_manager.py"
SPEC = importlib.util.spec_from_file_location("webgate_project_manager", MODULE_PATH)
assert SPEC is not None and SPEC.loader is not None
manager = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = manager
SPEC.loader.exec_module(manager)


class ProjectManagerTests(unittest.TestCase):
    def test_windows_rustup_url_uses_architecture(self) -> None:
        self.assertEqual(
            manager.rustup_url("Windows", "AMD64"),
            "https://win.rustup.rs/x86_64",
        )
        self.assertEqual(
            manager.rustup_url("Windows", "arm64"),
            "https://win.rustup.rs/aarch64",
        )

    def test_unix_rustup_url_uses_official_shell_installer(self) -> None:
        self.assertEqual(manager.rustup_url("Linux"), "https://sh.rustup.rs")
        self.assertEqual(manager.rustup_url("Darwin"), "https://sh.rustup.rs")

    def test_apt_servo_plan_contains_confirmed_fontconfig_prerequisite(self) -> None:
        commands = manager.servo_native_install_commands("Linux", "apt-get")
        flattened = " ".join(part for command in commands for part in command)
        self.assertIn("pkg-config", flattened)
        self.assertIn("libfontconfig1-dev", flattened)

    def test_dnf_servo_plan_contains_confirmed_fontconfig_prerequisite(self) -> None:
        commands = manager.servo_native_install_commands("Linux", "dnf")
        flattened = " ".join(part for command in commands for part in command)
        self.assertIn("pkgconf-pkg-config", flattened)
        self.assertIn("fontconfig-devel", flattened)

    def test_unknown_linux_package_manager_does_not_invent_install_command(self) -> None:
        self.assertEqual(
            manager.servo_native_install_commands("Linux", "unsupported-pm"),
            [],
        )

    def test_tool_status_serialization_is_machine_readable(self) -> None:
        status = manager.ToolStatus(
            name="cargo",
            required=True,
            ok=False,
            detail="missing",
            remediation="install rustup",
        )
        encoded = json.dumps(status.to_dict())
        decoded = json.loads(encoded)
        self.assertEqual(decoded["name"], "cargo")
        self.assertTrue(decoded["required"])
        self.assertFalse(decoded["ok"])

    def test_repository_root_is_derived_from_script_location(self) -> None:
        self.assertTrue((manager.ROOT / "Cargo.toml").is_file())
        self.assertTrue((manager.ROOT / "MASTER_PLAN.md").is_file())
        self.assertTrue((manager.ROOT / "server" / "go.mod").is_file())

    def test_dry_run_verify_does_not_require_tools_to_execute(self) -> None:
        self.assertEqual(manager.verify(dry_run=True), 0)

    def test_dry_run_compile_targets(self) -> None:
        self.assertEqual(manager.compile_target("all", release=False, dry_run=True), 0)
        self.assertEqual(manager.compile_target("client", release=True, dry_run=True), 0)
        self.assertEqual(manager.compile_target("server", release=True, dry_run=True), 0)
        self.assertEqual(manager.compile_target("android", release=False, dry_run=True), 0)
        self.assertEqual(manager.compile_target("workspace", release=False, dry_run=True), 0)

    def test_unknown_target_raises_error(self) -> None:
        with self.assertRaises(ValueError):
            manager.compile_target("non_existent_target", dry_run=True)

    def test_dry_run_test_suites(self) -> None:
        self.assertEqual(manager.test(dry_run=True), 0)

    def test_dry_run_package_distribution(self) -> None:
        self.assertEqual(
            manager.package_distribution(
                version="1.0.0",
                channel="stable",
                signing_secret="test-secret",
                dry_run=True,
            ),
            0,
        )

    def test_doctor_collects_go_and_server_status(self) -> None:
        statuses = manager.collect_doctor_status()
        names = {s.name for s in statuses}
        self.assertIn("go", names)
        self.assertIn("server/go.mod", names)
        self.assertIn("Cargo.lock", names)
        self.assertIn("repository-root", names)


if __name__ == "__main__":
    unittest.main()
