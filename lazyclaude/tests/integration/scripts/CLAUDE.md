# Visual E2E Test Scripts

## 原則

- **全スクリプトは視覚テスト**。毎ステップで `frame()` を呼び、capture-pane の UI 出力を表示する
- フレームは ANSI エスケープで同じ位置に上書き描画される（`sl(1)` のように）
- 固定サイズ **80x24** で統一
- **既存の verify_*.sh スクリプトは絶対に削除しない**。Go テストは PASS/FAIL のみで UI 画面を出力しないため代替にならない
- フレーム間の差分表示はデフォルト ON (`FRAME_DIFF=1`)

## 共通ライブラリ: test_lib.sh

全スクリプトは `test_lib.sh` を source して共通関数を使う。

### 主要関数

| 関数 | 用途 |
|------|------|
| `init_test "名前" binary [--mode MODE]` | 初期化、ソケット生成、trap 設定 |
| `frame "ステップ名"` | capture-pane + ANSI クリア + 固定サイズ描画 + diff |
| `frame_target "名前" "target"` | 別ペインの capture を frame 表示 |
| `check "チェック名" $result` | PASS/FAIL カウント |
| `wait_for "pattern" [timeout]` | capture-pane を polling |
| `send_keys [keys...]` | tmux send-keys |
| `capture` | tmux capture-pane -p |
| `start_lazyclaude` | TUI 起動 + "no sessions" 待ち + 初回 frame |
| `start_session "cmd"` | tmux new-session |
| `enqueue_notification "ToolName"` | モック通知 JSON ファイル生成 |
| `finish_test` | 結果表示、FAIL > 0 なら exit 1 |

### 環境変数

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `FRAME_DELAY` | 0.5 | フレーム間の秒数 |
| `FRAME_DIFF` | 1 | フレーム間差分表示 (0 で無効) |
| `MODE` | mock | 実行モード (mock/tmux/ssh/claude) |

## スクリプト一覧

| スクリプト | チェック数 | 必要環境 | 内容 |
|-----------|----------|---------|------|
| `verify_popup_stack.sh` | 7 | mock | カスケード表示、フォーカス切替、サスペンド/再開、ディスミス、一括承認 |
| `verify_option_count.sh` | 4 | mock | 2択 y/n、3択 y/a/n、デフォルト動作 |
| `verify_setup.sh` | 6 | mock | setup コマンド、ポートファイル、settings.json、hooks、冪等性 |
| `verify_key_order.sh` | 1 | Claude Code | IME キー順序保持 (あいうえお) |
| `verify_tmux_popup.sh` | 7 | tmux display-popup | script 記録 + PTY リプレイ + gocui 描画検証 |
| `verify_tmux_popup_runner.sh` | - | tmux | verify_tmux_popup.sh の PTY ラッパー |
| `verify_remote_popup.sh` | 3-5 | SSH + Claude Code | SSH 画面 → TUI → リモート Claude Code → popup |
| `verify_2option_detect.sh` | 5 | Claude Code | リアル Claude Code 2択 permission dialog 検出 |
| `measure_latency.sh` | 1 | Claude Code | 1st/2nd ランチのレイテンシ比較 |

## 実行方法

Docker 内で実行する。ホスト環境に影響しない。

```bash
cd lazyclaude/

# Docker イメージビルド
docker build -f Dockerfile.test -t lazyclaude-test .

# mock テスト (Claude Code 不要)
docker run --rm -it lazyclaude-test bash -c 'bash tests/integration/scripts/verify_popup_stack.sh lazyclaude'
docker run --rm -it lazyclaude-test bash -c 'bash tests/integration/scripts/verify_option_count.sh lazyclaude'
docker run --rm -it lazyclaude-test bash -c 'bash tests/integration/scripts/verify_setup.sh lazyclaude'

# tmux display-popup テスト
docker run --rm -it lazyclaude-test bash tests/integration/scripts/verify_tmux_popup_runner.sh lazyclaude

# Claude Code 必要なテスト (.env に CLAUDE_CODE_OAUTH_TOKEN が必要)
docker run --rm -it --env-file .env lazyclaude-test bash -c 'bash tests/integration/scripts/verify_key_order.sh lazyclaude'
docker run --rm -it --env-file .env lazyclaude-test bash -c 'bash tests/integration/scripts/verify_2option_detect.sh lazyclaude'
docker run --rm -it --env-file .env lazyclaude-test bash -c 'bash tests/integration/scripts/measure_latency.sh lazyclaude'

# SSH リモートテスト (Docker Compose)
docker compose -f docker-compose.ssh.yml build
docker compose -f docker-compose.ssh.yml run -it --rm local \
  "ssh-keyscan -H remote >> /root/.ssh/known_hosts 2>/dev/null && bash tests/integration/scripts/verify_remote_popup.sh lazyclaude"
docker compose -f docker-compose.ssh.yml down

# Makefile ターゲット
make test-visual              # mock の verify_*.sh を全実行
make test-visual-popup_stack  # 単体実行
make test-visual-ssh          # SSH モードで全実行
```

## 新しいテストを書くとき

```bash
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test_lib.sh"

init_test "テスト名" "${1:-lazyclaude}" "${@:2}"

start_lazyclaude            # TUI 起動 + Frame 1

enqueue_notification "Bash" # モック通知
sleep 1
wait_for "Bash" 3 || true
frame "popup appeared"      # Frame 2 (UI キャプチャ)

C=$(capture)
R=0; echo "$C" | grep -q "Bash" || R=1
check "popup shows Bash" $R

finish_test
```

## verify_tmux_popup.sh の特殊性

このスクリプトは外部の tmux ソケット (`LAZYCLAUDE_TMUX_SOCKET`) を使うため `init_test()` を呼ばない。
`_PREV_FRAME_FILE`, `_CURR_FRAME_FILE` を手動初期化し、`_draw_frame` を直接呼ぶ。
`verify_tmux_popup_runner.sh` が PTY を割り当てて実行する。

## capture-pane の制約

- capture-pane はペインのテキスト内容のみを返す
- カーソル位置、copy-mode ハイライト、tmux オーバーレイ (display-popup 含む) は含まれない
- capture-pane テストが PASS でも「表示が正しい」とは限らない
- 表示に関わる修正はユーザーの仮想環境 (Docker -it) で目視確認すること
