# Tab Focus Switching + DI Key Management

**Created**: 2026-03-22
**Branch**: feat/tmux-attach-popup

## CRITICAL Invariants

- Tab switches focus between **panels** (Sessions, Logs). Both always visible. Layout unchanged.
- "Panel" = focusable layout area. "Tab" (future) = content switcher within a panel. Do not conflate.
- `activeTabIdx` was dead code, removed. Do not resurrect.
- PanelManager owns focus state. Layout reads from PanelManager.

## Architecture

### Hierarchy

```
PanelManager          ← Tab/Shift+Tab でパネル間フォーカス切り替え
  ├── SessionsPanel   ← 将来: H/L でタブ切り替え (Sessions, History, ...)
  │     ├── Tab[0] "Sessions"  (default, 今回実装)
  │     └── Tab[1] "History"   (将来)
  └── LogsPanel
        └── Tab[0] "Logs"      (単一タブ、切り替えなし)
```

Panel はオプショナルに複数 Tab を持てる。
Tab を持たない Panel は単一コンテンツとして振る舞う (現在の実装)。
Tab 切り替えキー (H/L) は Panel.HandleKey 内で処理。Dispatcher は関与しない。

### Layer Diagram

```
Key event (gocui)
  |
  v
setupGlobalKeybindings() -- thin registration (~80 lines)
  |
  v
Dispatcher.Dispatch(ev, actions)
  |
  +-- 1. PopupHandler     (hasPopup: highest priority, consumes ALL keys)
  +-- 2. FullScreenHandler (isFullScreen: special keys only)
  +-- 3. PanelManager      -> ActivePanel().HandleKey(ev, actions)
  |      +-- SessionsPanel (focusIdx == 0)
  |      |     └── activeTab.HandleKey() (将来: タブが複数ある場合)
  |      +-- LogsPanel     (focusIdx == 1)
  +-- 4. GlobalHandler     (q, Ctrl+C, Tab, Shift+Tab, p, Ctrl+\)
```

### Package Structure

```
internal/gui/
  keyhandler/
    types.go         -- KeyEvent, HandlerResult, KeyHandler
    actions.go       -- AppActions interface (DI boundary)
    panel.go         -- Panel interface, PanelManager
    sessions.go      -- SessionsPanel
    logs.go          -- LogsPanel
    popup.go         -- PopupHandler
    fullscreen.go    -- FullScreenHandler
    global.go        -- GlobalHandler
  keydispatch/
    dispatcher.go    -- Dispatcher, Dispatch, ActiveOptionsBar
  app.go             -- add dispatcher, panelManager, logsScrollY, quitRequested
  app_actions.go     -- AppActions implementation on *App
  keybindings.go     -- 400 -> ~80 lines
  layout.go          -- dynamic options bar, focus highlight
```

## Key Conflict Resolution Table

| Key | Sessions | Logs | Popup | FullScreen |
|-----|----------|------|-------|------------|
| j/k | cursor up/down | scroll up/down | scroll/focus | forwarded |
| Arrow up/down | cursor up/down | scroll up/down | focus next/prev | forwarded |
| n | new session | unhandled | reject popup | forwarded |
| d | delete session | unhandled | consumed | forwarded |
| Enter | attach (suspend+tmux) | unhandled | consumed | forwarded |
| r | enter fullscreen | unhandled | consumed | forwarded |
| R | rename | unhandled | consumed | forwarded |
| D | purge orphans | unhandled | consumed | forwarded |
| G/g | unhandled | end/top | consumed | forwarded |
| y/a/Y | unhandled | unhandled | accept/allow/all | forwarded |
| Tab | focus next (global) | focus next (global) | consumed | forwarded via Editor |
| Shift+Tab | focus prev (global) | focus prev (global) | consumed | forwarded via Editor |
| q | quit (global) | quit (global) | consumed | forwarded via Editor |
| p | unsuspend (global) | unsuspend (global) | consumed | forwarded via Editor |
| Esc | no-op | no-op | suspend popup | forwarded |
| Ctrl+C | quit (always) | quit (always) | quit | quit |
| Ctrl+\ | quit (main) | quit (main) | no-op | exit fullscreen |
| Ctrl+D | no-op | no-op | no-op | exit fullscreen |
| Mouse scroll | no-op | no-op | no-op | scroll fullscreen |

## Interfaces

### AppActions (keyhandler/actions.go)

```go
type AppActions interface {
    HasPopup() bool
    IsFullScreen() bool
    Mode() int  // 0=Main, 1=Diff, 2=Tool

    MoveCursorDown()
    MoveCursorUp()
    CreateSession()
    DeleteSession()
    AttachSession()
    EnterFullScreen()
    StartRename()
    PurgeOrphans()

    DismissPopup(c choice.Choice)
    DismissAllPopups(c choice.Choice)
    SuspendPopups()
    UnsuspendPopups()
    PopupFocusNext()
    PopupFocusPrev()
    PopupScrollDown()
    PopupScrollUp()

    ExitFullScreen()
    ForwardSpecialKey(tmuxKey string)
    ForwardRuneKey(ch rune)

    LogsScrollDown()
    LogsScrollUp()
    LogsScrollToEnd()
    LogsScrollToTop()

    Quit()
}
```

### Panel (keyhandler/panel.go)

```go
// Panel is a focusable area. Tab/Shift+Tab switches between panels.
type Panel interface {
    Name() string       // gocui view name ("sessions", "logs")
    Label() string      // active tab label (or panel name if no tabs)
    HandleKey(ev KeyEvent, actions AppActions) HandlerResult
    OptionsBar() string

    // Tab support (optional). Panels with one tab return fixed values.
    TabCount() int      // number of tabs (1 = no tab switching)
    TabIndex() int      // active tab index
    TabLabels() []string // all tab labels for title bar rendering
}
```

単一タブの Panel は `TabCount()=1` を返す。layout は `TabCount()>1` のときだけタブバーを描画。
Tab 内切り替えキー (例: H/L) は Panel.HandleKey 内で処理し、Dispatcher は関与しない。

### PanelManager (keyhandler/panel.go)

```go
type PanelManager struct {
    panels   []Panel
    focusIdx int
}

func NewPanelManager(panels ...Panel) *PanelManager
func (pm *PanelManager) ActivePanel() Panel
func (pm *PanelManager) FocusNext()
func (pm *PanelManager) FocusPrev()
func (pm *PanelManager) Panels() []Panel
func (pm *PanelManager) FocusIdx() int
func (pm *PanelManager) PanelCount() int
```

## Implementation Phases

### Phase 1: New Packages (no App changes, testable in isolation)

| File | Content | Lines |
|------|---------|-------|
| `keyhandler/types.go` | KeyEvent, HandlerResult, KeyHandler | ~25 |
| `keyhandler/actions.go` | AppActions interface | ~50 |
| `keyhandler/panel.go` | Panel interface, PanelManager | ~65 |
| `keyhandler/sessions.go` | j/k/Arrow/n/d/Enter/r/R/D | ~50 |
| `keyhandler/logs.go` | j/k/Arrow/G/g | ~40 |
| `keyhandler/popup.go` | y/a/n/Y/Esc/j/k/Arrow + consume ALL | ~55 |
| `keyhandler/fullscreen.go` | Ctrl+\/Ctrl+D/Esc/Arrow/Enter | ~40 |
| `keyhandler/global.go` | Ctrl+C/q/Tab/BTab/p/Ctrl+\ | ~45 |
| `keydispatch/dispatcher.go` | Dispatch + ActiveOptionsBar | ~65 |

### Phase 2: App Integration

| File | Change |
|------|--------|
| `app.go` | Add `dispatcher`, `panelManager`, `logsScrollY`, `quitRequested`. `initDispatcher()` |
| `app_actions.go` (new) | AppActions on *App. `Quit()` sets `quitRequested` flag |
| `keybindings.go` | 400 -> ~80 lines. `dispatchRune(ch)` / `dispatchKey(key)` factories. Check `quitRequested` after Dispatch |
| `layout.go` | `FrameColor = Cyan` for active panel. `v4.Clear()` + `ActiveOptionsBar()` every frame. `SetCurrentView(pm.ActivePanel().Name())`. `renderServerLog` scroll support |

### Phase 3: Tests

Unit tests per handler + PanelManager + Dispatcher priority + integration.

### Phase 4: Cleanup

Evaluate `keyRegistry` removal. Verify `popupViewName` bindings with `tool-popup-N` views.

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Quit() can't return gocui.ErrQuit | High | `quitRequested` flag checked after Dispatch |
| FullScreen rune forwarding broken | High | inputEditor unchanged, dispatcher handles special keys only |
| Popup keys not firing on popup view | Medium | Dual registration (global + popupViewName) |
| Logs scroll OOB | Low | Clamp + sentinel -1 for follow tail |

## Extensibility

### New Panel

Implement Panel + register in `initDispatcher`. No Dispatcher/keybindings/layout changes.

### Panel 内 Tab 追加 (将来)

Panel 実装内で `tabs []TabContent` + `activeTab int` を持つ。
H/L キーを HandleKey 内で処理し activeTab を切り替える。
`Label()` は `tabs[activeTab].Label` を返す。
`TabLabels()` は全タブのラベルを返し、layout がタブバーを描画。
Dispatcher, PanelManager, GlobalHandler の変更不要。

```
例: SessionsPanel に History タブ追加
  SessionsPanel.tabs = [SessionsTab, HistoryTab]
  H -> activeTab-- , L -> activeTab++
  Title: " [Sessions]  History "  (activeTab=0 の場合)
```

## Success Criteria

- [ ] Tab/Shift+Tab cycles focus: Sessions <-> Logs
- [ ] Both panels always visible
- [ ] Focused panel: FrameColor = Cyan
- [ ] Options bar dynamic per panel
- [ ] Sessions: j/k cursor, n/d/Enter/r/R/D
- [ ] Logs: j/k scroll, G/g jump
- [ ] Popup consumes ALL keys when visible
- [ ] FullScreen rune forwarding unchanged
- [ ] keybindings.go < 100 lines
- [ ] All existing tests pass
- [ ] New tests for keyhandler + keydispatch
