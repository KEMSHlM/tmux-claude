# Plan: Remote session の MCP/plugin 編集対応 (Bug 2)

## Context

リモートセッションで MCP toggle や plugin 管理の UI 操作が何も起きない。ローカルでは動く。

## Root Cause

git 履歴調査で判明:

1. **`0630493 feat: support MCP toggle for SSH remote sessions via SSH commands`** — `internal/mcp/ssh.go` を追加、`Manager` に `host` フィールドと `SetHost()` method を追加、SSH 経由で remote の `~/.claude.json` / `settings.local.json` を read/write する機能
2. **`15fb347 fix: address security and error handling issues from code review`** — 上記への security fix
3. **`e1e1178 fix: disable plugin management for SSH remote sessions`** — SSH 時に plugin 管理を無効化するガード
4. **`6dc0d0d fix: disable MCP toggle for SSH remote sessions`** — SSH 時に MCP toggle を無効化するガード

**これら 4 commit はいずれも `daemon-arch` の ancestry に含まれていない** (`git merge-base --is-ancestor` で NOT 確認済)。`fix-ssh-mcp-toggle` / `fix-ssh-plugins` branch にのみ存在する。

現在の `daemon-arch` には:
- MCP/plugin manager が **host 概念を一切持たない** (local-only 実装)
- Remote session 選択時に **ガードすらない**
- 結果: remote 対象で操作しても、ローカルの `~/.claude.json` を読み書きするだけで、ユーザーには **silent fail** に見える (remote には何も反映されない、エラーも出ない)

`internal/mcp/manager.go` の現状:
- `Manager` struct: `userConfig string`, `projectDir string` のみ、`host` なし
- `SetHost()` 不在
- `Refresh()`, `ToggleDenied()` は local file のみ read/write
- `ssh.go` / `ssh_test.go` ファイル自体が存在しない

`internal/plugin/` も類似状況の可能性 (未確認)。

## Design Philosophy (前提)

lazyclaude の透過性原則: **リモートはローカル tmux のミラーウィンドウ**。ランタイム操作は local tmux 経由で一本化。しかし MCP/plugin の設定ファイルは **tmux ではなく SSH 越しのファイル IO** が必要な例外領域。

2 つの方針がある:
- **A. Ship-what-works**: remote 対象時は **機能を disable** して status message 表示 (`6dc0d0d` / `e1e1178` の merge + 必要な調整)
- **B. Full remote support**: SSH 越しに read/write する機能を復活 (`0630493` / `15fb347` の merge + daemon-arch 構造への適合)

## Decision Matrix

| | Option A (disable + status) | Option B (SSH read/write) |
|---|---|---|
| **ユーザー体験** | 操作できないが理由が分かる | 操作できる (本来の期待) |
| **実装工数** | 小 (guard 追加のみ) | 中～大 (ssh.go 復活 + daemon-arch 適合 + test) |
| **security risk** | ゼロ | SSH command injection (既に 15fb347 で対応済) |
| **設計原則適合** | 透過性を諦める表明 | 透過性維持 |
| **daemon-arch の制約** | 他の daemon-arch 変更と衝突しにくい | `Manager` struct に host を追加するが、daemon-arch は mcp 周りは触っていない模様 → 衝突小 |
| **Future-proof** | 後で Option B に拡張可能 | 最終形 |

### 推奨

**Phase 1: Option A (disable + status)** を先に merge して silent fail を解消、**Phase 2: Option B** を別 PR で追加。

理由:
- Phase 1 は小さく安全。即座に「反応なし」の UX 問題が解決
- Phase 2 は本来の機能だが、daemon-arch への統合で不確定要素あり (SSH connection の再利用、error handling、test)
- ユーザー即座にわかる改善がすぐに出せる

ただし、**ユーザーが「最終形 (Option B) まで一気に欲しい」と判断するなら Phase 2 を単独で実施** する選択肢もある。

## Phase 1: Disable + Status Message (this plan)

### Files to read first (worker)
- `fix-ssh-plugins` branch の commit `e1e1178` の diff
- `fix-ssh-mcp-toggle` branch の commit `6dc0d0d` の diff
- 現在の `internal/gui/app_actions.go` で MCP toggle / plugin 操作の action handler を特定

### Step 1: MCP toggle の remote guard
ファイル: `internal/gui/app_actions.go` (該当 action, 例: `MCPToggleDenied` 相当)

```go
func (a *App) MCPToggleDenied() {
    // 現在選択中の session/project の host を取得
    host := a.currentSessionHost()  // 既存ヘルパー or 新設
    if host != "" {
        a.showStatus("MCP toggle is not supported on remote sessions")
        return
    }
    // 既存の local 処理
    ...
}
```

### Step 2: Plugin 操作の remote guard
ファイル: `internal/gui/app_actions.go` の plugin 関連 action (install/uninstall/toggle)

同様に `host != ""` ガードを追加。既存の `e1e1178` commit を参考に。

### Step 3: 単体テスト
- 各 guard の unit test: mock で `host=AERO` を設定し、何も実行されず status message が立つことを assert
- 既存の local 動作が regression しないことも assert

### Step 4: Out of scope 明示
plan ファイル末尾と commit message に「Phase 1: guard only, Phase 2 (full SSH support) is a separate PR」と明記。

### Risks
- **Low**: guard 追加のみ、既存 local 動作には影響しない
- **Medium**: `a.currentSessionHost()` ヘルパーが既に存在するか要確認。なければ適切な既存 field (`pendingHost` / cursor host / sess.Host) から取る

## Phase 2 (Out of Scope for this plan, future PR)

`fix-ssh-mcp-toggle` branch の `0630493` + `15fb347` を cherry-pick または re-apply:
- `internal/mcp/ssh.go` (shellQuote + sshReadFile + sshWriteFile + splitHostPort)
- `internal/mcp/manager.go` の `host` field + `SetHost()` + `Refresh()` / `ToggleDenied()` の SSH 分岐
- `internal/plugin/` 側の同等機能 (もし必要なら別設計)
- daemon-arch の RemoteHostManager / RemoteProvider と統合できないか検討 (SSH connection 再利用、config.Paths の remote 版 etc.)

Phase 2 は plan 別ファイルで扱う。

## Verification (Phase 1)

1. `go build ./...`
2. `go vet ./...`
3. `go test -race ./internal/... ./cmd/lazyclaude/...`
4. `/go-review` → CRITICAL/HIGH ゼロ
5. `/codex --enable-review-gate` → APPROVED
6. **手動検証** (要ユーザー):
   - [ ] remote session を選択し MCP toggle キーを押す → "not supported on remote" status 表示
   - [ ] remote session を選択し plugin install/uninstall → "not supported on remote" status 表示
   - [ ] local session の MCP toggle が regression なく動く
   - [ ] local session の plugin 操作が regression なく動く

## Open Questions (要ユーザー判断)

1. **Phase 2 (full SSH support) も一気にやるか、Phase 1 だけで先に ship するか**
2. **Phase 1 で guard に使う host 判定**: `a.currentSessionHost()` 相当の既存ヘルパーを使うか、新規で追加するか
3. **status message の文言**: plugin と MCP で共通化するか個別にするか

## Files Changed (Phase 1)

| ファイル | 変更 |
|---------|------|
| `internal/gui/app_actions.go` | MCP toggle / plugin action に host guard 追加 |
| `internal/gui/app_actions_test.go` (or 該当 test file) | guard の unit test |
