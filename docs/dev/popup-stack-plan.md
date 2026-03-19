# Popup Stack Redesign Plan

## Requirements

1. **表示速度改善**: ポップアップ表示までの遅延を最小化（現在500msポーリング）
2. **複数ポップアップのスタック表示**: 重なって表示、gocuiのoverlap + SetViewOnTop活用
3. **上下キーでスタック切り替え**: フォーカスを移動して各ポップアップを操作
4. **Escでsuspend**: ポップアップを非表示にし、再表示可能にする

## Current State

- `pendingTool`: 単一の`*notify.ToolNotification`（1つしか保持できない）
- `hasPopup()`: `pendingTool != nil`で判定
- ポーリング: 500ms ticker + `!a.hasPopup()`ガードで**1つでもpopup表示中は新規取得しない**
- 通知ファイル: `lazyclaude-pending.json` 1ファイルのみ（atomic rename で Read は claim+delete）
- `notify.Write()` は上書き — 2つ目の通知が来ると1つ目が失われる
- gocui: `SupportOverlaps: false`（無効）

## Phase 0: 通知キュー化

**目標**: 複数通知が同時に存在できるようにする（通知ロスト防止）

### 問題

現在は単一ファイル `lazyclaude-pending.json` を上書き。
Claude Code が連続してツール許可を求めた場合、前の通知が消える:

```
T0:     Server writes notification #1 (Bash)
T50ms:  Server writes notification #2 (Write) → #1 OVERWRITTEN → LOST
T500ms: Ticker reads → gets #2 only
```

### 解決

単一ファイル → 番号付きファイルのキュー:

```
lazyclaude-pending-001.json
lazyclaude-pending-002.json
...
```

- `notify.Write()`: 連番ファイルを作成（上書きしない）
- `notify.ReadAll()`: 全ファイルを読み取り、読んだら削除
- `notify.Read()`: 最古の1つを読み取り（後方互換）

### SessionProvider変更

```go
// 現在
PendingNotification() *notify.ToolNotification

// 変更後
PendingNotifications() []*notify.ToolNotification
```

**ファイル**: `notify/notify.go`, `server/server.go`(変更なし — Write呼び出し側), `gui/app.go`(SessionProvider interface), `cmd/lazyclaude/root.go`(sessionAdapter)

## Phase 1: ポーリング高速化

**目標**: 通知検出から表示までの遅延を500ms→100ms以下に

- ticker間隔を500ms→100msに変更（通知ポーリング専用）
- `!a.hasPopup()` ガードを削除（Phase 2でスタック化するため、popup表示中も新規取得）

**ファイル**: `app.go` (Runループ)

## Phase 2: ポップアップスタック

**目標**: 複数の通知を同時に表示し、重ねて描画

### データ構造

```go
// pendingTool *notify.ToolNotification → 削除
type popupEntry struct {
    notification *notify.ToolNotification
    scrollY      int
    diffCache    []string
    diffKinds    []presentation.DiffLineKind
    suspended    bool
}

popupStack    []popupEntry  // スタック（末尾がactive）
popupFocusIdx int           // フォーカス中のインデックス
```

### gocui overlap 有効化

```go
gocui.NewGuiOpts{
    SupportOverlaps: true,
}
```

### レイアウト

- 各ポップアップに固有のview名: `tool-popup-0`, `tool-popup-1`, ...
- 位置をオフセット: popup[i]は(x0+i*2, y0+i*1)に配置（カスケード）
- `g.SetViewOnTop(activeViewName)` でフォーカス中を最前面に

**ファイル**: `app.go` (struct), `popup.go` (layout/render), `popup_stack.go` (スタック操作)

## Phase 3: 上下キーでスタック切り替え

**目標**: j/k or ↑/↓ でフォーカスを移動

- popup表示中にj/k → `popupFocusIdx`を変更
- `g.SetViewOnTop` + `g.SetCurrentView` で切り替え
- アクションバーに `[2/3]` のようなインジケーター表示

**ファイル**: `keybindings.go`, `popup.go`

## Phase 4: Esc で suspend / 再表示

**目標**: Escでポップアップを一時非表示、再度表示可能

- `Esc` → `entry.suspended = true`、viewを削除（描画しない）
- suspendされた通知はスタックに残る
- 専用キー（例: `p`）で suspended を一括復帰
- suspended 状態のpopupは `hasPopup()` に含めない（キー転送をブロックしない）

**ファイル**: `popup.go`, `popup_stack.go`, `keybindings.go`

## Risks

- **HIGH**: 通知ロスト — Phase 0 で解決必須
- **MEDIUM**: suspend中にClaude Codeがタイムアウトする可能性
- **LOW**: gocui overlay描画のパフォーマンス（popup数は通常1-3個）

## Implementation Order

Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4（各Phase完了後にテスト）
