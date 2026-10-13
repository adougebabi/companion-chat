package core

import (
	"context"
	"errors"
)

type capabilityRunScopeKey struct{}

func scopedCapabilityRegistry(ctx context.Context) (*CapabilityRegistry, bool) {
	registry, ok := ctx.Value(capabilityRunScopeKey{}).(*CapabilityRegistry)
	return registry, ok && registry != nil
}

func withCapabilityRunRegistry(ctx context.Context, registry *CapabilityRegistry) context.Context {
	return context.WithValue(ctx, capabilityRunScopeKey{}, registry)
}

func (a *App) capabilityRegistryForContext(ctx context.Context) *CapabilityRegistry {
	if registry, ok := scopedCapabilityRegistry(ctx); ok {
		return registry
	}
	return a.capabilityRegistry()
}

// A view is immutable after construction and has one implementation/definition
// per name. The shared App registry is never mutated while other runs execute.
func capabilityRegistryView(base *CapabilityRegistry, replacements ...Capability) (*CapabilityRegistry, error) {
	if base == nil {
		return nil, ErrCapabilityNotFound
	}
	entries := make(map[string]Capability, len(base.capabilities))
	for name, entry := range base.capabilities {
		entries[name] = entry
	}
	seen := map[string]bool{}
	for _, entry := range replacements {
		if entry == nil {
			return nil, errors.New("capability_scope_replacement_required")
		}
		name := entry.Definition().Name
		if _, exists := entries[name]; !exists || seen[name] {
			return nil, errors.New("capability_scope_replacement_invalid")
		}
		seen[name] = true
		entries[name] = entry
	}
	list := make([]Capability, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry)
	}
	return NewCapabilityRegistry(list...)
}
