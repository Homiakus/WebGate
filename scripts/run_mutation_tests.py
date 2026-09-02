#!/usr/bin/env python3
"""Automated Mutation Testing Engine for WebGate Security & Durability Gates.

Systematically introduces high-leverage security, authority, and state mutants,
executes the targeted verification gates, confirms all mutants are killed (fail-closed),
restores active source files unconditionally, and outputs a structured failure classification report.
"""

from __future__ import annotations

import dataclasses
import os
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


@dataclasses.dataclass(frozen=True)
class MutationDefinition:
    name: str
    description: str
    file_path: Path
    target_text: str
    replacement_text: str
    test_command: list[str]
    cwd: Path
    timeout_sec: int = 45


@dataclasses.dataclass
class MutationResult:
    name: str
    description: str
    status: str  # "KILLED", "SURVIVED", "TIMEOUT", "SETUP_ERROR"
    exit_code: int
    duration_sec: float
    detail: str


MUTATION_SUITE: list[MutationDefinition] = [
    MutationDefinition(
        name="M01_PERSIST_BEFORE_MEMORY",
        description="Mutate service registry to update in-memory map before durable persistence commits",
        file_path=ROOT / "server" / "pkg" / "registry" / "service_registry.go",
        target_text="""\tif err := r.saveServiceLocked(candidate); err != nil {
\t\treturn err
\t}

\tr.byID[candidate.ID] = candidate""",
        replacement_text="""\tr.byID[candidate.ID] = candidate
\tif err := r.saveServiceLocked(candidate); err != nil {
\t\treturn err
\t}""",
        test_command=["go", "test", "./pkg/registry", "-run", "TestServicePersistenceFailureNeverCommitsMemory"],
        cwd=ROOT / "server",
    ),
    MutationDefinition(
        name="M02_ORIGIN_REJECT_NONLOOPBACK",
        description="Bypass loopback check in Origin reverse agent to allow non-loopback forwarding",
        file_path=ROOT / "server" / "pkg" / "origin" / "agent.go",
        target_text='return fmt.Errorf("%w: %s", ErrNonLoopbackTarget, addr)',
        replacement_text="return nil",
        test_command=["go", "test", "./pkg/origin", "-run", "TestOriginAgentRejectsNonLoopbackTarget"],
        cwd=ROOT / "server",
    ),
    MutationDefinition(
        name="M03_RELAY_REQUIRE_AUTH",
        description="Bypass Relay origin authentication to allow unauthenticated reverse connections",
        file_path=ROOT / "server" / "pkg" / "relay" / "relay.go",
        target_text="tokenMatch := subtle.ConstantTimeCompare([]byte(authReq.Token), []byte(r.cfg.ClusterToken)) == 1",
        replacement_text="tokenMatch := true",
        test_command=["go", "test", "./pkg/relay", "-run", "TestRelayRejectsUnauthenticatedOrigin"],
        cwd=ROOT / "server",
    ),
    MutationDefinition(
        name="M04_CLIENT_CONFIG_FAIL_CLOSED",
        description="Bypass fail-closed validation in runtime client config binding to accept corrupt profile",
        file_path=ROOT / "crates" / "webgate-app" / "src" / "main.rs",
        target_text="let new_profile = ClientConfigProfile::from_toml_str(&content)?;",
        replacement_text="let new_profile = ClientConfigProfile::from_toml_str(&content).unwrap_or_default();",
        test_command=["cargo", "test", "-p", "webgate-app", "--bin", "webgate-app", "transactional_bind_config_fails_closed_on_invalid_syntax"],
        cwd=ROOT,
    ),
    MutationDefinition(
        name="M05_KEYSTORE_CORRUPT_HEADER",
        description="Bypass header verification in PersistentFileDeviceKeyStore to accept corrupted key file",
        file_path=ROOT / "crates" / "webgate-platform" / "src" / "keystore.rs",
        target_text='if lines.is_empty() || lines[0] != "webgate-device-key-v1" {',
        replacement_text='if lines.is_empty() || lines[0] != "webgate-device-key-v1" { return Ok(()); } if false {',
        test_command=["cargo", "test", "-p", "webgate-platform", "--test", "keystore_crypto", "persistent_keystore_corrupted_storage_fails_closed"],
        cwd=ROOT,
    ),
    MutationDefinition(
        name="M06_BROWSER_PROXY_FAIL_CLOSED",
        description="Bypass loopback proxy enforcement in BrowserCapsule",
        file_path=ROOT / "crates" / "webgate-browser" / "src" / "capsule.rs",
        target_text="if !proxy_endpoint.ip().is_loopback() {",
        replacement_text="if false && !proxy_endpoint.ip().is_loopback() {",
        test_command=["cargo", "test", "-p", "webgate-browser", "--test", "proxy_enforcement", "browser_capsule_rejects_non_loopback_proxy_and_never_falls_back"],
        cwd=ROOT,
    ),
    MutationDefinition(
        name="M07_DUAL_RELAY_FAILOVER_LIVE",
        description="Disable failover to fallback relay on primary upstream failure",
        file_path=ROOT / "crates" / "webgate-transport" / "src" / "dual_failover.rs",
        target_text="if should_failover {",
        replacement_text="if false && should_failover {",
        test_command=["cargo", "test", "-p", "webgate-transport", "--test", "dual_failover", "dual_relay_live_failover_during_primary_crash"],
        cwd=ROOT,
    ),
    MutationDefinition(
        name="M08_PROXY_LOOPBACK_UPSTREAM_RESTRICTION",
        description="Disable loopback upstream check in RestrictedSocks5Transport",
        file_path=ROOT / "crates" / "webgate-transport" / "src" / "restricted_socks5.rs",
        target_text="return Err(RestrictedProxyError::UpstreamNotLoopback);",
        replacement_text="return Ok(SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), self.config.upstream_port));",
        test_command=["cargo", "test", "-p", "webgate-transport", "--test", "restricted_socks5", "plaintext_sidecar_upstream_must_be_loopback"],
        cwd=ROOT,
    ),
]


def run_mutation(mutant: MutationDefinition) -> MutationResult:
    if not mutant.file_path.exists():
        return MutationResult(
            name=mutant.name,
            description=mutant.description,
            status="SETUP_ERROR",
            exit_code=-1,
            duration_sec=0.0,
            detail=f"File not found: {mutant.file_path}",
        )

    original_bytes = mutant.file_path.read_bytes()
    original_text = original_bytes.decode("utf-8")

    if mutant.target_text not in original_text:
        return MutationResult(
            name=mutant.name,
            description=mutant.description,
            status="SETUP_ERROR",
            exit_code=-1,
            duration_sec=0.0,
            detail=f"Target text not found in {mutant.file_path.name}",
        )

    mutated_text = original_text.replace(mutant.target_text, mutant.replacement_text, 1)

    start_time = time.monotonic()
    try:
        # Write mutated content
        mutant.file_path.write_bytes(mutated_text.encode("utf-8"))

        env = dict(os.environ)
        if "CGO_ENABLED" not in env:
            env["CGO_ENABLED"] = "0"

        # Execute test command
        proc = subprocess.run(
            mutant.test_command,
            cwd=str(mutant.cwd),
            env=env,
            capture_output=True,
            text=True,
            timeout=mutant.timeout_sec,
        )
        duration = time.monotonic() - start_time

        # A mutant is KILLED when the test suite FAILS (proc.returncode != 0)
        if proc.returncode != 0:
            return MutationResult(
                name=mutant.name,
                description=mutant.description,
                status="KILLED",
                exit_code=proc.returncode,
                duration_sec=duration,
                detail=f"Killed as expected by {mutant.test_command[0]} (exit {proc.returncode})",
            )
        else:
            return MutationResult(
                name=mutant.name,
                description=mutant.description,
                status="SURVIVED",
                exit_code=proc.returncode,
                duration_sec=duration,
                detail="SURVIVED: Test unexpectedly passed on mutated code!",
            )
    except subprocess.TimeoutExpired:
        duration = time.monotonic() - start_time
        return MutationResult(
            name=mutant.name,
            description=mutant.description,
            status="KILLED",  # Timeouts on broken loops also qualify as failure/kill
            exit_code=-2,
            duration_sec=duration,
            detail=f"Test timed out after {mutant.timeout_sec}s",
        )
    except Exception as e:
        duration = time.monotonic() - start_time
        return MutationResult(
            name=mutant.name,
            description=mutant.description,
            status="SETUP_ERROR",
            exit_code=-3,
            duration_sec=duration,
            detail=f"Execution error: {e}",
        )
    finally:
        # ALWAYS restore original file bytes
        mutant.file_path.write_bytes(original_bytes)


def main() -> int:
    print("=" * 80)
    print("WebGate Security & Invariant Mutation Test Engine")
    print("=" * 80)
    print(f"Total Mutants to Evaluate: {len(MUTATION_SUITE)}\n")

    results: list[MutationResult] = []
    survived_count = 0
    setup_error_count = 0

    for i, mutant in enumerate(MUTATION_SUITE, 1):
        print(f"[{i:02d}/{len(MUTATION_SUITE):02d}] Testing {mutant.name}...")
        res = run_mutation(mutant)
        results.append(res)
        if res.status == "KILLED":
            status_tag = "\033[92m[KILLED]\033[0m"
        elif res.status == "SURVIVED":
            status_tag = "\033[91m\033[1m[SURVIVED - FAILED]\033[0m"
            survived_count += 1
        else:
            status_tag = f"\033[93m[{res.status}]\033[0m"
            setup_error_count += 1
        print(f"       Status: {status_tag} in {res.duration_sec:.2f}s — {res.detail}")

    print("\n" + "=" * 80)
    print("MUTATION TEST SUMMARY REPORT")
    print("=" * 80)
    print(f"{'MUTANT':<40} | {'STATUS':<10} | {'TIME':<6} | {'DETAIL'}")
    print("-" * 80)
    for res in results:
        print(f"{res.name:<40} | {res.status:<10} | {res.duration_sec:4.2f}s | {res.detail}")
    print("-" * 80)

    killed_count = sum(1 for r in results if r.status == "KILLED")
    print(f"Total: {len(results)} | Killed: {killed_count} | Survived: {survived_count} | Errors: {setup_error_count}")

    if survived_count > 0 or setup_error_count > 0:
        print("\n\033[91m[FAIL] Mutation gate failed: Not all mutants were killed fail-closed.\033[0m")
        return 1

    print("\n\033[92m[PASS] All security and integrity mutants were successfully KILLED.\033[0m")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
