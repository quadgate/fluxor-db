package core

import (
	"sort"
	"sync"
)

// Sort sorts the array of strings in ascending order.
func (a *ArrayStore) Sort() {
	a.mu.Lock()
	defer a.mu.Unlock()
	sort.Strings(a.arr)
}

// ArrayStore is a thread-safe store for a single array of strings.
type ArrayStore struct {
	arr []string
	mu  sync.RWMutex
}

// NewArrayStore creates and returns a new ArrayStore.
func NewArrayStore() *ArrayStore {
	return &ArrayStore{
		arr: make([]string, 0),
	}
}

// Set replaces the entire array in the store.
func (a *ArrayStore) Set(value []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.arr = value
}

// Get returns the current array in the store.
func (a *ArrayStore) Get() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]string(nil), a.arr...)
}

// Clear removes all elements from the array.
func (a *ArrayStore) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.arr = a.arr[:0]
}

// Add appends a string to the array.
func (a *ArrayStore) Add(value string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.arr = append(a.arr, value)
}
