package aws

import (
	"context"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

type ResourceDiscoverer interface {
	DiscoverAll(ctx context.Context) ([]model.Resource, error)
}

type Provider struct {
	discoverer ResourceDiscoverer
}

func NewProvider(discoverer ResourceDiscoverer) *Provider {
	return &Provider{discoverer: discoverer}
}

func (p *Provider) ListResources(ctx context.Context) ([]model.Resource, error) {
	return p.discoverer.DiscoverAll(ctx)
}
