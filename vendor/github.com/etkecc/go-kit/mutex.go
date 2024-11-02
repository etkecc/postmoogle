package kit

import (
	"sync"
)

// Mutex is a key-based mutex that allows locking and unlocking based on a key.
type Mutex struct {
	mu    sync.RWMutex
	locks map[string]*sync.Mutex
}

// NewMutex creates a new Mutex instance
func NewMutex() *Mutex {
	return &Mutex{
		locks: make(map[string]*sync.Mutex),
	}
}

// Lock locks the mutex for a specific key.
func (km *Mutex) Lock(key string) {
	// First, try to acquire the lock with only a read lock on `mu`
	km.mu.RLock()
	m, exists := km.locks[key]
	km.mu.RUnlock()

	// If the key exists, we can lock it directly
	if exists {
		m.Lock()
		return
	}

	// If the key doesn't exist, we need to upgrade to a write lock
	km.mu.Lock()
	m, exists = km.locks[key]
	if !exists {
		m = &sync.Mutex{}
		km.locks[key] = m
	}
	km.mu.Unlock()

	// Finally, lock the mutex for the key
	m.Lock()
}

// Unlock unlocks the mutex for a specific key.
func (km *Mutex) Unlock(key string) {
	km.mu.RLock()
	m, exists := km.locks[key]
	km.mu.RUnlock()

	if exists {
		m.Unlock()
	}
}
