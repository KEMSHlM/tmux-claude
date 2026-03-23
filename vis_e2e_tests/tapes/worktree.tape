Output outputs/worktree/worktree.gif
Output outputs/worktree/worktree.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 60s

# Ctrl+\ で lazyclaude popup 起動
# entrypoint.sh が /app に git repo + .claude/worktrees/fix-popup を事前作成済み
Ctrl+\
Wait+Screen /No sessions/

# W (大文字) で既存 worktree セレクト
Type "W"
Wait+Screen /Select Worktree/
Sleep 1s

# fix-popup が選択肢に表示 → Enter で選択
Enter
Wait+Screen /Prompt/
Sleep 500ms

# プロンプト入力
Type "Continue fixing the bug"
Sleep 500ms

# Enter で確定 → 既存 worktree で Claude Code 起動
Enter
Wait+Screen /\[W\]/
Sleep 2s

# q で終了
Type "q" Sleep 1s
