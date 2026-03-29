# Lazygit Panel/View Management Architecture

## Summary

Lazygit organizes its UI through three layered abstractions: **views** (gocui rendering buffers), **windows** (named screen sections that host one or more views), and **contexts** (Go structs that own state, keybindings, and rendering logic for a view). Side panels are each a separate window that may contain multiple views selectable via tabs. The active side context drives what appears in the main panel by triggering `HandleRenderToMain` on focus.

---

## 1. Core Abstractions

### View (gocui layer)
A `*gocui.View` is the lowest-level unit: an internal byte buffer that gets rendered to the terminal each frame. Views are named strings (e.g., `"files"`, `"branches"`, `"commits"`). They know nothing about application logic.

### Window
A window is a named **screen region** that displays exactly one view at a time. The window name is typically the name of its default view (e.g., the `"branches"` window normally shows the `"localBranches"` view). When you push an enter on a stash entry, the `commitFiles` view can appear in the `"commits"` window — same window, different view.

The current mapping from window name to active view name is maintained in:

```go
// pkg/gui/gui.go  GuiRepoState
WindowViewNameMap *utils.ThreadSafeMap[string, string]
```

This map is initialized by iterating all flattened contexts and setting `windowName -> viewName`. At layout time, the layout function calls `helpers.Window.GetViewNameForWindow(windowName)` to decide which view's `Visible` flag is true.

### Context
A context is a Go struct that wraps a `*gocui.View` and adds:
- **State** (selected line, scroll offset, filter string, etc.)
- **Keybinding functions** (`[]KeybindingsFn`)
- **Render callbacks** (`HandleRender`, `HandleRenderToMain`, `HandleFocus`, `HandleFocusLost`)
- **Identity** (`ContextKey`, `ContextKind`, `WindowName`)

The central interface is in `pkg/gui/types/context.go`:

```go
type Context interface {
    IBaseContext
    HandleFocus(opts OnFocusOpts)
    HandleFocusLost(opts OnFocusLostOpts)
    FocusLine(scrollIntoView bool)
    HandleRender()
    HandleRenderToMain()
}

type IBaseContext interface {
    GetKind() ContextKind   // SIDE_CONTEXT, MAIN_CONTEXT, POPUP, ...
    GetViewName() string
    GetWindowName() string
    SetWindowName(string)
    GetKey() ContextKey     // unique string like "localBranches"
    IsFocusable() bool
    // ... keybinding and event registration
}
```

### ContextKind enum

```go
// pkg/gui/types/context.go
type ContextKind int

const (
    SIDE_CONTEXT     ContextKind = iota  // files, branches, commits, stash, status
    MAIN_CONTEXT                         // diff/log display areas
    PERSISTENT_POPUP                     // commit message editor
    TEMPORARY_POPUP                      // menus, confirmations
    EXTRAS_CONTEXT                       // command log panel
    GLOBAL_CONTEXT                       // global keybindings only
    DISPLAY_CONTEXT                      // read-only display (options bar, app status)
)
```

---

## 2. How Side Panels Are Registered

All contexts are created in a single constructor `NewContextTree` in `pkg/gui/context/setup.go`. Each context is instantiated with a `NewBaseContextOpts` that declares its window and kind:

```go
// Example: BranchesContext  (pkg/gui/context/branches_context.go)
NewBaseContextOpts{
    Kind:       types.SIDE_CONTEXT,
    WindowName: "branches",
    Key:        LOCAL_BRANCHES_CONTEXT_KEY,  // = "localBranches"
    Focusable:  true,
    View:       c.Views().LocalBranches,
    NeedsRerenderOnWidthChange: NEEDS_RERENDER_ON_WIDTH_CHANGE_WHEN_WIDTH_CHANGES,
}

// Example: WorkingTreeContext  (pkg/gui/context/working_tree_context.go)
NewBaseContextOpts{
    Kind:       types.SIDE_CONTEXT,
    WindowName: "files",
    Key:        FILES_CONTEXT_KEY,           // = "files"
    Focusable:  true,
    View:       c.Views().Files,
}
```

The five **side windows** and the views that live in each:

| Window name   | Views (contexts) inside                                      |
|---------------|--------------------------------------------------------------|
| `"status"`    | status view                                                  |
| `"files"`     | files, worktrees, submodules                                 |
| `"branches"`  | localBranches, remotes, remoteBranches, tags                 |
| `"commits"`   | commits, reflogCommits                                       |
| `"stash"`     | stash, (commitFiles when navigating stash entries)           |

These five windows are returned by `helpers.Window.SideWindows()`, which the `JumpToSideWindowController` iterates to bind keys 1–5.

The full catalogue of context keys is in `pkg/gui/context/context.go`:

```go
const (
    FILES_CONTEXT_KEY              types.ContextKey = "files"
    LOCAL_BRANCHES_CONTEXT_KEY     types.ContextKey = "localBranches"
    REMOTES_CONTEXT_KEY            types.ContextKey = "remotes"
    REMOTE_BRANCHES_CONTEXT_KEY    types.ContextKey = "remoteBranches"
    TAGS_CONTEXT_KEY               types.ContextKey = "tags"
    LOCAL_COMMITS_CONTEXT_KEY      types.ContextKey = "commits"
    REFLOG_COMMITS_CONTEXT_KEY     types.ContextKey = "reflogCommits"
    STASH_CONTEXT_KEY              types.ContextKey = "stash"
    // ... and many more
)
```

The `ContextTree` struct is the central registry that holds typed references to every context:

```go
// pkg/gui/context/context.go
type ContextTree struct {
    Global       types.Context
    Status       types.Context
    Files        *WorkingTreeContext
    Branches     *BranchesContext
    Tags         *TagsContext
    LocalCommits *LocalCommitsContext
    Remotes      *RemotesContext
    Stash        *StashContext
    Normal       *MainContext       // primary diff view
    NormalSecondary *MainContext    // secondary diff view (split pane)
    Staging      *PatchExplorerContext
    // ...
}

// Flatten() returns all contexts in initial stacking order within windows
func (self *ContextTree) Flatten() []types.Context { ... }
```

`ContextTree` is stored on `GuiRepoState.Contexts` and accessed everywhere via `self.c.Context()`.

---

## 3. Context Stack and Switching Mechanism

### ContextMgr (context stack)

`pkg/gui/context.go` defines `ContextMgr`:

```go
type ContextMgr struct {
    ContextStack []types.Context  // LIFO stack
    sync.RWMutex
    gui *Gui
    allContexts *allContexts
}
```

The three core stack operations (from `pkg/gui/types/context.go` `IContextMgr` interface):

```go
Push(context Context, opts OnFocusOpts)   // add to stack + fire HandleFocus
Pop()                                      // remove top + fire HandleFocusLost on old, HandleFocus on new
Replace(context Context)                   // swap top without returning to parent
```

**Push behavior by ContextKind:**

| Kind pushed       | Effect on stack                                                |
|-------------------|----------------------------------------------------------------|
| `SIDE_CONTEXT`    | All other SIDE_CONTEXTs removed; main contexts stay underneath |
| `MAIN_CONTEXT`    | Other MAIN_CONTEXTs removed; side context preserved            |
| `TEMPORARY_POPUP` | Stacked on top; popped on escape                               |
| `PERSISTENT_POPUP`| Stacked on top; survives unrelated context changes             |

This means pressing Escape always returns to a sensible parent: from a popup back to the side panel that spawned it.

### Number-Key Panel Jump (1–5)

`pkg/gui/controllers/jump_to_side_window_controller.go`:

```go
type JumpToSideWindowController struct {
    baseController
    c           *ControllerCommon
    nextTabFunc func() error   // called when same window key pressed again
}

func (self *JumpToSideWindowController) GetKeybindings(opts KeybindingsOpts) []*Binding {
    windows := self.c.Helpers().Window.SideWindows() // ["status","files","branches","commits","stash"]

    return lo.Map(windows, func(window string, index int) *Binding {
        return &Binding{
            ViewName: "",                                   // global binding
            Key:      opts.GetKey(opts.Config.Universal.JumpToBlock[index]),  // default: "1".."5"
            Handler:  opts.Guards.NoPopupPanel(self.goToSideWindow(window)),
        }
    })
}

func (self *JumpToSideWindowController) goToSideWindow(window string) func() error {
    return func() error {
        sideWindowAlreadyActive := self.c.Helpers().Window.CurrentWindow() == window
        if sideWindowAlreadyActive && self.c.UserConfig().Gui.SwitchTabsWithPanelJumpKeys {
            return self.nextTabFunc()   // cycle to next tab within the window
        }
        context := self.c.Helpers().Window.GetContextForWindow(window)
        self.c.Context().Push(context, types.OnFocusOpts{})
        return nil
    }
}
```

`GetContextForWindow(window)` looks up `WindowViewNameMap[window]` to get the currently active view name for that window, then resolves it to a context.

### Tab Switching within a Window

Each window with multiple tabs displays a tab bar at the top. "Each tab ... actually has a corresponding view which we bring to the front upon changing tabs" (Codebase Guide).

Tab switching updates `WindowViewNameMap[windowName] = newViewName`, then pushes the context for the new view. The layout function consults the map each frame to set `view.Visible`.

```go
// TabView struct (pkg/gui/context/context.go)
type TabView struct {
    Tab      string   // display name shown in the tab bar
    ViewName string   // the view to make visible when tab selected
}
```

---

## 4. How the Main Panel Updates

The main panel (right side, diff/log display) is driven by the currently focused side context through a callback chain:

1. User selects a line in a side panel (e.g., moves cursor in commits list).
2. The list controller fires `HandleFocus` on the side context, or the selection-change handler triggers a re-render.
3. The side context calls `HandleRenderToMain()`.
4. `HandleRenderToMain` on list-based contexts invokes the registered `onRenderToMainFn` callback, which was wired up during context initialization in `setup.go` to a presentation function.
5. The presentation function reads the currently selected model object from `GuiRepoState.Model` (e.g., the selected `*models.Commit`) and writes diff/log content into the `Normal` (main) view's buffer.

The `Context` interface exposes this as:

```go
HandleRenderToMain()  // implemented by every context; no-op for contexts with no main-panel content
```

For list contexts, the concrete implementation in `ListContextTrait` calls the registered function:

```go
// pkg/gui/context/list_context_trait.go
func (self *ListContextTrait) HandleRenderToMain() {
    if self.onRenderToMainFn != nil {
        self.onRenderToMainFn()
    }
}
```

The `onRenderToMainFn` for the branches context, for example, renders the branch's log; for the files context, it renders the file's diff.

The main panel itself is a `*MainContext` with `Kind: MAIN_CONTEXT` and `WindowName: "main"`. The secondary split pane is `NormalSecondary` with `WindowName: "secondary"`. Staging and patch-building views are `PatchExplorerContext` instances that overlay the main area.

---

## 5. Panel Group Organization in Code

### Dependency hierarchy

```
controllers  (keybindings + handlers)
    |
helpers      (shared logic between controllers)
    |
contexts     (state + rendering; can access views)
    |
views        (gocui buffers; no application logic)
```

Controllers cannot call other controllers' methods directly. Shared logic is extracted into helpers in `pkg/gui/controllers/helpers/`.

### File map

| File | Responsibility |
|------|---------------|
| `pkg/gui/context/context.go` | `ContextTree` struct, all `ContextKey` constants, `Flatten()`, `TabView` |
| `pkg/gui/context/setup.go` | `NewContextTree()` — constructs every context with its window/kind config |
| `pkg/gui/context/base_context.go` | `BaseContext` struct and `NewBaseContextOpts` |
| `pkg/gui/types/context.go` | `ContextKind` enum, `Context` interface, `IContextMgr` interface |
| `pkg/gui/context.go` | `ContextMgr` implementation (Push/Pop/Replace, focus lifecycle) |
| `pkg/gui/gui.go` | `Gui` struct, `GuiRepoState` (holds `ContextMgr`, `ContextTree`, `WindowViewNameMap`) |
| `pkg/gui/layout.go` | `layout()` — sets `view.Visible` per frame based on `WindowViewNameMap` |
| `pkg/gui/controllers/jump_to_side_window_controller.go` | Number-key 1–5 side panel switching |
| `pkg/gui/controllers/helpers/window_arrangement_helper.go` | `SideWindows()`, `GetContextForWindow()`, `CurrentWindow()` |
| `pkg/gui/context/branches_context.go` | `BranchesContext`, window `"branches"`, kind `SIDE_CONTEXT` |
| `pkg/gui/context/working_tree_context.go` | `WorkingTreeContext`, window `"files"`, kind `SIDE_CONTEXT` |
| `pkg/gui/context/local_commits_context.go` | `LocalCommitsContext`, window `"commits"`, kind `SIDE_CONTEXT` |
| `pkg/gui/context/list_context_trait.go` | Shared list behavior including `HandleRenderToMain` dispatch |

---

## Key References

- [Codebase Guide (official dev docs)](https://github.com/jesseduffield/lazygit/blob/master/docs/dev/Codebase_Guide.md)
- [pkg/gui/types/context.go — Context interfaces and ContextKind](https://raw.githubusercontent.com/jesseduffield/lazygit/master/pkg/gui/types/context.go)
- [pkg/gui/context/context.go — ContextTree and ContextKey constants](https://raw.githubusercontent.com/jesseduffield/lazygit/master/pkg/gui/context/context.go)
- [pkg/gui/controllers/jump_to_side_window_controller.go — number-key switching](https://raw.githubusercontent.com/jesseduffield/lazygit/master/pkg/gui/controllers/jump_to_side_window_controller.go)
- [DeepWiki: UI and View Management](https://deepwiki.com/jesseduffield/lazygit/3-user-interface-and-view-management)
- [DeepWiki: Context System](https://deepwiki.com/jesseduffield/lazygit/3.1-context-system)
- [pkg.go.dev: gui package (ContextMgr, GuiRepoState types)](https://pkg.go.dev/github.com/jesseduffield/lazygit/pkg/gui)
