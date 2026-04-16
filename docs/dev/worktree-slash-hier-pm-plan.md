# Worktree `/` Support + Hierarchical PM

## Context

現状のふたつの制約を同時に外す:

1. **Worktree branch name に `/` を含められない** — `ValidateWorktreeName`
   (`internal/session/worktree.go:31`) が `/` を拒否するため、`feat/x` のような
   git 標準的な branch 名で worktree が作れず、ユーザは `feat-x` の形を強いられる。
2. **PM セッションはプロジェクトルートから 1 つしか作れない** — `Project.PM *Session`
   (`internal/session/project.go:16`) はシングルで、`CreatePMSessionOpts` も
   `p.PM != nil` で二重生成を弾く (`manager.go:910`)。大きな機能を複数の
   サブチームに分割したいとき、サブ PM を立てられない。

両方ともユーザの作業構造に直結する UX 制約で、個別実装すると 2 度 UI/永続化/CLI を
触ることになるので **単一プラン** として扱う。ただし **内部的には直交** する変更なので
Phase を分け、Phase 2 (PM 階層) は Phase 1 (slash 対応) のマージ後に着手できる。

## Requirements Summary

| # | 項目 | 決定 |
|---|---|---|
| A1 | `/` を含む branch 名の受理 | branch name は git の refname 規則準拠で validate |
| A2 | worktree dir 名 | `/` を `-` に置換した **flattened** dir (例: `feat/x` → `.lazyclaude/worktrees/feat-x`) |
| A3 | dir 衝突 | 同一 dir 名に derive される branch が既存なら作成時エラー |
| A4 | 既存 worktree 互換 | `feat-x` のように `/` を含まない既存ディレクトリはそのまま読める |
| A5 | TUI 表示 | 一覧では branch 名 (`feat/x`) を表示、dir 名は内部実装詳細 |
| A6 | branch 起点 | 子 (worker/sub-PM) の branch は **親の branch** から派生する (作成時のみ、reparent 時に git branch は変更しない)。root PM は worktree を持ち branch が確定。sub-PM も worktree を持つ |
| B1 | PM 階層モデル | `Session.ParentID string` を追加。`ParentID == ""` が root PM、`ParentID != ""` は子 PM |
| B2 | 子 PM の working dir | 子 PM も **自身の worktree を持つ** (worker と同様)。PM が branch を持つことで、配下 worker の startPoint が確定する |
| B3 | 作成経路 | `msg create --from <parent-pm-id> --type pm` で親配下に子 PM |
| B4 | 二重生成制約の緩和 | `Project.PM *Session` の単一構造を廃止し、`ParentID` ベースに切替 |
| B5 | 既存 state.json 互換 | `parent_id` を `omitempty` で追加、Project.PM は deprecated として読み込み時に ParentID="" に変換 |
| B6 | TUI 表示 | project ノード直下に root PM、その下に worker/sub-PM とネスト表示 |
| B7 | Worker → PM の所属解決 | `ParentID` を見る (現在は Project.Sessions 一覧に混在) |
| B8 | 削除時の cascade | 親 PM を削除するとき、子 PM/Worker は切り離すか一緒に殺すか選択 — **この PR では切り離し (orphan 扱い) のみ**, cascade は次回 |

## Design

### Phase 1: Worktree branch with `/` (flattened dir)

#### Validation split

`internal/session/worktree.go` を 2 関数に分ける:

```go
// ValidateBranchName validates a git branch name. Allows `/` (namespacing).
// Rejects: empty, whitespace-only, control chars, git-invalid chars
// (".." "\\" "~" "^" ":" "?" "*" "[" "@{" leading "-" trailing ".lock"
// trailing "/" consecutive "//").
func ValidateBranchName(name string) error { ... }

// DirNameFromBranch converts a branch name to a flat directory name by
// replacing `/` with `-`. Input must pass ValidateBranchName first.
// Example: "feat/x" → "feat-x".
func DirNameFromBranch(branch string) string {
    return strings.ReplaceAll(branch, "/", "-")
}

// ValidateWorktreeName is kept for backward compat (deprecated).
// Equivalent to ValidateBranchName but additionally rejects `/`.
// Deprecated: prefer ValidateBranchName + DirNameFromBranch.
func ValidateWorktreeName(name string) error { ... }
```

#### Manager 側の変更

`worktreeOpts` の `Name` は **branch 名** を指す (slash OK) という契約に変更:

```go
// createWorktreeSession (manager.go:314)
if !opts.SkipGitAdd {
    if err := ValidateBranchName(opts.Name); err != nil {
        return nil, fmt.Errorf("invalid branch name: %w", err)
    }
}
// ...
dirName := DirNameFromBranch(opts.Name)
wtPath := opts.WtPath
if wtPath == "" {
    wtPath = WorktreePath(opts.ProjectRoot, dirName)  // dir 名を使う
}
// collision check
if existing := m.store.FindByName(opts.Name); existing != nil {
    return nil, fmt.Errorf("worktree %q already exists", opts.Name)
}
if err := CreateWorktreeWithRunner(ctx, runner, opts.ProjectRoot, wtPath, opts.Name); err != nil {
    // git worktree add -b <branch=opts.Name> <wtPath>
}
```

`Session.Name` には **branch 名** (`feat/x`) を保存。dir 名 (`feat-x`) は `Path` に
反映される。`WindowName()` は `lc-<id[:8]>` で name 依存していないので問題なし。

#### Resume / GC パス復元の整合 (codex CRITICAL #1 対応)

`Session.Name` が branch 名 (`feat/x`) で `Session.Path` が dir ベース
(`.../feat-x`) という分離により、name ↔ path の変換を全箇所で統一する必要がある。

**影響箇所の棚卸し:**

- `ResumeWorktreeOpts` (`manager.go:438`): 現状 `filepath.Base(opts.WorktreePath)`
  で `Name` を決めるため `feat-x` が入る。**修正**: resume 時は `git worktree list
  --porcelain` の branch 行から branch 名を取得し、それを `Session.Name` にする。
  fallback: branch 行がない場合 (detached HEAD) は `filepath.Base` を使う (既存挙動)。
- `SyncWithTmux` / GC: tmux window → session 解決は `Session.ID` ベース (window 名
  `lc-<id[:8]>`) なので name 非依存。問題なし。
- `FindByName(name)`: branch 名で検索。collision check でも dir 名ではなく
  branch 名をキーにする。
- `ListWorktrees` → `parseWorktreePorcelain`: `Name` を branch 行から取る
  (後述、既に設計済み)。

**不変条件**: `DirNameFromBranch(session.Name)` == `filepath.Base(session.Path)` が
常に成り立つ。テストで表明する。

**GC 済み `sessions resume --name` の扱い**:

GC fallback (state.json から消えた session) の resume では `--name` で worktree を
特定する。slash branch 導入後の `--name` は **branch 名** として解釈する:

```bash
lazyclaude sessions resume <id> --name feat/x
# → DirNameFromBranch("feat/x") = "feat-x"
# → wtPath = projectRoot/.lazyclaude/worktrees/feat-x
# → git worktree list --porcelain で branch 名を確認
# → Session.Name = "feat/x" (branch 名)
```

`ResumeSession` (`manager.go:1046`) 内で `DirNameFromBranch(name)` を使って
dir を解決する。`--name` に `feat-x` (dir 名) が直接渡された場合も対応:
`/` を含まない場合は dir 名としてそのまま使い、branch 名は
`git worktree list --porcelain` から取得する。

#### Worktree path collision

`feat/x` と `feat-x` がどちらも同じ dir 名に derive される:

- 既存 worktree (`feat-x`) があれば `m.store.FindByName("feat/x")` では見つからないが、
  `git worktree add` が既存 dir のため失敗する。
- 明示的に事前チェック: `dirName` を key に store 内をスキャンし、同じ dir に
  derive される他の session を検出 → `branch name %q collides with existing dir %q` エラー。
- 単体テスト: `feat/x` 作成後に `feat-x` 作成で衝突検出。

#### ListWorktrees / 表示

`parseWorktreePorcelain` (`worktree.go:67`) は `filepath.Base(path)` で name を
取るため `feat-x` が返る。これを **git の branch ラインから** 取るように変更:

```go
items = append(items, WorktreeInfo{
    Name:   branch,  // "feat/x" (not filepath.Base(path))
    Path:   path,
    Branch: branch,
})
```

TUI 側 (worktree chooser) は `WorktreeInfo.Name` をそのまま表示するので、`feat/x`
形式で一覧されるようになる。

#### `branchFromWorktreePath` の更新

`internal/session/role.go:111` の `branchFromWorktreePath` は
`SplitN(rel, "/", 2)[0]` で dir 名を取っている。この関数の戻り値は
**worktree レベルの prompt カスタム検索パス** を組み立てる用途
(`.lazyclaude/worktree/<branch>/.lazyclaude/prompts/...`)。

flat dir 方式では dir 名 (`feat-x`) が戻ってくるので、custom prompt パスも
`.lazyclaude/worktree/feat-x/...` のままで良い (ネスト回避の整合)。関数名と
コメントを `dirNameFromWorktreePath` に改名すると意図が明確。

#### Branch 起点: 親 branch から派生 (A6)

子の branch は親セッションの branch から切り出す。**全セッション (root PM 含む)
が worktree を持ち、branch が確定している** ことが前提:

```
projectRoot (HEAD: main)
├── .lazyclaude/worktrees/
│   ├── feat-profile/          ← root PM の worktree (branch: feat/profile)
│   ├── feat-profile-worker-a/ ← worker-a (branch: feat/profile/worker-a, 派生元: feat/profile)
│   ├── feat-profile-sub-pm/   ← sub-PM (branch: feat/profile/sub-pm, 派生元: feat/profile)
│   └── feat-profile-sub-pm-worker-b/ ← worker-b (派生元: feat/profile/sub-pm)
```

**root PM も worktree を持つ (B2 変更)**:
- 従来は root PM が projectRoot 直下で動いていたが、A6 との整合のため
  root PM も必ず worktree を持つように変更。
- `P` キーで PM 作成時も branch 名の入力を求める (profile dialog と同様のフロー)
- `CreatePMSessionOpts` は内部で `createWorktreeSession` を呼ぶ形に統一
  (Role=RolePM で worktree 作成)
- これにより全セッション (PM/Worker/sub-PM) が一貫して worktree ベースに

**`--name` は常に完全な branch refname** (leaf 名ではない):

- `--name feat/profile` → branch `feat/profile`、dir `feat-profile`
- `--name subteam-a` → branch `subteam-a`、dir `subteam-a`
- 親 branch からの自動合成 (例: 親が `feat/x` → 子が自動で `feat/x/worker-a`) は
  **行わない**。branch 名はユーザが完全に指定する。
- TUI の `w` / `P` の branch 入力欄も同様。入力された値がそのまま refname になる。
- 自動命名 (PM prompt 経由の `msg create` など) もフルの branch 名を渡す:
  例: `msg create --name feat/profile/worker-a`

**実装**: `CreateWorktreeWithRunner` に `startPoint` 引数を追加:

```go
// gitcmd.go
// startPoint が空なら HEAD から (従来互換)、非空なら指定 branch から派生
func CreateWorktreeWithRunner(ctx, runner, projectRoot, wtPath, branch, startPoint string) error {
    args := []string{"git", "worktree", "add", "-b", branch, wtPath}
    if startPoint != "" {
        args = append(args, startPoint)
    }
    out, err := runner.Run(ctx, projectRoot, args...)
    // ...
}
```

**startPoint の解決**: `createWorktreeSession` で親の branch を取得:

```go
// manager.go (createWorktreeSession 内)
var startPoint string
if opts.ParentID != "" {
    parent := m.store.FindByID(opts.ParentID)
    if parent != nil {
        // 親は必ず worktree を持つので branch が確定
        startPoint = branchForSession(ctx, runner, parent)
    }
}
```

`branchForSession` は `git -C <session.Path> rev-parse --abbrev-ref HEAD` で取得。

**ParentID="" (root 扱い) の場合**: `startPoint=""` → projectRoot の HEAD から派生。
PM なしで `w` キーを押した場合もこのパスを通る (従来互換)。

**reparent と git branch の関係**: reparent (`D` キー) は `Session.ParentID` のみ変更し、
git branch 名や派生元は変更しない。git branch tree と論理的な親子関係は **作成時のみ
一致** し、reparent 後は乖離しうる。これは意図的な制約で、git branch の rebase/rename
は破壊的操作であり自動実行しない。

#### GUI 側 (変更なし)

入力欄は既存 "Branch" フィールドをそのまま流用。プレースホルダのヒントに
"e.g. feat/xxx" を追記するだけ。

---

### Phase 2: Hierarchical PM

#### Data model

`internal/session/store.go` の `Session`:

```go
type Session struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    // ...
    Role      Role      `json:"role,omitempty"`
    Profile   string    `json:"profile,omitempty"`
    ParentID  string    `json:"parent_id,omitempty"`  // ← 新設
    // ...
}
```

`internal/session/project.go` の `Project`:

```go
// Before
type Project struct {
    // ...
    PM       *Session  `json:"pm,omitempty"`
    Sessions []Session `json:"sessions,omitempty"`
}

// After
type Project struct {
    // ...
    Sessions []Session `json:"sessions,omitempty"`  // 全 session (PM/Worker/Regular)
}
```

`PM *Session` を廃止。state.json 読み込み時に旧形式があれば「`PM` を `Sessions`
配列に転写して `ParentID=""` にする」ことでマイグレーション。`stateVersion` を
`2` → `3` に bump。

**既存 root PM の worktree 不在への対応** (v2 互換):

新設計では全 PM が worktree を持つが、v2 からの migration 直後の既存 root PM は
projectRoot 直下で動いており worktree がない。対応:

- **legacy PM モード**: `IsWorktreePath(sess.Path) == false` の PM は legacy 扱い。
  起動済み session はそのまま動作し続ける。
- **startPoint fallback**: legacy PM から子を切る際は `branchForSession` が
  `projectRoot` の HEAD を返す (worktree がないので `git -C projectRoot ...`)。
  新規 PM は必ず worktree 付きで作成されるため、legacy は自然消滅する。
- **resume**: legacy PM を resume する際も worktree 作成は行わない (projectRoot で起動)。
  明示的に worktree 付き PM を再作成するのはユーザの判断。

**新規 PM (worktree 付き) の resume**:

新規 PM は worker と同じ worktree ベースなので、resume も worker と同じ経路を使う。
現行 `ResumeSession` (`manager.go:988`) は PM を拒否しているため、この制限を外す:

- `ResumeSession` の PM 拒否ロジックを削除
- PM の resume は `ResumeWorktreeOpts` で行う (worktree が既に存在するので `SkipGitAdd=true`)
- resume 時は `BuildPMPrompt` で system prompt を再生成 (workerList を最新に)
- `sessions resume <id> --name <branch-name>` で PM も resume 可能に
- GC 済みの場合: `--name` から dir を `DirNameFromBranch` で解決し worktree path を組み立て
- **自動 migration (worktree 新設)**: ファイルシステム操作を伴うため load 時には行わない。
  将来の `lazyclaude migrate` コマンド候補として Out of Scope に記載。

#### 階層クエリ

`Store.ChildrenOf(parentID string) []Session` をヘルパとして追加:

```go
func (s *Store) ChildrenOf(parentID string) []Session {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var out []Session
    for _, p := range s.projects {
        for _, sess := range p.Sessions {
            if sess.ParentID == parentID {
                out = append(out, sess)
            }
        }
    }
    return out
}

// RootSessions returns sessions with ParentID == "" in project p.
func (p *Project) RootSessions() []Session { ... }
```

`Project.PM` を参照していた全コード (`manager.go:909` の二重生成チェック等) は
"同じ projectRoot で ParentID=="" かつ Role==PM の session が既にあるか" に差し替え。
これにより project 直下の root-level PM は依然 1 つに制限されるが、root PM の
配下に子 PM を作れる。

#### 作成 API

**CLI / daemon / MCP 3 経路で `--parent` を追加**:

```bash
lazyclaude msg create \
  --from <pm-id> \
  --name subteam-a \
  --type pm \
  [--parent <parent-pm-id>]    # 未指定なら --from を parent として採用
```

`--from` が子 PM 自身になりうるため (PM チェーン)、`--parent` で明示指定できる
ほうが意図が明確。

**`--parent` 未指定時のデフォルト (codex review HIGH #2 対応)**:
- `--type pm`: `--from` の session を parent として採用 (root PM が子 PM を作るケース)
- `--type worker`: `--from` の session を parent として採用 (PM が worker を作るケース)
- 両 type 共に `--from` の session が PM であることを `validateParentID` で検証
- `--from` が PM でない場合 (例: worker が worker を spawn): `ParentID=""` (root 扱い)

```go
// PMOpts に ParentID 追加
type PMOpts struct {
    ProjectRoot string
    Profile     string
    Options     string
    ParentID    string  // 親 PM の ID、空なら root PM
}

// WorkerOpts にも追加
type WorkerOpts struct {
    // ...
    ParentID string  // 親 PM の ID、空なら root 扱い (従来互換)
}

// WorktreeOpts にも追加 (codex review #2: w/W 経路の ParentID 配線)
type WorktreeOpts struct {
    // ...
    ParentID string  // 親 PM の ID
}

// ResumeOpts にも追加
type ResumeOpts struct {
    // ...
    ParentID string  // state.json の ParentID から復元。GC 済みの場合は後述
}
```

**GC 済み worker の resume 時の ParentID 復元** (codex review HIGH #1 対応):

GC により state.json から session が消えた場合、`lazyclaude sessions resume <id>`
は session 情報を復元する。ParentID も復元する必要がある:

- **CLI resume**: `sessions resume --parent <pm-id>` フラグを追加。
  GC 済みの場合、ユーザ (or PM) が親 PM を明示指定する。
- **PM からの resume**: PM は自分の session ID を `--parent` に渡す
  (`lazyclaude sessions resume <id> --name <name> --parent <pm-id>`)。
- **state.json に残っている場合**: `sess.ParentID` をそのまま使用。
- **未指定の場合**: `ParentID=""` (root 扱い、従来互換)。

```go
// ResumeSession の変更
func (m *Manager) ResumeSession(ctx, id, prompt, name, parentID string) { ... }
```

**ParentID の validation 共通ルール** (codex review MEDIUM #3 対応):

全 Opts 型 (PMOpts / WorkerOpts / WorktreeOpts / ResumeOpts) で同一の
validation を適用:

```go
// validateParentID は PM/Worker/Worktree/Resume の全作成経路で共通使用。
// CreatePMSessionOpts も含め全 Opts は必ずこのヘルパを通す (個別ロジック禁止)。
// projectRoot は canonical 化 (filepath.Clean + EvalSymlinks) 済みの値を渡すこと。
func (m *Manager) validateParentID(parentID, projectRoot string) error {
    if parentID == "" {
        return nil // root 扱い、OK
    }
    parent := m.store.FindByID(parentID)
    if parent == nil {
        return fmt.Errorf("parent session %q not found", parentID)
    }
    if parent.Role != RolePM {
        return fmt.Errorf("parent %q is not a PM session", parentID)
    }
    // canonical root で比較 (symlink/relative path の差を吸収)
    parentRoot := canonicalProjectRoot(parent.Path)
    if parentRoot != canonicalProjectRoot(projectRoot) {
        return fmt.Errorf("parent PM belongs to different project")
    }
    return nil
}

// canonicalProjectRoot は InferProjectRoot + filepath.EvalSymlinks + Clean で
// 一貫した比較キーを返す。EvalSymlinks 失敗時は Clean 値を返す (symlink なし環境)。
func canonicalProjectRoot(path string) string {
    root := InferProjectRoot(path)
    if resolved, err := filepath.EvalSymlinks(root); err == nil {
        return filepath.Clean(resolved)
    }
    return filepath.Clean(root)
}
```

- 親は PM であること (worker を親にはできない)
- 親と子は同じ projectRoot であること
- resume 時に stale parent (GC/削除済) → error `"parent PM %q not found"`、
  ユーザは `--parent` なし (root 扱い) で retry 可
```

#### CreatePMSessionOpts の変更

```go
func (m *Manager) CreatePMSessionOpts(ctx context.Context, opts PMOpts) (*Session, error) {
    // PM も worktree ベースに統一 (A6/B2)。
    // opts.Name が branch 名、opts.ProjectRoot が worktree の起点。
    if err := m.validateParentID(opts.ParentID, opts.ProjectRoot); err != nil {
        return nil, err
    }

    if opts.ParentID == "" {
        if existing := findRootPM(m.store, opts.ProjectRoot); existing != nil {
            return nil, fmt.Errorf("root PM already exists for %q", opts.ProjectRoot)
        }
    } else {
        parent := m.store.FindByID(opts.ParentID)
        opts.ProjectRoot = InferProjectRoot(parent.Path)
    }

    // createWorktreeSession に委譲 (Role=RolePM)
    return m.createWorktreeSession(ctx, worktreeOpts{
        Name:        opts.Name,      // branch 名
        UserPrompt:  "",             // PM は system prompt のみ
        ProjectRoot: opts.ProjectRoot,
        Role:        RolePM,
        ParentID:    opts.ParentID,
        Profile:     opts.Profile,
        ExtraFlags:  splitOptions(opts.Options),
    })
}
```

**PMOpts に Name (branch 名) を追加**:
```go
type PMOpts struct {
    Name        string  // branch 名 (PM の worktree 用)
    ProjectRoot string
    Profile     string
    Options     string
    ParentID    string
}
```

#### PM prompt への階層情報

`BuildPMPrompt` (`role.go:137`) に階層情報を追加:

```go
func BuildPMPrompt(ctx context.Context, projectRoot, sessionID, workerList, parentPMID string) string {
    // pm.md テンプレに ParentPM 行を差し込み
    // Root PM なら "You are the root PM of this project."
    // Sub-PM なら "You report to PM <parent-id>. Escalate blockers there."
}
```

ワーカーリストは root PM の場合 **直接の子 + unparented worker** を含め、
sub-PM は直接の子のみ (codex HIGH #3 参照):

```go
var workerLines []string
for _, s := range m.store.ChildrenOf(id) {
    workerLines = append(workerLines, ...)
}
if parentPMID == "" { // root PM: unparented workers も含める
    for _, s := range project.Sessions {
        if s.ParentID == "" && s.Role == RoleWorker {
            workerLines = append(workerLines, ...)
        }
    }
}
```

#### Worker prompt への親 PM ID 注入 — codex HIGH #5 対応

`BuildWorkerPrompt` に `parentPMID string` を追加。worker.md テンプレに
review_request の送り先として親 PM の session ID を明示:

```go
func BuildWorkerPrompt(ctx context.Context, worktreePath, projectRoot, sessionID, parentPMID string) string {
    // worker.md に %s で差し込み:
    // "Send review_request to PM session %s."
}
```

Worker が `msg send --from <worker-id> <parent-pm-id> "review_request: ..."` を
使えるよう、Worker prompt にはその PM の ID が注入される。
sub-PM 配下の Worker は sub-PM の ID を受け取り、root PM 配下の Worker は
root PM の ID を受け取る。

**注意**: 既存コードでは Worker の `--from` が PM に送り返す想定で動いているが、
PM session ID はスポーン時のコンテキスト (`workerList`) にしか載っていなかった。
本変更で `parentPMID` を worker.md テンプレに明示的に差し込むことで、Worker が
review_request の宛先を常に知っている状態にする。

**PM 不在プロジェクトの Worker (codex review MEDIUM #3 対応)**:

PM が存在しないプロジェクトで `ParentID=""` の worker/worktree を作成した場合
(例: 従来の `n` キーで作った普通のセッション、`w` で PM を作らず worktree だけ作る):

- `BuildWorkerPrompt` の `parentPMID` パラメタに `""` が渡る
- **Go 側で条件分岐** して review_request 行を組み立て、`fmt.Sprintf` のスロットに
  差し込む (テンプレ側は `%s` のまま):
  ```go
  var reviewLine string
  if parentPMID != "" {
      reviewLine = fmt.Sprintf("Send review_request to PM session %s.", parentPMID)
  }
  // reviewLine が空なら worker.md の該当スロットは空行になる
  ```
  現行の `fmt.Sprintf` ベース方式を維持。テンプレ言語は導入しない。
- Worker は PM なしの standalone モードで動作 (従来挙動と同一)
- PM が後から作られた場合、既存 Worker の reparent は手動
  (`msg send` で Worker に通知 or resume)

これにより PM 必須化は行わず、既存の flat な使い方を壊さない。

#### GUI ツリー表示

`internal/gui/tree.go` (または presentation 層) の木構造を再設計:

```
ProjectNode
├── SessionNode (ParentID=="")
│   ├── SessionNode (ParentID==parent.ID)
│   │   ├── ...
│   ...
```

再帰レンダリング。現状の flat な "PM + workers" 表示を、親子関係ベースのネストに変更。
expand/collapse は **PM ノード単位** で持つ (Project は既存の `Expanded` を踏襲)。

表示例:
```
▾ lazyclaude
  ▾ [PM] root-pm         running
      [W] worker-a       idle
    ▾ [PM] subteam-x     running
        [W] worker-x1    running
        [W] worker-x2    needs-input
    ▸ [PM] subteam-y     running  (collapsed)
```

#### フォーカスとキーバインド

全キー (`n`/`N`/`w`/`W`/`P`) は **カーソル位置から parent を解決** する共通ルール:

```
resolveParentFromCursor(cursorNode) → parentID:
  Project ノード  → ParentID=""  (root 直下)
  PM ノード       → ParentID=PM.ID  (その PM の子)
  Worker ノード   → ParentID=Worker.ParentID  (兄弟として同じ親の下)
```

| キー | 動作 |
|---|---|
| `n` | カーソル位置の parent + projectRoot でセッション作成 |
| `N` | pane CWD ベース (pendingHost)、parent はカーソル位置から解決 |
| `w` | カーソル位置の parent で worktree 作成 (branch 入力ダイアログ) |
| `W` | 既存 worktree 選択 → resume。parent は選択元カーソルから解決 |
| `P` | カーソル位置の parent で PM 作成 (branch 入力ダイアログ) |

`resolveParentFromCursor` を `app_actions.go` に 1 箇所集約。
`CreateSession`/`CreateSessionAtCWD`/`CreatePMSession`/`CreateWorktree` の
全エントリポイントで呼び出し、profile dialog の hidden state として `ParentID` を保持。

#### 削除 (cascade の扱い) — codex HIGH #2 対応

**orphan 昇格の矛盾解消**: 旧案では `D` で子の `ParentID = ""` にしていたが、
root PM 単一制約と衝突する (子 PM が root PM に昇格 → 既存 root PM と重複)。

**修正**: orphan 化は **親の親 (grandparent) に reparent** する:

```
削除前:           削除後 (D で pm1 削除):
root-pm           root-pm
├── pm1           ├── worker-a   (reparent: pm1→root-pm)
│   ├── worker-a  └── pm2        (reparent: pm1→root-pm)
│   └── pm2           └── worker-b
│       └── worker-b
```

- `d`: 子ありならエラー `"cannot delete pm with N children"`
- `D`: 2 つのケースに分ける:

  **Case A: sub-PM を削除 (ParentID != "")**
  子を grandparent (`parent.ParentID`) に reparent → 親を削除。
  grandparent は既存 PM なので衝突なし。

  **Case B: root PM を削除 (ParentID == "")**
  root PM は `D` では削除不可。先に全ての子を手動で削除/移動するか、
  子が 0 になってから `d` で削除する。root PM はプロジェクトの anchor であり、
  自動昇格の暗黙ロジックは複雑さに見合わないため禁止する。
  `"cannot delete root PM with children; delete children first"`

- cascade kill (`Shift+D` 等) は **本 PR スコープ外**。

#### Reparent 後の live セッション整合 — codex review MEDIUM 対応

Worker/sub-PM の prompt は起動時に `parentPMID` を焼き込む。`D` で reparent
すると state.json の `ParentID` は更新されるが、実行中の Claude Code セッション
の prompt は変わらない (起動時に固定)。

**対策**: reparent 後、影響を受ける子セッションに対して `msg send` で通知メッセージ
を送る。**送信者は新しい親 PM** (grandparent) の session ID を `--from` に使う:

```bash
lazyclaude msg send --from <new-parent-pm-id> <child-session-id> \
  "[system] Your parent PM has changed to <new-parent-pm-id>. Send future review_request to <new-parent-pm-id>."
```

- 新親 PM が送信者なのは、削除された旧 PM の ID は既に無効であり、
  `msg send --from <deleted-id>` は validation で弾かれるため
- この `msg send` は Manager 内部から自動実行 (TUI/CLI 操作の一部として)
- 子がまだ Running でない場合 (Dead/Orphan): 送信スキップ。次回 resume 時に
  新 prompt が注入される

セッション restart (kill+resume) は過剰。msg send で十分な理由:
- Claude Code は直近のメッセージを優先して行動する
- 次回 resume 時には新しい prompt が注入される (resume は BuildWorkerPrompt を再実行)
- msg send の仕組みは既に存在する (`lazyclaude msg send`)

#### Root worker の視認性 — codex HIGH #3 対応

`ParentID=""` の worker は ChildrenOf(pm) に入らないため、どの PM の視界にも
入らない。**対策**: root PM の prompt 構築時に、直接の子 + **同プロジェクト内の
unparented worker (ParentID=="" && Role!=RolePM)** を合わせてワーカーリストに含める:

```go
// BuildPMPrompt の workerList 構築
children := m.store.ChildrenOf(pmID)
if parentID == "" { // root PM の場合
    // unparented workers も含む
    for _, s := range project.Sessions {
        if s.ParentID == "" && s.Role == RoleWorker {
            children = append(children, s)
        }
    }
}
```

これにより既存の flat worker (ParentID なし) も root PM の管轄下に可視化される。
sub-PM はこの拡張を行わない (自分の直接の子のみ)。

## Phases / Worker Tree

```
Phase 1: Worktree branch with `/` (独立)
├─ P1-A: internal/session validation + path derive + resume 整合
│       * ValidateBranchName / DirNameFromBranch 新設
│       * ValidateWorktreeName を deprecated で残す
│       * createWorktreeSession を dir/branch 分離
│       * ResumeWorktreeOpts: git worktree list --porcelain から branch 名取得
│       * parseWorktreePorcelain を branch ベースに変更
│       * collision check (dir 名ベース: FindByDirName ヘルパ)
│       * CreateWorktreeWithRunner に startPoint 引数追加 (親 branch 起点)
│       * branchForSession ヘルパ (git rev-parse --abbrev-ref HEAD)
│       * 不変条件テスト: DirNameFromBranch(s.Name) == filepath.Base(s.Path)
│       * ユニットテスト (branch validation 境界、collision, list, resume, startPoint)
│
├─ P1-B: GUI / daemon / CLI 配線
│       * GUI: placeholder ヒント "feat/xxx"
│       * daemon.MsgCreateRequest: Name が branch 名という契約を追記
│       * CLI: `msg create --name feat/x` が通ることを verify
│       * VHS tape (既存 worktree_pm.tape に "feat/x" ケース追加)
│
└─ P1-C: branchFromWorktreePath → dirNameFromWorktreePath 改名 + prompts/ 検索整合

Phase 2: Hierarchical PM (Phase 1 マージ後、または並行)
├─ P2-A: Data model + migration
│       * Session.ParentID 追加
│       * Project.PM 廃止、Sessions に統合
│       * stateVersion 2 → 3, loader でマイグレーション
│       * state.json round-trip テスト (旧→新、新→新)
│
├─ P2-B: Manager API + 階層クエリ
│       * PMOpts/WorkerOpts に ParentID
│       * CreatePMSessionOpts の root/sub 分岐
│       * Store.ChildrenOf, Project.RootSessions
│       * 単体テスト: 深さ 3 のチェーン作成、親不存在エラー、同階層重複
│
├─ P2-C: Prompt + 子リスト絞り込み + Worker 宛先注入
│       * BuildPMPrompt に親情報 (案 C: 2 行差し込み)
│       * workerList: root PM は ChildrenOf + unparented workers、sub-PM は ChildrenOf のみ
│       * BuildWorkerPrompt に parentPMID 追加、worker.md に review_request 宛先差し込み
│
├─ P2-D: CLI / daemon / MCP の --parent 伝播 (end-to-end 全経路)
│       * msg create --parent フラグ
│       * sessions resume --parent フラグ (GC 済み復元用)
│       * MsgCreateRequest.ParentID (daemon + server handler_msg)
│       * WorktreeCreateRequest.ParentID, WorktreeResumeRequest.ParentID
│       * SessionResumeRequest.ParentID (daemon session-resume 経路)
│       * SessionCreator interface: CreateWorkerSession/CreatePMSession に ParentID 引数
│       * sessionCreatorAdapter: PMOpts/WorkerOpts に ParentID 載せる
│       * APIVersion bump 4 → 5 (contract 変更)
│       * Manager.validateParentID 共通 validation (全 Opts 型で呼出)
│       * 単体テスト: msg create worker --parent が end-to-end で Session.ParentID に到達
│
├─ P2-E: GUI ツリーレンダリング
│       * tree ノード構築を再帰に (presentation 層)
│       * PM 展開/折畳トグル (Space または Enter)
│       * カーソル位置ベースの作成キー動線 (P/n/w/W の挙動拡張)
│
├─ P2-F: 削除ガード + grandparent reparent + reparent 通知
│       * d: 子ありならエラー
│       * D (sub-PM): 子を grandparent に reparent → 親削除
│       * D (root PM): 禁止 (子がいる限り)
│       * reparent 後: 影響子セッションに msg send で新親 PM ID を通知
│       * 単体テスト (reparent chain, root PM 削除禁止) + VHS tape
│
└─ P2-G: PM/Worker テンプレ更新
        * pm.md, base.md に階層の読み方 + msg create --parent 追記

Phase 3: E2E + docs
├─ P3-A: VHS tapes
│       * worktree_slash.tape (Phase 1)
│       * hierarchical_pm.tape (Phase 2)
│
└─ P3-B: README / README_ja
        * "Hierarchical PM" セクション追加
        * worktree 命名のスラッシュ可の note
```

## Files to Modify

| Phase | File | Changes |
|---|---|---|
| P1-A | `internal/session/worktree.go` | `ValidateBranchName`, `DirNameFromBranch` 追加、`ValidateWorktreeName` deprecated |
| P1-A | `internal/session/worktree_test.go` | branch validation, dir derive, collision |
| P1-A | `internal/session/manager.go` | `createWorktreeSession` で dir/branch 分離、collision check |
| P1-A | `internal/session/gitcmd.go` | `CreateWorktreeWithRunner` に `startPoint string` 引数追加。`git worktree add -b <branch> <wtPath> <startPoint>` で親 branch から派生 |
| P1-A | `internal/session/manager.go` (ResumeWorktreeOpts) | resume 時に git worktree list --porcelain から branch 名取得 → Session.Name に設定 |
| P1-A | `internal/session/manager_test.go` | slash branch 名の作成・list・resume テスト、DirNameFromBranch(name)==filepath.Base(path) 不変条件テスト |
| P1-B | `internal/gui/layout.go` | worktree input placeholder に "feat/xxx" |
| P1-B | `vis_e2e_tests/tapes/worktree_slash.tape` (新) | slash branch で worktree 作成フロー |
| P1-C | `internal/session/role.go` | `branchFromWorktreePath` → `dirNameFromWorktreePath` (改名 + コメント) |
| P1-C | `internal/session/resolve_prompt_test.go` | 改名分の追従 |
| P2-A | `internal/session/store.go` | `Session.ParentID`, `stateVersion=3`, loader マイグレーション |
| P2-A | `internal/session/project.go` | `Project.PM` 廃止、`RootSessions()` 追加 |
| P2-A | `internal/session/store_test.go` | v2→v3 マイグレーションテスト, ParentID round-trip |
| P2-B | `internal/session/manager.go` | `PMOpts.ParentID`, `WorkerOpts.ParentID`, `findRootPM` ヘルパ、作成経路の分岐 |
| P2-B | `internal/session/store.go` | `ChildrenOf(parentID)`, `FindByID` (既存あれば再利用) |
| P2-C | `internal/session/role.go` | `BuildPMPrompt` に parentPMID パラメタ追加 |
| P2-C | `prompts/pm.md` | 階層情報テンプレ (案 C 2 行) + root PM の unparented worker 含有ロジック |
| P2-C | `internal/session/role.go` (BuildWorkerPrompt) | `parentPMID` パラメタ追加、worker.md に review_request 宛先を差し込み |
| P2-C | `prompts/worker.md` | 親 PM session ID の差し込みスロット追加 |
| P2-D | `internal/daemon/api.go` | `MsgCreateRequest.ParentID`, `WorktreeCreateRequest.ParentID`, `WorktreeResumeRequest.ParentID`, `SessionResumeRequest.ParentID`, `APIVersion 4 → 5` |
| P2-D | `internal/daemon/server.go` | `handleMsgCreate`, `handleWorktreeCreate`, `handleWorktreeResume`, `handleSessionResume` で ParentID 伝播 |
| P2-D | `internal/daemon/server_test.go`, `connection_impl_test.go` | バージョン一致テスト更新 |
| P2-D | `internal/server/handler_msg.go` | `SessionCreator` interface: `CreateWorkerSession` / `CreatePMSession` に ParentID 追加。`msgCreateRequest` に `ParentID` フィールド追加。handler で type="worker"/"pm" 両方に ParentID 伝播 |
| P2-D | `cmd/lazyclaude/msg.go` | `--parent` フラグ追加、request に載せる |
| P2-D | `cmd/lazyclaude/sessions.go` | `resume` に `--parent` フラグ追加 |
| P2-D | `cmd/lazyclaude/root.go` | `sessionCreatorAdapter`: `CreateWorkerSession` / `CreatePMSession` に ParentID 引数追加、`WorkerOpts{ParentID}` / `PMOpts{ParentID}` に伝播 |
| P2-D | `internal/gui/app.go` | `SessionProvider.ResumeWorktree` / `ResumeWorktreeWithOpts` に ParentID 追加 |
| P2-D | `cmd/lazyclaude/gui_adapter.go` | `guiCompositeAdapter.ResumeWorktree*` で ParentID を `ResumeOpts` に伝播 |
| P2-D | `cmd/lazyclaude/session_command.go` | `SessionCommandService.ResumeWorktree*` で ParentID 伝播 |
| P2-D | `internal/daemon/composite_provider.go` | `CompositeProvider.ResumeWorktree` に ParentID 追加 |
| P2-D | `internal/server/client.go` | MCP client の `/msg/create` payload + resume request に ParentID 追加 |
| P2-D | `internal/daemon/remote_provider.go` | create/resume request 組み立てに ParentID フィールド追加 |
| P2-D | `cmd/lazyclaude/local_provider.go` | local provider の create/resume シグネチャに ParentID 追加、Manager Opts に伝播 |
| P2-D | `internal/session/manager.go` | `ResumeSession` の PM 拒否ロジック削除、PM resume を `ResumeWorktreeOpts` に委譲 |
| P2-E | `internal/gui/tree.go` (または presentation 層) | 再帰ノード構築、ParentID ベース |
| P2-E | `internal/gui/app.go` | `SessionProvider`: `CreatePMSession` / `CreatePMSessionWithOpts` に `name string` (branch 名) 引数追加。`CreatePMSession(name, projectRoot)` |
| P2-E | `internal/gui/layout.go` | `P` キーの dialog に branch 名入力欄追加、ネスト表示、PM 展開/折畳 |
| P2-E | `internal/gui/keybindings.go`, `app_actions.go` | カーソル位置に応じた `P`/`n`/`w`/`W` 動作 |
| P2-E | `internal/daemon/composite_provider.go`, `remote_provider.go` | `CreatePMSession` の name 引数追加に追従 |
| P2-E | `cmd/lazyclaude/root.go` | `guiCompositeAdapter.CreatePMSession` に name 引数追加 |
| P2-F | `internal/session/manager.go` | `DeleteSession` で子ありエラー、`OrphanAndDelete` 追加 |
| P2-F | `internal/gui/keybindings.go` | `d` ガード、`D` で orphan 昇格 |
| P2-G | `prompts/pm.md`, `prompts/base.md` | 階層読解 + --parent 注記 |
| P3-A | `vis_e2e_tests/tapes/hierarchical_pm.tape` (新), `entrypoint.sh` | 親 PM → 子 PM → Worker の作成フロー |
| P3-B | `README.md`, `README_ja.md` | Features に "Hierarchical PM" 追加 |

## Codex / security pre-checks

1. **git refname injection** — `ValidateBranchName` は git の [check-ref-format][ref]
   ルールに準拠。特に:
   - `..` / `@{` の substring 禁止
   - `/` の連続 (`//`) 禁止、末尾 `/` 禁止
   - 制御文字 (0x00-0x1F, 0x7F) 禁止
   - `.` で始まる / `.lock` で終わる禁止
   - ASCII 空白禁止 (cobra が受ける前提なので shell-meta は別途確認)

   `shell.Quote` が git の arg で効くため injection はそこで切られるが、validation
   自体は refname 規則で切る。

2. **Dir 衝突の TOCTOU** — `FindByName` と `git worktree add` の間に別プロセスが
   同名 dir を作るレース。`git worktree add` は既存 dir で失敗するので fatal
   ではないが、エラーメッセージをハンドリングして "branch derived to existing
   dir %q" と親切化。

3. **Migration の事故防止** — v2 → v3 マイグレーションは **pure function** として
   実装し、上書きは `StoreAtomic()` 経由の tmp file + rename のみ。失敗時は
   旧 state.json が無傷で残ることを単体テストで担保。`stateFile.Version` の
   fallthrough 値 (0, 1) は v2 扱い。

4. **APIVersion bump + remote 互換** — Session Profile 機能で 3→4 したのと同じ流儀。
   `--parent` がないリモート daemon と通信すると connect 時点で弾かれる (ランタイム
   失敗より早期検知)。Remote daemon binary は同一ビルドなので state.json v3
   マイグレーションは自動適用。daemon API `/sessions` は flat list で Project 構造体
   を直接 expose しないため、`ParentID` フィールド追加だけで API 面は充足。

5. **Concurrent parent lookup** — `CreatePMSessionOpts` の `parent := m.store.FindByID`
   と `sess.ParentID = opts.ParentID` の間で親削除が入った race は、
   `m.mu.Lock` 区間内で両方実行することで防ぐ。

6. **Cycle detection** — theoretically `ParentID` チェーンに循環が入ると無限ループ。
   本 PR では作成時に `findRootPM` が parent chain を辿るので、深さ制限 (例: 16) を
   入れて cycle/bomb を防ぐ。循環自体は新 session 作成時の parent が既存 session
   である限り発生しないはずだが、defence in depth。

7. **collision test**: `feat/x` 作成後に `feat-x` を作る、逆順でも同じ衝突。
   `ValidateBranchName` は通るが `createWorktreeSession` 内で検出してエラー。

[ref]: https://git-scm.com/docs/git-check-ref-format

## Verification

### Static

```bash
go build ./...
go vet ./...
go test -race ./internal/session/...
go test -race ./internal/daemon/...
go test -cover ./internal/session/...
```

### Manual (Phase 1)

```bash
# slash branch で worktree 作成
# TUI: w → "feat/slash-test" → Enter
# → .lazyclaude/worktrees/feat-slash-test/ が作られる
# → `git -C .lazyclaude/worktrees/feat-slash-test branch --show-current` が "feat/slash-test"
# → 一覧 (W) に "feat/slash-test" と表示される
```

### Manual (Phase 2)

```bash
# 階層 PM 作成
lazyclaude msg create --from <pm0-id> --name subteam-a --type pm  # 子 PM
lazyclaude msg create --from <pm0-id> --name subteam-a --type pm --parent <pm0-id>  # 同等
lazyclaude msg create --from <subteam-a-id> --name worker-a1 --type worker --prompt "..."

# TUI で階層表示:
#   lazyclaude
#     [PM] pm0
#       [PM] subteam-a
#         [W] worker-a1

# 削除ガード
# cursor on subteam-a, press d → error (2 children)
# press D → worker-a1 が pm0 直下に昇格し subteam-a 削除
```

### Regression

```bash
make test-vhs TAPE=worktree_pm     # 既存のフラット PM ケース
make test-vhs TAPE=worktree_slash  # Phase 1
make test-vhs TAPE=hierarchical_pm # Phase 2
make test                          # 全ユニット + race
```

## Out of Scope

- **Cascade kill** (親 PM 削除で子も全部殺す): 次 PR。今は orphan 昇格のみ。
- **PM → worker 昇格 / Worker → PM 昇格**: role 切替は非対応。
- **Cross-project PM hierarchy**: 親と子は同じ projectRoot に限定。
- **Remote PM hierarchy**: ローカルのみ実装。**ただし remote daemon binary も同一
  ビルドから出力されるため、state.json v3 マイグレーションは remote 側にも自動適用
  される (同一バイナリ配布)** (codex HIGH #4 対応)。daemon API `/sessions` は flat
  session list を返しており Project 構造体を直接 expose しないため、API 面の追加
  変更は `ParentID` フィールドの追加のみ。remote 側の PM 階層 UI 操作 (sub-PM 作成等)
  は Phase 4 以降。
- **深さ制限 UI**: 深さ 16 の絶対上限のみ。UI で nested depth を制限する機能は無し。
- **Worktree 命名の任意ディレクトリ指定**: "branch と dir を別フィールドに" という
  UX option 3 は採用せず (Option 2 の flatten で十分)。
- **`lazyclaude migrate`**: legacy PM (worktree なし) を自動で worktree 付きに
  変換するコマンド。load 時にファイルシステム操作を行うのは危険なため、明示的な
  コマンドとして将来実装候補。
- **Reparent 時の git branch rebase/rename**: `D` で ParentID は変わるが git branch は
  変更しない。branch tree と論理親子は作成時のみ一致する設計上の制約。
