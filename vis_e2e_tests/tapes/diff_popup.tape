Output outputs/diff_popup/diff_popup.gif
Output outputs/diff_popup/diff_popup.txt

Set Shell "bash"
Set Width 1200
Set Height 800
Set FontSize 16
Set WaitTimeout 30s

# lazyclaude 起動
Type "lazyclaude" Enter Sleep 5s

# セッション作成
Type "n" Sleep 5s

# フルスクリーン
Enter Sleep 5s

# Claude に編集依頼
Type "edit /tmp/test.go and change hello to goodbye" Enter Sleep 15s

# diff popup が表示される → y で承認
Type "y" Sleep 3s

# フルスクリーンから戻る → 終了
Ctrl+\ Sleep 1s
Type "q" Sleep 1s
