Output outputs/popup_stack/popup_stack.gif
Output outputs/popup_stack/popup_stack.txt

Set Shell "bash"
Set Width 1200
Set Height 800
Set FontSize 16
Set WaitTimeout 60s

# lazyclaude 起動
Type "lazyclaude" Enter Sleep 5s

# セッション作成
Type "n" Sleep 5s

# フルスクリーン
Enter Sleep 5s

# Claude に複数ツールを使うタスクを依頼
Type "create 3 files: /tmp/a.txt with 'aaa', /tmp/b.txt with 'bbb', /tmp/c.txt with 'ccc'" Enter Sleep 15s

# popup が表示される → y で承認 (3回)
Type "y" Sleep 3s
Type "y" Sleep 3s
Type "y" Sleep 3s

# フルスクリーンから戻る → 終了
Ctrl+\ Sleep 1s
Type "q" Sleep 1s
