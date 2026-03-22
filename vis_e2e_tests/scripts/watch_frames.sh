#!/bin/bash
# VHS の .txt をリアルタイム監視し、──── 区切りでフレームを出力。
#
# 必要な変数: TXT
# 必要な関数: show_frame (show_frame.sh)

for i in $(seq 1 100); do [ -f "$TXT" ] && break; sleep 0.1; done

block=""
tail -f "$TXT" 2>/dev/null | while IFS= read -r line; do
    if echo "$line" | grep -q '^────'; then
        if [ -n "$block" ]; then
            show_frame "$block"
        fi
        block=""
    else
        block="${block:+${block}
}${line}"
    fi
done
