package smtp

import (
	"math"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gitlab.com/etke.cc/go/logger"
)

const fsdelay = 100 * time.Millisecond

type FSWatcher struct {
	watcher *fsnotify.Watcher
	files   []string
	log     *logger.Logger
	mu      sync.Mutex
	t       map[string]*time.Timer
}

func NewFSWatcher(files []string, loglevel string) (*FSWatcher, error) {
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

	fswatcher := &FSWatcher{
		watcher: watcher,
		files:   files,
		log:     logger.New("fs.", loglevel),
		t:       make(map[string]*time.Timer),
	}

	return fswatcher, nil
}

func (w *FSWatcher) watch(handler func(e fsnotify.Event)) {
	for {
		select {
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.log.Error("%v", err)
		case e, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			handler(e)
		}
	}
}

// Start watcher
func (w *FSWatcher) Start(handler func(e fsnotify.Event)) {
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
				w.log.Info("handling fs event %+v", e)
				handler(e)
			})
			t.Stop()

			w.mu.Lock()
			w.t[e.Name] = t
			w.mu.Unlock()
		}
		t.Reset(fsdelay)
	})
}

// Stop watcher
func (w *FSWatcher) Stop() {
	err := w.watcher.Close()
	if err != nil {
		w.log.Error("cannot stop fs watcher: %v", err)
	}
}
