package drift

import (
	"sort"
	"strings"
	"time"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

type Kind string

const (
	KindAdded   Kind = "added"
	KindMissing Kind = "missing"
	KindChanged Kind = "changed"
)

type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

type Change struct {
	Field   string `json:"field"`
	Desired string `json:"desired,omitempty"`
	Actual  string `json:"actual,omitempty"`
}

type Item struct {
	Kind         Kind     `json:"kind"`
	Severity     Severity `json:"severity"`
	ResourceID   string   `json:"resource_id"`
	ResourceType string   `json:"resource_type"`
	Summary      string   `json:"summary"`
	Changes      []Change `json:"changes,omitempty"`
}

type Report struct {
	GeneratedAt time.Time `json:"generated_at"`
	Added       int       `json:"added"`
	Missing     int       `json:"missing"`
	Changed     int       `json:"changed"`
	Items       []Item    `json:"items"`
}

type Config struct {
	IgnoredMetadataKeys []string
	Now                 func() time.Time
}

func BuildReport(desired []model.Resource, actual []model.Resource, cfg Config) Report {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	ignored := map[string]struct{}{
		"last_updated": {},
		"updated_at":   {},
		"timestamp":    {},
		"created_at":   {},
	}
	for _, key := range cfg.IgnoredMetadataKeys {
		ignored[key] = struct{}{}
	}

	desiredByKey := map[string]model.Resource{}
	actualByKey := map[string]model.Resource{}

	for _, resource := range desired {
		desiredByKey[resourceKey(resource)] = resource
	}
	for _, resource := range actual {
		actualByKey[resourceKey(resource)] = resource
	}

	items := make([]Item, 0)

	for key, desiredResource := range desiredByKey {
		actualResource, exists := actualByKey[key]
		if !exists {
			items = append(items, Item{
				Kind:         KindMissing,
				Severity:     SeverityHigh,
				ResourceID:   desiredResource.ID,
				ResourceType: desiredResource.Type,
				Summary:      "resource missing from actual infrastructure",
			})
			continue
		}

		changes := compareResource(desiredResource, actualResource, ignored)
		if len(changes) == 0 {
			continue
		}

		items = append(items, Item{
			Kind:         KindChanged,
			Severity:     severityForChanges(changes),
			ResourceID:   desiredResource.ID,
			ResourceType: desiredResource.Type,
			Summary:      "resource drift detected",
			Changes:      changes,
		})
	}

	for key, actualResource := range actualByKey {
		if _, exists := desiredByKey[key]; exists {
			continue
		}
		items = append(items, Item{
			Kind:         KindAdded,
			Severity:     SeverityMedium,
			ResourceID:   actualResource.ID,
			ResourceType: actualResource.Type,
			Summary:      "resource exists in infrastructure but not in desired state",
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].ResourceID == items[j].ResourceID {
			return items[i].Kind < items[j].Kind
		}
		return items[i].ResourceID < items[j].ResourceID
	})

	report := Report{
		GeneratedAt: now().UTC(),
		Items:       items,
	}
	for _, item := range items {
		switch item.Kind {
		case KindAdded:
			report.Added++
		case KindMissing:
			report.Missing++
		case KindChanged:
			report.Changed++
		}
	}

	return report
}

func compareResource(desired model.Resource, actual model.Resource, ignoredMetadata map[string]struct{}) []Change {
	changes := make([]Change, 0)

	if strings.TrimSpace(desired.State) != strings.TrimSpace(actual.State) {
		changes = append(changes, Change{
			Field:   "state",
			Desired: desired.State,
			Actual:  actual.State,
		})
	}

	for key, desiredValue := range desired.Tags {
		if actual.Tags[key] != desiredValue {
			changes = append(changes, Change{
				Field:   "tag." + key,
				Desired: desiredValue,
				Actual:  actual.Tags[key],
			})
		}
	}
	for key, actualValue := range actual.Tags {
		if _, exists := desired.Tags[key]; exists {
			continue
		}
		changes = append(changes, Change{
			Field:  "tag." + key,
			Actual: actualValue,
		})
	}

	for key, desiredValue := range desired.Metadata {
		if _, skip := ignoredMetadata[key]; skip {
			continue
		}
		if actual.Metadata[key] != desiredValue {
			changes = append(changes, Change{
				Field:   "metadata." + key,
				Desired: desiredValue,
				Actual:  actual.Metadata[key],
			})
		}
	}
	for key, actualValue := range actual.Metadata {
		if _, skip := ignoredMetadata[key]; skip {
			continue
		}
		if _, exists := desired.Metadata[key]; exists {
			continue
		}
		changes = append(changes, Change{
			Field:  "metadata." + key,
			Actual: actualValue,
		})
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Field < changes[j].Field
	})
	return changes
}

func severityForChanges(changes []Change) Severity {
	for _, change := range changes {
		if change.Field == "state" {
			return SeverityHigh
		}
		if strings.HasPrefix(change.Field, "tag.gocools:") {
			return SeverityHigh
		}
	}
	if len(changes) > 3 {
		return SeverityMedium
	}
	return SeverityLow
}

func resourceKey(resource model.Resource) string {
	return resource.Type + "::" + resource.ID
}
