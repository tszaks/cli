package strategy

import (
	"fmt"
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

// Factory creates a new strategy instance
type Factory func() Strategy

// Register adds a strategy factory to the registry.
// This is typically called from init() functions in strategy implementations.
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

// Get retrieves a strategy by name.
// Returns an error if the strategy is not registered.
//

func Get(name string) (Strategy, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown strategy: %s (available: %v)", name, List())
	}

	return factory(), nil
}

// List returns all registered strategy names in sorted order.
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
