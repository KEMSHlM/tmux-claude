# VHS by Charmbracelet: Terminal Recording Tool

## One-Paragraph Summary

VHS is a Go-based CLI tool that records terminal sessions as GIFs, MP4s, or WebM videos by executing declarative `.tape` script files. Rather than recording pixels from a running terminal emulator, VHS spins up a headless Chromium browser, connects it to a ttyd (web-based terminal) process, and drives the terminal by injecting keystrokes through browser automation (go-rod). This architecture means VHS produces pixel-perfect, font-rendered output but carries heavyweight dependencies (Chromium, ttyd, FFmpeg). It has limited native support for assertions, no knowledge of tmux sessions, and cannot capture an already-running terminal---it only controls terminals it spawns itself.

---

## 1. What Is VHS and How Does It Work

### Purpose

VHS is described as "Write terminal GIFs as code." The primary use cases are:

- Creating reproducible demo GIFs for CLI tool documentation
- Generating consistent CI-updated visuals for README files
- Light-weight integration testing via golden-file comparison of terminal text output

### Internal Architecture

The execution pipeline is:

```
.tape file
  -> Lexer / Parser (Go)
  -> Command objects
  -> VHS executor
       -> spawns ttyd (web terminal server)
       -> launches headless Chromium via go-rod
       -> connects Chromium to ttyd WebSocket
       -> injects keystrokes via browser automation
       -> captures rendered frames from xterm.js canvas
  -> FFmpeg
  -> GIF / MP4 / WebM / PNG sequence / .txt output
```

Key dependencies:

| Dependency | Role |
|---|---|
| ttyd | Web-based terminal emulator; exposes a PTY over WebSocket |
| Chromium (headless) | Renders xterm.js; captured frame-by-frame by go-rod |
| go-rod | Go browser automation library (DevTools Protocol) |
| FFmpeg | Encodes captured frames into video output |

The shell that runs inside the virtual terminal is configurable (`Set Shell bash`). VHS does not attach to any external terminal; it always owns the PTY.

---

## 2. The .tape File Format

Tape files are plain-text scripts. Each line is one command. Comments start with `#`.

### Minimal example

```
Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 600

Type "echo hello world"
Enter
Sleep 1s
```

### Full command reference

**Output / configuration**

| Command | Description |
|---|---|
| `Output <path>` | Declare output file; extension determines format (`.gif`, `.mp4`, `.webm`, `.txt`, `.ascii`, `.png`) |
| `Require <binary>` | Abort if binary is not on PATH |
| `Set <key> <value>` | Configure terminal or recording settings (see below) |
| `Env <KEY> <value>` | Set environment variable inside the shell |
| `Source <file>` | Include another tape file |

**Key `Set` settings**

| Setting | Example |
|---|---|
| `Shell` | `Set Shell "bash"` |
| `FontSize` | `Set FontSize 14` |
| `Width` / `Height` | `Set Width 1200` / `Set Height 600` |
| `Theme` | `Set Theme "Dracula"` |
| `TypingSpeed` | `Set TypingSpeed 50ms` |
| `Framerate` | `Set Framerate 24` |
| `PlaybackSpeed` | `Set PlaybackSpeed 2` |
| `WaitTimeout` | `Set WaitTimeout 30s` |
| `WaitPattern` | `Set WaitPattern /\$\s*$/` (default prompt regex) |
| `CursorBlink` | `Set CursorBlink false` |
| `WindowBar` | `Set WindowBar Colorful` |

**Input commands**

| Command | Description |
|---|---|
| `Type "<text>"` | Emulate keystrokes with configured typing speed |
| `Type@<duration> "<text>"` | Override speed for this line |
| `Enter`, `Tab`, `Space`, `Backspace`, `Escape` | Named keys |
| `Up`, `Down`, `Left`, `Right` | Arrow keys |
| `Ctrl+<char>`, `Alt+<char>`, `Shift+<key>` | Modifier combinations; e.g. `Ctrl+C` |
| `PageUp`, `PageDown`, `ScrollUp`, `ScrollDown` | Scroll commands |

**Flow control**

| Command | Description |
|---|---|
| `Sleep <duration>` | Pause; e.g. `Sleep 2s`, `Sleep 500ms` |
| `Wait` | Wait until default shell prompt regex matches on current line |
| `Wait /regex/` | Wait until regex matches on current line |
| `Wait+Screen /regex/` | Wait until regex matches anywhere on screen |
| `Wait+Line /regex/` | Explicit current-line scope (same as `Wait`) |
| `Wait@<duration> /regex/` | Override timeout for this wait |
| `Hide` / `Show` | Pause / resume frame recording |
| `Screenshot <filename>` | Capture current terminal frame to PNG |

---

## 3. Testing and Assertion Support

VHS has **no native assertion command**. Testing is done through two indirect mechanisms:

### 3a. Golden-file comparison (text output)

Declare a `.txt` or `.ascii` output alongside the GIF:

```
Output out.gif
Output out.txt
```

The `.txt` file captures the terminal buffer as plain text after all commands execute. Check it into git; CI fails if the file changes unexpectedly (standard `diff` or `git diff --exit-code`).

The `testing.go` source exposes a `SaveOutput()` method that writes the terminal buffer (one line per row) to the test output file, and a `Buffer()` method that reads all rows from the live xterm.js buffer via JavaScript evaluation.

### 3b. `Wait` as a synchronization check

`Wait` will time out (default 15 s) and cause the tape to fail if the expected regex never appears. This is a soft liveness check, not a value assertion.

Example:

```
Type "myapp --version"
Enter
Wait /v[0-9]+\.[0-9]+/
```

If the version string never appears within the timeout, vhs exits non-zero.

### What is NOT supported

- No `Assert` or `Expect` command
- No line-by-line content checking during playback
- No structured test result output (JUnit, TAP, etc.)
- No diff of terminal state against an expected string at an arbitrary point

For bubbletea-based apps, Charmbracelet provides the separate `teatest` library (not vhs), which does support `RequireEqualOutput` assertions and model-state inspection.

---

## 3.5. Error Handling and Exit Code Behavior

This section answers four specific questions about how VHS handles command failures.

### Q1: Is there a way to abort tape execution when a command exits non-zero?

**No.** VHS has no built-in mechanism to detect that a shell command typed via `Type`/`Enter` exited with a non-zero status code. The tape continues executing regardless of what the typed command returned. VHS is architecturally blind to exit codes: it only sees rendered terminal text, not PTY metadata.

The feature was explicitly proposed in issue #14 (October 2022) as a `set -e`-like option. The maintainers declined it in its general form because "sometimes demos may involve exiting with non-zero status codes to be part of the GIF." The issue was closed by implementing `Require` instead, which only checks PATH availability before execution starts.

As of March 2026, issue #653 (opened August 2025) remains open and is the active tracker for the request to abort recording when the shell process itself exits unexpectedly. There is no fix or PR merged for it.

### Q2: Is there a `Set FailOnError` or similar setting?

**No.** There is no `FailOnError`, `Set ExitOnError`, `Set StopOnError`, or equivalent setting in VHS. The complete list of `Set` settings is documented in section 2 above; none relate to command exit codes.

The only settings that affect error behavior are:
- `Set WaitTimeout <duration>` -- controls how long `Wait` will poll before giving up and failing the tape
- `Set WaitPattern /regex/` -- sets the default regex for bare `Wait`

### Q3: What happens when a command fails inside a tape?

VHS continues normally. Specifically:

1. The failing command's error output appears in the terminal (visible in the recording and in `.txt` output).
2. The shell prompt reappears (assuming the shell itself has not exited).
3. VHS moves to the next tape instruction without any knowledge that a failure occurred.
4. If the shell was configured with `set -e` (either via `Set Shell "bash -e"` or by typing `set -e` in the tape), the shell exits on the first failure. After the shell exits, the TTY shows a static blinking cursor. VHS then continues executing remaining tape instructions (typing characters, waiting, sleeping) against a dead shell, which is essentially a no-op. The tape does not abort; it runs to completion or until a `Wait` times out.

This "zombie shell" scenario is the motivation for issue #653. The only current workaround documented in that issue is to insert a `Wait` with a short timeout after critical operations:

```
Type "./build.sh"
Enter
Wait@5s /Build succeeded/
```

If the build fails and the prompt returns without the expected text, `Wait` times out and vhs exits non-zero. This provides failure detection but not graceful abort: the rest of the tape still runs (and may produce garbage output) before the timeout fires.

### Q4: Alternatives to `Wait+Screen` for error detection

There is no purpose-built error detection primitive. The available approaches, ranked by reliability:

**Option A: `Wait` with a success pattern (recommended)**

Wait for a string that only appears on success. If the command fails, the string never appears and `Wait` times out with a non-zero exit.

```
Type "go build ./..."
Enter
Wait /\$\s*$/       # waits for prompt; does NOT distinguish success from failure
```

A better form uses a success-specific pattern:

```
Type "go test ./... && echo BUILD_OK"
Enter
Wait+Screen /BUILD_OK/
```

The `&& echo BUILD_OK` idiom is the most robust workaround: it appends a sentinel only on success. If the command fails, `BUILD_OK` never appears, and `Wait+Screen` times out.

**Option B: Inspect `.txt` output after the tape**

Declare `Output out.txt` and check the captured terminal buffer with `grep` or `diff` in CI after `vhs` returns. This detects failures post-hoc rather than aborting mid-tape.

**Option C: Configure the shell with `set -e` + a narrow `Wait` timeout**

```
Type "set -e"
Enter
Set WaitTimeout 5s
Type "failing-command"
Enter
Wait /\$\s*$/
```

If `failing-command` exits non-zero and `set -e` kills the shell, the prompt never reappears, `Wait` times out after 5 s, and vhs exits non-zero. This is fragile: if the command legitimately takes longer than the timeout, it produces a false failure.

**Option D: `AwaitPrompt` (proposed, not yet merged)**

PR #708 proposes an `AwaitPrompt` command that detects shell prompt readiness more reliably than a regex wait. As of March 2026 it is open and not available in any release.

### Summary table

| Capability | Available | Mechanism |
|---|---|---|
| Abort tape on non-zero exit | No | Not implemented; issue #653 open |
| `Set FailOnError` or equivalent | No | No such setting exists |
| Detect command failure mid-tape | Partial | `Wait+Screen /sentinel/` with `&& echo sentinel` idiom |
| Detect failure after tape finishes | Yes | Grep `.txt` output in CI |
| Pre-execution binary check | Yes | `Require <binary>` |
| Shell prompt detection | No (future) | PR #708 proposes `AwaitPrompt` |

---

## 4. Applicability to gocui-based TUI Apps in tmux

### What vhs can do

VHS can spawn a shell, run any TUI application (gocui, tview, termbox, etc.) inside it, send keystrokes, wait for expected output, and capture a golden `.txt` file. It is framework-agnostic at the TUI layer---it only sees rendered terminal output.

Example tape for a gocui app:

```
Output e2e.txt
Set Shell "bash"
Set Width 220
Set Height 50
Set WaitTimeout 10s

Type "./myapp"
Enter
Wait /Welcome/

Ctrl+N
Sleep 500ms
Wait /Panel 2/

Screenshot panel2.png
Ctrl+C
```

### Critical limitation: vhs does not know about tmux

VHS spawns and owns its own PTY via ttyd. It cannot:

- Attach to an existing tmux session
- Drive a TUI running inside a user-managed tmux pane
- Use `tmux send-keys` or `capture-pane`

If the application under test requires tmux to be present (e.g., it calls `tmux` internally or reads `$TMUX`), VHS can run `tmux new-session ...` as a command inside its shell. But VHS captures what it renders in its own terminal window, not the tmux pane's output.

For lazyclaude's use case---where the TUI itself is a tmux plugin and interacts with tmux---VHS is a poor architectural fit. The existing bash-script + `tmux send-keys` + `capture-pane` approach directly controls the actual tmux session, which is what the code under test actually runs inside.

---

## 5. Docker Support

VHS ships an official Docker image:

```
docker run --rm -v $PWD:/vhs ghcr.io/charmbracelet/vhs <cassette>.tape
```

Image: `ghcr.io/charmbracelet/vhs`

### What the image contains (Dockerfile summary)

Base: `debian:stable-slim`

Includes:
- Chromium (headless, with `--no-sandbox` enabled via `VHS_NO_SANDBOX`)
- ttyd binary (from `tsl0922/ttyd:alpine`)
- FFmpeg
- Font collection: Fira Code, JetBrains Mono, Hack, DejaVu, Noto, Inconsolata, Source Code Pro, and others

### Chromium sandbox issue in CI

Running Chromium inside a Docker container (especially in unprivileged or rootless containers) requires disabling the sandbox. Set the environment variable:

```
VHS_NO_SANDBOX=true
```

or use the official image, which sets this automatically. Without it, vhs will fail with `[launcher] Failed to launch the browser`.

---

## 6. Comparison: VHS vs. bash scripts + tmux capture-pane

| Dimension | VHS | bash + tmux capture-pane |
|---|---|---|
| Terminal ownership | Owns its own PTY via ttyd | Attaches to existing tmux session |
| tmux awareness | None (can run tmux inside its shell, but capture is indirect) | Native; uses `send-keys`, `capture-pane` |
| Assertion mechanism | Golden-file diff + `Wait` timeout | Arbitrary bash; `grep`, exact string match, regex |
| Output formats | GIF, MP4, WebM, PNG, plain text | Plain text (capture-pane output) |
| Visual output | Yes (font-rendered, themed GIFs) | No |
| Dependencies | Chromium, ttyd, FFmpeg | tmux (already present) |
| Portability | Needs heavy runtime; Docker image is 1+ GB | Minimal; tmux is always available |
| CI integration | Official GitHub Action; Docker image | Standard bash in any CI runner |
| Framework coupling | Framework-agnostic (any TUI) | Framework-agnostic |
| Speed | Slow (browser + video encoding) | Fast (shell commands) |
| Suitability for tmux-plugin testing | Poor | Native fit |

### Summary judgment

VHS is well-suited for generating documented, reproducible demo GIFs of CLI tools in CI. For E2E testing of a TUI that lives inside tmux (like lazyclaude), the existing `tmux send-keys` + `capture-pane` approach is architecturally superior. VHS's golden-file comparison pattern (output to `.txt`, diff in CI) is worth borrowing as a technique, but it does not require adopting vhs itself.

---

## 7. Installation Methods

| Method | Command |
|---|---|
| Homebrew (macOS/Linux) | `brew install vhs` |
| Go install | `go install github.com/charmbracelet/vhs@latest` |
| Arch Linux | `pacman -S vhs` |
| Nix | `nix-env -iA nixpkgs.vhs` |
| Scoop (Windows) | `scoop install vhs` |
| winget (Windows) | `winget install charmbracelet.vhs` |
| Debian/RPM package | Available via Charm's apt/yum repo |
| Docker | `docker run --rm -v $PWD:/vhs ghcr.io/charmbracelet/vhs <tape>` |

Requires on PATH: `ttyd`, `ffmpeg` (not needed when using Docker image).

Latest release: v0.11.0 (March 2025).

---

## Key References

- Charmbracelet, "VHS: Your CLI Home Video Recorder," GitHub repository, https://github.com/charmbracelet/vhs
- VHS README (main branch): https://github.com/charmbracelet/vhs/blob/main/README.md
- VHS Dockerfile: https://github.com/charmbracelet/vhs/blob/main/Dockerfile
- VHS testing.go (golden file mechanism): https://github.com/charmbracelet/vhs/blob/main/testing.go
- VHS command.go (full command implementation): https://github.com/charmbracelet/vhs/blob/main/command.go
- VHS GitHub Action: https://github.com/charmbracelet/vhs-action
- DeepWiki architecture overview: https://deepwiki.com/charmbracelet/vhs
- VHS_NO_SANDBOX issue: https://github.com/charmbracelet/vhs/issues/504
- "Why does this need a browser?" discussion: https://github.com/charmbracelet/vhs/discussions/291
- Charmbracelet, "Writing Bubble Tea Tests" (teatest, separate library): https://charm.land/blog/teatest/
- Issue #14 "Fail early when missing required programs": https://github.com/charmbracelet/vhs/issues/14
- Issue #537 "Wait for command to finish / exit on nonzero": https://github.com/charmbracelet/vhs/issues/537
- Issue #653 "Abort recording if the shell exits" (open): https://github.com/charmbracelet/vhs/issues/653
- PR #708 "AwaitPrompt command" (open): https://github.com/charmbracelet/vhs/pull/708
