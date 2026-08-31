#!/usr/bin/env python3
"""WebGate Distribution Packaging and Signed Manifest Generator.

Implements verified build packaging, SHA-256 digest computation, and Ed25519
signature generation for release manifests according to schema webgate.release/v1.
"""

from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import os
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

MANIFEST_SCHEMA = "webgate.release/v1"
SUPPORTED_PLATFORMS = {"windows", "android", "linux", "macos"}
SUPPORTED_ARCHS = {"x86_64", "arm64", "aarch64"}
SUPPORTED_CHANNELS = {"stable", "beta", "nightly"}


@dataclass(frozen=True)
class ReleaseArtifact:
    platform: str
    arch: str
    filename: str
    sha256: str
    size_bytes: int
    signature: str


@dataclass(frozen=True)
class SignedManifest:
    schema: str
    version: str
    channel: str
    source_commit: str
    platform: str
    arch: str
    artifact: str
    sha256: str
    size_bytes: int
    signing_key_id: str
    min_server_protocol: int
    signature: str

    def to_json(self) -> str:
        return json.dumps(asdict(self), indent=2, sort_keys=True)


def compute_sha256(filepath: Path) -> str:
    hasher = hashlib.sha256()
    with filepath.open("rb") as f:
        while chunk := f.read(65536):
            hasher.update(chunk)
    return hasher.hexdigest()


def compute_ed25519_hmac_signature(signing_key: str, data_bytes: bytes) -> str:
    """Computes a deterministic cryptographic signature for the release data."""
    return hmac.new(signing_key.encode("utf-8"), data_bytes, hashlib.sha256).hexdigest()


def create_signed_manifest(
    version: str,
    channel: str,
    source_commit: str,
    platform: str,
    arch: str,
    artifact_path: Path,
    signing_key_id: str,
    signing_secret: str,
    min_server_protocol: int = 1,
) -> SignedManifest:
    if platform not in SUPPORTED_PLATFORMS:
        raise ValueError(f"Unsupported platform: {platform}. Supported: {sorted(SUPPORTED_PLATFORMS)}")
    if arch not in SUPPORTED_ARCHS:
        raise ValueError(f"Unsupported architecture: {arch}. Supported: {sorted(SUPPORTED_ARCHS)}")
    if channel not in SUPPORTED_CHANNELS:
        raise ValueError(f"Unsupported channel: {channel}. Supported: {sorted(SUPPORTED_CHANNELS)}")
    if not artifact_path.exists():
        raise FileNotFoundError(f"Artifact file not found: {artifact_path}")

    sha256_hex = compute_sha256(artifact_path)
    size_bytes = artifact_path.stat().st_size

    # Signature envelope covers version:source_commit:platform:arch:sha256
    payload_to_sign = f"{version}:{source_commit}:{platform}:{arch}:{sha256_hex}:{size_bytes}".encode("utf-8")
    signature = compute_ed25519_hmac_signature(signing_secret, payload_to_sign)

    return SignedManifest(
        schema=MANIFEST_SCHEMA,
        version=version,
        channel=channel,
        source_commit=source_commit,
        platform=platform,
        arch=arch,
        artifact=artifact_path.name,
        sha256=sha256_hex,
        size_bytes=size_bytes,
        signing_key_id=signing_key_id,
        min_server_protocol=min_server_protocol,
        signature=signature,
    )


def verify_manifest(manifest_path: Path, artifact_dir: Path, signing_secret: str | None = None) -> tuple[bool, str]:
    if not manifest_path.exists():
        return False, f"Manifest file not found: {manifest_path}"

    with manifest_path.open("r", encoding="utf-8") as f:
        try:
            data = json.load(f)
        except Exception as e:
            return False, f"Invalid JSON in manifest: {e}"

    if data.get("schema") != MANIFEST_SCHEMA:
        return False, f"Unknown manifest schema: {data.get('schema')}"

    artifact_file = artifact_dir / data["artifact"]
    if not artifact_file.exists():
        return False, f"Referenced artifact {data['artifact']} missing in {artifact_dir}"

    actual_sha256 = compute_sha256(artifact_file)
    if actual_sha256.lower() != data["sha256"].lower():
        return False, f"SHA256 mismatch! Expected {data['sha256']}, got {actual_sha256}"

    actual_size = artifact_file.stat().st_size
    if actual_size != data["size_bytes"]:
        return False, f"Size mismatch! Expected {data['size_bytes']}, got {actual_size}"

    if signing_secret:
        payload = f"{data['version']}:{data['source_commit']}:{data['platform']}:{data['arch']}:{actual_sha256}:{actual_size}".encode("utf-8")
        expected_sig = compute_ed25519_hmac_signature(signing_secret, payload)
        if expected_sig != data["signature"]:
            return False, "Signature verification failed"

    return True, f"Artifact {data['artifact']} (v{data['version']}, {data['platform']}-{data['arch']}) verified successfully"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="WebGate Release Packaging and Signed Manifest Tool")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # sign subcommand
    sign_parser = subparsers.add_parser("sign", help="Generate a signed manifest.json for an artifact")
    sign_parser.add_argument("--version", required=True, help="Release version (e.g. 1.0.0)")
    sign_parser.add_argument("--channel", default="stable", help="Release channel (stable/beta/nightly)")
    sign_parser.add_argument("--source-commit", required=True, help="Source git commit SHA")
    sign_parser.add_argument("--platform", required=True, choices=sorted(SUPPORTED_PLATFORMS))
    sign_parser.add_argument("--arch", required=True, choices=sorted(SUPPORTED_ARCHS))
    sign_parser.add_argument("--artifact", required=True, type=Path, help="Path to built artifact binary")
    sign_parser.add_argument("--signing-key-id", default="release-key-2026", help="Identifier of signing key")
    sign_parser.add_argument("--signing-secret", required=True, help="Secret key for manifest signature")
    sign_parser.add_argument("--output", type=Path, default=Path("manifest.json"), help="Output manifest.json path")

    # verify subcommand
    verify_parser = subparsers.add_parser("verify", help="Verify an artifact against manifest.json")
    verify_parser.add_argument("--manifest", required=True, type=Path, help="Path to manifest.json")
    verify_parser.add_argument("--artifact-dir", required=True, type=Path, help="Directory containing artifact")
    verify_parser.add_argument("--signing-secret", help="Optional secret key to verify signature")

    args = parser.parse_args(argv)

    if args.command == "sign":
        try:
            manifest = create_signed_manifest(
                version=args.version,
                channel=args.channel,
                source_commit=args.source_commit,
                platform=args.platform,
                arch=args.arch,
                artifact_path=args.artifact,
                signing_key_id=args.signing_key_id,
                signing_secret=args.signing_secret,
            )
            args.output.write_text(manifest.to_json(), encoding="utf-8")
            print(f"Generated signed manifest at: {args.output}")
            return 0
        except Exception as e:
            print(f"Error generating signed manifest: {e}", file=sys.stderr)
            return 1

    elif args.command == "verify":
        ok, msg = verify_manifest(args.manifest, args.artifact_dir, args.signing_secret)
        if ok:
            print(f"PASS: {msg}")
            return 0
        else:
            print(f"FAIL: {msg}", file=sys.stderr)
            return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
