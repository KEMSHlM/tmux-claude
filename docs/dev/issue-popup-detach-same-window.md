# Issue: 別クライアントが同じウィンドウを表示中だと popup が即終了する

## 症状

AnyContext 等の非 claude セッションから `prefix o` で Claude を起動しようとすると、popup が一瞬表示されて即座に消える。

## 再現条件

1. claude セッションに /dev/ttys010 がアタッチ済みで、ウィンドウ `claude-XXXXXXXX` を表示中
2. AnyContext セッションから `prefix o` を押す
3. `pane_current_path` のハッシュが同じ `XXXXXXXX` に一致する
4. `claude-popup.sh` が `attach-session -t claude:=claude-XXXXXXXX` を実行
5. 同じウィンドウを2つのクライアントが見る状態になり、popup 内の attach が即 `[detached]` で終了

## 根本原因

`claude-popup.sh` は `env -u TMUX tmux attach-session -t claude:=$WINDOW` を実行する。既に別のクライアントが同じセッション・同じウィンドウを表示中だと、attach は成功するが即座に detach される。

`prefix space` (claude-switch.sh) 経由では動作する理由: fzf でウィンドウを選択する際に、現在表示中でないウィンドウを選ぶため競合が起きない。

## 回避策（現状）

claude セッションのクライアントを別のウィンドウに切り替えてから `prefix o` を押す。

## 修正案

1. `claude-popup.sh` で attach 前に `select-window` で対象ウィンドウに切り替える
2. `claude-launch.sh` で対象ウィンドウが既に別クライアントに表示されている場合を検知し、`select-window` + `attach-session` に切り替える
3. `display-popup` の代わりに `switch-client` を使う方式を検討

## 影響範囲

- `scripts/claude-popup.sh`
- `scripts/claude-launch.sh`
- execSync 非同期化とは無関係
