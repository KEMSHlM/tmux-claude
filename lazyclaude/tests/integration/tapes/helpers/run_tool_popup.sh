#!/bin/bash
# Run lazyclaude tool popup with test data.
# Usage: run_tool_popup.sh [--window WINDOW]
WINDOW="${1:-@0}"
TOOL_NAME=Bash \
TOOL_INPUT='{"command":"ls /tmp"}' \
TOOL_CWD=/tmp \
lazyclaude tool --window "$WINDOW"
