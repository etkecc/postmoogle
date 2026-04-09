package kit

import (
	"cmp"
	"sync"
)

// List is a concurrency-safe ordered unique set that stores elements of type T without duplicates.
// T must be cmp.Ordered so elements can be sorted; V is a type parameter used only to satisfy
// the AddMapKeys method signature (which accepts map[T]V). The List is safe for concurrent use
// across all methods. The zero value is not usable — always construct a List with NewList or NewListFrom.
type List[T cmp.Ordered, V any] struct {
	mu   *sync.Mutex
	data map[T]struct{}
}

// NewList creates and returns a new empty List.
// Both type parameters T and V must be explicitly provided.
// Use NewListFrom when starting a List from an existing slice.
func NewList[T cmp.Ordered, V any]() *List[T, V] {
	return &List[T, V]{
		mu:   &sync.Mutex{},
		data: make(map[T]struct{}),
	}
}

// NewListFrom creates a new List and populates it from the provided slice.
// The V type parameter is fixed to T (since no map is involved).
// The returned List owns a deduplicated copy of the slice contents;
// duplicate elements in the input slice are included only once in the List.
func NewListFrom[T cmp.Ordered](slice []T) *List[T, T] {
	list := NewList[T, T]()
	list.AddSlice(slice)
	return list
}

// AddMapKeys adds all keys from the provided map to the List.
// Duplicate keys are ignored due to the List's uniqueness guarantee.
func (l *List[T, V]) AddMapKeys(datamap map[T]V) {
	for k := range datamap {
		l.Add(k)
	}
}

// AddSlice adds all elements from the provided slice to the List.
// Duplicate elements are ignored due to the List's uniqueness guarantee.
func (l *List[T, V]) AddSlice(dataslice []T) {
	for _, k := range dataslice {
		l.Add(k)
	}
}

// Add inserts an item into the List.
// If the item already exists, Add is a no-op and the item is not duplicated.
func (l *List[T, V]) Add(item T) {
	l.mu.Lock()
	if _, ok := l.data[item]; !ok {
		l.data[item] = struct{}{}
	}
	l.mu.Unlock()
}

// RemoveSlice removes all elements from the provided slice from the List.
// If an element does not exist, it is ignored.
func (l *List[T, V]) RemoveSlice(dataslice []T) {
	for _, k := range dataslice {
		l.Remove(k)
	}
}

// Remove deletes an item from the List.
// If the item does not exist, Remove is a no-op.
func (l *List[T, V]) Remove(item T) {
	l.mu.Lock()
	delete(l.data, item)
	l.mu.Unlock()
}

// Len returns the number of items in the List.
// Note: this method does not acquire the mutex — it reads the map length directly.
// While Go map length reads are atomic for reading purposes, the returned value may be stale
// if called concurrently with Add or Remove operations.
func (l *List[T, V]) Len() int {
	return len(l.data)
}

// Slice returns the contents of the List as a sorted slice in ascending order.
// The returned slice is a fresh allocation each time and is not a view into internal state.
// The sorting is provided by the underlying MapKeys function.
func (l *List[T, V]) Slice() []T {
	return MapKeys(l.data)
}
