#!/usr/bin/env python3
"""WebGate project manager.

Interactive developer menu plus non-interactive commands for environment checks,
controlled tool bootstrap, CI-parity verification, compilation and diagnostics.
The script uses only the Python standard library so it stays independent from the
Rust dependency graph it manages.
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import shutil
import subprocess
import sys
import tempfile
import urllib.request
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterable, Sequence

ROOT = Path(__file__).resolve().parents[1]
MIN_PYTHON = (3, 11)
RUSTUP_SH_URL = "https://sh.rustup.rs"
RUSTUP_WIN_BASE_URL = "https://win.rustup.rs"


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
) -> int:
    printable = subprocess.list2cmdline(list(command)) if is_windows() else " ".join(command)
    print(f"\n$ {printable}")
    if dry_run:
        return 0
    proc = subprocess.run(list(command), cwd=cwd, env=env, check=False)
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
            "repository-root",
            True,
            (ROOT / "Cargo.toml").is_file() and (ROOT / "MASTER_PLAN.md").is_file(),
            str(ROOT),
            "run the manager from a complete WebGate checkout",
        )
    )
    return statuses


def print_statuses(statuses: Iterable[ToolStatus]) -> None:
    print("\nWebGate environment")
    print("-" * 78)
    for item in statuses:
        marker = "OK" if item.ok else ("MISS" if item.required else "OPT")
        req = "required" if item.required else "optional"
        print(f"[{marker:4}] {item.name:28} {req:8} {item.detail}")
        if not item.ok and item.remediation:
            print(f"       -> {item.remediation}")


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


def verify(*, dry_run: bool = False) -> int:
    commands = [
        [sys.executable, "-m", "unittest", "discover", "-s", "scripts/tests", "-p", "test_*.py", "-v"],
        [sys.executable, "scripts/check_architecture.py"],
        ["cargo", "metadata", "--locked", "--no-deps", "--format-version", "1"],
        ["cargo", "fmt", "--all", "--", "--check"],
        ["cargo", "check", "--workspace", "--all-targets", "--locked"],
        ["cargo", "test", "--workspace", "--locked"],
        ["cargo", "clippy", "--workspace", "--all-targets", "--locked", "--", "-D", "warnings"],
        ["cargo", "deny", "check", "--all-features"],
        ["git", "diff", "--check"],
    ]
    for command in commands:
        run(command, dry_run=dry_run)
    print("\nVerification: PASS")
    return 0


def build(*, release: bool, dry_run: bool = False) -> int:
    command = ["cargo", "build", "--workspace", "--locked"]
    if release:
        command.append("--release")
    run(command, dry_run=dry_run)
    return 0


def test(*, dry_run: bool = False) -> int:
    run(["cargo", "test", "--workspace", "--locked"], dry_run=dry_run)
    return 0


def security(*, dry_run: bool = False) -> int:
    run(["cargo", "deny", "check", "--all-features"], dry_run=dry_run)
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


def clean(*, assume_yes: bool, dry_run: bool = False) -> int:
    if not confirm("Run cargo clean and remove local .webgate diagnostics?", assume_yes):
        return 0
    run(["cargo", "clean"], dry_run=dry_run)
    local_state = ROOT / ".webgate"
    if local_state.exists() and not dry_run:
        shutil.rmtree(local_state)
    return 0


def interactive_menu() -> int:
    actions = {
        "1": ("Environment doctor", lambda: doctor()),
        "2": ("Install / repair required developer tools", lambda: install_missing(assume_yes=False, dry_run=False, with_mutation=False)),
        "3": ("Full verification (CI parity)", lambda: verify()),
        "4": ("Build debug", lambda: build(release=False)),
        "5": ("Build release", lambda: build(release=True)),
        "6": ("Run tests", lambda: test()),
        "7": ("Dependency / security policy", lambda: security()),
        "8": ("Servo native prerequisites", servo_doctor),
        "9": ("Android development doctor", android_doctor),
        "10": ("Clean build artifacts", lambda: clean(assume_yes=False)),
    }
    while True:
        print("\n=== WebGate Project Manager ===")
        for key, (label, _) in actions.items():
            print(f" {key:>2}. {label}")
        print("  0. Exit")
        choice = input("Select: ").strip()
        if choice == "0":
            return 0
        action = actions.get(choice)
        if action is None:
            print("Unknown menu item")
            continue
        try:
            code = action[1]()
            print(f"\nResult: {'PASS' if code == 0 else 'ATTENTION'}")
        except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
            print(f"\nERROR: {exc}", file=sys.stderr)


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description="WebGate project environment and build manager")
    sub = root.add_subparsers(dest="command")

    doctor_parser = sub.add_parser("doctor", help="check required project tools and native prerequisites")
    doctor_parser.add_argument("--json", action="store_true", help="emit machine-readable status")

    install_parser = sub.add_parser("install", help="install/repair allowlisted required developer tools")
    install_parser.add_argument("--yes", action="store_true", help="accept allowlisted installation prompts")
    install_parser.add_argument("--dry-run", action="store_true", help="show installation commands without executing")
    install_parser.add_argument("--with-mutation", action="store_true", help="also install cargo-mutants")

    verify_parser = sub.add_parser("verify", help="run the local CI-parity gate")
    verify_parser.add_argument("--dry-run", action="store_true")

    build_parser = sub.add_parser("build", help="compile the workspace")
    build_parser.add_argument("--release", action="store_true")
    build_parser.add_argument("--dry-run", action="store_true")

    test_parser = sub.add_parser("test", help="run workspace tests")
    test_parser.add_argument("--dry-run", action="store_true")

    security_parser = sub.add_parser("security", help="run cargo-deny policy checks")
    security_parser.add_argument("--dry-run", action="store_true")

    sub.add_parser("servo", help="check confirmed Servo native prerequisites")
    sub.add_parser("android", help="check Android development prerequisites")

    clean_parser = sub.add_parser("clean", help="remove build/local manager artifacts")
    clean_parser.add_argument("--yes", action="store_true")
    clean_parser.add_argument("--dry-run", action="store_true")

    sub.add_parser("menu", help="open the interactive project menu")
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
        if command == "verify":
            return verify(dry_run=args.dry_run)
        if command == "build":
            return build(release=args.release, dry_run=args.dry_run)
        if command == "test":
            return test(dry_run=args.dry_run)
        if command == "security":
            return security(dry_run=args.dry_run)
        if command == "servo":
            return servo_doctor()
        if command == "android":
            return android_doctor()
        if command == "clean":
            return clean(assume_yes=args.yes, dry_run=args.dry_run)
    except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
