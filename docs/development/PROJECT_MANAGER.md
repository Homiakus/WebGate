# WebGate Project Manager

`scripts/project_manager.py` is the canonical local developer entry point for environment diagnostics, controlled tool bootstrap, CI-parity verification and compilation.

It intentionally uses only the Python standard library so it does not depend on the Rust graph it is responsible for checking.

## Launch

Windows:

```powershell
.\scripts\webgate.ps1
```

Linux/macOS:

```sh
./scripts/webgate.sh
```

Direct CLI:

```sh
python3 scripts/project_manager.py doctor
python3 scripts/project_manager.py install
python3 scripts/project_manager.py verify
python3 scripts/project_manager.py build
python3 scripts/project_manager.py build --release
python3 scripts/project_manager.py test
python3 scripts/project_manager.py security
python3 scripts/project_manager.py servo
python3 scripts/project_manager.py android
```

Running the script without a subcommand opens the interactive menu.

## Menu

1. Environment doctor.
2. Install/repair required development tools.
3. Full CI-parity verification.
4. Debug build.
5. Release build.
6. Workspace tests.
7. Cargo dependency/security policy.
8. Servo native prerequisite diagnostics.
9. Android development diagnostics.
10. Clean generated build/local-manager state.

## Installation policy

The manager is deliberately not a general-purpose package installer.

It may install only known development prerequisites through a small allowlist:

- Rust through the official `rustup.rs` HTTPS bootstrap endpoint;
- `rustfmt` and `clippy` through `rustup component add`;
- `cargo-deny` through `cargo install ... --locked`;
- optional `cargo-mutants` only when explicitly requested;
- Git and confirmed Servo native prerequisites through a detected system package manager.

Linux Servo bootstrap currently includes only prerequisites confirmed by empirical WebGate/Servo build evidence: `pkg-config` plus the fontconfig development package. New native dependencies must first become a Finding in `MASTER_PLAN.md`; the manager must not guess package lists.

Use `install --dry-run` to inspect all commands before execution. Without `--yes`, installation remains interactive.

## CI parity

`verify` runs:

```text
Python manager tests
scripts/check_architecture.py
cargo metadata --locked
cargo fmt --check
cargo check --workspace --all-targets --locked
cargo test --workspace --locked
cargo clippy --workspace --all-targets --locked -- -D warnings
cargo deny check --all-features
git diff --check
```

This command is the preferred local pre-push gate. GitHub Actions remains authoritative for exact-commit verification before `main` is advanced.

## Servo prerequisite doctor

`servo` checks `pkg-config`/`fontconfig` on Linux and prints the detected package-manager remediation. This exists because the first Servo 0.5.0 candidate reached `yeslogic-fontconfig-sys` and failed when `fontconfig.pc` was unavailable on the Linux runner.

The manager does not interpret a successful native prerequisite check as proof that Servo itself is qualified. Servo compilation, browser compatibility, network fail-closed and compromise-containment remain separate plan gates.

## Android doctor

The Android command reports Java, `adb`, `sdkmanager`, SDK-root and NDK configuration. They are informational until the Android implementation milestone because the project deliberately avoids installing an Android SDK/accepting Android licenses implicitly.

## Security properties

- no `shell=True` command execution;
- no arbitrary package names from config/user input;
- remote bootstrap is restricted to the official rustup endpoints;
- destructive clean requires confirmation unless `--yes` is explicit;
- tool installation never changes WebGate source files;
- project verification uses the committed lockfile and existing security policy;
- tool bootstrap and project runtime/security credentials remain completely separate concerns.
