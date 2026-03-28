package tmuxadapter

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/core/tmux"
)

// toolPopupReq holds a queued tool popup request.
type toolPopupReq struct {
	window   string
	toolName string
	input    string
	cwd      string
}

// defaultMaxConcurrentPopups is the default global limit on concurrent popups
// across all windows. Prevents overwhelming the tmux server with too many
// simultaneous display-popup commands.
const defaultMaxConcurrentPopups = 3

// PopupOrchestrator spawns tool/diff popups inside tmux display-popup.
// For tool popups, it queues requests per window so only one popup
// is active at a time per window. A global semaphore limits the total
// number of concurrent popups across all windows.
type PopupOrchestrator struct {
	binary     string      // path to lazyclaude binary
	socket     string      // lazyclaude tmux socket name (e.g., "lazyclaude")
	tmux       tmux.Client // lazyclaude tmux (-L lazyclaude)
	hostTmux   tmux.Client // user's tmux (for display-popup)
	runtimeDir string      // for consuming notification files after popup
	log        *log.Logger

	mu     sync.Mutex
	active map[string]bool          // window -> popup currently open
	queues map[string][]toolPopupReq // window -> queued requests

	globalSem chan struct{} // counting semaphore for global concurrency limit
}

// NewPopupOrchestrator creates a popup orchestrator with the default global
// concurrency limit.
// socket is the lazyclaude tmux socket name, passed to popup subprocesses via
// LAZYCLAUDE_TMUX_SOCKET so they can send keys to the correct tmux server.
// hostTmux is the user's tmux client (for display-popup). Can be nil if unknown.
func NewPopupOrchestrator(binary, socket, runtimeDir string, tmuxClient, hostTmux tmux.Client, logger *log.Logger) *PopupOrchestrator {
	return NewPopupOrchestratorWithMax(binary, socket, runtimeDir, tmuxClient, hostTmux, logger, defaultMaxConcurrentPopups)
}

// NewPopupOrchestratorWithMax creates a popup orchestrator with an explicit
// global concurrency limit. maxConcurrent controls the maximum number of
// simultaneous display-popup commands across all windows.
func NewPopupOrchestratorWithMax(binary, socket, runtimeDir string, tmuxClient, hostTmux tmux.Client, logger *log.Logger, maxConcurrent int) *PopupOrchestrator {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentPopups
	}
	return &PopupOrchestrator{
		binary:     binary,
		socket:     socket,
		runtimeDir: runtimeDir,
		tmux:       tmuxClient,
		hostTmux:   hostTmux,
		log:        logger,
		active:     make(map[string]bool),
		queues:     make(map[string][]toolPopupReq),
		globalSem:  make(chan struct{}, maxConcurrent),
	}
}

// QueueLen returns the number of queued (not active) popups for a window.
func (p *PopupOrchestrator) QueueLen(window string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.queues[window])
}

// SpawnToolPopup spawns a tool confirmation popup via tmux display-popup.
// If a popup is already active for this window, the request is queued.
// Returns immediately; the popup runs in a goroutine.
func (p *PopupOrchestrator) SpawnToolPopup(ctx context.Context, window, toolName, toolInput, toolCWD string) {
	req := toolPopupReq{
		window:   window,
		toolName: toolName,
		input:    toolInput,
		cwd:      toolCWD,
	}

	p.mu.Lock()
	if p.active[window] {
		p.queues[window] = append(p.queues[window], req)
		p.mu.Unlock()
		p.log.Printf("popup: queued tool %s for window %s (queue=%d)", toolName, window, len(p.queues[window]))
		return
	}
	p.active[window] = true
	p.mu.Unlock()

	p.log.Printf("popup: spawning tool %s for window %s", toolName, window)
	go p.runToolPopup(ctx, req)
}

// runToolPopup executes a single tool popup and drains the queue on exit.
// It acquires the global semaphore before each popup and releases it after.
// On context cancellation, the window's active flag and queue are cleaned up.
func (p *PopupOrchestrator) runToolPopup(ctx context.Context, req toolPopupReq) {
	defer p.clearWindow(req.window)

	if !p.acquireSem(ctx) {
		return
	}
	p.spawnToolPopupBlocking(ctx, req)
	<-p.globalSem // release global slot

	// Drain queue: check for next request after popup closes
	for {
		time.Sleep(200 * time.Millisecond)

		p.mu.Lock()
		q := p.queues[req.window]
		if len(q) == 0 {
			p.mu.Unlock()
			return
		}
		next := q[0]
		p.queues[req.window] = q[1:]
		p.mu.Unlock()

		if !p.acquireSem(ctx) {
			return
		}
		p.spawnToolPopupBlocking(ctx, next)
		<-p.globalSem // release global slot
	}
}

// acquireSem acquires a global semaphore slot, respecting context cancellation.
func (p *PopupOrchestrator) acquireSem(ctx context.Context) bool {
	select {
	case p.globalSem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// clearWindow removes the active flag and queue for a window.
func (p *PopupOrchestrator) clearWindow(window string) {
	p.mu.Lock()
	delete(p.active, window)
	delete(p.queues, window)
	p.mu.Unlock()
}

// spawnToolPopupBlocking runs a single tool popup (blocks until close).
func (p *PopupOrchestrator) spawnToolPopupBlocking(ctx context.Context, req toolPopupReq) {
	w, h := EstimatePopupSize(req.toolName, req.input, 200, 50)
	cmd := fmt.Sprintf("%s tool --window %s --send-keys", p.binary, req.window)
	env := map[string]string{
		"TOOL_NAME":  req.toolName,
		"TOOL_INPUT": req.input,
		"TOOL_CWD":   req.cwd,
	}
	if p.socket != "" {
		env["LAZYCLAUDE_TMUX_SOCKET"] = p.socket
	}
	opts := tmux.PopupOpts{
		Width:  w,
		Height: h,
		Cmd:    cmd,
		Env:    env,
	}
	// Decide which tmux to use for display-popup:
	// 1. If lazyclaude tmux has an active client (user is in fullscreen/attach),
	//    use lazyclaude tmux — the user is looking at it.
	// 2. Otherwise use hostTmux (user's tmux) — TUI is closed.
	popupTmux := p.tmux
	if client, err := p.tmux.FindActiveClient(ctx); err == nil && client != nil {
		opts.Client = client.Name
		opts.Target = req.window
	} else if p.hostTmux != nil {
		popupTmux = p.hostTmux
		if client, err := p.hostTmux.FindActiveClient(ctx); err == nil && client != nil {
			opts.Client = client.Name
		}
	}
	if err := popupTmux.DisplayPopup(ctx, opts); err != nil {
		p.log.Printf("popup: spawn tool: %v", err)
	}
}

// SpawnDiffPopup spawns a diff viewer popup via tmux display-popup.
// Blocks until the popup closes so the caller can read the choice file.
// This is not subject to the global semaphore because diff popups are
// synchronous (caller blocks), so concurrency is bounded by the caller.
func (p *PopupOrchestrator) SpawnDiffPopup(ctx context.Context, window, oldPath, newContentsFile string) {
	cmd := fmt.Sprintf("%s diff --window %s --send-keys --old %s --new %s",
		p.binary, window, oldPath, newContentsFile)
	env := map[string]string{}
	if p.socket != "" {
		env["LAZYCLAUDE_TMUX_SOCKET"] = p.socket
	}
	opts := tmux.PopupOpts{
		Width:  80,
		Height: 80,
		Cmd:    cmd,
		Env:    env,
	}
	popupTmux := p.tmux
	if client, err := p.tmux.FindActiveClient(ctx); err == nil && client != nil {
		opts.Client = client.Name
		opts.Target = window
	} else if p.hostTmux != nil {
		popupTmux = p.hostTmux
		if client, err := p.hostTmux.FindActiveClient(ctx); err == nil && client != nil {
			opts.Client = client.Name
		}
	}
	if err := popupTmux.DisplayPopup(ctx, opts); err != nil {
		p.log.Printf("popup: spawn diff: %v", err)
	}
}
