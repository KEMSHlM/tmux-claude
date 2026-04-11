# Plan: Remote session の fullscreen copy mode (Ctrl+V) 対応 (Bug 3)

## Context

fullscreen mode で Ctrl+V で起動する lazyclaude 自前 copy mode が remote session で正しく機能しない。local では動く。

## Root Cause Hypothesis

### 経路
1. Ctrl+V → `ActionScrollEnter` → `App.ScrollModeEnter()` (`internal/gui/app_actions.go:927` 付近)
2. `captureScrollbackWithHistorySize()` が `a.sessions.CaptureScrollback(id, w, startLine, endLine)` を呼ぶ
3. → `guiCompositeAdapter` → `CompositeProvider.CaptureScrollback(id, ...)`
4. → `providerForSession(id)` で local provider 選択 (remote mirror も local Store にいる、設計通り)
5. → `localDaemonProvider.CaptureScrollback` (`cmd/lazyclaude/local_provider.go:117`):
   ```go
   target := sess.TmuxWindow
   if target == "" {
       target = "lazyclaude:" + sess.WindowName()
   }
   content, err := p.tmux.CapturePaneANSIRange(ctx, target, startLine, endLine)
   ```
6. `tmux capture-pane -t <target> -ep -S <start> -E <end>`

### 失敗シナリオ

remote mirror session の場合:
- **Path A**: `sess.TmuxWindow` が bare `@ID` (例: `@42`) — store.go:585 の reconcile で設定される — grouped session 環境では `@42` だけでは target 解決が曖昧になる可能性
- **Path B**: `sess.TmuxWindow` が bare `rm-xxxx` (window name) — MirrorManager 生成直後 — tmux が session:window 形式を期待する場面で解決失敗の可能性
- **Path C**: desync 時 `TmuxWindow` が空 → fallback で `"lazyclaude:lc-xxxx"` (常に `lc-` prefix) → remote には存在しない window → capture 空または error

どのパスも `lazyclaude:` prefix + 正しい window 名の解決で修正される。

### Bug 1 (TmuxTarget helper) との関係

**本 bug は Bug 1 の fix (`Session.TmuxTarget()` helper) で副次的に解決する可能性が高い** (70%+)。

Bug 1 plan (`docs/dev/attach-bug-fix-plan.md`) では:
- `CapturePreview` / `CaptureScrollback` / `HistorySize` も `sess.TmuxTarget()` に統一
- `TmuxTarget()` は常に `lazyclaude:` prefix を付け、desync 時は host に応じて `MirrorWindowName` / `WindowName` を選ぶ
- → remote mirror の capture target が常に `"lazyclaude:rm-xxxx"` 形式に正規化される

従って **本 plan は Bug 1 merge 後の verification task** として位置付ける。もし Bug 1 merge 後も copy mode が壊れていたら追加調査 + 独立 fix を投入する。

## Strategy

### Phase 1: Bug 1 merge 後の verification (this plan)

1. `daemon-arch` に Bug 1 (TmuxTarget helper) が merge されたことを確認
2. 手動検証: remote session で fullscreen + Ctrl+V が期待通り動作するか
3. **動いたら**: 本 plan は完了扱い、docs で言及するだけ
4. **動かなかったら**: Phase 2 の追加調査へ進む

### Phase 2: 追加調査 (Bug 1 merge 後も壊れている場合のみ)

調査項目:

1. **実際に capture される内容**: debug log で CaptureScrollback の target と返値を確認
   - target が `lazyclaude:rm-xxxx` になっているか
   - content が空か、違う session の内容か、期待通りか
2. **ScrollState の state transition**: `internal/gui/scroll_state.go`
   - `SetLines(nil)` で初期化されているか
   - selection range が remote session 専有で保持されているか
3. **HistorySize の呼び出し**: `localDaemonProvider.HistorySize` (`cmd/lazyclaude/local_provider.go:130`) も同じ pattern で問題ないか
4. **copy mode のキー dispatch**: remote session の fullscreen 中に Ctrl+V キーが scroll mode に正しく dispatch されているか、それとも fullscreen editor に吸収されているか
5. **tmux の `-L lazyclaude` socket 上の window 解決**: grouped session (production では複数 mirror が存在しうる) で `rm-xxxx` name の一意性

### Phase 2 候補の fix (調査結果次第)

- **Fix A**: `localDaemonProvider.CaptureScrollback` で target を生成する際に `TmuxTarget()` を使わず、特殊な capture 用 helper を別途用意する (Bug 1 の helper で対応できない場合)
- **Fix B**: `CapturePaneANSIRange` で tmux コマンドに `-t lazyclaude:<window>` 形式を強制する adapter layer の修正
- **Fix C**: `ScrollModeEnter` で fullscreen target の正規化を修正 (`a.fullscreen.Target()` が返す値が session ID でなく window name だった場合)
- **Fix D**: desync 時の store 状態に起因する場合は `Store.Sync` 側の修正

## Out of Scope

- Bug 1 の修正 (`docs/dev/attach-bug-fix-plan.md` で扱う)
- Bug 2 の修正 (`docs/dev/mcp-plugin-remote-plan.md` で扱う)
- copy mode 自体の UX 改善 (scroll 範囲 UI、clipboard 書き込み方式 etc.)

## Verification

### Phase 1
1. Bug 1 が daemon-arch に merge されていることを確認 (`git log`)
2. 再ビルド + 手動検証:
   - [ ] local session で Ctrl+V → fullscreen copy mode 起動 (regression チェック)
   - [ ] remote session で Ctrl+V → fullscreen copy mode 起動、scrollback 表示、選択、clipboard 書き込み
3. 動けば本 plan は「解決済 (side effect of Bug 1)」として docs を更新しクローズ

### Phase 2 (動かなかった場合)
- Phase 2 調査項目を worker にアサイン
- 調査結果に基づき Fix A/B/C/D のいずれかで独立 PR

## Dependencies

- **Blocks on Bug 1 merge** (`fix-attach-remote` worker の review → merge 完了を待つ)
- Bug 1 は既に plan 承認済・worker (`dd5c5946-870d-4cd0-9708-79d9ab10fc85`) 実行中

## Files Changed

### Phase 1
なし (verification のみ)。

### Phase 2 (conditional)
調査結果次第だが、想定される範囲:
- `cmd/lazyclaude/local_provider.go` (CaptureScrollback の target 生成)
- `internal/core/tmux/exec.go` (CapturePaneANSIRange の target 正規化)
- `internal/gui/scroll_state.go` (state transition 修正)

## Risk Assessment

- **Low**: Bug 1 で解決するなら変更ゼロ
- **Medium**: Bug 1 で解決しなかった場合、capture 周りのどこかに remote 前提の壊れがある可能性。調査工数は中程度

## Open Questions

1. Bug 1 merge 後すぐ本 plan の verification に着手するか、他の優先 bug (Bug 2) が先か
2. Phase 2 に入る場合、追加の仮説検証 (debug log 有効化、tmux コマンドの直接実行) をユーザー環境で依頼するか
