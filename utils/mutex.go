package utils

import "sync"

// Mutex map
type Mutex map[string]*sync.Mutex

// NewMutex map
func NewMutex() Mutex {
	return Mutex{}
}

// Lock by key
func (m Mutex) Lock(key string) {
	_, ok := m[key]
	if !ok {
		m[key] = &sync.Mutex{}
	}

	m[key].Lock()
}

// Unlock by key
func (m Mutex) Unlock(key string) {
	_, ok := m[key]
	if !ok {
		return
	}

	m[key].Unlock()
	delete(m, key)
}
