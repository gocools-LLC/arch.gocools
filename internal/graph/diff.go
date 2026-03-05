package graph

import (
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
	beforeNodes := filteredAndSortedNodes(before.Nodes, query)
	afterNodes := filteredAndSortedNodes(after.Nodes, query)

	changes := make([]NodeDiff, 0, estimateChangeCapacity(len(beforeNodes), len(afterNodes)))

	beforeIndex := 0
	afterIndex := 0
	for beforeIndex < len(beforeNodes) && afterIndex < len(afterNodes) {
		previousNode := beforeNodes[beforeIndex]
		nextNode := afterNodes[afterIndex]

		switch {
		case previousNode.ID == nextNode.ID:
			fieldChanges := compareNodeFields(previousNode, nextNode)
			if len(fieldChanges) > 0 {
				changes = append(changes, NodeDiff{
					Kind:         DiffKindModified,
					NodeID:       nextNode.ID,
					ResourceType: nextNode.Type,
					Changes:      fieldChanges,
				})
			}
			beforeIndex++
			afterIndex++

		case previousNode.ID < nextNode.ID:
			changes = append(changes, NodeDiff{
				Kind:         DiffKindRemoved,
				NodeID:       previousNode.ID,
				ResourceType: previousNode.Type,
			})
			beforeIndex++

		default:
			changes = append(changes, NodeDiff{
				Kind:         DiffKindAdded,
				NodeID:       nextNode.ID,
				ResourceType: nextNode.Type,
			})
			afterIndex++
		}
	}

	for ; beforeIndex < len(beforeNodes); beforeIndex++ {
		previousNode := beforeNodes[beforeIndex]
		changes = append(changes, NodeDiff{
			Kind:         DiffKindRemoved,
			NodeID:       previousNode.ID,
			ResourceType: previousNode.Type,
		})
	}
	for ; afterIndex < len(afterNodes); afterIndex++ {
		nextNode := afterNodes[afterIndex]
		changes = append(changes, NodeDiff{
			Kind:         DiffKindAdded,
			NodeID:       nextNode.ID,
			ResourceType: nextNode.Type,
		})
	}

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
	changes := make([]FieldChange, 0, 8)

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
	if len(before) == 0 && len(after) == 0 {
		return nil
	}

	keys := make([]string, 0, len(before)+len(after))
	seen := make(map[string]struct{}, len(before)+len(after))
	for key := range before {
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	for key := range after {
		if _, exists := seen[key]; exists {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	changes := make([]FieldChange, 0, len(keys))
	for _, key := range keys {
		beforeValue, beforeExists := before[key]
		afterValue, afterExists := after[key]

		field := joinField(prefix, key)
		switch {
		case beforeExists && afterExists:
			if beforeValue == afterValue {
				continue
			}
			changes = append(changes, FieldChange{
				Field:  field,
				Before: beforeValue,
				After:  afterValue,
			})
		case beforeExists:
			changes = append(changes, FieldChange{
				Field:  field,
				Before: beforeValue,
			})
		case afterExists:
			changes = append(changes, FieldChange{
				Field: field,
				After: afterValue,
			})
		}
	}

	return changes
}

func filteredAndSortedNodes(nodes []Node, query Query) []Node {
	filtered := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		if !matchesNodeFilter(node, query) {
			continue
		}
		filtered = append(filtered, node)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})
	return filtered
}

func estimateChangeCapacity(beforeCount int, afterCount int) int {
	if beforeCount == 0 {
		return afterCount
	}
	if afterCount == 0 {
		return beforeCount
	}
	if beforeCount > afterCount {
		return beforeCount
	}
	return afterCount
}

func joinField(prefix string, key string) string {
	return prefix + "." + key
}
