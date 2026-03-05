package graph

import (
	"context"
	"time"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

type ResourceProvider interface {
	ListResources(ctx context.Context) ([]model.Resource, error)
}

type Service struct {
	provider ResourceProvider
	now      func() time.Time
}

func NewService(provider ResourceProvider) *Service {
	return &Service{
		provider: provider,
		now:      time.Now,
	}
}

func (s *Service) Query(ctx context.Context, query Query) (Graph, error) {
	resources, err := s.provider.ListResources(ctx)
	if err != nil {
		return Graph{}, err
	}

	graph := FromResources(resources, s.now())
	return graph.Filter(query), nil
}

type StaticResourceProvider struct {
	resources []model.Resource
}

func NewStaticResourceProvider(resources []model.Resource) *StaticResourceProvider {
	cloned := make([]model.Resource, len(resources))
	copy(cloned, resources)
	return &StaticResourceProvider{resources: cloned}
}

func (p *StaticResourceProvider) ListResources(_ context.Context) ([]model.Resource, error) {
	cloned := make([]model.Resource, len(p.resources))
	copy(cloned, p.resources)
	return cloned, nil
}
