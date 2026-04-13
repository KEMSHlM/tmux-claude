# Diff Popup Format Overhaul

## Goal

Reformat the DiffPopup display to match the ToolPopup (Edit) style: clean inline format without raw git headers or dual line numbers. Also enable j/k scroll for all popup types and add arrow-key switch hints.

## Proposed Format

```
+----------------------------------------------+
| Diff: server.go                              |  <- title (frame)
|                                              |
|  File: internal/server.go                    |  <- dim label
|                                              |
|  @@ func (s *Server) Start()                 |  <- cyan, function name only
|                                              |
|    ctx, cancel := context.WithCancel(s.ctx)  |  <- plain (context)
|    defer cancel()                            |  <- plain (context)
|  - return s.listen(ctx)                      |  <- red
|  + if err := s.listen(ctx); err != nil {     |  <- green
|  +     return fmt.Errorf("listen: %w", err)  |  <- green
|  + }                                         |  <- green
|  + return nil                                |  <- green
|    }                                         |  <- plain (context)
|                                              |
+----------------------------------------------+
 1 accept  2 allow  3 reject  j/k scroll  up/dn switch [1/3]
```

## Steps

### Step 1: New compact diff formatter (`presentation/diff.go`)

Add `FormatDiffLineClean(dl DiffLine) string`:
- `DiffHeader` -> skip (return empty; filtered by caller)
- `DiffHunk` -> `@@ <function-name>` (extract text after the closing `@@`, trim range info)
- `DiffAdd` -> `  + <content>` (2-space indent + plus + space + content)
- `DiffDel` -> `  - <content>` (2-space indent + minus + space + content)
- `DiffContext` -> `    <content>` (4-space indent, aligns with `  + `)

Add `ExtractHunkLabel(hunkLine string) string`:
- Parse `@@ -old,len +new,len @@ <function context>` 
- Return `@@ <function context>` (drop line-number range)
- If no function context, return `@@`

Add corresponding unit tests.

### Step 2: Update `DiffPopup.ensureCache()` (`popup_types.go`)

Replace the current `FormatDiffLine(dl, 4)` call with the new format:

1. Prepend a `File:` line using `p.notification.OldFilePath` (not from diff headers):
   - New DiffLineKind: `DiffFilePath` (must NOT reuse `DiffHeader` -- `DiffHeader` renders bold, `DiffFilePath` must render dim)
   - Format: `"  File: " + relativePath` where relativePath is derived from OldFilePath
2. Skip `DiffHeader` lines entirely (diff --git, ---, +++)
3. For `DiffHunk`: use `ExtractHunkLabel`, add blank line before it
4. For `DiffAdd`/`DiffDel`/`DiffContext`: use `FormatDiffLineClean`
5. Handle `\\ No newline at end of file` marker: classify as `DiffContext` and display as-is (dim)
6. Trim trailing empty `DiffContext` line: `ParseUnifiedDiff()` emits a trailing empty segment from `strings.Split`; strip it before formatting to avoid double blank lines
7. Add blank line (empty DiffContext) after file path and before first hunk

Output `lines[]` and `kinds[]` arrays remain the same structure. `DiffFilePath` is a new kind value.

### Step 3: Update `renderDiffPopup` (`render.go`)

Add rendering for the new `DiffFilePath` kind:
```go
case presentation.DiffFilePath:
    fmt.Fprintf(v, "\x1b[2m%s\x1b[0m\n", line)  // dim
```

Blank lines (kind=DiffContext, empty content) render as plain `\n` -- no change needed.

### Step 4: Enable scroll for all popup types (`app_actions.go`, `render.go`)

**4a. Remove the `p.IsDiff()` guard** from `PopupScrollDown()` and `PopupScrollUp()`:

Before:
```go
if p != nil && p.IsDiff() {
```
After:
```go
if p != nil {
```

**4b. Update `renderToolPopup()` to honor `ScrollY()`** (`render.go`):

Currently `renderToolPopup()` prints all lines without slicing. Add viewport-based slicing identical to `renderDiffPopup()`:

```go
func renderToolPopup(v *gocui.View, p Popup) {
    v.Title = p.Title()
    lines := p.ContentLines()
    _, viewH := v.Size()
    visibleLines := viewH - 1

    start := p.ScrollY()
    end := start + visibleLines
    if end > len(lines) { end = len(lines) }
    if start < 0 { start = 0 }

    for i := start; i < end; i++ {
        fmt.Fprintln(v, lines[i])
    }
}
```

**4c. Fix hardcoded viewport height `20`** in `PopupScrollDown()`:

The current scroll bound uses `maxScrollFor(len(p.ContentLines()), 20)` with hardcoded `20`. This should use the same viewport height that the renderer computes. Options:
- Store last-known viewport height in the popup (set during layout)
- Or use `p.MaxScroll(viewportHeight)` where viewportHeight comes from the popup view size

Recommended: store `lastViewportHeight` on the popup during `layoutToolPopup()` and use it in scroll actions. Add `ViewportHeight() int` and `SetViewportHeight(h int)` to the `Popup` interface.

### Step 5: Add switch hint to options bar (`keymap/registry.go`, `popup.go`)

In `keymap/registry.go` - add hint labels to FocusNext:
```go
r.Register(ActionDef{
    Action:      ActionPopupFocusNext,
    Bindings:    []KeyBinding{{Key: gocui.KeyArrowDown}},
    Scope:       ScopePopup,
    HintLabel:   "switch",
    HintKey:     "up/dn",
    Description: "Focus next notification",
    DocSection:  "popup_navigate",
})
```
Remove HintLabel/HintKey from FocusPrev (avoid duplicate hint display).

In `popup.go` - show switch hint only when multiple popups:
```go
case keymap.ActionPopupFocusNext:
    if visible <= 1 {
        continue
    }
```

Remove the `!p.IsDiff()` condition from scroll hint skip logic, so `j/k scroll` shows for all popup types.

### Step 6: Update tests

- `presentation/diff_test.go`: Add tests for `FormatDiffLineClean` and `ExtractHunkLabel`
- `popup_types_test.go` (if exists): Verify DiffPopup.ContentLines() produces the new format
- `app_actions_test.go` / `popup_controller_test.go`: Verify scroll works for ToolPopup (not just DiffPopup)
- `keymap/registry_test.go`: Update hint expectations if any

## Files to Modify

| File | Changes |
|------|---------|
| `internal/gui/presentation/diff.go` | Add `FormatDiffLineClean`, `ExtractHunkLabel`, `DiffFilePath` kind |
| `internal/gui/presentation/diff_test.go` | Tests for new functions |
| `internal/gui/popup_types.go` | Rewrite `ensureCache()`, add `ViewportHeight()`/`SetViewportHeight()` to both popup types |
| `internal/gui/render.go` | Add `DiffFilePath` rendering case, update `renderToolPopup()` for scroll |
| `internal/gui/app_actions.go` | Remove `IsDiff()` guard, use stored viewport height for scroll bounds |
| `internal/gui/keymap/registry.go` | Add HintLabel/HintKey to FocusNext |
| `internal/gui/popup.go` | Adjust hint conditions, set viewport height during layout |
| `internal/server/server.go` | Extract file_path/content from Write input in dispatchToolNotification |
| `internal/server/server_test.go` or `server_broker_test.go` | Test Write notifications populate diff fields |

### Step 7: Enable DiffPopup for Write tool notifications (`server.go`)

**Bug**: `dispatchToolNotification()` creates `ToolNotification` without `OldFilePath`/`NewContents`, so Write tool always displays as ToolPopup (only shows file path, no content).

**Fix**: In `dispatchToolNotification()`, after creating the base notification, parse `input` JSON for Write tool and populate diff fields:

```go
if toolName == "Write" {
    var parsed map[string]any
    if err := json.Unmarshal([]byte(input), &parsed); err == nil {
        if fp, ok := parsed["file_path"].(string); ok && fp != "" {
            n.OldFilePath = fp
        }
        if content, ok := parsed["content"].(string); ok {
            n.NewContents = content
        }
    }
}
```

This enables `IsDiff()` to return true for Write, routing the notification to `DiffPopup` which:
- For existing files: shows git diff between current file and new content
- For new files: shows synthetic diff with all lines as additions (already handled in `generateDiffFromContents`)

**Note**: Edit tool is NOT included here because Edit's ToolPopup already shows old_string/new_string clearly. Converting Edit to DiffPopup would require computing the full new file content (apply edit to old file), which is more complex and not needed now.

### Step 8: Update tests for Write DiffPopup routing

- `server_test.go` or `server_broker_test.go`: Verify Write notifications have `OldFilePath`/`NewContents` set
- `popup_types_test.go`: Verify Write notification routes to DiffPopup

## Out of Scope

- Side-by-side diff view
- Syntax highlighting within diff lines
- New file detection (`/dev/null` header) -- keep existing behavior
- ToolPopup (Edit) display format -- already in desired format
