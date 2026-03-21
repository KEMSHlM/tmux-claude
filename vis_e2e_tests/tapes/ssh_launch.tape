Output outputs/ssh_launch/ssh_launch.gif
Output outputs/ssh_launch/ssh_launch.txt

Set Shell "bash"
Set Width 1200
Set Height 800
Set FontSize 16
Set WaitTimeout 30s

# SSH 先にログイン
Type "ssh remote" Enter Sleep 3s

# Ctrl+\ で lazyclaude popup 起動
Ctrl+\ Sleep 5s

# n でリモートセッション作成
Type "n" Sleep 8s

# q で終了
Type "q" Sleep 1s
