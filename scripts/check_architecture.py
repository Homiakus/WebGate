#!/usr/bin/env python3
"""Fail CI when WebGate's internal crate dependency boundaries are violated."""

from __future__ import annotations

import sys
import tomllib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

ALLOWED_INTERNAL_DEPENDENCIES: dict[str, set[str]] = {
    "webgate-core": set(),
    "webgate-browser": {"webgate-core"},
    "webgate-browser-servo": {"webgate-browser"},
    "webgate-transport": {"webgate-core"},
    "webgate-platform": {"webgate-core"},
    "webgate-app": {
        "webgate-browser",
        "webgate-core",
        "webgate-platform",
        "webgate-transport",
    },
}

DEPENDENCY_TABLES = ("dependencies", "dev-dependencies", "build-dependencies")


def load_toml(path: Path) -> dict:
    with path.open("rb") as handle:
        return tomllib.load(handle)


def collect_dependency_names(document: dict) -> set[str]:
    names: set[str] = set()
    for table_name in DEPENDENCY_TABLES:
        names.update(document.get(table_name, {}))

    for target in document.get("target", {}).values():
        if not isinstance(target, dict):
            continue
        for table_name in DEPENDENCY_TABLES:
            names.update(target.get(table_name, {}))
    return names


def main() -> int:
    workspace = load_toml(ROOT / "Cargo.toml")
    members = workspace["workspace"]["members"]

    actual_packages: dict[str, Path] = {}
    for member in members:
        manifest = ROOT / member / "Cargo.toml"
        document = load_toml(manifest)
        name = document["package"]["name"]
        actual_packages[name] = manifest

    expected = set(ALLOWED_INTERNAL_DEPENDENCIES)
    actual = set(actual_packages)
    if actual != expected:
        print(
            "workspace package set changed without architecture-policy update:\n"
            f"  expected={sorted(expected)}\n"
            f"  actual={sorted(actual)}",
            file=sys.stderr,
        )
        return 1

    errors: list[str] = []
    for package, manifest in actual_packages.items():
        document = load_toml(manifest)
        internal = {
            dependency
            for dependency in collect_dependency_names(document)
            if dependency.startswith("webgate-")
        }
        allowed = ALLOWED_INTERNAL_DEPENDENCIES[package]
        forbidden = internal - allowed
        if forbidden:
            errors.append(
                f"{package} has forbidden internal dependencies: {sorted(forbidden)}; "
                f"allowed={sorted(allowed)}"
            )

    if errors:
        print("architecture dependency violations:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print("architecture dependency policy: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
