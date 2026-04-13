# Session Resume Command

## Context

Worker セッション (`lazyclaude msg create`) が終了すると StatusDead になり、PM から `msg send` しても "recipient session not found" エラーになる。セッション ID を維持したまま再起動 (respawn) する手段がない。

`lazyclaude session resume <id>` コマンドを追加し、Dead/Orphan セッションを同じ ID で再起動する。Claude Code の `--session-id` により会話履歴も引き継がれる。ローカル・リモート両対応。

## Phase 1: Core + API (このタスク)

Manager, daemon API, MCP server, CLI コマンド。

## Phase 2: GUI 統合 (後続タスク)

TUI キーバインド (`r` key)、GUI adapter。

---

## Design

### Core: `Manager.ResumeSession(ctx, id, prompt)`

`internal/session/manager.go` に追加。

1. `m.mu.Lock()` で排他制御
2. `store.FindByID(id)` でセッション取得
3. Status が Dead/Orphan でなければエラー (Detached は将来対応)
4. `store.FindProjectForSession(id)` で projectRoot を取得
5. 旧 tmux window を kill (best-effort, Orphan は skip)
6. `store.Remove(id)` で旧エントリ削除
7. Role に応じて再起動 (同じ ID を再利用):
   - **RoleWorker**: `launchWorktreeSession` に sessionID パラメータを渡す
   - **RolePM**: PM 用プロンプトを再構築して `launchWorktreeSession` に sessionID を渡す
   - **RoleNone**: `buildClaudeCommand` + `launchSession()` (旧 ID を Session.ID にセット)

### ID 注入方式 (codex レビュー反映)

`worktreeOpts` に OverrideID を追加するのではなく、`launchWorktreeSession` のシグネチャに `sessionID string` パラメータを追加する。空文字なら `uuid.New()` (既存動作維持)。

```go
func (m *Manager) launchWorktreeSession(ctx context.Context, name, wtPath, userPrompt, projectRoot string, role Role, sessionID string) (*Session, error) {
    if sessionID == "" {
        sessionID = uuid.New().String()
    }
    // ... rest uses sessionID instead of uuid.New()
}
```

既存の呼び出し元 (`createWorktreeSession`) は `""` を渡す。

### GC Safety (codex レビュー反映: CRITICAL)

**問題**: `GC.collect()` が `Sessions()` でスナップショットを取った後、`ResumeSession` が同じ ID で新セッションを作成。GC が stale スナップショットに基づき `Delete(id)` を呼ぶと、Running 状態の新セッションが削除される。

**修正**: `Manager.DeleteIfStale(ctx, id)` を追加。GC 専用の削除メソッドで、ロック内で再度ステータスを確認し、Dead/Orphan のままの場合のみ削除する。

```go
// DeleteIfStale is used by GC. Only deletes if session is still Dead/Orphan.
func (m *Manager) DeleteIfStale(ctx context.Context, id string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    sess := m.store.FindByID(id)
    if sess == nil {
        return nil // already gone
    }
    if sess.Status != StatusDead && sess.Status != StatusOrphan {
        return nil // resumed or otherwise no longer stale
    }
    // ... existing kill + remove logic from Delete()
}
```

`gc.go` の `collect()` を `gc.svc.Delete` → `gc.svc.DeleteIfStale` に変更。

### Edge Cases (codex レビュー反映)

1. **Worktree 削除済み**: `ResumeSession` で Worker resume 前に `os.Stat(old.Path)` チェック。存在しなければ "worktree directory not found: <path>" エラー
2. **PM 重複**: `ResumeSession` で PM resume 前に同プロジェクト内の既存 PM をチェック。既に Running PM があればエラー
3. **名前衝突**: `ResumeSession` は同じ名前で再作成するため衝突しない (Remove → Add で入れ替え)

### Layer Stack

```
CLI: lazyclaude session resume <id> [--prompt "..."]
  └→ server.Client.ResumeSession()          # HTTP POST /session/{id}/resume
       └→ server.Handler (handler_msg.go)
            └→ SessionCreator.ResumeSession()
                 └→ sessionCreatorAdapter (root.go)
                      └→ session.Manager.ResumeSession()

Remote:
  └→ SessionCommandService.ResumeSession()
       └→ daemon.CompositeProvider → RemoteProvider → daemon API
            └→ remote Manager.ResumeSession()
       └→ MirrorManager: delete old mirror → create new mirror
```

---

## Steps

### Step 1: GC safety - `DeleteIfStale` (`internal/session/manager.go`, `internal/session/gc.go`)

- `Manager.DeleteIfStale(ctx, id)`: ロック内で status 再確認 → Dead/Orphan のみ削除
- `gc.go`: `gc.svc.Delete` → `gc.svc.DeleteIfStale` に変更
- `GCSvc` interface に `DeleteIfStale` 追加 (`internal/session/gc.go` の interface 定義)
- テスト: GC が Running セッションを削除しないことを確認

### Step 2: `launchWorktreeSession` に sessionID パラメータ追加 (`internal/session/manager.go`)

- `launchWorktreeSession(ctx, name, wtPath, userPrompt, projectRoot string, role Role, sessionID string)`
- `sessionID == ""` なら `uuid.New()` (後方互換)
- `createWorktreeSession` からの呼び出しは `""` を渡す
- 既存テスト変更なし

### Step 3: `Manager.ResumeSession()` 実装 (`internal/session/manager.go`)

- `ResumeSession(ctx context.Context, id, prompt string) (*Session, error)`
- Role ごとの分岐:
  - Worker: `launchWorktreeSession(ctx, old.Name, old.Path, prompt, projectRoot, RoleWorker, id)`
  - PM: `launchWorktreeSession(ctx, old.Name, old.Path, "", projectRoot, RolePM, id)`
  - Plain: `Session{ID: id, ...}` を構築 → `buildClaudeCommand` → `launchSession`
- Edge case チェック (worktree 存在確認、PM 重複チェック)
- テスト: Dead Worker resume, Dead PM resume, Dead Plain resume, Running 拒否, Orphan resume

### Step 4: Daemon API (`internal/daemon/`)

- `api.go`: `SessionResumeRequest{Prompt string}` 追加
- `server.go`: `POST /session/{id}/resume` ハンドラ追加
- `http_client.go`: `ResumeSession` 実装
- `client.go`: `ClientAPI` interface に追加
- `composite_provider.go`: `SessionActioner` に `ResumeSession(id, prompt string) error` 追加
- `remote_provider.go`: `ResumeSession` 実装 (daemon HTTP client 経由)
- `cmd/lazyclaude/local_provider.go`: `ResumeSession` 実装 (Manager 委譲)
- テスト: daemon endpoint テスト

### Step 5: MCP Server endpoint (`internal/server/`)

- `handler_msg.go`: `SessionCreator` interface に `ResumeSession(ctx, id, prompt) (*SessionCreateResult, error)` 追加
- `handleSessionResume` ハンドラ実装
- `server.go`: route 登録 `POST /session/{id}/resume`
- `client.go`: `Client.ResumeSession(ctx, id, prompt)` 追加
- `cmd/lazyclaude/root.go`: `sessionCreatorAdapter.ResumeSession()` 実装

### Step 6: SessionCommandService (`cmd/lazyclaude/session_command.go`)

- `remoteSessionAPI` に `ResumeSession(id, prompt string) error` 追加
- `SessionCommandService.ResumeSession(id, prompt string) error`:
  - ローカル: `s.localMgr.ResumeSession(ctx, id, prompt)`
  - リモート: `rp.ResumeSession(id, prompt)` → `s.mirrors.DeleteMirror(id)` → `s.mirrors.CreateMirror(...)` でミラー再作成

### Step 7: CLI コマンド (`cmd/lazyclaude/`)

- `sessions.go`: `lazyclaude session resume <id-or-name>` サブコマンド追加
- `--prompt` フラグ対応
- 名前→ID 解決
- `root.go`: コマンド登録

### Step 8: テスト

- `internal/session/manager_test.go`: ResumeSession (Worker/PM/Plain各Role, Dead/Orphan/Running各Status)
- `internal/session/gc_test.go`: DeleteIfStale テスト、GC と resume の競合テスト
- `internal/daemon/server_test.go`: daemon resume endpoint
- `internal/server/handler_msg_test.go`: MCP server resume endpoint
- keyhandler/dispatcher テスト: mock に ResumeSession stub (Phase 2 準備)

## Files to Modify

| File | Changes |
|------|---------|
| `internal/session/manager.go` | `launchWorktreeSession` sessionID パラメータ, `ResumeSession()`, `DeleteIfStale()` |
| `internal/session/gc.go` | `GCSvc` interface に `DeleteIfStale` 追加, `collect` 変更 |
| `internal/daemon/api.go` | `SessionResumeRequest` |
| `internal/daemon/server.go` | `handleSessionResume` handler |
| `internal/daemon/http_client.go` | `ResumeSession` client method |
| `internal/daemon/client.go` | `ClientAPI` interface 拡張 |
| `internal/daemon/composite_provider.go` | `SessionActioner` に `ResumeSession` 追加 |
| `internal/daemon/remote_provider.go` | `ResumeSession` |
| `cmd/lazyclaude/local_provider.go` | `ResumeSession` |
| `internal/server/handler_msg.go` | `SessionCreator` interface, handler |
| `internal/server/server.go` | route 登録 |
| `internal/server/client.go` | `Client.ResumeSession()` |
| `cmd/lazyclaude/session_command.go` | `SessionCommandService.ResumeSession()`, `remoteSessionAPI` |
| `cmd/lazyclaude/root.go` | `sessionCreatorAdapter.ResumeSession()`, CLI 登録 |
| `cmd/lazyclaude/sessions.go` | CLI `session resume` subcommand |

## Verification

1. `go build ./...` && `go vet ./...`
2. `go test -race ./...`
3. ローカルテスト: Worker spawn → exit → `lazyclaude session resume <id>` → 復帰 → `lazyclaude msg send` で PM から通信成功
4. リモートテスト: SSH worker 終了 → resume → ミラー再作成確認
