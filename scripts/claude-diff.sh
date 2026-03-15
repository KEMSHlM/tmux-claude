#!/bin/bash
# claude-diff.sh - Show diff and send approval to Claude session
# Args: old_file new_file window

OLD_FILE="$1"
NEW_FILE="$2"
WINDOW="$3"

COLS=$(tput cols 2>/dev/null || echo 80)
FILENAME=$(basename "$OLD_FILE")

# ANSI colors
RESET='\033[0m'
BOLD='\033[1m'
DIM='\033[2m'
CYAN='\033[38;2;23;146;153m'
YELLOW='\033[38;2;223;142;29m'
GREEN='\033[38;2;64;160;43m'
RED='\033[38;2;192;72;72m'
BLUE='\033[38;2;30;102;245m'

# Header
TITLE=" Claude Code · ${FILENAME} "
TITLE_LEN=${#TITLE}
LINE_LEN=$(( COLS - 2 ))
RIGHT_LEN=$(( LINE_LEN - TITLE_LEN - 1 ))
printf "${CYAN}╭─${BOLD}${TITLE}${RESET}${CYAN}"
printf '─%.0s' $(seq 1 $RIGHT_LEN)
printf '╮\n'

# Diff output
echo ""
git diff --no-index -- "$OLD_FILE" "$NEW_FILE" | delta --paging=never
echo ""

# Footer border
printf "${CYAN}╰"
printf '─%.0s' $(seq 1 $LINE_LEN)
printf "╯${RESET}\n"

# Action bar
echo ""
printf "  ${GREEN}${BOLD}y${RESET}  Yes        "
printf "${YELLOW}${BOLD}a${RESET}  Allow all in session        "
printf "${RED}${BOLD}n${RESET}  No\n"
printf "  ${BOLD}❯${RESET} "

# Read single keypress
read -r -n1 choice
echo ""

# Send to Claude session pane
if [ -n "$WINDOW" ]; then
  case "$choice" in
    y|Y|1) tmux send-keys -t "claude:=$WINDOW" "1" Enter ;;
    a|A|2) tmux send-keys -t "claude:=$WINDOW" "2" Enter ;;
    n|N|3) tmux send-keys -t "claude:=$WINDOW" "3" Enter ;;
    *)     tmux send-keys -t "claude:=$WINDOW" "3" Enter ;;
  esac
fi

rm -f "$NEW_FILE"
