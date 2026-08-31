#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANAGER="${SCRIPT_DIR}/project_manager.py"

find_python() {
    for cmd in python3 python py; do
        if command -v "$cmd" >/dev/null 2>&1; then
            if "$cmd" -c "import sys; sys.exit(0 if sys.version_info >= (3, 11) else 1)" 2>/dev/null; then
                echo "$cmd"
                return 0
            fi
        fi
    done
    return 1
}

PYTHON_BIN="$(find_python || true)"
if [ -z "$PYTHON_BIN" ]; then
    echo "ОШИБКА: Для запуска менеджера проектов WebGate требуется Python 3.11+." >&2
    exit 1
fi

exec "$PYTHON_BIN" "$MANAGER" "$@"
