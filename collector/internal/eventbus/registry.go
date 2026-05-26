package eventbus

import (
	"context"
	"fmt"
)

// Factory creates a Bus from a Config.
type Factory func(ctx context.Context, cfg Config) (Bus, error)

var factories = map[string]Factory{}

// Register adds a named bus factory. Call this from adapter init() functions.
func Register(name string, factory Factory) {
	factories[name] = factory
}

// New creates a Bus using the registered factory for cfg.Type.
// Defaults to "memory" if Type is empty.
func New(ctx context.Context, cfg Config) (Bus, error) {
	if cfg.Type == "" {
		cfg.Type = "memory"
	}
	factory, ok := factories[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedType, cfg.Type)
	}
	return factory(ctx, cfg)
}
