package lifecycle_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/any-context/lazyclaude/internal/core/lifecycle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestLifecycle_RegisterAndClose verifies that a registered cleanup function
// is called exactly once when Close is invoked.
func TestLifecycle_RegisterAndClose(t *testing.T) {
	t.Parallel()

	lc := lifecycle.New()
	require.NotNil(t, lc)

	called := 0
	lc.Register("counter", func() { called++ })

	lc.Close()

	assert.Equal(t, 1, called, "cleanup function must be called exactly once")
}

// TestLifecycle_ReverseOrder verifies LIFO ordering: last registered is first closed.
func TestLifecycle_ReverseOrder(t *testing.T) {
	t.Parallel()

	lc := lifecycle.New()

	var order []string
	lc.Register("first", func() { order = append(order, "first") })
	lc.Register("second", func() { order = append(order, "second") })
	lc.Register("third", func() { order = append(order, "third") })

	lc.Close()

	assert.Equal(t, []string{"third", "second", "first"}, order,
		"cleanup functions must run in reverse registration order (LIFO)")
}

// TestLifecycle_Idempotent verifies that calling Close more than once is safe
// and that cleanup functions run exactly once in total.
func TestLifecycle_Idempotent(t *testing.T) {
	t.Parallel()

	lc := lifecycle.New()

	var callCount int32
	lc.Register("once", func() { atomic.AddInt32(&callCount, 1) })

	lc.Close()
	lc.Close()
	lc.Close()

	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount),
		"cleanup functions must run exactly once regardless of how many times Close is called")
}

// TestLifecycle_PanicRecovery verifies that a panicking cleanup function does
// not propagate the panic and that subsequent cleanup functions still run.
func TestLifecycle_PanicRecovery(t *testing.T) {
	t.Parallel()

	lc := lifecycle.New()

	afterPanicCalled := false
	lc.Register("before-panic", func() { afterPanicCalled = true })
	lc.Register("panicker", func() { panic("intentional test panic") })

	// Close must not panic.
	assert.NotPanics(t, lc.Close,
		"Close must not propagate panics from cleanup functions")

	// The function registered before the panicker (runs after it in LIFO order)
	// must still execute.
	assert.True(t, afterPanicCalled,
		"cleanup functions after a panicking one must still run")
}

// TestLifecycle_ConcurrentRegister verifies that Register is safe when called
// from multiple goroutines simultaneously.
func TestLifecycle_ConcurrentRegister(t *testing.T) {
	t.Parallel()

	const n = 100
	lc := lifecycle.New()

	var wg sync.WaitGroup
	var callCount int32

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			lc.Register("goroutine", func() { atomic.AddInt32(&callCount, 1) })
		}()
	}
	wg.Wait()

	assert.Equal(t, n, lc.Len(),
		"all concurrently registered functions must be tracked")

	lc.Close()

	assert.Equal(t, int32(n), atomic.LoadInt32(&callCount),
		"all concurrently registered cleanup functions must be called")
}

// TestLifecycle_Empty verifies that calling Close on a Lifecycle with no
// registered functions is safe.
func TestLifecycle_Empty(t *testing.T) {
	t.Parallel()

	lc := lifecycle.New()

	assert.NotPanics(t, lc.Close, "Close on empty Lifecycle must not panic")
	assert.NotPanics(t, lc.Close, "second Close on empty Lifecycle must not panic")
}

// TestLifecycle_Len verifies the count of registered cleanup functions.
func TestLifecycle_Len(t *testing.T) {
	t.Parallel()

	lc := lifecycle.New()
	assert.Equal(t, 0, lc.Len(), "new Lifecycle must have zero entries")

	lc.Register("a", func() {})
	assert.Equal(t, 1, lc.Len())

	lc.Register("b", func() {})
	lc.Register("c", func() {})
	assert.Equal(t, 3, lc.Len())
}

// TestLifecycle_RegisterAfterClose verifies that functions registered after
// Close has been called are NOT invoked (the lifecycle is already closed).
func TestLifecycle_RegisterAfterClose(t *testing.T) {
	t.Parallel()

	lc := lifecycle.New()

	called := false
	lc.Close()
	lc.Register("late", func() { called = true })

	// A second Close must not run late-registered functions.
	lc.Close()

	assert.False(t, called,
		"functions registered after Close must not be called on subsequent Close")
}
