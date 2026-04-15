Output outputs/ssh_profile/ssh_profile.gif
Output outputs/ssh_profile/ssh_profile.txt

Set Shell "bash"
Set Width 1600
Set Height 1200
Set FontSize 16
Set WaitTimeout 60s

# SSH into remote-host, cd to project directory
Sleep 1s
Type "ssh -t remote 'cd /root/app/test-app && exec bash'" Enter
Sleep 3s

# Launch lazyclaude (Ctrl+\) — detects SSH pane, pendingHost="remote"
Ctrl+\
Wait+Screen /No sessions/
Sleep 3s

# Press `n` -> profile chooser should show remote profiles (gpu default)
Type "n"
Sleep 3s

# Escape
Escape
Sleep 1s

# Quit
Type "q"
Sleep 1s
