#!/usr/bin/env bash
# tmux-claude TPM plugin entry point.
# Finds the lazyclaude binary and runs `lazyclaude setup`.

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BINARY="${CURRENT_DIR}/bin/lazyclaude"

if [ ! -x "$BINARY" ]; then
    BINARY="$(command -v lazyclaude 2>/dev/null)"
fi

if [ -z "$BINARY" ]; then
    echo "lazyclaude: binary not found in ${CURRENT_DIR}/bin/ or PATH" >&2
    exit 1
fi

"$BINARY" setup
