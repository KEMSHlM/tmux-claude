# Implementation Plan: Fix Paste Failure in gocui Claude Pane

## Requirements Restatement

- gocui 上の claude pane にペーストすると画面がフリーズ/バグる
- tmux send-keys に頼らない、ネイティブに近い動作を実現する
- tmux attach への逃避は禁止

## Root Cause Analysis

### 現状のアーキテクチャと問題点

```
ペースト → ターミナル → tcell PollEvent → gEvents chan (容量20) → processEvent → inputEditor.Edit() → pasteBuf → tmux paste-buffer
```

**ボトルネック 1: gEvents チャネル容量 20**
`third_party_gocui/gui.go:251` で `make(chan GocuiEvent, 20)` — 大きなペーストで即座にフル。
pollEvent ゴルーチンがブロックし、EventPaste{End} が gEvents に入れない → デッドロック。

**ボトルネック 2: tmux display-popup の bracketed paste バグ**
tmux issue #4431: display-popup 内で tcell が EventPaste を受信できないケースがある。
ESC[200~ が個別の EventKey として到着し、フォールバック検出が必要。

**ボトルネック 3: 1文字ずつの処理**
各ペースト文字が独立した GocuiEvent → processEvent → Editor.Edit() を通過。
inputEditor の escBuf/pasteBuf 状態マシンが複雑で、エッジケースに弱い。

### 3回の修正失敗の推定原因

gEvents チャネルのオーバーフローが根本原因。inputEditor 側でいくら
バッファリングを改善しても、pollEvent → gEvents の経路が詰まれば
イベントループ全体が停止する。

## Research Findings

### tcell の EventPaste API

```go
screen.EnablePaste()  // \e[?2004h を出力
// EventPaste{Start=true} → EventKey... → EventPaste{Start=false}
```

tcell は bracketed paste を認識するが、ペースト内容は個別の EventKey として配信。
アプリ側で Start/End 間のイベントを集約する責任がある。

### lazygit の IsPasting フラグ

jesseduffield/gocui は `g.IsPasting bool` を提供 (本プロジェクトも使用)。
しかし lazygit はコミットメッセージ入力など限定的な用途で、
大量テキストのペースト転送は想定外。

### bubbletea の設計 (charmbracelet)

bubbletea は tea.PasteMsg として全ペーストテキストを単一メッセージで配信。
**これが理想形** — pollEvent レベルで集約し、アプリには完成テキストだけ渡す。

### OSC 52 / システムクリップボード

`atotto/clipboard` (pbpaste) や OSC 52 は能動的読み取り。
ペーストはユーザーの能動的操作で発火するため、
イベント駆動の bracketed paste 集約が正しいアプローチ。

## Solution: pollEvent レベルのペースト集約

### Core Idea

ペースト文字を gEvents に流す前に pollEvent ゴルーチン内で集約し、
単一の「ペースト完了」イベントとして gEvents に送る。
gEvents チャネルにはペースト全体で **1スロット** しか消費しない。

```
[Before]  paste chars → gEvents (20 slots, overflow!) → Edit() 1 char at a time
[After]   paste chars → pollEvent accumulates → gEvents (1 event) → handlePasteContent()
```

### Design

1. **新しいイベントタイプ `eventPasteContent`** を GocuiEvent に追加
   - `PasteText string` フィールドを追加
   - handleEvent で直接 paste callback を呼び出し

2. **pollEvent 内のペースト集約ループ**
   - `EventPaste{Start}` 検出 → 集約モード開始
   - `EventKey` を strings.Builder に追加 (pollEvent ゴルーチン内)
   - `EventPaste{End}` 検出 → `eventPasteContent` を gEvents に送信
   - タイムアウト (500ms 無入力) でフォールバック flush

3. **ESC[200~ フォールバック検出** (tmux popup バグ対策)
   - pollEvent 内で ESC → [ → 2 → 0 → 0 → ~ を検出
   - 検出成功 → 同じ集約モードに遷移
   - ESC[201~ で集約終了
   - パターン不一致 → バッファ済みイベントを通常通り gEvents に送信

4. **inputEditor の大幅簡素化**
   - escBuf, pasteBuf, watchdog, pasteMu → 全削除
   - inputEditor.Edit() は単純なキー転送のみ
   - ペースト処理は App.handlePasteContent() に移行

## Implementation Phases

### Phase 1: gocui 層 — eventPasteContent の導入

**ファイル:** `third_party_gocui/tcell_driver.go`, `third_party_gocui/gui.go`

1. GocuiEvent に `PasteText string` フィールドと `eventPasteContent` 定数を追加
2. `pollEvent()` を修正:
   - `EventPaste{Start=true}` 受信時、内部ループで EventKey を集約
   - `EventPaste{Start=false}` 受信時、`eventPasteContent` イベントを返す
   - 500ms タイムアウトで部分 flush (巨大ペースト対策)
3. `handleEvent()` に `eventPasteContent` ケースを追加
4. `Gui` に `OnPasteContent func(text string) error` コールバックを追加
5. `IsPasting` フラグは維持 (非 Editable view のガード用)

**リスク:** pollEvent はシングルゴルーチン。集約中はリサイズ等のイベントが遅延するが、
ペーストは通常 100ms 以内に完了するため許容範囲。

### Phase 2: ESC[200~ フォールバック検出 (pollEvent 内)

**ファイル:** `third_party_gocui/tcell_driver.go`

1. pollEvent 内に ESC シーケンスバッファを追加
2. EventKey(Esc) → 後続の '[', '2', '0', '0', '~' をパターンマッチ
3. 一致 → Phase 1 と同じ集約ループに遷移 (EventKey ベース)
4. 不一致 → バッファ済みイベントを通常の GocuiEvent として返す
5. ESC[201~ で集約終了、eventPasteContent を返す
6. タイムアウト: 10ms (escTimeout と同じ) で standalone ESC と判定

**重要:** tcell が EventPaste を正しく送る場合は Phase 1 のパスを使用。
フォールバックは EventPaste が来ない環境 (tmux popup) 専用。

### Phase 3: App 層 — OnPasteContent の接続

**ファイル:** `internal/gui/app.go`, `internal/gui/state.go`

1. App 初期化時に `gui.OnPasteContent` にハンドラを登録
2. ハンドラ内容: fullscreen active なら `forwardPaste(text)` を呼ぶ
3. fullscreen でなければ無視 (将来: input view へのペースト対応も可)

### Phase 4: inputEditor の簡素化

**ファイル:** `internal/gui/input.go`, `internal/gui/app.go`

1. inputEditor から以下を削除:
   - `inPaste`, `nativePaste`, `pasteBuf`, `pasteMu` (ペースト状態)
   - `escBuf`, `escTimer`, `escGen` (ESC シーケンス検出)
   - `pasteNotify` (watchdog 通知)
   - `handleEscSeq()`, `handlePaste()`, `appendPasteChar()`
   - `flushPaste()`, `drainPaste()`, `notifyWatchdog()`
   - `startEscTimer()`, `cancelEscTimer()`
2. `Edit()` を簡素化: `forwardAny()` への直接ディスパッチのみ
3. App から watchdog ゴルーチン (`startPasteWatchdog`) を削除
4. ESC キーは `forwardAny()` で即座に "Escape" として転送
   (pollEvent 側で ESC[200~ は既に処理済みのため、Edit() に到達する ESC は全て standalone)

### Phase 5: テスト

1. `input_test.go` の既存ペーストテストを新アーキテクチャに合わせて書き換え
2. 新規テスト:
   - `TestPollEventPasteAccumulation` — EventPaste{Start} + EventKey... + EventPaste{End} → 単一 eventPasteContent
   - `TestPollEventFallbackESCDetection` — ESC[200~ raw バイト → eventPasteContent
   - `TestPollEventESCTimeout` — standalone ESC → 通常の eventKey
   - `TestPollEventLargePaste` — 10KB ペースト → タイムアウト flush → 複数 eventPasteContent
   - `TestOnPasteContentForward` — OnPasteContent → forwardPaste 経路
3. `go test -race ./internal/... ./third_party_gocui/...`

## Dependencies

- なし (外部ライブラリ追加不要)
- tcell の既存 API のみ使用

## Risks

| Level | Risk | Mitigation |
|-------|------|------------|
| HIGH | pollEvent 集約中のイベント遅延 | 500ms タイムアウトで部分 flush、ペースト完了後は即座に通常モードに復帰 |
| MEDIUM | ESC フォールバック検出の誤検知 | 10ms タイムアウトで standalone ESC を判定、pasteStartSuffix の完全一致を要求 |
| MEDIUM | third_party_gocui の変更による副作用 | IsPasting フラグと execKeybindings ガードは維持、既存 Editor 互換性を保つ |
| LOW | 非常に遅いターミナルでのタイムアウト | タイムアウト値を定数化し調整可能に |

## Complexity: MEDIUM-HIGH

- gocui 層の変更は慎重さが必要 (pollEvent はホットパス)
- inputEditor の大幅削除はリスクが低い (複雑さの除去)
- テストカバレッジで安全性を担保
