# lazyclaude

## テスト

### ユニットテスト (ホストで実行可能)

```bash
# 全パッケージ
go test ./internal/... -count=1

# gui パッケージのみ (42テスト)
go test -v ./internal/gui/ -count=1

# カバレッジ
go test -cover ./internal/...
```

### E2E テスト (Docker 必須)

tmux + capture-pane で TUI の動作を検証。Docker 内でのみ実行可能。

```bash
# Docker イメージビルド
docker build -f Dockerfile.test -t lazyclaude-test .

# E2E テスト実行
docker run --rm lazyclaude-test go test -v -timeout 120s ./tests/integration/ -run TestE2E

# 全テスト (ユニット + E2E)
docker run --rm lazyclaude-test
```

E2E テストは lazyclaude 内部の tmux ソケットを共有するため、各テストで
`cleanLazyClaudeState()` を呼んで状態をリセットする必要がある。

### 手動テスト (Docker 対話モード)

```bash
# ソースをマウントして対話モードで入る
docker run --rm -it --env-file .env -v "$(pwd)":/app lazyclaude-test bash

# Docker 内でビルド + 起動
go build -o /usr/local/bin/lazyclaude ./cmd/lazyclaude/ && lazyclaude --debug

# デバッグログ確認
cat /tmp/lazyclaude-debug.log
cat /tmp/lazyclaude-debug-tmux-cmds.log
```

### capture-pane の制約

- capture-pane はペインのテキスト内容のみを返す
- カーソル位置、copy-mode ハイライト、tmux オーバーレイは含まれない
- capture-pane テストが PASS でも「表示が正しい」とは限らない
- 表示に関わる修正はユーザーの仮想環境で確認すること
- 参考: tmux issues #1949, #3787

## Docker 仮想環境

全テスト・動作確認は Docker 内で行う。ホスト環境に一切影響しない。

### Claude Code 認証 (サブスクリプション)

Docker 内で Claude Code を使うには `.env` が必要:

```bash
# 1. トークン取得 (ホストで1回だけ実行)
claude setup-token

# 2. .env に保存
echo "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-..." > .env

# 3. 認証確認
docker run --rm --env-file .env lazyclaude-test claude auth status
```

`.env` は `.gitignore` 登録済み。コミットされない。

### Production Isolation

Docker 内で全て完結。ホスト環境への影響はゼロ。

| リソース | Docker 内 | ホスト |
|---------|----------|-------|
| tmux | Docker 内の tmux | 影響なし |
| ~/.claude/ide/ | Docker の /root/.claude/ide/ | 影響なし |
| /tmp/ | Docker の /tmp/ | 影響なし |
| state.json | Docker の /root/.local/share/ | 影響なし |

## gocui の注意点

### ErrUnknownView の比較

jesseduffield/gocui は `go-errors` の `Wrap` を使うため、`==` や `errors.Is` では一致しない。
文字列比較を使う:

```go
func isUnknownView(err error) bool {
    return err != nil && strings.Contains(err.Error(), "unknown view")
}
```

### Editor と keybinding の dispatch 順序

```
1. View-specific bindings (popupViewName 等)
2. Editor.Edit() — Editable=true の view のみ
3. Global bindings — ただし Editable view では rune キー (ch!=0) のグローバルバインドはスキップ
```

- Edit() が `true` を返すとキーは「処理済み」、global binding は呼ばれない
- Edit() が `false` を返すと global binding に fallthrough
- Editable view では rune のグローバルバインドは無視される (gocui 仕様)
- Special key (ch=0: Ctrl+\, Ctrl+D, Esc, Enter, 矢印) は Editable でも global binding が動く

### Ctrl+[ と Esc

Ctrl+[ と Esc は同じバイト (0x1B)。gocui/tcell で区別不可能。
lazyclaude は **Ctrl+\\** を normal mode 切替に使用。

## tmux アーキテクチャ

### 2つの tmux サーバー

lazyclaude は2つの独立した tmux サーバーを使用する:

1. **ユーザーの tmux** (デフォルトソケット) — `display-popup` で lazyclaude TUI を表示
2. **lazyclaude tmux** (`-L lazyclaude` ソケット) — Claude Code セッションを管理

### キー入力の流れ

```
popup 外: キー → ユーザーの tmux root table → マッチなら実行
popup 内: キー → popup プロセスに直接渡る (ユーザーの tmux root table はバイパス)
attach 中: キー → lazyclaude tmux の root table → マッチなら実行
```

- popup 内ではユーザーの tmux の keybinding は発火しない
- そのため同じキー (Ctrl+\) をユーザー側 (popup 起動) と lazyclaude 側 (detach) に登録しても競合しない

### display-popup の動作 (tmux 3.4+)

- popup 内から `display-popup` を呼ぶと既存 popup を **変更** できる (ネストではない)
- `-b rounded` / `-B` で枠線を動的に切り替え可能
- `-T " title "` でタイトルを動的に変更可能
- popup 内のプロセスが終了すると変更も消える

### `tmux source` はキーバインドをリセットしない

`tmux source ~/.config/tmux/tmux.conf` は既存のキーバインドを削除しない。上書きまたは追加のみ。
完全リセットは tmux サーバーの再起動が必要。デバッグ時は `tmux list-keys | grep <key>` と `tmux unbind-key` を使う。

### cleanSessionCommands の制約

- `PostCommands` は `NewSession` 時のみ実行される (`NewWindow` では実行されない)
- lazyclaude サーバー上で実行されるため `-L lazyclaude` は不要
- `-g` (global) を使えばサーバー全体に適用される
- `bind-key` に `-t` フラグは存在しない (tmux エラーになる)
- `pane-border-lines` に `rounded` は存在しない (`single`, `double`, `heavy`, `simple` のみ)
- `popup-border-lines` には `rounded` がある (別のオプション)
- lazyclaude サーバー固有の設定は `tmux -L lazyclaude set-option/bind-key` で `lazyclaude.tmux` から直接設定可能

### MCP サーバー

- `lazyclaude setup` (lazyclaude.tmux 実行時) に起動される常駐デーモン
- Claude Code の hooks から通知を受け、`tmux display-popup` で tool/diff 確認を表示
- lazyclaude TUI が閉じていても popup は表示される
- サーバーログ: `/tmp/lazyclaude-server.log`
- 重複起動防止: `server.IsAlive()` で port file + TCP dial チェック

### パフォーマンス

- `tmux display-message` (subprocess) を capture ごとに呼ぶと ~5ms のオーバーヘッドで体感速度が劣化する
- パフォーマンス問題は git bisect でバイナリ比較して特定する (コード分析より確実)
- チェックポイントは `.claude/checkpoints.log` に記録

## 設計ドキュメント

- `docs/dev/popup-redesign-plan.md` — ポップアップ再設計計画 (全フェーズ完了)
- `docs/dev/issue-normal-mode-navigation.md` — normal mode ナビゲーション issue
- `docs/dev/issue-popup-fullmode.md` — tmux display-popup の失敗記録
- `docs/research/go-pty-terminal-in-terminal.md` — Go PTY/terminal-in-terminal 調査