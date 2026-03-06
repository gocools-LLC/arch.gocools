package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

func TestProviderListResources(t *testing.T) {
	want := []model.Resource{
		{ID: "i-1", Type: "aws.ec2.instance"},
		{ID: "db-1", Type: "aws.rds.instance"},
	}

	provider := NewProvider(fakeResourceDiscoverer{
		resources: want,
	})

	got, err := provider.ListResources(context.Background())
	if err != nil {
		t.Fatalf("list resources failed: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d resources, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Type != want[i].Type {
			t.Fatalf("unexpected resource at index %d: %+v", i, got[i])
		}
	}
}

func TestProviderListResourcesError(t *testing.T) {
	wantErr := errors.New("discover failed")
	provider := NewProvider(fakeResourceDiscoverer{err: wantErr})

	_, err := provider.ListResources(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

type fakeResourceDiscoverer struct {
	resources []model.Resource
	err       error
}

func (f fakeResourceDiscoverer) DiscoverAll(_ context.Context) ([]model.Resource, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resources, nil
}
