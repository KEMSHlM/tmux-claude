# コマンドルーティング定義

各キーコマンドの path / host 解決フローと期待動作。

## サイドバー構造

```
▼ local: lazyclaude              ← ProjectNode (Host="")
  lazyclaude (PM)                 ← SessionNode (Host="")
▼ remote: test-app                ← ProjectNode (Host="AERO")
  [PM] pm                         ← SessionNode (Host="AERO")
  [W] feat-test                   ← SessionNode (Host="AERO")
```

## 入力値の取得

### currentProjectRoot()

```
Focus が ProjectNode:
  → project.Path を返す
    ローカル: "/Users/kenshin/.local/share/tmux/plugins/lazyclaude"
    リモート: "/root/app/test-app"

Focus が SessionNode:
  → 親プロジェクトの Path を返す (projectPathByID)
  → 見つからなければ InferProjectRoot(session.Path)

Focus なし:
  → filepath.Abs(".") のプロジェクトルート (ローカル CWD)
```

### resolveHost()

```
1. cachedHost (Sessions() で毎 layout cycle 更新)
   → 空でなければ返す
2. pendingHost (SSH ペイン検出 or connect ダイアログ後)
   → フォールバック
```

### resolveRemotePath(path, host)

```
host == "":
  → path をそのまま返す (ローカル)

path == localProjectRoot or path == ".":
  → queryRemoteCWD(host) (初回接続、ローカルパスはリモートで無意味)
  → 失敗時 "." にフォールバック

path が上記以外 (既存リモートプロジェクトから取得されたパス):
  → path をそのまま返す
```

## コマンド別ルーティング

### n (CreateSession) — プロジェクトルートに新規セッション

```
path = currentProjectRoot()
host = resolveHost()
→ SessionCommandService.Create({host, path})
   ├─ host == ""
   │  └─ localMgr.Create(path)
   └─ host != ""
      └─ completeRemoteCreate (goroutine)
         ├─ ensureConnected(host)
         ├─ remotePath = resolveRemotePath(path, host)
         ├─ remoteAPI.Create(remotePath)
         └─ mirrorMgr.CreateMirror(host, remotePath, resp)
```

**期待動作:**
- Focus on local project → そのプロジェクトルートで作成
- Focus on remote project → そのリモートプロジェクトルートで作成
- Focus なし + pendingHost あり → リモート daemon CWD で作成

### N (CreateSessionAtCWD) — pane の CWD に新規セッション

```
path = "."
host = resolveHost()
→ SessionCommandService.Create({host, "."})
   ├─ host == ""
   │  └─ localMgr.Create(".")
   └─ host != ""
      └─ completeRemoteCreate
         ├─ ensureConnected(host)
         ├─ remotePath = resolveRemotePath(".", host) → queryRemoteCWD
         ├─ remoteAPI.Create(remotePath)
         └─ mirrorMgr.CreateMirror(host, remotePath, resp)
```

**期待動作:**
- ローカル → ローカルの pane CWD で作成
- リモート → リモートの daemon CWD で作成
- 親子関係はパスから自動推定

### w (CreateWorktree) — プロジェクトに worktree 作成

```
path = currentProjectRoot()
host = resolveHost()
→ SessionCommandService.CreateWorktree({host, path}, name, prompt)
   ├─ prepareRemote → ensureConnected + resolveRemotePath
   └─ cp.CreateWorktree(name, prompt, path, host)
      ├─ host == "" → localProvider.CreateWorktree
      └─ host != "" → RemoteProvider.CreateWorktree → daemon API
         └─ postCreate hook → mirrorMgr.CreateMirror
```

**期待動作:**
- Focus on project → そのプロジェクトに worktree 作成
- worktree は path/.lazyclaude/worktrees/{name} に配置

### W (SelectWorktree) — プロジェクトの worktree 一覧

```
path = currentProjectRoot()
host = resolveHost()
→ SessionCommandService.ListWorktrees({host, path})
   ├─ prepareRemote → ensureConnected + resolveRemotePath
   └─ cp.ListWorktrees(path, host)
```

**期待動作:**
- Focus on project → そのプロジェクトの worktree を一覧表示
- 選択 → ResumeWorktree で復帰

### P (CreatePMSession) — プロジェクトに PM セッション

```
path = currentProjectRoot()
host = resolveHost()
→ SessionCommandService.CreatePMSession({host, path})
   ├─ prepareRemote → ensureConnected + resolveRemotePath
   └─ cp.CreatePMSession(path, host)
      └─ postCreate hook → mirrorMgr.CreateMirror
```

**期待動作:**
- Focus on project → そのプロジェクトに [PM] セッション作成
- プロジェクトに PM は1つだけ

### d (Delete) — セッション削除

```
id = currentSession().ID
→ SessionCommandService.Delete(id)
   ├─ sess.Host == ""
   │  └─ cp.Delete(id) → localMgr.Delete → KillWindow(lc-xxxx)
   └─ sess.Host != ""
      ├─ remoteAPI.Delete(id) (best-effort, 3秒 timeout)
      └─ mirrorMgr.DeleteMirror(id) → KillWindow(rm-xxxx) + Store.Remove
```

**期待動作:**
- ローカル session → tmux window 削除 + store 削除
- リモート session → daemon API 削除 + mirror 削除 + store 削除
- daemon 不達時もローカル cleanup は実行

### R (Rename) — セッション名変更

```
id = currentSession().ID
newName = dialog input
→ SessionCommandService.Rename(id, newName)
   ├─ sess.Host == ""
   │  └─ cp.Rename → localMgr.Rename → tmux RenameWindow + Store update
   └─ sess.Host != ""
      ├─ remoteAPI.Rename(id, newName)
      └─ Store.UpdateSession + Save
```

### g (LaunchLazygit) — lazygit 起動

```
path = currentProjectRoot()
host = resolveHost()
→ SessionCommandService.LaunchLazygit({host, path})
   ├─ prepareRemote → ensureConnected + resolveRemotePath
   └─ cp.LaunchLazygit(path, host)
      ├─ host == "" → localProvider → exec lazygit
      └─ host != "" → RemoteProvider → ssh -t host lazygit
```

### a (Attach) — tmux attach

```
id = currentSession().ID
→ cp.AttachSession(id) → providerForSession → AttachSession
```

### Enter (Fullscreen) — gocui 内 fullscreen

```
id = currentSession().ID
→ capture-pane(mirror window) → gocui 描画
→ send-keys(mirror window) → SSH 経由でリモートに転送
```
