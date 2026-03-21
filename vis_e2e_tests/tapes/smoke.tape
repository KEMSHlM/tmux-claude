Output outputs/smoke/smoke.gif
Output outputs/smoke/smoke.txt

Set Shell "bash"
Set Width 1200
Set Height 800
Set FontSize 16
Set WaitTimeout 5s

Type "pwd" Enter Sleep 1s
Type "ls" Enter Sleep 1s
Type "echo hello world" Enter Sleep 1s
Type "cat /nonexistent" Enter Sleep 1s
Type "ls /no/such/dir" Enter Sleep 1s
