#!/bin/bash
# Enqueue mock notifications for lazyclaude.
# Usage: enqueue.sh [--max-option N] Tool1 Tool2 Tool3 ...
MAX_OPTION=""
while [ $# -gt 0 ]; do
    case "$1" in
        --max-option) MAX_OPTION=",\"max_option\":$2"; shift 2 ;;
        *) break ;;
    esac
done
for name in "$@"; do
    TS=$(date +%s%N)
    echo "{\"tool_name\":\"$name\",\"input\":\"{\\\"a\\\":\\\"1\\\"}\",\"window\":\"@0\",\"timestamp\":\"2026-01-01T00:00:00Z\"${MAX_OPTION}}" \
        > "/tmp/lazyclaude-q-${TS}.json"
    sleep 0.05
done
