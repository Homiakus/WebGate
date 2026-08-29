#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
MANAGER="$SCRIPT_DIR/project_manager.py"

if command -v python3 >/dev/null 2>&1; then
    exec python3 "$MANAGER" "$@"
fi

if command -v python >/dev/null 2>&1; then
    exec python "$MANAGER" "$@"
fi

cat >&2 <<'EOF'
Python 3.11+ is required to launch the WebGate project manager.
Install Python using your system package manager, then rerun scripts/webgate.sh.
EOF
exit 1
