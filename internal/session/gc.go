package session

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// GC periodically syncs with tmux and removes dead sessions.
type GC struct {
	svc      Service
	interval time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	log      func(msg string, args ...any) // optional debug logger
}

// NewGC creates a garbage collector that runs at the given interval.
func NewGC(svc Service, interval time.Duration) *GC {
	return &GC{
		svc:      svc,
		interval: interval,
	}
}

// Start begins the background sync loop.
func (gc *GC) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	gc.cancel = cancel

	gc.wg.Add(1)
	go func() {
		defer gc.wg.Done()
		ticker := time.NewTicker(gc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				gc.collect(ctx)
			}
		}
	}()
}

// Stop halts the background loop and waits for it to finish.
func (gc *GC) Stop() {
	if gc.cancel != nil {
		gc.cancel()
	}
	gc.wg.Wait()
}

// gcGracePeriod is the minimum age before GC considers deleting a session.
// Prevents race: Create → Sync (before tmux window is fully ready) → Orphan → Delete.
const gcGracePeriod = 10 * time.Second

func (gc *GC) collect(ctx context.Context) {
	if err := gc.svc.Sync(ctx); err != nil {
		gc.debugLog("gc.sync.error", "err", err)
		return
	}

	now := time.Now()
	sessions := gc.svc.Sessions()
	for _, s := range sessions {
		if s.Status == StatusDead || s.Status == StatusOrphan {
			if now.Sub(s.CreatedAt) < gcGracePeriod {
				gc.debugLog("gc.skip.grace", "name", s.Name, "age", now.Sub(s.CreatedAt))
				continue
			}
			gc.debugLog("gc.delete", "name", s.Name, "id", s.ID[:8], "status", s.Status)
			// Crash diagnosis: log GC deletes with stack trace.
			if f, err := os.OpenFile("/tmp/lazyclaude/crash.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
				fmt.Fprintf(f, "[%s] GC DELETE name=%s status=%s age=%s\n", time.Now().Format(time.RFC3339), s.Name, s.Status, time.Since(s.CreatedAt))
				buf := make([]byte, 2048)
				n := runtime.Stack(buf, false)
				f.Write(buf[:n])
				fmt.Fprintln(f)
				f.Sync()
				f.Close()
			}
			gc.svc.Delete(ctx, s.ID)
		}
	}
}

func (gc *GC) debugLog(msg string, args ...any) {
	if gc.log != nil {
		gc.log(msg, args...)
	}
}
