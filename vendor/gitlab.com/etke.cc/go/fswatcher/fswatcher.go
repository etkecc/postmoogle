package fswatcher

import (
	"math"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDelay to avoid unnecessary handler calls
const DefaultDelay = 100 * time.Millisecond

// Watcher of file system changes
type Watcher struct {
	watcher *fsnotify.Watcher
	delay   time.Duration
	files   []string
	mu      sync.Mutex
	t       map[string]*time.Timer
}

// Creates FS Watcher
func New(files []string, delay time.Duration) (*Watcher, error) {
	if delay <= 0 {
		delay = DefaultDelay
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		err = watcher.Add(filepath.Dir(file))
		if err != nil {
			return nil, err
		}
	}

	fswatcher := &Watcher{
		watcher: watcher,
		delay:   delay,
		files:   files,
		t:       make(map[string]*time.Timer),
	}

	return fswatcher, nil
}

func (w *Watcher) watch(handler func(e fsnotify.Event)) {
	for e := range w.watcher.Events {
		handler(e)
	}
}

// Start watcher
func (w *Watcher) Start(handler func(e fsnotify.Event)) {
	w.watch(func(e fsnotify.Event) {
		var found bool
		for _, f := range w.files {
			if f == e.Name {
				found = true
			}
		}
		if !found {
			return
		}
		w.mu.Lock()
		t, ok := w.t[e.Name]
		w.mu.Unlock()
		if !ok {
			t = time.AfterFunc(math.MaxInt64, func() {
				handler(e)
			})
			t.Stop()

			w.mu.Lock()
			w.t[e.Name] = t
			w.mu.Unlock()
		}
		t.Reset(w.delay)
	})
}

// Stop watcher
func (w *Watcher) Stop() error {
	return w.watcher.Close()
}
