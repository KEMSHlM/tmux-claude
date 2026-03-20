Output outputs/measure_latency.gif

Require lazyclaude

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 30s

Sleep 500ms
Type "lazyclaude"
Enter
Wait+Screen /no sessions/
Sleep 1s

Type "n"
Sleep 2s

Enter
Sleep 3s
Wait+Screen /INSERT/

Sleep 5s

Type@100ms "ZXQWJ"
Sleep 2s

Escape
Sleep 500ms
Ctrl+\
Sleep 500ms
Type "q"
Sleep 500ms
Type "q"
Sleep 1s
