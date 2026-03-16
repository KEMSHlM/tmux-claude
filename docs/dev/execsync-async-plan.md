# Implementation Plan: execSync/spawnSync の非同期化

## 概要

`scripts/mcp-server.js` 内の全 `execSync` / `spawnSync` 呼び出しを非同期 (`exec` / `spawn` の Promise ラッパー) に変換し、Node.js event loop のブロックを解消する。特に `/notify` HTTP ハンドラ内の `resolveWindow()` → `findTmuxWindowForPid()` が最大 31 回（resolveWindow ループ 15 + tmux list-panes 1 + findTmuxWindowForPid ループ 15）の同期プロセス起動を行う問題を修正する。

## 要件

1. 全 `execSync` / `spawnSync` を非同期代替に変換
2. 既存の動作・エラーハンドリングを維持
3. 呼び出し元関数の async 伝播を正しく処理
4. レースコンディション・エラー伝播のリスクを考慮
5. `/notify` 内の `resolveWindow()` 二重呼び出しを1回に統合

---

## execSync/spawnSync 使用箇所の完全監査

### execSync (6 箇所)

| # | 行 | 関数 | コマンド | 呼び出し頻度 | 影響度 |
|---|-----|------|---------|-------------|--------|
| E1 | 153 | `findActiveClient()` | `tmux list-clients` | popup 起動時毎回 | MEDIUM |
| E2 | 166 | `findTmuxWindowForPid()` | `tmux list-panes -a` | ide_connected + resolveWindow 毎回 | HIGH |
| E3 | 176 | `findTmuxWindowForPid()` | `ps -o ppid=` (ループ最大15回) | ide_connected + resolveWindow 毎回 | **CRITICAL** |
| E4 | 191 | `resolveWindow()` | `ps -o ppid=` (ループ最大15回) | /notify 毎回 | **CRITICAL** |
| E5 | 275 | `getNotifyType()` | `tmux show-option` | popup 起動時毎回 | LOW |
| E6 | 750 | `updateRemoteLockFiles()` | `tmux list-windows` | サーバー起動時1回 | LOW |

### spawnSync (11 箇所)

| # | 行 | 関数/コンテキスト | コマンド | 影響度 |
|---|-----|-------------------|---------|--------|
| S1 | 320 | `installChoiceHandler` callback | `tmux capture-pane` | MEDIUM |
| S2 | 324 | `installChoiceHandler` callback | `tmux send-keys` | LOW |
| S3 | 376 | `triggerDiffPopupForWindow()` | `tmux display-message` (サイズ取得) | MEDIUM |
| S4 | 381 | `triggerDiffPopupForWindow()` | `tmux display-message` (CWD取得) | MEDIUM |
| S5 | 412 | `triggerPopupForWindow()` menu | `tmux display-menu` | **MEDIUM** |
| S6 | 428 | `triggerPopupForWindow()` popup | `tmux display-message` (CWD取得) | MEDIUM |
| S7 | 432 | `triggerPopupForWindow()` popup | `tmux display-message` (サイズ取得) | MEDIUM |
| S8 | 504 | `handleMcpMessage` openDiff | `tmux display-message` (サイズ取得) | MEDIUM |
| S9 | 549 | `handleMcpMessage` openDiff | `tmux display-popup` | HIGH |
| S10 | 662 | `/notify` handler pendingChoice | `tmux send-keys` | LOW |
| S11 | 773 | `updateRemoteLockFiles()` | `ssh` (リモート更新) | LOW |

**注**: S5 `display-menu` はメニューが開いている間ずっとブロックするため MEDIUM に引き上げ。

---

## 依存チェーン（async 伝播）

```
resolveWindow()        → async 化が必要
  └─ findTmuxWindowForPid() → async 化が必要
       └─ execSync x (1 + 最大15)

findActiveClient()     → async 化が必要

getNotifyType()        → async 化が必要

triggerPopupForWindow() → async 化が必要        ← triggerDiffPopup より先に async 化
  ├─ findActiveClient()     → async
  ├─ getNotifyType()        → async
  ├─ spawnSync (menu/dim/cwd) → async
  └─ installChoiceHandler() → callback 内の spawnSync → async

triggerDiffPopupForWindow() → async 化が必要    ← triggerPopupForWindow を呼ぶため後
  ├─ findActiveClient()     → async
  ├─ spawnSync (dim, cwd)   → async
  ├─ triggerPopupForWindow() → async             ← 依存!
  └─ installChoiceHandler() → callback 内の spawnSync → async

triggerPopup()          → async 化が必要
  └─ triggerPopupForWindow() → async

handleMcpMessage()      → async 化が必要
  ├─ findTmuxWindowForPid() → async
  ├─ findActiveClient()     → async
  └─ spawnSync (dim, popup) → async

/notify readBody callback → async 化が必要
  ├─ resolveWindow() x1    → async（二重呼び出しを統合）
  ├─ triggerDiffPopupForWindow() → async
  └─ triggerPopupForWindow()     → async

updateRemoteLockFiles() → async 化が必要
  ├─ execSync (list-windows) → async
  └─ spawnSync (ssh) → async
```

---

## 実装手順

### Phase 1: 非同期ユーティリティの追加 + リファクタリング（基盤）

**Step 1.1**: `execAsync` ヘルパーを作成

- ファイル冒頭（import セクション直後）に配置
- `child_process.exec` を `util.promisify` でラップ
- エラー時は null を返すバリアント `execQuiet` も用意
- `spawnAsync` は原則不要 — ほとんどの `spawnSync` は stdout のみ使用するため `execAsync` で統一可能
- 例外: S9 の `tmux display-popup`（close event が必要）のみ spawn + Promise wrap
- 依存: なし
- リスク: LOW

**Step 1.2**: tmux `display-message` のバッチ化

- S3+S4, S6+S7 で CWD とサイズを別々に取得している箇所を 1 回の呼び出しに統合
- `tmux display-message -p '#{pane_current_path} #{client_width} #{client_height}'`
- spawnSync 呼び出し数を削減（11 → 9）
- リスク: LOW

**Step 1.3**: `/notify` 内の `resolveWindow()` 二重呼び出しを統合

- 行 644 と 653 で同一 rawPid に対して 2 回呼ぶ構造をリファクタ
- 先頭で 1 回だけ呼んで結果を変数に保持:
  ```js
  const window = await resolveWindow(rawPid);
  if (!window) { /* 404 */ }
  if (data.type === 'tool_info') { ... }
  else { ... }
  ```
- 最悪ケースの execSync 呼び出し回数が 62 → 31 に半減
- リスク: LOW

### Phase 2: リーフ関数の非同期化（CRITICAL パス）

**Step 2.1**: `findTmuxWindowForPid()` → `async findTmuxWindowForPid()`

- 行 163-182
- `execSync('tmux list-panes ...')` → `execAsync`
- `execSync('ps -o ppid= ...')` ループ → `execAsync` + await（逐次）
- 各 `execAsync` にタイムアウト 3000ms を設定（ハング防止）
- 呼び出し元: `resolveWindow()`, `handleMcpMessage()` (ide_connected)
- リスク: MEDIUM
- **注意**: PID tree walk は本質的に逐次（前の ppid が次の入力）なので並列化不可

**Step 2.2**: `resolveWindow()` → `async resolveWindow()`

- 行 185-199
- `execSync('ps -o ppid= ...')` ループ → `execAsync` + await
- `findTmuxWindowForPid()` 呼び出し → await
- 各 `execAsync` にタイムアウト 3000ms を設定
- 呼び出し元: `/notify` handler
- リスク: MEDIUM

**Step 2.3**: `findActiveClient()` → `async findActiveClient()`

- 行 150-161
- `execSync('tmux list-clients ...')` → `execAsync`
- 呼び出し元: `triggerDiffPopupForWindow()`, `triggerPopupForWindow()`, `handleMcpMessage()` (openDiff)
- リスク: LOW

**Step 2.4**: `getNotifyType()` → `async getNotifyType()`

- 行 273-278
- `execSync('tmux show-option ...')` → `execAsync`
- 呼び出し元: `triggerPopupForWindow()`
- リスク: LOW

### Phase 3: 中間関数の非同期化（popup 系）

**実装順序: 3.1 → 3.3 → 3.2 → 3.4**（依存関係に基づく）

**Step 3.1**: `installChoiceHandler()` 内の spawnSync → execAsync

- 行 311-329
- setTimeout callback 内なので callback を async にする
- popup close 後の処理なのでユーザー体験への影響は小さい
- ただし複数 popup が同時に閉じた場合の event loop 詰まりを防止
- リスク: LOW

**Step 3.3** (先に実施): `triggerPopupForWindow()` → `async`

- 行 402-446
- `findActiveClient()`, `getNotifyType()` → await
- `spawnSync` x3 → `execAsync`（S5 の display-menu 含む）
- **triggerDiffPopupForWindow がこの関数を呼ぶため、先に async 化が必要**
- リスク: MEDIUM

**Step 3.2** (後に実施): `triggerDiffPopupForWindow()` → `async`

- 行 332-398
- `findActiveClient()` → await
- `spawnSync('tmux', ['display-message', ...])` → `execAsync`（Step 1.2 でバッチ化済み）
- `triggerPopupForWindow()` → await（Step 3.3 で async 化済み）
- **httpSocket.end() を await の前に呼ぶこと**（async 処理中の socket 切断防止）
- リスク: MEDIUM

**Step 3.4**: `triggerPopup()` → `async`

- 行 448-451
- `triggerPopupForWindow()` → await
- リスク: LOW

### Phase 4: トップレベルハンドラの非同期化

**実装順序: Phase 3 完了後に実施**（Phase 3 の関数が async 化されていないと await 不可）

**Step 4.1**: `handleMcpMessage()` → `async`

- 行 455-586
- `ide_connected`: `findTmuxWindowForPid()` → await
- `openDiff`: `findActiveClient()`, `spawnSync` x1 → await
- **WebSocket frame 解析ループ**: 同期のまま維持し、handleMcpMessage は fire-and-forget（`.catch(console.warn)`）
- **openDiff**: reply() を popup close 後に送信するため await 必須（Phase 5 と連動）
- **socket 切断ガード**: popup 待機中に WebSocket が切断された場合に備え、reply() 呼び出し前に `socket.writable` チェックを追加
- リスク: HIGH

**Step 4.2**: `/notify` readBody callback の async 化

- 行 635-687
- `resolveWindow()` x1 → await（Step 1.3 で二重呼び出し統合済み）
- `triggerDiffPopupForWindow()` / `triggerPopupForWindow()` → await
- callback を `async (body) => { ... }` にし最外で `.catch()` をつける
- **httpSocket.destroyed チェック**: 各 await の後、特に `triggerDiffPopupForWindow` 内の `httpSocket.end()` 呼び出し前に確認
- リスク: HIGH

**Step 4.3**: `updateRemoteLockFiles()` → `async`

- 行 738-780
- SSH 呼び出しを各ホストに対して並列化（`Promise.all`）
- 起動時のみ実行、fire-and-forget でも可
- リスク: LOW

### Phase 5: openDiff の spawnSync popup + choiceFile 管理の統一

**Step 5.1**: openDiff 内の `spawnSync('tmux', ['display-popup', ...])` → `spawn`（非同期）

- 行 549-552
- 現在 spawnSync で popup 完了を待っている（ユーザーが閉じるまでブロック）
- `spawn` に変更し、Promise で wrap:
  ```js
  const choice = await new Promise((resolve) => {
    const proc = spawn('tmux', ['display-popup', ...]);
    proc.on('close', () => {
      // choiceFile 読み取り → resolve(choice)
    });
  });
  reply(socket, id, { content: [{ type: 'text', text: choice }] });
  ```
- **choiceFile 管理の統一**: 現在は行 562-577 の直接読み取りと `installChoiceHandler` の 2 パスが存在。openDiff では `installChoiceHandler` を使わず直接読み取りしているが、async 化後は close event 内で統一的に choiceFile を読む
- **socket 切断ガード**: Promise 内で `socket.writable` をチェックしてから `reply()` を呼ぶ。切断時は choiceFile の cleanup のみ行い、reply() をスキップ
- リスク: HIGH

---

## リスク評価

### HIGH リスク

1. **openDiff の spawnSync popup + choiceFile 二重構造 (S9)**
   - 現在: spawnSync で popup 完了を待ち → 行 562-577 で choiceFile 直接読み取り → reply()
   - 問題: `installChoiceHandler` (行 311-329) と openDiff 直接読み取り (行 562-577) の 2 パスが存在
   - async 化後: spawn + close event で統一。close event 内で choiceFile 読み取り → reply()
   - **リスク**: reply() タイミングがずれると Claude Code が timeout
   - **緩和策**: Promise wrap + close event で resolve → await して reply()

2. **popup 中の WebSocket 切断**
   - async な popup 待機中に WebSocket が切断された場合（socket.on('close') で pidToWindow.delete が実行される）
   - **リスク**: closed socket への reply() 書き込みでエラー
   - **緩和策**: reply() 呼び出し前に `socket.writable` チェック。切断時は choiceFile cleanup のみ

3. **handleMcpMessage の async 化**
   - WebSocket フレーム処理ループ内から呼ばれる
   - **リスク**: 次のフレームが async 処理中に到着すると順序が乱れる
   - **緩和策**: frame 解析は同期のまま。openDiff 以外は fire-and-forget

4. **/notify handler の async 化**
   - HTTP ソケットのライフサイクル管理
   - **リスク**: async 処理中にソケットが閉じられる可能性
   - **緩和策**: `httpSocket.destroyed` チェックを各 await 後に追加。特に `httpSocket.end()` 呼び出し前

### MEDIUM リスク

5. **PID tree walk のレースコンディション**
   - async 化により実行中に PID tree が変わる可能性
   - **緩和策**: 元々 execSync でも同じタイミング問題がある。実質的リスク増加は小さい

6. **複数 /notify の並行実行 — pendingToolInfo 二重処理**
   - 同期時は逐次だった /notify が async 化で並行実行される
   - 同じ window に対する 2 つの /notify が pendingToolInfo.get() で同じ info を取得し、両方が popup を起動する可能性
   - **緩和策**: pendingToolInfo.get() 直後に即 delete() する（取得と削除をアトミックに）

7. **S5 display-menu のブロッキング**
   - メニューが開いている間ずっと event loop がブロックされる
   - **緩和策**: spawn + close event で async 化（Phase 3.3 で対応）

### LOW リスク

8. **buildNewContents 内の fs.readFileSync (行 296)**
   - ローカルファイルの小さな読み取り。event loop ブロックは無視できるレベル
   - スコープ外。必要なら後で async 化

9. **execAsync のタイムアウト**
   - 高負荷時に ps/tmux コマンドが返らないケース
   - **緩和策**: execAsync にタイムアウト 3000ms を設定（Phase 2 で対応）

---

## 優先順位（修正版）

1. **Phase 1** — 基盤（execAsync ヘルパー + display-message バッチ化 + resolveWindow 二重呼び出し統合）
2. **Phase 2** — CRITICAL: リーフ関数の非同期化（resolveWindow / findTmuxWindowForPid）
3. **Phase 3** — popup 系関数（順序: 3.1 → 3.3 → 3.2 → 3.4）
4. **Phase 4** — トップレベルハンドラ（Phase 3 完了後に実施）
5. **Phase 5** — openDiff popup + choiceFile 統一

---

## テスト戦略

### ユニットテスト
- `findTmuxWindowForPid()`: モック exec で PID tree walk が正しく動作すること
- `resolveWindow()`: pidToWindow キャッシュヒット / ミス両パターン
- PID tree walk の打ち切り条件: `ppid === '1'`, `ppid === '0'`, `ppid === current` の各パターン
- `execAsync` タイムアウト: 3000ms 超過時に null を返すこと

### 統合テスト
- `/notify` エンドポイント: async 化後も正しくレスポンスが返ること
- WebSocket `ide_connected`: PID → window 解決が非同期でも正しく動作すること
- WebSocket `openDiff`: popup が非同期でも reply() タイミングが正しいこと
- **並行 `/notify`**: 2 つの並行リクエストが同一 window に届く場合の pendingToolInfo 二重処理防止
- **socket 切断**: popup 待機中に WebSocket が切断された場合のエラーハンドリング

### 手動テスト
- Claude Code で Edit/Write → diff popup 表示、選択後に send-keys が届くこと
- 複数ウィンドウ同時 permission dialog のレースコンディション確認
- display-menu タイプで通知 → メニュー表示中に他のリクエストが処理されること
- openDiff 中に Claude Code を Ctrl+C で終了 → サーバーがクラッシュしないこと

---

## 成功基準

- [ ] `execSync` が mcp-server.js から完全に除去されている
- [ ] `spawnSync` が mcp-server.js から完全に除去されている（fire-and-forget の spawn は OK）
- [ ] `/notify` ハンドラ実行中に他のリクエストが処理可能であること
- [ ] diff popup / tool popup の選択結果が正しく send-keys されること
- [ ] openDiff の WebSocket reply タイミングが維持されること
- [ ] ide_connected の PID → window 解決が正しく動作すること
- [ ] popup 中の WebSocket 切断時にクラッシュしないこと
- [ ] 既存の全手動テストシナリオが通ること
