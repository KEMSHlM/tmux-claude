Output outputs/notify_popup/notify_popup.gif
Output outputs/notify_popup/notify_popup.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 60s

# lazyclaude 起動 (tmux plugin 経由)
Ctrl+\
Wait+Screen /No sessions/

# n でセッション作成
Type "n"
Wait+Screen /app/

# セッションに移動してフルスクリーン
Down Enter
Wait+Screen /Sonnet/

# Claude にファイル作成を依頼 (Write ツールは許可が必要)
Type "create a file /tmp/hello.txt with content 'hello world'" Enter

# フルスクリーンを抜けてセッション一覧に戻る
Sleep 3s
Ctrl+\

# gocui overlay popup が表示されるのを待つ
Wait+Screen /C-y/

# Ctrl+y で承認
Ctrl+y Sleep 3s
