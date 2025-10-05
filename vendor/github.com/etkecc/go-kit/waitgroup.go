package kit

import "sync"

// WaitGroup is a wrapper around sync.WaitGroup to have some syntax sugar
// It does not provide any additional functionality
type WaitGroup struct {
	wg *sync.WaitGroup
}

// NewWaitGroup creates a new WaitGroup
func NewWaitGroup() *WaitGroup {
	return &WaitGroup{wg: &sync.WaitGroup{}}
}

// Do runs the given functions in separate goroutines and waits for them to complete
func (w *WaitGroup) Do(f ...func()) {
	w.wg.Add(len(f))
	for _, fn := range f {
		go func() {
			defer w.wg.Done()
			fn()
		}()
	}
}

// Get returns the underlying sync.WaitGroup
func (w *WaitGroup) Get() *sync.WaitGroup {
	return w.wg
}

// Wait waits for all functions to complete
func (w *WaitGroup) Wait() {
	w.wg.Wait()
}
