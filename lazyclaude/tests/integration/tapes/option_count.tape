Output outputs/option_count.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 10s

# Enqueue 2-option, then 3-option, then default after delays
Sleep 500ms
Type "( sleep 2 && bash /app/tests/integration/tapes/helpers/enqueue.sh --max-option 2 Bash && sleep 5 && bash /app/tests/integration/tapes/helpers/enqueue.sh --max-option 3 Write && sleep 5 && bash /app/tests/integration/tapes/helpers/enqueue.sh Read ) &"
Enter
Sleep 500ms

# Start lazyclaude
Sleep 500ms
Type "lazyclaude"
Enter
Wait+Screen /no sessions/
Sleep 1s

# Wait for 2-option popup (y/n)
Wait+Screen /Bash/
Sleep 1s

# Dismiss
Type "y"
Sleep 1s

# Wait for 3-option popup (y/a/n)
Wait+Screen /Write/
Sleep 1s

# Dismiss
Type "y"
Sleep 1s

# Wait for default popup (y/a/n)
Wait+Screen /Read/
Sleep 1s

Type "y"
Sleep 1s
