#!/usr/bin/env python3
"""WebGate Progressive Project & Compilation Manager.

Comprehensive developer toolkit, compilation pipeline, runner orchestrator,
diagnostics, CI-parity verification, and distribution packaging.
Uses only the Python standard library for zero external runtime dependencies.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import os
import platform
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Iterable, Sequence

# Ensure safe output encoding on all platforms
if hasattr(sys.stdout, "reconfigure"):
    try:
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass
if hasattr(sys.stderr, "reconfigure"):
    try:
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = Path(__file__).resolve().parents[1]
MIN_PYTHON = (3, 11)
MIN_GO = (1, 23)
RUSTUP_SH_URL = "https://sh.rustup.rs"
RUSTUP_WIN_BASE_URL = "https://win.rustup.rs"

# ANSI Color formatting
USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None


class Color:
    RESET = "\033[0m" if USE_COLOR else ""
    BOLD = "\033[1m" if USE_COLOR else ""
    DIM = "\033[2m" if USE_COLOR else ""
    RED = "\033[31m" if USE_COLOR else ""
    GREEN = "\033[32m" if USE_COLOR else ""
    YELLOW = "\033[33m" if USE_COLOR else ""
    BLUE = "\033[34m" if USE_COLOR else ""
    MAGENTA = "\033[35m" if USE_COLOR else ""
    CYAN = "\033[36m" if USE_COLOR else ""
    WHITE = "\033[37m" if USE_COLOR else ""


@dataclass(frozen=True)
class ToolStatus:
    name: str
    required: bool
    ok: bool
    detail: str
    remediation: str = ""

    def to_dict(self) -> dict[str, object]:
        return asdict(self)


def is_windows() -> bool:
    return platform.system().lower() == "windows"


def is_macos() -> bool:
    return platform.system().lower() == "darwin"


def is_linux() -> bool:
    return platform.system().lower() == "linux"


def executable(name: str) -> str | None:
    return shutil.which(name)


def executable_ext() -> str:
    return ".exe" if is_windows() else ""


def command_version(command: Sequence[str]) -> tuple[bool, str]:
    try:
        proc = subprocess.run(
            list(command),
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=20,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        return False, str(exc)
    output = (proc.stdout or "").strip().splitlines()
    detail = output[0] if output else f"exit={proc.returncode}"
    return proc.returncode == 0, detail


def run(
    command: Sequence[str],
    *,
    check: bool = True,
    cwd: Path = ROOT,
    env: dict[str, str] | None = None,
    dry_run: bool = False,
    label: str | None = None,
) -> int:
    printable = subprocess.list2cmdline(list(command)) if is_windows() else " ".join(command)
    display_label = f" [{label}]" if label else ""
    print(f"\n{Color.CYAN}${display_label} {Color.BOLD}{printable}{Color.RESET}")
    if dry_run:
        print(f"{Color.YELLOW}[DRY RUN]{Color.RESET} command skipped")
        return 0

    start_time = time.perf_counter()
    proc = subprocess.run(list(command), cwd=cwd, env=env, check=False)
    elapsed = time.perf_counter() - start_time

    if proc.returncode == 0:
        print(f"{Color.GREEN}[OK] Completed in {elapsed:.2f}s{Color.RESET}")
    else:
        print(f"{Color.RED}[FAIL] Exit code {proc.returncode} ({elapsed:.2f}s){Color.RESET}")

    if check and proc.returncode != 0:
        raise RuntimeError(f"command failed with exit code {proc.returncode}: {printable}")
    return proc.returncode


def confirm(prompt: str, assume_yes: bool) -> bool:
    if assume_yes:
        return True
    answer = input(f"{prompt} [y/N]: ").strip().lower()
    return answer in {"y", "yes", "д", "да"}


def cargo_home_bin() -> Path:
    cargo_home = Path(os.environ.get("CARGO_HOME", Path.home() / ".cargo"))
    return cargo_home / "bin"


def refresh_cargo_path() -> None:
    cargo_bin = str(cargo_home_bin())
    current = os.environ.get("PATH", "")
    if cargo_bin not in current.split(os.pathsep):
        os.environ["PATH"] = cargo_bin + os.pathsep + current


def rustup_windows_arch(machine: str | None = None) -> str:
    value = (machine or platform.machine()).lower()
    if value in {"arm64", "aarch64"}:
        return "aarch64"
    return "x86_64"


def rustup_url(system_name: str | None = None, machine: str | None = None) -> str:
    system_value = (system_name or platform.system()).lower()
    if system_value == "windows":
        return f"{RUSTUP_WIN_BASE_URL}/{rustup_windows_arch(machine)}"
    return RUSTUP_SH_URL


def privileged(command: Sequence[str]) -> list[str]:
    if not is_linux() or getattr(os, "geteuid", lambda: 1)() == 0:
        return list(command)
    if executable("sudo"):
        return ["sudo", *command]
    return list(command)


def detect_linux_package_manager() -> str | None:
    for candidate in ("apt-get", "dnf", "pacman", "zypper"):
        if executable(candidate):
            return candidate
    return None


def servo_native_install_commands(
    system_name: str | None = None,
    manager: str | None = None,
) -> list[list[str]]:
    system_value = (system_name or platform.system()).lower()
    if system_value == "linux":
        package_manager = manager or detect_linux_package_manager()
        if package_manager == "apt-get":
            return [
                privileged(["apt-get", "update"]),
                privileged(["apt-get", "install", "-y", "pkg-config", "libfontconfig1-dev"]),
            ]
        if package_manager == "dnf":
            return [privileged(["dnf", "install", "-y", "pkgconf-pkg-config", "fontconfig-devel"])]
        if package_manager == "pacman":
            return [privileged(["pacman", "-S", "--needed", "--noconfirm", "pkgconf", "fontconfig"])]
        if package_manager == "zypper":
            return [privileged(["zypper", "--non-interactive", "install", "pkg-config", "fontconfig-devel"])]
        return []
    if system_value == "darwin" and executable("brew"):
        return [["brew", "install", "pkg-config", "fontconfig"]]
    return []


def git_install_commands() -> list[list[str]]:
    if is_windows() and executable("winget"):
        return [["winget", "install", "--id", "Git.Git", "-e", "--source", "winget"]]
    if is_macos() and executable("brew"):
        return [["brew", "install", "git"]]
    if is_linux():
        manager = detect_linux_package_manager()
        if manager == "apt-get":
            return [privileged(["apt-get", "update"]), privileged(["apt-get", "install", "-y", "git"])]
        if manager == "dnf":
            return [privileged(["dnf", "install", "-y", "git"])]
        if manager == "pacman":
            return [privileged(["pacman", "-S", "--needed", "--noconfirm", "git"])]
        if manager == "zypper":
            return [privileged(["zypper", "--non-interactive", "install", "git"])]
    return []


def install_rustup(*, assume_yes: bool, dry_run: bool) -> None:
    source = rustup_url()
    print(f"Rust bootstrap source: {source}")
    if not confirm("Download and execute the official rustup installer?", assume_yes):
        raise RuntimeError("rustup installation cancelled")
    if dry_run:
        print("DRY RUN: rustup installer would be downloaded and executed")
        return

    with tempfile.TemporaryDirectory(prefix="webgate-rustup-") as tmp:
        tmp_path = Path(tmp)
        if is_windows():
            installer = tmp_path / "rustup-init.exe"
            urllib.request.urlretrieve(source, installer)
            run([str(installer), "-y", "--profile", "minimal"])
        else:
            installer = tmp_path / "rustup-init.sh"
            urllib.request.urlretrieve(source, installer)
            run(["sh", str(installer), "-y", "--profile", "minimal"])
    refresh_cargo_path()


def check_fontconfig() -> ToolStatus:
    if is_linux():
        if not executable("pkg-config"):
            return ToolStatus(
                "servo-native/fontconfig",
                True,
                False,
                "pkg-config is missing",
                "install pkg-config and the fontconfig development package",
            )
        ok, _ = command_version(["pkg-config", "--exists", "fontconfig"])
        if not ok:
            return ToolStatus(
                "servo-native/fontconfig",
                True,
                False,
                "fontconfig.pc is not visible to pkg-config",
                "install libfontconfig1-dev/fontconfig-devel",
            )
        version_ok, version = command_version(["pkg-config", "--modversion", "fontconfig"])
        return ToolStatus(
            "servo-native/fontconfig",
            True,
            version_ok,
            version if version_ok else "fontconfig exists but version query failed",
            "repair pkg-config/fontconfig development files",
        )
    if is_macos():
        if executable("pkg-config"):
            ok, version = command_version(["pkg-config", "--modversion", "fontconfig"])
            return ToolStatus(
                "servo-native/fontconfig",
                False,
                ok,
                version if ok else "not detected; only required if the selected Servo graph requests it",
                "brew install pkg-config fontconfig",
            )
        return ToolStatus(
            "servo-native/fontconfig",
            False,
            False,
            "pkg-config not installed; not a confirmed blocker on macOS baseline",
            "brew install pkg-config fontconfig if Servo build requests it",
        )
    return ToolStatus(
        "servo-native/fontconfig",
        False,
        True,
        "no confirmed fontconfig prerequisite for this platform baseline",
    )


def collect_doctor_status() -> list[ToolStatus]:
    statuses: list[ToolStatus] = []
    py_ok = sys.version_info >= MIN_PYTHON
    statuses.append(
        ToolStatus(
            "python",
            True,
            py_ok,
            platform.python_version(),
            f"install Python {MIN_PYTHON[0]}.{MIN_PYTHON[1]}+",
        )
    )

    # Go toolchain check
    if not executable("go"):
        statuses.append(
            ToolStatus(
                "go",
                True,
                False,
                "not found in PATH",
                f"install Go {MIN_GO[0]}.{MIN_GO[1]}+ from https://go.dev/dl/",
            )
        )
    else:
        ok, detail = command_version(["go", "version"])
        statuses.append(
            ToolStatus(
                "go",
                True,
                ok,
                detail,
                "ensure Go toolchain is functioning correctly",
            )
        )

    for name, command, remediation in (
        ("git", ["git", "--version"], "install Git"),
        ("rustup", ["rustup", "--version"], "install rustup from rustup.rs"),
        ("rustc", ["rustc", "--version"], "install/sync the Rust toolchain"),
        ("cargo", ["cargo", "--version"], "install/sync the Rust toolchain"),
        ("rustfmt", ["cargo", "fmt", "--version"], "rustup component add rustfmt"),
        ("clippy", ["cargo", "clippy", "--version"], "rustup component add clippy"),
        ("cargo-deny", ["cargo-deny", "--version"], "cargo install cargo-deny --locked"),
    ):
        if not executable(command[0]):
            statuses.append(ToolStatus(name, True, False, "not found in PATH", remediation))
            continue
        ok, detail = command_version(command)
        statuses.append(ToolStatus(name, True, ok, detail, remediation if not ok else ""))

    statuses.append(check_fontconfig())
    statuses.append(
        ToolStatus(
            "Cargo.lock",
            True,
            (ROOT / "Cargo.lock").is_file(),
            "present" if (ROOT / "Cargo.lock").is_file() else "missing",
            "generate and commit Cargo.lock deliberately",
        )
    )
    statuses.append(
        ToolStatus(
            "server/go.mod",
            True,
            (ROOT / "server" / "go.mod").is_file(),
            "present" if (ROOT / "server" / "go.mod").is_file() else "missing",
            "ensure Go server module is initialized",
        )
    )
    statuses.append(
        ToolStatus(
            "repository-root",
            True,
            (ROOT / "Cargo.toml").is_file() and (ROOT / "MASTER_PLAN.md").is_file(),
            str(ROOT),
            "run the manager from a complete WebGate checkout",
        )
    )
    return statuses


def print_statuses(statuses: Iterable[ToolStatus]) -> None:
    print(f"\n{Color.BOLD}WebGate Environment & Toolchain Status{Color.RESET}")
    print("-" * 80)
    for item in statuses:
        if item.ok:
            marker = f"{Color.GREEN}[ OK ]{Color.RESET}"
        elif item.required:
            marker = f"{Color.RED}[MISS]{Color.RESET}"
        else:
            marker = f"{Color.YELLOW}[OPT ]{Color.RESET}"

        req = f"{Color.DIM}required{Color.RESET}" if item.required else f"{Color.DIM}optional{Color.RESET}"
        print(f"{marker} {Color.BOLD}{item.name:26}{Color.RESET} {req:18} {item.detail}")
        if not item.ok and item.remediation:
            print(f"       {Color.YELLOW}-> {item.remediation}{Color.RESET}")


def doctor(*, json_output: bool = False) -> int:
    statuses = collect_doctor_status()
    if json_output:
        print(json.dumps([item.to_dict() for item in statuses], ensure_ascii=False, indent=2))
    else:
        print_statuses(statuses)
    return 0 if all(item.ok for item in statuses if item.required) else 1


def install_missing(*, assume_yes: bool, dry_run: bool, with_mutation: bool) -> int:
    if not executable("git"):
        commands = git_install_commands()
        if not commands:
            raise RuntimeError("Git is missing and no supported package manager was detected")
        if confirm("Install Git using the detected system package manager?", assume_yes):
            for command in commands:
                run(command, dry_run=dry_run)

    if not executable("rustup") or not executable("cargo"):
        install_rustup(assume_yes=assume_yes, dry_run=dry_run)
    refresh_cargo_path()

    if executable("rustup"):
        run(["rustup", "component", "add", "rustfmt", "clippy"], dry_run=dry_run)

    if not executable("cargo-deny"):
        if confirm("Install cargo-deny with Cargo?", assume_yes):
            run(["cargo", "install", "cargo-deny", "--locked"], dry_run=dry_run)

    native = check_fontconfig()
    if native.required and not native.ok:
        commands = servo_native_install_commands()
        if not commands:
            raise RuntimeError(
                "Servo native prerequisites are missing and no supported package manager was detected"
            )
        if confirm("Install confirmed Servo native prerequisites?", assume_yes):
            for command in commands:
                run(command, dry_run=dry_run)

    if with_mutation and not executable("cargo-mutants"):
        if confirm("Install optional cargo-mutants test-of-tests tool?", assume_yes):
            run(["cargo", "install", "cargo-mutants", "--locked"], dry_run=dry_run)

    if dry_run:
        return 0
    print_statuses(collect_doctor_status())
    return doctor(json_output=False)


# ==============================================================================
# Progressive Compilation & Build Engine
# ==============================================================================


def get_git_commit() -> str:
    try:
        proc = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
        )
        if proc.returncode == 0:
            return proc.stdout.strip()
    except Exception:
        pass
    return "dev"


def build_client(
    *,
    release: bool = False,
    target_triple: str | None = None,
    out_dir: Path | None = None,
    dry_run: bool = False,
) -> int:
    """Compiles the Rust WebGate client desktop application."""
    print(f"\n{Color.BOLD}[BUILD] Building WebGate Client (Rust Desktop)...{Color.RESET}")
    cmd = ["cargo", "build", "--package", "webgate-app", "--locked"]
    if release:
        cmd.append("--release")
    if target_triple:
        cmd.extend(["--target", target_triple])

    code = run(cmd, dry_run=dry_run, label="Rust Client Build")
    if code != 0 or dry_run:
        return code

    # Copy output binary if out_dir specified
    mode = "release" if release else "debug"
    src_bin_name = f"webgate-app{executable_ext()}"
    if target_triple:
        src_bin = ROOT / "target" / target_triple / mode / src_bin_name
    else:
        src_bin = ROOT / "target" / mode / src_bin_name

    target_out = out_dir or (ROOT / "bin")
    target_out.mkdir(parents=True, exist_ok=True)
    dst_bin = target_out / src_bin_name

    if src_bin.exists():
        shutil.copy2(src_bin, dst_bin)
        print(f"{Color.GREEN}[STAGED] Client binary staged at: {dst_bin}{Color.RESET}")
    return 0


def build_server(
    *,
    release: bool = False,
    out_dir: Path | None = None,
    dry_run: bool = False,
) -> int:
    """Compiles the Go WebGate Server Gateway & Control Plane."""
    print(f"\n{Color.BOLD}[BUILD] Building WebGate Server (Go Gateway)...{Color.RESET}")
    target_out = out_dir or (ROOT / "bin")
    target_out.mkdir(parents=True, exist_ok=True)
    dst_bin = target_out / f"webgate-server{executable_ext()}"

    commit = get_git_commit()
    ldflags = f"-s -w -X main.Version=1.0.0 -X main.GitCommit={commit}" if release else f"-X main.GitCommit={commit}"

    cmd = [
        "go",
        "build",
        "-trimpath" if release else "-v",
        "-ldflags",
        ldflags,
        "-o",
        str(dst_bin),
        "./cmd/webgate-server",
    ]

    code = run(cmd, cwd=ROOT / "server", dry_run=dry_run, label="Go Server Build")
    if code == 0 and not dry_run and dst_bin.exists():
        print(f"{Color.GREEN}[BUILT] Server binary built at: {dst_bin}{Color.RESET}")
    return code


def build_relay(
    *,
    release: bool = False,
    out_dir: Path | None = None,
    dry_run: bool = False,
) -> int:
    """Compiles the Go WebGate Transit Relay node."""
    print(f"\n{Color.BOLD}[BUILD] Building WebGate Relay (Go Relay Node)...{Color.RESET}")
    target_out = out_dir or (ROOT / "bin")
    target_out.mkdir(parents=True, exist_ok=True)
    dst_bin = target_out / f"webgate-relay{executable_ext()}"

    commit = get_git_commit()
    ldflags = f"-s -w -X main.Version=1.0.0 -X main.GitCommit={commit}" if release else f"-X main.GitCommit={commit}"

    cmd = [
        "go",
        "build",
        "-trimpath" if release else "-v",
        "-ldflags",
        ldflags,
        "-o",
        str(dst_bin),
        "./cmd/webgate-relay",
    ]

    code = run(cmd, cwd=ROOT / "server", dry_run=dry_run, label="Go Relay Build")
    if code == 0 and not dry_run and dst_bin.exists():
        print(f"{Color.GREEN}[BUILT] Relay binary built at: {dst_bin}{Color.RESET}")
    return code


def build_android(
    *,
    arch: str = "aarch64",
    release: bool = False,
    dry_run: bool = False,
) -> int:
    """Builds WebGate Android platform library."""
    print(f"\n{Color.BOLD}[BUILD] Building WebGate Android ({arch})...{Color.RESET}")
    triple_map = {
        "aarch64": "aarch64-linux-android",
        "arm64": "aarch64-linux-android",
        "x86_64": "x86_64-linux-android",
        "x86": "i686-linux-android",
    }
    triple = triple_map.get(arch.lower(), arch)

    if executable("cargo-ndk"):
        cmd = ["cargo", "ndk", "-t", arch, "build", "--package", "webgate-platform"]
        if release:
            cmd.append("--release")
        return run(cmd, dry_run=dry_run, label=f"Android NDK ({arch})")

    # Fallback to direct target compile
    cmd = ["cargo", "build", "--package", "webgate-platform", "--target", triple]
    if release:
        cmd.append("--release")
    return run(cmd, dry_run=dry_run, label=f"Android Rust ({triple})")


def compile_target(
    target: str = "all",
    *,
    release: bool = False,
    out_dir: Path | None = None,
    dry_run: bool = False,
) -> int:
    """Unified compilation orchestrator for all programs in WebGate."""
    target_clean = target.strip().lower()
    print(f"\n{Color.BOLD}>>> WebGate Compilation Pipeline [Target: {target_clean}, Release: {release}]{Color.RESET}")

    if target_clean in {"client", "desktop", "app", "webgate-app"}:
        return build_client(release=release, out_dir=out_dir, dry_run=dry_run)

    if target_clean in {"server", "gw", "webgate-server"}:
        return build_server(release=release, out_dir=out_dir, dry_run=dry_run)

    if target_clean in {"relay", "webgate-relay"}:
        return build_relay(release=release, out_dir=out_dir, dry_run=dry_run)

    if target_clean in {"android"}:
        return build_android(release=release, dry_run=dry_run)

    if target_clean in {"workspace", "rust"}:
        cmd = ["cargo", "build", "--workspace", "--locked"]
        if release:
            cmd.append("--release")
        return run(cmd, dry_run=dry_run, label="Rust Workspace")

    if target_clean == "all":
        # 1. Build Client
        code_client = build_client(release=release, out_dir=out_dir, dry_run=dry_run)
        if code_client != 0:
            return code_client

        # 2. Build Server
        code_server = build_server(release=release, out_dir=out_dir, dry_run=dry_run)
        if code_server != 0:
            return code_server

        # 3. Build Relay
        code_relay = build_relay(release=release, out_dir=out_dir, dry_run=dry_run)
        if code_relay != 0:
            return code_relay

        print(f"\n{Color.GREEN}{Color.BOLD}[SUCCESS] ALL programs compiled into {out_dir or (ROOT / 'bin')}!{Color.RESET}")
        return 0

    raise ValueError(f"Unknown compilation target '{target}'. Valid targets: all, client, server, relay, android, workspace")


# ==============================================================================
# Program Runner & Development Orchestrator
# ==============================================================================


def run_server(*, args: Sequence[str] = (), dry_run: bool = False) -> int:
    """Executes the WebGate Server Gateway."""
    bin_path = ROOT / "bin" / f"webgate-server{executable_ext()}"
    if not bin_path.exists() and not dry_run:
        print(f"{Color.YELLOW}Server binary not found in bin/. Compiling now...{Color.RESET}")
        build_server()

    cmd = [str(bin_path), *args] if bin_path.exists() else ["go", "run", "./cmd/webgate-server", *args]
    cwd = ROOT if bin_path.exists() else ROOT / "server"
    return run(cmd, cwd=cwd, dry_run=dry_run, label="Server Gateway")


def run_client(*, args: Sequence[str] = (), dry_run: bool = False) -> int:
    """Executes the WebGate Client Application."""
    bin_path = ROOT / "bin" / f"webgate-app{executable_ext()}"
    if not bin_path.exists() and not dry_run:
        print(f"{Color.YELLOW}Client binary not found in bin/. Compiling now...{Color.RESET}")
        build_client()

    cmd = [str(bin_path), *args] if bin_path.exists() else ["cargo", "run", "--package", "webgate-app", "--locked", "--", *args]
    return run(cmd, dry_run=dry_run, label="WebGate Client")


def run_dev_concurrent() -> int:
    """Runs Server and Client concurrently for live end-to-end testing."""
    print(f"\n{Color.BOLD}{Color.MAGENTA}=== Launching WebGate Dev Environment (Server + Client) ==={Color.RESET}")
    print(f"{Color.DIM}Press Ctrl+C to terminate both services gracefully.{Color.RESET}\n")

    # Ensure binaries are compiled
    compile_target("all", release=False)

    server_bin = ROOT / "bin" / f"webgate-server{executable_ext()}"
    client_bin = ROOT / "bin" / f"webgate-app{executable_ext()}"

    procs: list[subprocess.Popen] = []

    def stream_output(proc: subprocess.Popen, prefix: str, color: str) -> None:
        if proc.stdout is None:
            return
        for line in iter(proc.stdout.readline, ""):
            if line:
                print(f"{color}[{prefix}]{Color.RESET} {line.rstrip()}")

    try:
        # Start Server
        p_server = subprocess.Popen(
            [str(server_bin)],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding="utf-8",
            errors="replace",
        )
        procs.append(p_server)
        time.sleep(1.0)  # Brief warm-up

        # Start Client
        p_client = subprocess.Popen(
            [str(client_bin)],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding="utf-8",
            errors="replace",
        )
        procs.append(p_client)

        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
            executor.submit(stream_output, p_server, "SERVER", Color.CYAN)
            executor.submit(stream_output, p_client, "CLIENT", Color.GREEN)

        p_client.wait()
        return p_client.returncode
    except KeyboardInterrupt:
        print(f"\n{Color.YELLOW}Shutting down processes...{Color.RESET}")
    finally:
        for p in procs:
            if p.poll() is None:
                p.terminate()
                try:
                    p.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    p.kill()
    return 0


# ==============================================================================
# Verification, Testing & CI Parity
# ==============================================================================


def test_python(*, dry_run: bool = False) -> int:
    return run(
        [sys.executable, "-m", "unittest", "discover", "-s", "scripts/tests", "-p", "test_*.py", "-v"],
        dry_run=dry_run,
        label="Python Test Suite",
    )


def test_rust(*, dry_run: bool = False) -> int:
    return run(["cargo", "test", "--workspace", "--locked"], dry_run=dry_run, label="Rust Test Suite")


def test_go(*, dry_run: bool = False) -> int:
    return run(["go", "test", "./..."], cwd=ROOT / "server", dry_run=dry_run, label="Go Server Test Suite")


def test(*, dry_run: bool = False) -> int:
    """Executes full multi-stack test suites (Rust + Go + Python)."""
    print(f"\n{Color.BOLD}[TEST] Running Full WebGate Test Matrix...{Color.RESET}")
    code = test_python(dry_run=dry_run)
    if code != 0:
        return code
    code = test_rust(dry_run=dry_run)
    if code != 0:
        return code
    code = test_go(dry_run=dry_run)
    if code != 0:
        return code
    print(f"\n{Color.GREEN}{Color.BOLD}[PASS] All Test Suites PASSED!{Color.RESET}")
    return 0


def verify(*, dry_run: bool = False) -> int:
    """Full CI-parity quality gate across all stacks."""
    print(f"\n{Color.BOLD}[GATE] Starting WebGate CI-Parity Quality Gate...{Color.RESET}")
    commands: list[tuple[Sequence[str], Path, str]] = [
        ([sys.executable, "-m", "unittest", "discover", "-s", "scripts/tests", "-p", "test_*.py", "-v"], ROOT, "Python Tests"),
        ([sys.executable, "scripts/check_architecture.py"], ROOT, "Architecture Boundaries"),
        (["cargo", "metadata", "--locked", "--no-deps", "--format-version", "1"], ROOT, "Cargo Metadata"),
        (["cargo", "fmt", "--all", "--", "--check"], ROOT, "Rust Format Check"),
        (["cargo", "check", "--workspace", "--all-targets", "--locked"], ROOT, "Rust Workspace Check"),
        (["cargo", "test", "--workspace", "--locked"], ROOT, "Rust Workspace Tests"),
        (["cargo", "clippy", "--workspace", "--all-targets", "--locked", "--", "-D", "warnings"], ROOT, "Rust Clippy"),
        (["go", "vet", "./..."], ROOT / "server", "Go Vet"),
        (["go", "test", "./..."], ROOT / "server", "Go Tests"),
        (["go", "test", "-race", "./pkg/persistence", "./pkg/registry", "./pkg/origin", "./pkg/relay", "./pkg/gateway"], ROOT / "server", "Go Race Checks"),
        ([sys.executable, "scripts/run_mutation_tests.py"], ROOT, "Automated Mutation Testing Gate"),
    ]
    if executable("cargo-deny"):
        commands.append((["cargo", "deny", "check", "--all-features"], ROOT, "Cargo Security Policy"))
    else:
        print(f"\n{Color.YELLOW}[SKIP]{Color.RESET} cargo-deny binary not found locally. Skipping offline security gate.")
    commands.append((["git", "diff", "--check"], ROOT, "Git Whitespace Check"))
    for command, cwd, label in commands:
        code = run(command, cwd=cwd, dry_run=dry_run, label=label)
        if code != 0:
            return code
    print(f"\n{Color.GREEN}{Color.BOLD}[PASS] Verification: PASS (All Quality Gates Cleared){Color.RESET}")
    return 0


def mutate(*, dry_run: bool = False) -> int:
    """Executes automated mutation test suite verifying all security/durability mutants are killed."""
    return run([sys.executable, "scripts/run_mutation_tests.py"], cwd=ROOT, dry_run=dry_run, label="Mutation Invariant Suite")


def fuzz(*, duration_sec: int = 2, dry_run: bool = False) -> int:
    """Executes native Go fuzz targets on critical protocol and persistence parsers."""
    print(f"\n{Color.BOLD}[FUZZ] Executing Protocol and State Fuzzing Matrix...{Color.RESET}")
    targets = [
        (["go", "test", "-fuzz=FuzzReadFrame", f"-fuzztime={duration_sec}s", "./pkg/relay"], ROOT / "server", "Relay Frame Protocol Fuzzer"),
        (["go", "test", "-fuzz=FuzzDurableServerConfigUnmarshal", f"-fuzztime={duration_sec}s", "./pkg/persistence"], ROOT / "server", "Durable Server Config Fuzzer"),
        (["go", "test", "-fuzz=FuzzAuditEventUnmarshal", f"-fuzztime={duration_sec}s", "./pkg/persistence"], ROOT / "server", "Audit Event Unmarshal Fuzzer"),
    ]
    for cmd, cwd, label in targets:
        code = run(cmd, cwd=cwd, dry_run=dry_run, label=label)
        if code != 0:
            return code
    print(f"\n{Color.GREEN}{Color.BOLD}[PASS] Fuzzing Matrix: PASS{Color.RESET}")
    return 0


def security(*, dry_run: bool = False) -> int:
    return run(["cargo", "deny", "check", "--all-features"], dry_run=dry_run, label="Security Audit")


# ==============================================================================
# Release & Distribution Packaging
# ==============================================================================


def package_distribution(
    version: str = "1.0.0",
    channel: str = "stable",
    signing_secret: str = "webgate-secret-key-2026",
    *,
    dry_run: bool = False,
) -> int:
    """Compiles release artifacts, generates digests, and signs release manifests."""
    print(f"\n{Color.BOLD}[DIST] Packaging WebGate Release [Version {version}, Channel: {channel}]...{Color.RESET}")
    dist_dir = ROOT / "dist" / f"webgate-{version}"
    dist_dir.mkdir(parents=True, exist_ok=True)

    # 1. Compile Release Binaries
    compile_target("all", release=True, out_dir=dist_dir, dry_run=dry_run)

    if dry_run:
        print(f"{Color.YELLOW}[DRY RUN] Manifest creation and signing simulated.{Color.RESET}")
        return 0

    # 2. Run Packaging Script
    dist_script = ROOT / "scripts" / "build_distribution.py"
    client_bin = dist_dir / f"webgate-app{executable_ext()}"
    server_bin = dist_dir / f"webgate-server{executable_ext()}"

    commit = get_git_commit()
    plat = "windows" if is_windows() else ("macos" if is_macos() else "linux")
    arch = "x86_64"

    # Sign Client Manifest
    if client_bin.exists():
        manifest_client = dist_dir / "manifest-client.json"
        cmd = [
            sys.executable,
            str(dist_script),
            "sign",
            "--version",
            version,
            "--channel",
            channel,
            "--source-commit",
            commit,
            "--platform",
            plat,
            "--arch",
            arch,
            "--artifact",
            str(client_bin),
            "--signing-secret",
            signing_secret,
            "--output",
            str(manifest_client),
        ]
        run(cmd, label="Sign Client Manifest")

    # Sign Server Manifest
    if server_bin.exists():
        manifest_server = dist_dir / "manifest-server.json"
        cmd = [
            sys.executable,
            str(dist_script),
            "sign",
            "--version",
            version,
            "--channel",
            channel,
            "--source-commit",
            commit,
            "--platform",
            plat,
            "--arch",
            arch,
            "--artifact",
            str(server_bin),
            "--signing-secret",
            signing_secret,
            "--output",
            str(manifest_server),
        ]
        run(cmd, label="Sign Server Manifest")

    # Sign Relay Manifest
    relay_bin = dist_dir / f"webgate-relay{executable_ext()}"
    if relay_bin.exists():
        manifest_relay = dist_dir / "manifest-relay.json"
        cmd = [
            sys.executable,
            str(dist_script),
            "sign",
            "--version",
            version,
            "--channel",
            channel,
            "--source-commit",
            commit,
            "--platform",
            plat,
            "--arch",
            arch,
            "--artifact",
            str(relay_bin),
            "--signing-secret",
            signing_secret,
            "--output",
            str(manifest_relay),
        ]
        run(cmd, label="Sign Relay Manifest")

    # 3. Verify all generated manifests against artifacts
    for manifest_name in ("manifest-client.json", "manifest-server.json", "manifest-relay.json"):
        manifest_file = dist_dir / manifest_name
        if manifest_file.exists():
            cmd = [
                sys.executable,
                str(dist_script),
                "verify",
                "--manifest",
                str(manifest_file),
                "--artifact-dir",
                str(dist_dir),
                "--signing-secret",
                signing_secret,
            ]
            run(cmd, label=f"Verify {manifest_name}")

    print(f"\n{Color.GREEN}{Color.BOLD}[SUCCESS] Distribution package generated and verified in {dist_dir}{Color.RESET}")
    return 0


# ==============================================================================
# Maintenance & Diagnostics
# ==============================================================================


def clean(*, assume_yes: bool, dry_run: bool = False) -> int:
    if not confirm("Clean all build artifacts (target/, bin/, dist/, .webgate)?", assume_yes):
        return 0
    run(["cargo", "clean"], dry_run=dry_run, label="Cargo Clean")
    for d in ("bin", "dist", ".webgate"):
        p = ROOT / d
        if p.exists() and not dry_run:
            shutil.rmtree(p)
            print(f"Removed {p}")
    return 0


def servo_doctor() -> int:
    status = check_fontconfig()
    print_statuses([status])
    commands = servo_native_install_commands()
    if commands and not status.ok:
        print("\nDetected install plan:")
        for command in commands:
            print("  " + " ".join(command))
    return 0 if (status.ok or not status.required) else 1


def android_doctor() -> int:
    sdk = os.environ.get("ANDROID_SDK_ROOT") or os.environ.get("ANDROID_HOME")
    ndk = os.environ.get("ANDROID_NDK_HOME") or os.environ.get("NDK_HOME")
    statuses = [
        ToolStatus("java", False, executable("java") is not None, "found" if executable("java") else "not found", "install a supported JDK"),
        ToolStatus("adb", False, executable("adb") is not None, "found" if executable("adb") else "not found", "install Android platform-tools"),
        ToolStatus("sdkmanager", False, executable("sdkmanager") is not None, "found" if executable("sdkmanager") else "not found", "install Android command-line tools"),
        ToolStatus("ANDROID_SDK_ROOT", False, bool(sdk), sdk or "not set", "point it to the Android SDK"),
        ToolStatus("Android NDK", False, bool(ndk), ndk or "not explicitly configured", "set ANDROID_NDK_HOME/NDK_HOME when the Android task begins"),
    ]
    print_statuses(statuses)
    return 0


# ==============================================================================
# Interactive Terminal Menu
# ==============================================================================


def interactive_menu() -> int:
    actions: dict[str, tuple[str, Any]] = {
        "1": ("Диагностика окружения и инструментов (Doctor)", lambda: doctor()),
        "2": ("Установка / бутстрап инструментов разработчика", lambda: install_missing(assume_yes=False, dry_run=False, with_mutation=False)),
        "3": ("Компиляция ВСЕХ программ (Клиент + Сервер)", lambda: compile_target("all", release=False)),
        "4": ("Сборка релизных пакетов (Оптимизированная)", lambda: compile_target("all", release=True)),
        "5": ("Сборка только клиента (Rust Desktop)", lambda: compile_target("client", release=False)),
        "6": ("Сборка только сервера (Go Gateway)", lambda: compile_target("server", release=False)),
        "7": ("Сборка платформенной библиотеки Android", lambda: compile_target("android", release=False)),
        "8": ("Запуск клиентского приложения", lambda: run_client()),
        "9": ("Запуск серверного шлюза", lambda: run_server()),
        "10": ("Режим совместной разработки (Сервер + Клиент)", lambda: run_dev_concurrent()),
        "11": ("Запуск всех тестов (Rust + Go + Python)", lambda: test()),
        "12": ("Проверка политик безопасности", lambda: security()),
        "13": ("Полный шлюз качества (CI-Parity Verification)", lambda: verify()),
        "14": ("Сборка дистрибутива и подписание манифестов", lambda: package_distribution()),
        "15": ("Диагностика окружения Android", android_doctor),
        "16": ("Очистка артефактов сборки", lambda: clean(assume_yes=False)),
    }
    while True:
        print(f"\n{Color.BOLD}{Color.CYAN}===================================================={Color.RESET}")
        print(f"{Color.BOLD}{Color.WHITE}          Менеджер Проектов WebGate                 {Color.RESET}")
        print(f"{Color.BOLD}{Color.CYAN}===================================================={Color.RESET}")
        for key, (label, _) in actions.items():
            print(f" {Color.YELLOW}{key:>2}.{Color.RESET} {label}")
        print(f" {Color.RED} 0.{Color.RESET} Выход")
        print(f"{Color.CYAN}----------------------------------------------------{Color.RESET}")
        choice = input(f"{Color.BOLD}Выберите действие [0-16]: {Color.RESET}").strip()
        if choice == "0":
            return 0
        action = actions.get(choice)
        if action is None:
            print(f"{Color.RED}Неизвестный пункт меню.{Color.RESET}")
            continue
        try:
            code = action[1]()
            print(f"\nРезультат: {Color.GREEN}УСПЕШНО{Color.RESET}" if code == 0 else f"\nРезультат: {Color.YELLOW}ВНИМАНИЕ ({code}){Color.RESET}")
        except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
            print(f"\n{Color.RED}ОШИБКА: {exc}{Color.RESET}", file=sys.stderr)


# ==============================================================================
# CLI Parser & Entrypoint
# ==============================================================================


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(
        description="Прогрессивный менеджер проектов, компиляции и оркестрации WebGate",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    sub = root.add_subparsers(dest="command")

    # doctor
    doctor_parser = sub.add_parser("doctor", help="Проверка окружения, Go, Rust, Android и нативных зависимостей")
    doctor_parser.add_argument("--json", action="store_true", help="Вывод статуса в формате JSON")

    # install
    install_parser = sub.add_parser("install", help="Установка разрешенных инструментов разработчика")
    install_parser.add_argument("--yes", action="store_true", help="Автоматическое подтверждение установки")
    install_parser.add_argument("--dry-run", action="store_true", help="Показать команды без выполнения")
    install_parser.add_argument("--with-mutation", action="store_true", help="Также установить cargo-mutants")

    # compile / build
    build_parser = sub.add_parser("build", aliases=["compile"], help="Компиляция программ (Client, Server, Android или All)")
    build_parser.add_argument("--target", default="all", choices=["all", "client", "server", "android", "workspace"], help="Цель компиляции")
    build_parser.add_argument("--release", action="store_true", help="Оптимизированная релизная сборка")
    build_parser.add_argument("--out-dir", type=Path, help="Директория для собранных бинарников (по умолчанию: ./bin)")
    build_parser.add_argument("--dry-run", action="store_true", help="Показать команды без выполнения")

    # run
    run_parser = sub.add_parser("run", help="Запуск программ проекта (server, client или dev)")
    run_parser.add_argument("program", choices=["server", "client", "dev", "all"], help="Программа для запуска")
    run_parser.add_argument("extra_args", nargs="*", help="Аргументы, передаваемые в программу")
    run_parser.add_argument("--dry-run", action="store_true", help="Показать команду без запуска")

    # test
    test_parser = sub.add_parser("test", help="Запуск матричных тестов (Rust + Go + Python)")
    test_parser.add_argument("--dry-run", action="store_true")

    # verify
    verify_parser = sub.add_parser("verify", help="Полная проверка качества и архитектурных гейтов (CI-parity)")
    verify_parser.add_argument("--dry-run", action="store_true")

    # mutate
    mutate_parser = sub.add_parser("mutate", help="Запуск автоматизированных тестов мутаций безопасности и инвариантов")
    mutate_parser.add_argument("--dry-run", action="store_true")

    # fuzz
    fuzz_parser = sub.add_parser("fuzz", help="Запуск фаззинг-тестов парсеров протоколов и состояния")
    fuzz_parser.add_argument("--duration", type=int, default=2, help="Длительность фаззинга каждого таргета в секундах")
    fuzz_parser.add_argument("--dry-run", action="store_true")

    # security
    security_parser = sub.add_parser("security", help="Проверка политик безопасности и зависимостей")
    security_parser.add_argument("--dry-run", action="store_true")

    # dist
    dist_parser = sub.add_parser("dist", aliases=["package"], help="Сборка дистрибутива с хешами и подписанными манифестами")
    dist_parser.add_argument("--version", default="1.0.0", help="Версия релиза")
    dist_parser.add_argument("--channel", default="stable", choices=["stable", "beta", "nightly"], help="Канал релиза")
    dist_parser.add_argument("--secret", default="webgate-secret-key-2026", help="Секретный ключ подписи манифеста")
    dist_parser.add_argument("--dry-run", action="store_true")

    # native & platform helpers
    sub.add_parser("servo", help="Проверка нативных зависимостей браузера Servo")
    sub.add_parser("android", help="Проверка окружения разработки Android и NDK")

    # clean
    clean_parser = sub.add_parser("clean", help="Очистка всех артефактов сборки (target/, bin/, dist/)")
    clean_parser.add_argument("--yes", action="store_true")
    clean_parser.add_argument("--dry-run", action="store_true")

    sub.add_parser("menu", help="Открыть интерактивное терминальное меню")
    return root


def main(argv: Sequence[str] | None = None) -> int:
    args = parser().parse_args(argv)
    command = args.command or "menu"
    try:
        if command == "menu":
            return interactive_menu()
        if command == "doctor":
            return doctor(json_output=args.json)
        if command == "install":
            return install_missing(
                assume_yes=args.yes,
                dry_run=args.dry_run,
                with_mutation=args.with_mutation,
            )
        if command in {"build", "compile"}:
            return compile_target(
                target=args.target,
                release=args.release,
                out_dir=args.out_dir,
                dry_run=args.dry_run,
            )
        if command == "run":
            if args.program == "server":
                return run_server(args=args.extra_args, dry_run=args.dry_run)
            if args.program == "client":
                return run_client(args=args.extra_args, dry_run=args.dry_run)
            if args.program in {"dev", "all"}:
                return run_dev_concurrent()
        if command == "test":
            return test(dry_run=args.dry_run)
        if command == "verify":
            return verify(dry_run=args.dry_run)
        if command == "mutate":
            return mutate(dry_run=args.dry_run)
        if command == "fuzz":
            return fuzz(duration_sec=args.duration, dry_run=args.dry_run)
        if command == "security":
            return security(dry_run=args.dry_run)
        if command in {"dist", "package"}:
            return package_distribution(
                version=args.version,
                channel=args.channel,
                signing_secret=args.secret,
                dry_run=args.dry_run,
            )
        if command == "servo":
            return servo_doctor()
        if command == "android":
            return android_doctor()
        if command == "clean":
            return clean(assume_yes=args.yes, dry_run=args.dry_run)
    except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
        print(f"{Color.RED}ERROR: {exc}{Color.RESET}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

