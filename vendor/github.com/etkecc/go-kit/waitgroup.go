package kit

import "sync"

// WaitGroup wraps sync.WaitGroup so you hand it funcs instead of juggling Add and Done by hand.
// The one thing to keep in your head: Do launches and returns immediately, it does NOT block,
// so you still call Wait yourself.
//
// The zero value is NOT usable; construct with NewWaitGroup.
//
//	wg := kit.NewWaitGroup()
//	wg.Do(f1, f2, f3)  // launches, doesn't block
//	wg.Wait()          // now you block
type WaitGroup struct {
	wg *sync.WaitGroup
}

// NewWaitGroup returns a WaitGroup ready to use.
func NewWaitGroup() *WaitGroup {
	return &WaitGroup{wg: &sync.WaitGroup{}}
}

// Do launches each f in its own goroutine and returns right away, wrapping the Add/Done so you
// never touch the counter. Zero funcs is a no-op; a nil func in the list panics when its goroutine
// runs, same as calling nil() yourself. Call Do as many times as you like before Wait, every call
// feeds the same counter, so Wait won't return until all of them have finished.
func (w *WaitGroup) Do(f ...func()) {
	w.wg.Add(len(f))
	for _, fn := range f {
		go func() {
			defer w.wg.Done()
			fn()
		}()
	}
}

// Get returns the underlying *sync.WaitGroup, for when you need something Do doesn't wrap (a
// TryWait) or have to hand a real *sync.WaitGroup to stdlib-shaped code.
func (w *WaitGroup) Get() *sync.WaitGroup {
	return w.wg
}

// Wait blocks until all goroutines launched via Do have returned.
//
// If Do has not been called, Wait returns immediately.
func (w *WaitGroup) Wait() {
	w.wg.Wait()
}
