Output outputs/remote_popup.gif

Require lazyclaude
Require ssh

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 60s

Sleep 500ms
Type "ssh -o BatchMode=yes -o ConnectTimeout=5 ${REMOTE_HOST:-remote}"
Enter
Sleep 3s

Sleep 500ms
Type "hostname"
Enter
Sleep 1s

Sleep 500ms
Type "uname -a"
Enter
Sleep 1s

Sleep 500ms
Type "exit"
Enter
Sleep 1s

Sleep 500ms
Type "lazyclaude"
Enter
Wait+Screen /no sessions/
Sleep 3s

Type "q"
Sleep 1s
