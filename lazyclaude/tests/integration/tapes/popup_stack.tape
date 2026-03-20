Output outputs/popup_stack.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 10s

# Enqueue 3 notifications after 2s in background
Sleep 500ms
Type "( sleep 2 && bash /app/tests/integration/tapes/helpers/enqueue.sh Tool1 Tool2 Tool3 ) &"
Enter
Sleep 500ms

# Start lazyclaude
Sleep 500ms
Type "lazyclaude"
Enter
Wait+Screen /no sessions/
Sleep 1s

# Wait for cascade [3/3]
Wait+Screen /3\/3/
Sleep 1s

# Arrow Up -> [2/3]
Up
Sleep 1s

# Esc -> suspend
Escape
Sleep 1s

# p -> reopen
Type "p"
Sleep 1s

# y -> dismiss focused
Type "y"
Sleep 1s

# y -> dismiss Tool2
Type "y"
Sleep 1s

# Enqueue 2 more for Y (accept all)
Hide
Type@0ms "bash /app/tests/integration/tapes/helpers/enqueue.sh Tool4 Tool5"
Enter
Show
Sleep 2s

# Y -> accept all
Type "Y"
Sleep 1s
