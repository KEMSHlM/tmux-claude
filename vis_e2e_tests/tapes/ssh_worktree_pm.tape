Output outputs/ssh_worktree_pm/ssh_worktree_pm.gif
Output outputs/ssh_worktree_pm/ssh_worktree_pm.txt

Set Shell "bash"
Set Width 1600
Set Height 1200
Set FontSize 16
Set WaitTimeout 90s

# SSH into remote to create SSH pane context
Sleep 1s
Type "ssh remote" Enter
Sleep 3s

# Launch lazyclaude (Ctrl+\) — detects SSH pane, pendingHost="remote"
Ctrl+\
Wait+Screen /No sessions/
Sleep 1s

# w: create worktree on remote
Type "w"
Wait+Screen /Branch/
Type "feat-test"
Tab
Type "test worktree"
Enter

# Verify: session appears under remote project with remote path
Wait+Screen /remote: root/
Sleep 2s

# Enter fullscreen on worktree session — verify remote CWD /root
Down
Enter
Wait+Screen /\/root/
Sleep 3s

# Exit fullscreen
Ctrl+\
Sleep 2s

# P: create PM session on remote
Type "P"
Wait+Screen /pm/
Sleep 2s

# Navigate to PM session and enter fullscreen — verify remote
Down
Enter
Wait+Screen /\/root/
Sleep 3s

# Exit fullscreen
Ctrl+\
Sleep 2s

# Quit
Type "q"
Sleep 1s
