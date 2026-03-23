# Implementation Plan: Worktree Command

## Overview

2つのキーで worktree を操作:
- `w` (小文字): 新規 worktree 作成。Branch+Prompt ダイアログを表示し、`git worktree add` + Claude Code 起動
- `W` (大文字): 既存 worktree セレクト。`git worktree list` で検出した worktree のリストを表示し、選択した worktree で Claude Code 起動

## 実装済み (main にマージ済み)

### Phase 0: ダイアログとポップアップの競合修正
- `DialogKind` 型で統一管理 (`DialogNone`, `DialogRename`, `DialogWorktree`)
- `HasActiveDialog()` による focus guard
- rename / worktree ダイアログ中にポップアップがフォーカスを奪わない

### Phase 1-5: 新規 worktree 作成 (`w` キー)
- `w` → Branch+Prompt ダイアログ (Enter 確定、Tab 切替、Esc キャンセル、Ctrl+J 改行)
- `git worktree add -b <branch> .claude/worktrees/<name>` で git worktree 作成
- `--append-system-prompt` (isolation) + positional argument (user task) で Claude Code 起動
- launcher script でシェルクォート問題を回避
- `[W]` アイコン + `[worktree]` プレビュータイトル
- DefaultEditor + view-specific binding の統一パターン

## 未実装: 既存 worktree セレクト (`W` キー)

### 修正方針

`7bdce42` で `w` に入れた chooser 分岐を撤回し、`W` に移す:
- `w` → 変更なし (Branch+Prompt ダイアログ)
- `W` → 新規アクション `SelectWorktree()`

### Phase 6: キー分離

1. `StartWorktreeInput()` から chooser 分岐を削除 (元の実装に戻す)
2. `AppActions` に `SelectWorktree()` 追加
3. `SessionsPanel` に `W` キー → `SelectWorktree()` 追加
4. OptionsBar 更新 (`W:select`)

### Phase 7: Chooser UI (既存実装を流用)

chooser / resume-prompt の UI・keybinding は `7bdce42` で実装済み。`SelectWorktree()` から呼ぶだけ:
- `W` → `ListWorktrees` で既存 worktree 取得
- 結果が空 → ステータスバーに "No worktrees found" 表示
- 結果あり → `showWorktreeChooser(g, items)` 表示
- j/k で選択、Enter で確定、Esc でキャンセル
- Enter → `showWorktreeResumePrompt(g, name)` → プロンプト入力 → `ResumeWorktree` で Claude Code 起動

### Phase 8: VHS E2E テスト

- `entrypoint.sh` で `/app` に `git init` + `git worktree add` で事前作成
- tape: `W` で chooser 表示 → 選択 → プロンプト → 起動 → `[W]` 確認

## 変更ファイル

| File | Change |
|------|--------|
| `app_actions.go` | `StartWorktreeInput` を元に戻す、`SelectWorktree()` 追加 |
| `keyhandler/actions.go` | `SelectWorktree()` 追加 |
| `keyhandler/sessions.go` | `W` キー追加、OptionsBar 更新 |
| `keyhandler/mock_actions_test.go` | `SelectWorktree()` 追加 |
| `keydispatch/dispatcher_test.go` | `SelectWorktree()` 追加 |
| `keybindings.go` | rune リストに大文字不要 (既に `R`, `D` あり) |
| `worktree.tape` | `W` キーでテスト |
| `entrypoint.sh` | worktree テープ用セットアップ |

## Success Criteria

- [x] `w` で Branch+Prompt ダイアログ表示 (新規作成)
- [x] Tab / Enter / Esc / Ctrl+J が正しく動作
- [x] `[W]` アイコン + `[worktree]` プレビュー
- [x] 不正なブランチ名は拒否
- [ ] `W` で既存 worktree のセレクトリスト表示
- [ ] j/k で選択、Enter で確定、Esc でキャンセル
- [ ] 既存 worktree がない場合 "No worktrees found" 表示
- [ ] 選択後プロンプト入力 → Claude Code 起動
- [ ] VHS E2E で `W` キーフローを検証
- [ ] 既存テスト全通過
