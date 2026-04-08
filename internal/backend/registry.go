// Package backend defines the interface all proxy backends must satisfy.
package backend

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Backend{}
)

// Register makes b available by name. It panics on nil or duplicate backends.
func Register(b Backend) {
	if b == nil {
		panic("backend.Register: nil backend")
	}
	name := b.Name()
	if name == "" {
		panic("backend.Register: backend name must not be empty")
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic("backend.Register: duplicate backend " + name)
	}
	registry[name] = b
}

// Lookup returns the registered backend with the given name.
func Lookup(name string) (Backend, error) {
	mu.RLock()
	b, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported backend %q: available backends: %v", name, Names())
	}
	return b, nil
}

// Names returns registered backend names in stable order.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
