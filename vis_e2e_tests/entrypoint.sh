#!/bin/bash
# VHS テープの実行エントリポイント。
# テープは人間の操作のみ。テスト都合は全てここ。
set -euo pipefail

TAPE="${1:?Usage: entrypoint.sh <tape-file>}"
TAPE_NAME="$(basename "$TAPE" .tape)"
OUTDIR="/app/outputs/${TAPE_NAME}"
TXT="${OUTDIR}/${TAPE_NAME}.txt"
mkdir -p "$OUTDIR"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/scripts/show_frame.sh"

# --- セットアップ ---
case "$TAPE_NAME" in
    ssh_launch)
        tmux new-session -d -s main -x 125 -y 37
        /app/bin/lazyclaude setup 2>/dev/null || true
        bash-real /app/lazyclaude.tmux 2>/dev/null || true
        sleep 2
        export VHS_AUTO_TMUX=1
        ;;
    diff_popup)
        /app/bin/lazyclaude setup 2>/dev/null || true
        export LAZYCLAUDE_POPUP_MODE=tmux
        # テスト用ファイル作成
        cat > /tmp/test.go << 'GOEOF'
package main

import "fmt"

func hello() string {
    return "hello"
}

func main() {
    fmt.Println(hello())
}
GOEOF
        ;;
esac

# --- lazyclaude を --debug で起動するようにラッパーを上書き ---
DEBUG_LOG="${OUTDIR}/debug.log"
rm -f /usr/local/bin/lazyclaude
cat > /usr/local/bin/lazyclaude << WRAPPER
#!/bin/bash-real
exec /app/bin/lazyclaude --debug --log-file "$DEBUG_LOG" "\$@"
WRAPPER
chmod +x /usr/local/bin/lazyclaude
mkdir -p /tmp/lazyclaude
ln -sf "${OUTDIR}/server.log" /tmp/lazyclaude/server.log

# --- フレーム監視 (バックグラウンド) ---
LOG="${OUTDIR}/${TAPE_NAME}.log"
source "$SCRIPT_DIR/scripts/watch_frames.sh" | tee >(sed 's/\x1b\[[0-9;]*[a-zA-Z]//g' > "$LOG") &
WATCHER_PID=$!

# --- VHS 実行 ---
VHS_RC=0
vhs -q "$TAPE" || VHS_RC=$?

sleep 1
# tail -f を含むパイプ全体を停止
kill "$WATCHER_PID" 2>/dev/null || true
pkill -f "tail -f $TXT" 2>/dev/null || true
wait "$WATCHER_PID" 2>/dev/null || true

cleanup_frames
exit "$VHS_RC"
