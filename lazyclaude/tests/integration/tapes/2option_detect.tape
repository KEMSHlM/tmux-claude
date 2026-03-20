Output outputs/2option_detect.gif

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

Sleep 8s

Sleep 500ms
Type "Run this exact bash command: echo hello"
Enter
Sleep 15s
