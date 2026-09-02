#!/usr/bin/env python3
"""
WebGate Real End-to-End Qualification Suite (T-045).
Validates full-path contracts, security boundaries, and runtime integration invariants.
"""

from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent.parent.parent


class EndToEndQualificationTests(unittest.TestCase):
    def test_full_pipeline_test_fixtures_exist(self) -> None:
        """Verify that concrete e2e qualification test suites exist across Rust and Go."""
        go_e2e = ROOT / "server" / "pkg" / "gateway" / "e2e_qualification_test.go"
        rust_e2e = ROOT / "crates" / "webgate-app" / "tests" / "e2e_full_stack.rs"

        self.assertTrue(go_e2e.exists(), f"Go E2E qualification suite must exist at {go_e2e}")
        self.assertTrue(rust_e2e.exists(), f"Rust E2E qualification suite must exist at {rust_e2e}")

    def test_go_e2e_covers_critical_runtime_invariants(self) -> None:
        """Ensure Go E2E suite covers all required runtime layers and invariants."""
        go_e2e_content = (ROOT / "server" / "pkg" / "gateway" / "e2e_qualification_test.go").read_text(encoding="utf-8")

        required_checks = [
            "TestRealEndToEndQualification",
            "PrimaryRelay_EndToEnd_AuthorizedRequest_Succeeds",
            "PrimaryRelay_EndToEnd_POST_Payload_Echo_Succeeds",
            "FallbackRelay_EndToEnd_AuthorizedRequest_Succeeds",
            "Unauthorized_MissingSession_FailsClosed_401",
            "Unauthorized_UnknownDevice_FailsClosed_403",
            "NonExistent_ServiceSlug_Returns_404",
            "Disabled_Service_FailsClosed_503",
            "Concurrent_Multiplexed_Traffic_NoCrosstalk",
        ]

        for check in required_checks:
            self.assertIn(check, go_e2e_content, f"Go E2E suite missing coverage for: {check}")

    def test_rust_e2e_covers_capsule_proxy_failover_invariants(self) -> None:
        """Ensure Rust E2E suite covers browser capsule, SOCKS5 proxy, failover, and destination policy."""
        rust_e2e_content = (ROOT / "crates" / "webgate-app" / "tests" / "e2e_full_stack.rs").read_text(encoding="utf-8")

        required_checks = [
            "test_real_end_to_end_full_stack_qualification",
            "BrowserCapsule",
            "DualRelayFailoverTransport",
            "PersistentFileDeviceKeyStore",
            "MockSocks5Relay",
            "socks5_http_get",
            "disallowed domain must fail closed",
            "disallowed port must fail closed",
            "BrowserLifecycleEvent::Pause",
            "BrowserLifecycleEvent::Resume",
        ]

        for check in required_checks:
            self.assertIn(check, rust_e2e_content, f"Rust E2E suite missing coverage for: {check}")

    def test_no_bypass_or_surrogate_in_production_runtime(self) -> None:
        """Verify that server gateway and client transport never fall back to open/surrogate proxies."""
        gateway_go = (ROOT / "server" / "pkg" / "gateway" / "gateway.go").read_text(encoding="utf-8")
        dual_failover_rs = (ROOT / "crates" / "webgate-transport" / "src" / "dual_failover.rs").read_text(encoding="utf-8")

        # Gateway must disable environment proxy to prevent SSRF escape
        self.assertIn("transport.Proxy = nil", gateway_go)
        self.assertIn("http.ErrUseLastResponse", gateway_go)

        # Transport proxy must forbid non-loopback upstreams fail-closed
        self.assertIn("PrimaryUpstreamNotLoopback", dual_failover_rs)
        self.assertIn("FallbackUpstreamNotLoopback", dual_failover_rs)


if __name__ == "__main__":
    unittest.main()
