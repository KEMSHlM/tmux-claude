# Implementation Plan: Marketplace / Plugin Management

## Overview

lazyclaude TUI にマーケットプレイス/プラグイン管理機能を追加する。
Claude Code のネイティブプラグインシステム (`~/.claude/plugins/`) と連携し、
`claude plugins` CLI の `--json` 出力をパースして gocui TUI でブラウズ・インストール・有効/無効切替を実現する。

## Background

- Claude Code は公式プラグインシステムを持つ (`~/.claude/plugins/`)
- NikiforovAll/lazyclaude (Python) は同様の機能を TUI で提供済み
- このプロジェクト (KEMSHlM/lazyclaude) は Go + gocui + tmux + MCP サーバーという独自のアーキテクチャを持つ
- main ブランチは #14 で Store v2 (Project hierarchy) に移行済み

## Requirements

1. インストール済みプラグインの一覧表示
2. プラグインのプレビュー (README 等の内容確認)
3. マーケットプレイスからのプラグイン検索・インストール
4. プラグインの有効/無効切替
5. プラグインのアンインストール
6. スコープ管理 (user / project / local)

## Data Sources: 実際のファイル構造

### CLI (`claude plugins --json`) の出力 (一次情報源)

```bash
# インストール済み一覧
claude plugins list --json
# → [{id, version, scope, enabled, installPath, installedAt, lastUpdated}]

# インストール済み + マーケットプレイスの全プラグイン
claude plugins list --available --json
# → {installed: [...], available: [...]}
# available: [{pluginId, name, description, marketplaceName, source, installCount}]

# マーケットプレイス一覧
claude plugins marketplace list --json
# → [{name, source, repo, installLocation}]
```

### ファイルシステム (参照のみ、CLI が一次情報源)

```
~/.claude/plugins/
├── installed_plugins.json    # version 2, plugins は "id@marketplace" → [{scope, installPath, version, ...}]
├── known_marketplaces.json   # marketplace名 → {source, installLocation, lastUpdated}
├── config.json               # {repositories: {}}
├── blocklist.json            # {fetchedAt, plugins: [{plugin, reason, text}]}
├── cache/{marketplace}/{plugin}/{version}/  # インストール済みプラグインファイル
├── data/{plugin-id}/         # プラグイン永続データ
└── marketplaces/{marketplace}/  # マーケットプレイスのローカルクローン
    ├── .claude-plugin/marketplace.json
    └── plugins/{name}/       # 各プラグインのソース
```

### CLI サブコマンド一覧

| コマンド | 用途 |
|----------|------|
| `claude plugins list [--json] [--available]` | プラグイン一覧 |
| `claude plugins install <plugin> [-s scope]` | インストール |
| `claude plugins uninstall <plugin>` | アンインストール |
| `claude plugins enable <plugin> [-s scope]` | 有効化 |
| `claude plugins disable [plugin] [-a] [-s scope]` | 無効化 |
| `claude plugins update <plugin>` | アップデート |
| `claude plugins validate <path>` | マニフェスト検証 |
| `claude plugins marketplace list [--json]` | マーケットプレイス一覧 |
| `claude plugins marketplace add <source>` | マーケットプレイス追加 |
| `claude plugins marketplace remove <name>` | マーケットプレイス削除 |
| `claude plugins marketplace update [name]` | マーケットプレイス更新 |

### 設計方針

1. **CLI `--json` 出力を一次情報源とする**: ファイルを直接パースしない。CLI の出力フォーマットが安定した API として機能する
2. **全ての変更操作は CLI に委譲**: install/uninstall/enable/disable は `claude plugins` コマンド経由
3. **マーケットプレイスソースの管理も CLI に委譲**: `claude plugins marketplace add/remove` で管理。lazyclaude 側で別途永続化しない
4. **TUI はビュー & トリガー**: gocui でデータ表示とユーザー操作の受付

---

## Phase 1: プラグインモデルと CLI ラッパー

### 目標
`claude plugins` CLI の JSON 出力をパースするモデルと、CLI 呼び出しをラップするインターフェースを実装する。

### 新規ファイル

#### `internal/plugin/model.go`
```go
package plugin

// InstalledPlugin represents a plugin from `claude plugins list --json`.
type InstalledPlugin struct {
    ID          string `json:"id"`          // e.g. "lua-lsp@claude-plugins-official"
    Version     string `json:"version"`
    Scope       string `json:"scope"`       // "user", "project", "local"
    Enabled     bool   `json:"enabled"`
    InstallPath string `json:"installPath"`
    InstalledAt string `json:"installedAt"` // ISO 8601
    LastUpdated string `json:"lastUpdated"` // ISO 8601
}

// AvailablePlugin represents a marketplace plugin from `claude plugins list --available --json`.
type AvailablePlugin struct {
    PluginID        string `json:"pluginId"`        // e.g. "code-review@claude-plugins-official"
    Name            string `json:"name"`
    Description     string `json:"description"`
    MarketplaceName string `json:"marketplaceName"`
    Source          Source `json:"source"`
    InstallCount    int    `json:"installCount"`
}

// Source describes the origin of a plugin.
type Source struct {
    Source string `json:"source"` // "github", "url", "path", "npm"
    Repo   string `json:"repo,omitempty"`
    URL    string `json:"url,omitempty"`
    Ref    string `json:"ref,omitempty"`
    SHA    string `json:"sha,omitempty"`
}

// ListResult is the output of `claude plugins list --available --json`.
type ListResult struct {
    Installed []InstalledPlugin `json:"installed"`
    Available []AvailablePlugin `json:"available"`
}

// MarketplaceInfo represents a marketplace from `claude plugins marketplace list --json`.
type MarketplaceInfo struct {
    Name            string `json:"name"`
    Source          string `json:"source"` // "github"
    Repo            string `json:"repo"`
    InstallLocation string `json:"installLocation"`
}

// PluginName extracts the plugin name (before @) from a full plugin ID.
func PluginName(id string) string {
    for i, c := range id {
        if c == '@' {
            return id[:i]
        }
    }
    return id
}

// MarketplaceName extracts the marketplace name (after @) from a full plugin ID.
func MarketplaceName(id string) string {
    for i, c := range id {
        if c == '@' {
            return id[i+1:]
        }
    }
    return ""
}
```

#### `internal/plugin/cli.go`
```go
package plugin

import "context"

// CLI defines the interface for executing claude plugins commands.
type CLI interface {
    ListInstalled(ctx context.Context) ([]InstalledPlugin, error)
    ListAll(ctx context.Context) (*ListResult, error)
    ListMarketplaces(ctx context.Context) ([]MarketplaceInfo, error)
    Install(ctx context.Context, plugin string, scope string) error
    Uninstall(ctx context.Context, plugin string) error
    Enable(ctx context.Context, plugin string) error
    Disable(ctx context.Context, plugin string) error
    Update(ctx context.Context, plugin string) error
    MarketplaceAdd(ctx context.Context, source string) error
    MarketplaceRemove(ctx context.Context, name string) error
    MarketplaceUpdate(ctx context.Context, name string) error
}

// ExecCLI implements CLI by spawning `claude plugins` subprocesses.
type ExecCLI struct {
    claudePath string // path to claude binary (default: "claude")
}
```

### テスト
- `internal/plugin/model_test.go`: PluginName/MarketplaceName のユニットテスト、JSON パースの検証
- `internal/plugin/cli_test.go`: モック CLI 出力のパーステスト (実際の JSON スナップショットを fixtures に)

---

## Phase 2: プラグインマネージャー

### 目標
GUI と CLI の間のビジネスロジック層。キャッシュ、状態管理、エラーハンドリングを担当。

#### `internal/plugin/manager.go`
```go
package plugin

import (
    "context"
    "log/slog"
    "sync"
)

// Manager coordinates plugin operations between the TUI and CLI.
type Manager struct {
    cli   CLI
    log   *slog.Logger
    mu    sync.RWMutex

    // Cached state (refreshed on demand)
    installed []InstalledPlugin
    available []AvailablePlugin
    markets   []MarketplaceInfo
}

// Refresh reloads all plugin data from the CLI.
func (m *Manager) Refresh(ctx context.Context) error { ... }

// Installed returns the cached installed plugins.
func (m *Manager) Installed() []InstalledPlugin { ... }

// Available returns the cached available plugins.
func (m *Manager) Available() []AvailablePlugin { ... }

// Marketplaces returns the cached marketplace list.
func (m *Manager) Marketplaces() []MarketplaceInfo { ... }

// Install installs a plugin and refreshes the cache.
func (m *Manager) Install(ctx context.Context, pluginID string, scope string) error { ... }

// Uninstall removes a plugin and refreshes the cache.
func (m *Manager) Uninstall(ctx context.Context, pluginID string) error { ... }

// ToggleEnabled enables a disabled plugin or disables an enabled one.
func (m *Manager) ToggleEnabled(ctx context.Context, pluginID string) error { ... }
```

### テスト
- `internal/plugin/manager_test.go`: モック CLI を注入した CRUD テスト

---

## Phase 3: TUI 統合 - プラグインモード

### 目標
メイン TUI にプラグイン管理モードを追加する。

### 設計

既存の AppMode パターンに倣う:

```go
// gui/app.go
const (
    ModeMain     AppMode = iota
    ModeDiff
    ModeTool
    ModePlugin   // NEW
)
```

プラグインモード内のサブモード:

```
ModePlugin
├── SubInstalled   (デフォルト: インストール済み一覧)
└── SubMarketplace (Tab で切替: マーケットプレイスブラウズ)
```

### 新規ファイル

#### `internal/gui/plugin_panel.go`
- プラグインモードの状態管理
- インストール済み / マーケットプレイスのサブモード切替
- プラグイン一覧の表示 (j/k でナビゲーション)
- プレビュー表示 (右パネルに README やプラグイン詳細)

#### `internal/gui/keydispatch/keyhandler/plugins.go`
- プラグインパネル専用のキーハンドラー

| キー | 動作 |
|------|------|
| j/k | ナビゲーション |
| Tab | Installed / Marketplace 切替 |
| i | インストール (marketplace サブモード) |
| d | アンインストール (installed サブモード) |
| e | 有効/無効切替 |
| u | アップデート |
| / | 検索/フィルタ |
| Enter | プレビュー (fullscreen) |
| q/Esc | メインモードに戻る |

#### `internal/gui/presentation/plugins.go`
- `FormatInstalledLine()`: インストール済みプラグイン行
  - `[E] lua-lsp@official  v1.0.0  user` / `[D] code-review@official  v1.0.0  user`
- `FormatAvailableLine()`: マーケットプレイスプラグイン行
  - `code-review@official  Cross-platform ad...  899 installs`
  - インストール済みの場合 `[I]` マーカー
- `FormatPluginPreview()`: プラグイン詳細表示

### 既存ファイルの変更

#### `internal/gui/app.go`
- `ModePlugin` 定数追加
- `PluginProvider` インターフェース追加
- プラグイン関連フィールド追加 (pluginCursor, pluginSubMode, pluginFilter)

#### `internal/gui/layout.go`
- `ModePlugin` 時のレイアウト:
  - 上部: タブバー `[Installed] [Marketplace]`
  - 左パネル: プラグイン一覧
  - 右パネル: プレビュー
  - 下部: options バー

#### `internal/gui/keydispatch/dispatcher.go`
- プラグインパネルハンドラーの登録

#### `cmd/lazyclaude/root.go`
- プラグインマネージャーの初期化とワイヤリング

### キーバインド追加 (メインモード)

- `P`: プラグインモードに切替

---

## Phase 4: 非同期操作 & UX 改善

### 目標
CLI コマンドの非同期実行、ローディング表示、エラーハンドリング。

### 設計

- `install` / `uninstall` / `update` は非同期で実行 (goroutine)
- 実行中はステータスバーにスピナー表示
- 完了時にリスト自動リフレッシュ
- エラー時はポップアップで表示

### 変更ファイル

#### `internal/gui/plugin_panel.go`
- 非同期操作の状態管理 (loading, error)
- `gui.Update()` でリフレッシュをスケジュール

#### `internal/gui/presentation/plugins.go`
- ローディング表示
- エラー表示

---

## Implementation Order

```
Phase 1 (モデル & CLI)    → 実装開始
Phase 2 (マネージャー)    → Phase 1 完了後
Phase 3 (TUI 統合)        → Phase 2 完了後
Phase 4 (非同期 & UX)     → Phase 3 完了後
```

## Technical Decisions

### なぜ CLI `--json` 出力を一次情報源とするか
- `installed_plugins.json` や `known_marketplaces.json` は Claude Code の内部フォーマットであり、予告なく変更される可能性がある
- `--json` フラグは公開 API として設計されており、フォーマットの安定性が高い
- ファイルの直接パースより CLI 呼び出しのオーバーヘッドは許容範囲 (数百ms)

### なぜ lazyclaude 側でマーケットプレイスソースを永続化しないか
- `claude plugins marketplace add/remove/list` で既に管理されている
- `~/.claude/plugins/known_marketplaces.json` が正規の永続化先
- lazyclaude が二重管理すると同期問題が発生する
- Session Store (state.json) はセッション/プロジェクト状態の管理に特化させる (#14 の Project hierarchy と整合)

### なぜ新しい AppMode か
- プラグイン管理はセッション管理と独立した関心事
- 画面レイアウト (一覧 + プレビュー) は再利用できるが、データソースとキーバインドが異なる
- 既存の ModeDiff/ModeTool パターンに倣い、コードの見通しを良く保つ

## Dependencies

- Claude Code CLI (`claude` コマンド) がインストール済みであること
- `claude plugins` サブコマンドが `--json` フラグをサポートしていること
- ネットワーク接続 (マーケットプレイスの fetch に必要、オフラインではインストール済みのみ表示)

## Risks & Mitigations

| リスク | 軽減策 |
|--------|--------|
| `claude plugins --json` 出力フォーマットの変更 | JSON パースエラー時に graceful degradation、テストに実際の出力スナップショットを fixtures として保持 |
| CLI コマンドが遅い (ネットワーク依存) | 非同期実行 + スピナー表示、キャッシュ利用 |
| `claude` バイナリが見つからない | 起動時にチェック、見つからない場合はプラグインモード無効化 |
| オフライン環境 | インストール済みプラグインは `list --json` で取得可能、マーケットプレイスは unavailable 表示 |
