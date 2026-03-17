# lazyclaude

## Docker 仮想環境

全テスト・動作確認は Docker 内で行う。ホスト環境 (IDE, tmux, ~/.claude) に一切影響しない。

### ビルド

```bash
cd lazyclaude/
docker build -f Dockerfile.test -t lazyclaude-test .
```

### Claude Code 認証 (サブスクリプション)

Docker 内で Claude Code を使うには `.env` が必要:

```bash
# 1. トークン取得 (ホストで1回だけ実行、ブラウザ認証)
claude setup-token

# 2. .env に保存
echo "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-..." > .env

# 3. 認証確認
docker run --rm --env-file .env lazyclaude-test claude auth status
```

`.env` は `.gitignore` に登録済み。コミットされない。

### デフォルト: 全テスト実行

```bash
docker run --rm lazyclaude-test
```

### UI の確認 (対話用)

gocui TUI の見た目をテキストで確認する:

```bash
docker run --rm lazyclaude-test bash -c '
  tmux -f /dev/null new-session -d -s ui -x 80 -y 20 "lazyclaude; sleep 999"
  sleep 2
  tmux capture-pane -p -t ui
  tmux kill-server 2>/dev/null
'
```

サイズを変えて確認:

```bash
docker run --rm lazyclaude-test bash -c '
  tmux -f /dev/null new-session -d -s ui -x 120 -y 40 "lazyclaude; sleep 999"
  sleep 2
  tmux capture-pane -p -t ui
  tmux kill-server 2>/dev/null
'
```

### キー操作 → UI 変化の確認

```bash
docker run --rm lazyclaude-test bash -c '
  tmux -f /dev/null new-session -d -s ui -x 80 -y 20 "lazyclaude; sleep 999"
  sleep 2
  tmux send-keys -t ui j          # カーソル下
  sleep 0.3
  tmux capture-pane -p -t ui      # 変化後のUIを確認
  tmux send-keys -t ui q          # 終了
  tmux kill-server 2>/dev/null
'
```

### サブコマンドの確認

```bash
# diff popup
docker run --rm lazyclaude-test bash -c '
  tmux -f /dev/null new-session -d -s ui -x 80 -y 20 \
    "lazyclaude diff --window test --old tests/testdata/old.go --new tests/testdata/new.go; sleep 999"
  sleep 2
  tmux capture-pane -p -t ui
  tmux kill-server 2>/dev/null
'

# help
docker run --rm lazyclaude-test bash -c '
  tmux -f /dev/null new-session -d -s ui -x 80 -y 20 "lazyclaude --help; sleep 999"
  sleep 1
  tmux capture-pane -p -t ui
  tmux kill-server 2>/dev/null
'
```

### MCP サーバーの確認

```bash
docker run --rm -p 7899:7899 lazyclaude-test bash -c '
  lazyclaude server --port 7899 --token test123 &
  sleep 1
  # 認証あり
  curl -s -X POST http://127.0.0.1:7899/notify \
    -H "Content-Type: application/json" \
    -H "X-Auth-Token: test123" \
    -d "{\"pid\": 12345}"
  echo ""
  # 認証なし → 401
  curl -s -o /dev/null -w "status: %{http_code}\n" \
    -X POST http://127.0.0.1:7899/notify \
    -H "Content-Type: application/json" \
    -d "{\"pid\": 1}"
  kill %1
'
```

### Claude Code を使う操作 (--env-file 必須)

```bash
# Claude Code の認証確認
docker run --rm --env-file .env lazyclaude-test claude auth status

# Claude Code を tmux 内で起動
docker run --rm --env-file .env lazyclaude-test bash -c '
  tmux -f /dev/null new-session -d -s ui -x 120 -y 40 \
    "claude --print \"hello\" 2>&1; sleep 999"
  sleep 5
  tmux capture-pane -p -t ui
  tmux kill-server 2>/dev/null
'
```

`--env-file .env` を付けないと Claude Code は未認証で失敗する。

### 任意のコマンド実行

```bash
# bash で入る (認証付き)
docker run --rm -it --env-file .env lazyclaude-test bash

# bash で入る (認証なし、lazyclaude のみ)
docker run --rm -it lazyclaude-test bash

# 特定テストだけ
docker run --rm lazyclaude-test go test -v ./internal/server/ -run TestServer_WebSocket

# カバレッジ
docker run --rm lazyclaude-test go test -cover ./internal/...
```

## gocui の注意点

### ErrUnknownView の比較

jesseduffield/gocui は `go-errors` の `Wrap` を使うため、`==` や `errors.Is` では一致しない。
文字列比較を使う:

```go
func isUnknownView(err error) bool {
    return err != nil && strings.Contains(err.Error(), "unknown view")
}
```

### alternate screen buffer

gocui は alternate screen buffer (`\e[?1049h`) を使う。
`tmux capture-pane -p` はこれを正しくキャプチャし、罫線・テキストがそのまま文字列で取得できる。

## Production Isolation

Docker 内で全て完結するため、ホスト環境への影響はゼロ。

| リソース | Docker 内 | ホスト |
|---------|----------|-------|
| tmux | Docker 内の tmux | 影響なし |
| ~/.claude/ide/ | Docker の /root/.claude/ide/ | 影響なし |
| /tmp/ | Docker の /tmp/ | 影響なし |
| state.json | Docker の /root/.local/share/ | 影響なし |
| ネットワーク | Docker 内 loopback | `-p` 指定時のみ expose |

## Development Plan

`docs/dev/go-migration-plan.md` (親ディレクトリ) を参照。