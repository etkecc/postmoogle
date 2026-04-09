package kit

import (
	"sync"
)

// keyMutex pairs a per-key sync.Mutex with a reference count.
// refcount tracks how many goroutines are currently either waiting to acquire
// or actively holding the per-key lock. It is incremented by Lock before
// blocking and decremented by Unlock before releasing, both under the global
// Mutex.mu lock. When refcount reaches zero, Unlock removes the entry from
// the map, keeping memory bounded to keys in active use.
type keyMutex struct {
	mu       sync.Mutex
	refcount int
}

// Mutex is a key-based mutex that allows independent locking of different keys.
// Different keys can be locked simultaneously without blocking each other —
// only goroutines contending on the exact same key block.
//
// # Memory management
//
// Per-key entries are reference-counted and removed automatically: when the
// last goroutine holding or waiting for a key releases it, the entry is deleted
// from the internal map. The map therefore stays bounded to the number of keys
// currently in active use rather than accumulating every key ever seen.
//
// # Thread safety
//
// A single Mutex may be used concurrently from any number of goroutines without
// external synchronization.
//
// The zero value is not usable; always construct a Mutex using NewMutex.
type Mutex struct {
	// mu protects the locks map and the refcount field of every keyMutex in it.
	// It is a plain Mutex (not RWMutex) because every Lock call must both read
	// and write refcount atomically — a read lock would not be sufficient.
	mu sync.Mutex

	// locks maps each key to its per-key mutex and reference count.
	// Entries are created on first Lock and deleted when refcount reaches zero.
	locks map[string]*keyMutex
}

// NewMutex creates and returns a new, initialized Mutex ready for use.
func NewMutex() *Mutex {
	return &Mutex{
		locks: make(map[string]*keyMutex),
	}
}

// Lock locks the mutex for the specified key, blocking until the lock is acquired.
// Multiple goroutines may hold locks on different keys simultaneously.
//
// The operation proceeds in two steps:
//  1. The global lock is acquired, the per-key entry is created if absent, and its
//     reference count is incremented. The global lock is then released. Incrementing
//     refcount before releasing the global lock ensures a concurrent Unlock cannot
//     delete the entry while this goroutine is still waiting for it.
//  2. The per-key mutex is locked (potentially blocking until the current holder calls Unlock).
func (km *Mutex) Lock(key string) {
	km.mu.Lock()
	m, exists := km.locks[key]
	if !exists {
		m = &keyMutex{}
		km.locks[key] = m
	}
	m.refcount++
	km.mu.Unlock()

	m.mu.Lock()
}

// Unlock unlocks the mutex for the specified key.
//
// It is a no-op if the key has no entry in the internal map (i.e. it was never locked
// or its entry was already cleaned up). Calling Unlock on a key that is locked but
// whose per-key mutex is not held by the caller will panic, consistent with the
// behavior of sync.Mutex.
//
// The operation proceeds in two steps:
//  1. The global lock is acquired, the reference count is decremented, and the entry
//     is deleted from the map if the count reaches zero. A count of zero guarantees
//     no other goroutine is waiting — because Lock increments refcount before waiting —
//     so removing the entry is safe. The global lock is then released.
//  2. The per-key mutex is unlocked. This happens after the entry may have been removed
//     from the map; that is safe because the local variable m still holds a reference
//     to the keyMutex and no new goroutine can acquire the same pointer.
func (km *Mutex) Unlock(key string) {
	km.mu.Lock()
	m, exists := km.locks[key]
	if !exists {
		km.mu.Unlock()
		return
	}
	m.refcount--
	if m.refcount == 0 {
		delete(km.locks, key)
	}
	km.mu.Unlock()

	m.mu.Unlock()
}
