package kit

import "sync"

// WaitGroup is a wrapper around sync.WaitGroup that provides ergonomic improvements
// for concurrent goroutine management.
//
// Unlike sync.WaitGroup, WaitGroup's Do method accepts functions directly and
// automatically manages Add and Done calls, eliminating boilerplate. However, Do
// does not block until goroutines complete — the caller must explicitly call Wait
// after launching goroutines.
//
// The zero value is NOT usable; always construct with NewWaitGroup.
//
// Example:
//
//	wg := kit.NewWaitGroup()
//	wg.Do(f1, f2, f3)  // launches goroutines but does not block
//	wg.Wait()          // blocks until all goroutines complete
type WaitGroup struct {
	wg *sync.WaitGroup
}

// NewWaitGroup constructs a new WaitGroup ready for use.
func NewWaitGroup() *WaitGroup {
	return &WaitGroup{wg: &sync.WaitGroup{}}
}

// Do launches the given functions concurrently in separate goroutines and returns
// immediately without blocking.
//
// Each function f is wrapped with automatic Add and Done calls, so the caller
// need not manage the sync.WaitGroup counter manually. Passing zero functions is
// a no-op.
//
// Do may be called multiple times before Wait; each call adds to the same counter,
// so Wait will not return until all goroutines launched across all Do calls have
// completed.
//
// The caller must explicitly call Wait() to block until all goroutines finish.
func (w *WaitGroup) Do(f ...func()) {
	w.wg.Add(len(f))
	for _, fn := range f {
		go func() {
			defer w.wg.Done()
			fn()
		}()
	}
}

// Get returns the underlying *sync.WaitGroup.
//
// This allows callers to perform advanced operations directly on the underlying
// WaitGroup, such as calling TryWait, or to pass the pointer to other code that
// expects *sync.WaitGroup for interoperability with the standard library.
func (w *WaitGroup) Get() *sync.WaitGroup {
	return w.wg
}

// Wait blocks until all goroutines launched via Do have returned.
//
// If Do has not been called, Wait returns immediately.
func (w *WaitGroup) Wait() {
	w.wg.Wait()
}
