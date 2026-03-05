package graph

import (
	"slices"
	"time"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

const SchemaVersion = "arch.gocools/v1alpha1"

type Graph struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Nodes         []Node    `json:"nodes"`
	Edges         []Edge    `json:"edges"`
}

type Node struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Provider string            `json:"provider"`
	Region   string            `json:"region"`
	Name     string            `json:"name,omitempty"`
	State    string            `json:"state,omitempty"`
	ARN      string            `json:"arn,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Edge struct {
	From     string            `json:"from"`
	To       string            `json:"to"`
	Type     string            `json:"type"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Query struct {
	StackID     string
	Environment string
}

func FromResources(resources []model.Resource, generatedAt time.Time) Graph {
	nodes := make([]Node, 0, len(resources))
	for _, resource := range resources {
		nodes = append(nodes, Node{
			ID:       resource.ID,
			Type:     resource.Type,
			Provider: resource.Provider,
			Region:   resource.Region,
			Name:     resource.Name,
			State:    resource.State,
			ARN:      resource.ARN,
			Tags:     cloneMap(resource.Tags),
			Metadata: cloneMap(resource.Metadata),
		})
	}

	slices.SortFunc(nodes, func(a, b Node) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})

	return Graph{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   generatedAt.UTC(),
		Nodes:         nodes,
		Edges:         []Edge{},
	}
}

func (g Graph) Filter(query Query) Graph {
	if query.StackID == "" && query.Environment == "" {
		return g
	}

	nodeIDs := map[string]struct{}{}
	filteredNodes := make([]Node, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		if !matchesNodeFilter(node, query) {
			continue
		}
		filteredNodes = append(filteredNodes, node)
		nodeIDs[node.ID] = struct{}{}
	}

	filteredEdges := make([]Edge, 0, len(g.Edges))
	for _, edge := range g.Edges {
		if _, ok := nodeIDs[edge.From]; !ok {
			continue
		}
		if _, ok := nodeIDs[edge.To]; !ok {
			continue
		}
		filteredEdges = append(filteredEdges, edge)
	}

	return Graph{
		SchemaVersion: g.SchemaVersion,
		GeneratedAt:   g.GeneratedAt,
		Nodes:         filteredNodes,
		Edges:         filteredEdges,
	}
}

func matchesNodeFilter(node Node, query Query) bool {
	if query.StackID != "" {
		if node.Tags["gocools:stack-id"] != query.StackID {
			return false
		}
	}
	if query.Environment != "" {
		if node.Tags["gocools:environment"] != query.Environment {
			return false
		}
	}
	return true
}

func cloneMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
