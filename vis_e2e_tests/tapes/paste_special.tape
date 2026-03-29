Output outputs/paste_special/paste_special.gif
Output outputs/paste_special/paste_special.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 60s

# lazyclaude 起動 (tmux plugin 経由)
Ctrl+\
Wait+Screen /No sessions/

# セッション作成
Type "n"
Sleep 5s

# セッション行に移動してフルスクリーン
Type "j"
Sleep 1s
Enter
Sleep 3s

# entrypoint が 15 秒後に bracketed paste を TTY に直接送信
# ペースト処理後も TUI が応答することを確認
Wait+Screen /exit/

# フルスクリーン解除
Ctrl+\
Sleep 2s

Wait+Screen /quit/

Type "q"
Sleep 1s
