package graph

import (
	"fmt"
	"sort"
)

type DiffKind string

const (
	DiffKindAdded    DiffKind = "added"
	DiffKindRemoved  DiffKind = "removed"
	DiffKindModified DiffKind = "modified"
)

type FieldChange struct {
	Field  string `json:"field"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type NodeDiff struct {
	Kind         DiffKind      `json:"kind"`
	NodeID       string        `json:"node_id"`
	ResourceType string        `json:"resource_type"`
	Changes      []FieldChange `json:"changes,omitempty"`
}

type DiffReport struct {
	Added    int        `json:"added"`
	Removed  int        `json:"removed"`
	Modified int        `json:"modified"`
	Changes  []NodeDiff `json:"changes"`
}

func DiffGraphs(before Graph, after Graph, query Query) DiffReport {
	filteredBefore := before.Filter(query)
	filteredAfter := after.Filter(query)

	beforeByID := map[string]Node{}
	afterByID := map[string]Node{}
	for _, node := range filteredBefore.Nodes {
		beforeByID[node.ID] = node
	}
	for _, node := range filteredAfter.Nodes {
		afterByID[node.ID] = node
	}

	changes := make([]NodeDiff, 0)

	for id, previousNode := range beforeByID {
		nextNode, exists := afterByID[id]
		if !exists {
			changes = append(changes, NodeDiff{
				Kind:         DiffKindRemoved,
				NodeID:       id,
				ResourceType: previousNode.Type,
			})
			continue
		}

		fieldChanges := compareNodeFields(previousNode, nextNode)
		if len(fieldChanges) == 0 {
			continue
		}

		changes = append(changes, NodeDiff{
			Kind:         DiffKindModified,
			NodeID:       id,
			ResourceType: nextNode.Type,
			Changes:      fieldChanges,
		})
	}

	for id, nextNode := range afterByID {
		if _, exists := beforeByID[id]; exists {
			continue
		}
		changes = append(changes, NodeDiff{
			Kind:         DiffKindAdded,
			NodeID:       id,
			ResourceType: nextNode.Type,
		})
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].NodeID == changes[j].NodeID {
			return changes[i].Kind < changes[j].Kind
		}
		return changes[i].NodeID < changes[j].NodeID
	})

	report := DiffReport{
		Changes: changes,
	}
	for _, item := range changes {
		switch item.Kind {
		case DiffKindAdded:
			report.Added++
		case DiffKindRemoved:
			report.Removed++
		case DiffKindModified:
			report.Modified++
		}
	}

	return report
}

func compareNodeFields(before Node, after Node) []FieldChange {
	changes := make([]FieldChange, 0)

	if before.Type != after.Type {
		changes = append(changes, FieldChange{Field: "type", Before: before.Type, After: after.Type})
	}
	if before.Provider != after.Provider {
		changes = append(changes, FieldChange{Field: "provider", Before: before.Provider, After: after.Provider})
	}
	if before.Region != after.Region {
		changes = append(changes, FieldChange{Field: "region", Before: before.Region, After: after.Region})
	}
	if before.Name != after.Name {
		changes = append(changes, FieldChange{Field: "name", Before: before.Name, After: after.Name})
	}
	if before.State != after.State {
		changes = append(changes, FieldChange{Field: "state", Before: before.State, After: after.State})
	}
	if before.ARN != after.ARN {
		changes = append(changes, FieldChange{Field: "arn", Before: before.ARN, After: after.ARN})
	}

	changes = append(changes, mapDiff("tag", before.Tags, after.Tags)...)
	changes = append(changes, mapDiff("metadata", before.Metadata, after.Metadata)...)

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Field < changes[j].Field
	})

	return changes
}

func mapDiff(prefix string, before map[string]string, after map[string]string) []FieldChange {
	changes := make([]FieldChange, 0)

	for key, beforeValue := range before {
		afterValue, exists := after[key]
		if !exists {
			changes = append(changes, FieldChange{
				Field:  fmt.Sprintf("%s.%s", prefix, key),
				Before: beforeValue,
			})
			continue
		}
		if beforeValue != afterValue {
			changes = append(changes, FieldChange{
				Field:  fmt.Sprintf("%s.%s", prefix, key),
				Before: beforeValue,
				After:  afterValue,
			})
		}
	}
	for key, afterValue := range after {
		if _, exists := before[key]; exists {
			continue
		}
		changes = append(changes, FieldChange{
			Field: fmt.Sprintf("%s.%s", prefix, key),
			After: afterValue,
		})
	}

	return changes
}
