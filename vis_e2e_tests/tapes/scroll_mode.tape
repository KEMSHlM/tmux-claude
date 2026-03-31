Output outputs/scroll_mode/scroll_mode.gif
Output outputs/scroll_mode/scroll_mode.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 30s

# Ctrl+\ で lazyclaude popup 起動
Ctrl+\
Wait+Screen /No sessions/

# n でセッション作成
Type "n"
Wait+Screen /app/
Sleep 8s

# Enter で fullscreen
Enter
Sleep 3s

# Ctrl+V で scroll mode に入る
Ctrl+V
Sleep 3s

# j で下にスクロール
Type "j"
Sleep 1s
Type "j"
Sleep 1s
Type "j"
Sleep 1s

# k で上にスクロール
Type "k"
Sleep 1s

# v で visual selection 開始
Type "v"
Sleep 1s

# j で選択範囲を拡張
Type "j"
Sleep 1s
Type "j"
Sleep 1s

# y でコピー (scroll mode を終了)
Type "y"
Sleep 2s

# Ctrl+\ で fullscreen 終了
Ctrl+\
Sleep 1s

# q で lazyclaude 終了
Type "q"
Sleep 1s
