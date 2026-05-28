package topology

import (
	"context"
	"fmt"
	"time"

	"github.com/astraive/loxa/cortex/internal/models"
	"github.com/astraive/loxa/cortex/internal/storage"
)

// AliasCallback is called when a service alias is registered.
type AliasCallback func(ctx context.Context, from, to string, validFrom time.Time)

type Resolver struct {
	topologyStore storage.TopologyStore
	callbacks     []AliasCallback
}

func NewResolver(store storage.TopologyStore) *Resolver {
	return &Resolver{topologyStore: store}
}

// RegisterCallback adds a callback that fires when a new alias is registered.
func (r *Resolver) RegisterCallback(cb AliasCallback) {
	r.callbacks = append(r.callbacks, cb)
}

func (r *Resolver) ResolveService(ctx context.Context, alias string, timestamp time.Time) (string, error) {
	aliasObj, err := r.topologyStore.GetAlias(ctx, alias, timestamp.Format(time.RFC3339))
	if err != nil {
		return alias, nil
	}
	return aliasObj.Canonical, nil
}

func (r *Resolver) RegisterAlias(ctx context.Context, from string, to string, validFrom time.Time, validTo *time.Time) error {
	if validTo != nil && validFrom.After(*validTo) {
		return fmt.Errorf("valid_from must be before valid_to")
	}

	existingAliases, err := r.topologyStore.GetHistory(ctx, from)
	if err == nil {
		for _, existing := range existingAliases {
			if existing.Canonical == to {
				continue
			}
			if existing.ValidTo == nil || existing.ValidTo.After(validFrom) {
				return fmt.Errorf("overlapping alias period for service %s", from)
			}
		}
	}

	alias := &models.ServiceAlias{
		ID:        fmt.Sprintf("%s->%s-%d", from, to, validFrom.Unix()),
		Alias:     from,
		Canonical: to,
		ValidFrom: validFrom,
		ValidTo:   validTo,
	}

	if err := r.topologyStore.SaveAlias(ctx, alias); err != nil {
		return err
	}

	// Fire callbacks
	for _, cb := range r.callbacks {
		cb(ctx, from, to, validFrom)
	}

	return nil
}

func (r *Resolver) GetServiceHistory(ctx context.Context, service string) ([]*models.ServiceAlias, error) {
	aliases, err := r.topologyStore.GetHistory(ctx, service)
	if err != nil {
		return nil, err
	}

	if len(aliases) == 0 {
		return []*models.ServiceAlias{
			{
				Alias:     service,
				Canonical: service,
				ValidFrom: time.Time{},
			},
		}, nil
	}

	return aliases, nil
}