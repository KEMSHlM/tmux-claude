Output outputs/setup.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 10s

Sleep 500ms
Type "rm -f ~/.claude/settings.json /tmp/lazyclaude-mcp.port"
Enter
Sleep 1s

Sleep 500ms
Type "lazyclaude setup"
Enter
Sleep 3s

Sleep 500ms
Type "cat /tmp/lazyclaude-mcp.port"
Enter
Sleep 1s

Sleep 500ms
Type "cat ~/.claude/settings.json | head -5"
Enter
Sleep 2s

# Idempotent re-run
Sleep 500ms
Type "lazyclaude setup"
Enter
Sleep 3s

