<!-- Generated: 2026-03-30 | Files scanned: 41 gui files | Token estimate: ~700 -->

# Frontend (TUI)

## GUI Package (internal/gui/)

gocui-based terminal UI with keybinding-driven navigation.

## View Hierarchy

```
Main Layout (layout.go, 672 lines)
+-- projectListView     (project tree sidebar)
+-- sessionListView     (session list per project)
+-- previewView         (live session preview)
+-- statusBarView       (bottom status bar)
+-- Overlays:
    +-- popupView       (permission prompt dialog)
    +-- diffView        (diff viewer popup)
    +-- worktreeDialog  (worktree selection)
    +-- confirmView     (confirmation dialog)
    +-- inputView       (text input)
```

## Key Input Pipeline

```
1. View-specific bindings (popup, dialog)
2. Editor.Edit() -- Editable=true views only
3. Global bindings -- rune keys skipped in Editable views
```

Dispatched via:
```
keydispatch/dispatcher.go  -- key event routing
keyhandler/handler.go      -- per-view handlers
keyhandler/panel.go        -- panel navigation
keyhandler/plugins.go      -- plugin view keys
keymap/registry.go (491 lines) -- configurable keybinding registry
```

## Core GUI Files

```
app.go            (352 lines) -- App state machine, lifecycle
app_actions.go    (719 lines) -- session/panel action handlers
layout.go         (672 lines) -- view creation, layout calculation
render.go         (294 lines) -- main rendering pipeline
keybindings.go    (321 lines) -- default keybinding setup
popup.go          (249 lines) -- permission prompt overlay
fullscreen.go     (234 lines) -- direct keyboard forwarding mode
input.go          (211 lines) -- text input component
state.go          -- app state (focused view, sessions, projects)
```

## Presentation Layer (gui/presentation/)

```
sessions.go  -- session list formatting
diff.go      -- diff rendering
style.go     -- ANSI styling
tool.go      -- tool notification display
```

## Adapters (cmd/lazyclaude/)

GUI interfaces are satisfied by adapters in root.go:
```
SessionProvider -> sessionAdapter (wraps session.Manager)
PluginProvider  -> pluginAdapter  (wraps plugin.Manager)
MCPProvider     -> mcpAdapter     (wraps mcp.Manager)
```
