Output outputs/tmux_popup.gif

Require tmux
Require lazyclaude

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 10s

Sleep 500ms
Type "tmux new-session -s test -x 80 -y 24"
Enter
Sleep 1s

Sleep 500ms
Type "bash /app/tests/integration/tapes/helpers/run_tool_popup.sh @0"
Enter
Sleep 3s
