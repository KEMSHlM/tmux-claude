Output outputs/profile/profile.gif
Output outputs/profile/profile.txt

Set Shell "bash"
Set Width 1600
Set Height 1200
Set FontSize 16
Set WaitTimeout 30s

# Verify CLI `profile list` before TUI
Type "lazyclaude profile list" Enter
Wait+Screen /opus/
Sleep 1s

# Launch lazyclaude via tmux plugin (Ctrl+\)
Ctrl+\
Wait+Screen /No sessions/
Sleep 2s

# Press `n` -> profile chooser dialog should appear
Type "n"
Sleep 2s

# Escape the profile dialog
Escape
Sleep 1s

# Press `P` -> PM session profile dialog (chooser + options)
Type "P"
Sleep 2s

# Escape
Escape
Sleep 1s

# Quit
Type "q"
Sleep 1s
