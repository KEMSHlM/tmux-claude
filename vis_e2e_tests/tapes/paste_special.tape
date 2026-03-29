Output outputs/paste_special/paste_special.gif
Output outputs/paste_special/paste_special.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16

# No WaitTimeout — rely on explicit Sleeps only.

Sleep 2s
Type "lazyclaude --debug" Enter
Sleep 5s

# Create a session
Type "n"
Sleep 5s

# Move cursor to session row, then enter fullscreen.
Type "j"
Sleep 1s
Enter
Sleep 15s

# By now (~28s), entrypoint has sent paste at 20s.
# Type chars to verify TUI is responsive.
Type "hello"
Sleep 3s

# Screenshot should show fullscreen still alive (not frozen).
Sleep 2s
