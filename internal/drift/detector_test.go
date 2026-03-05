package drift

import (
	"testing"
	"time"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

func TestBuildReportAddedMissingChanged(t *testing.T) {
	desired := []model.Resource{
		{
			ID:    "i-1",
			Type:  "aws.ec2.instance",
			State: "running",
			Tags: map[string]string{
				"gocools:stack-id": "dev-stack",
			},
			Metadata: map[string]string{
				"instance_type": "t3.micro",
			},
		},
		{
			ID:   "db-1",
			Type: "aws.rds.db_instance",
		},
	}
	actual := []model.Resource{
		{
			ID:    "i-1",
			Type:  "aws.ec2.instance",
			State: "stopped",
			Tags: map[string]string{
				"gocools:stack-id": "dev-stack",
			},
			Metadata: map[string]string{
				"instance_type": "t3.large",
				"last_updated":  "2026-03-05T00:00:00Z",
			},
		},
		{
			ID:   "extra-1",
			Type: "aws.s3.bucket",
		},
	}

	report := BuildReport(desired, actual, Config{
		Now: func() time.Time { return time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC) },
	})

	if report.Added != 1 || report.Missing != 1 || report.Changed != 1 {
		t.Fatalf("unexpected counts: added=%d missing=%d changed=%d", report.Added, report.Missing, report.Changed)
	}

	if len(report.Items) != 3 {
		t.Fatalf("expected 3 drift items, got %d", len(report.Items))
	}
}

func TestBuildReportIgnoresConfiguredMetadataFields(t *testing.T) {
	desired := []model.Resource{
		{
			ID:   "i-1",
			Type: "aws.ec2.instance",
			Metadata: map[string]string{
				"last_seen": "A",
			},
		},
	}
	actual := []model.Resource{
		{
			ID:   "i-1",
			Type: "aws.ec2.instance",
			Metadata: map[string]string{
				"last_seen": "B",
			},
		},
	}

	report := BuildReport(desired, actual, Config{
		IgnoredMetadataKeys: []string{"last_seen"},
	})

	if report.Changed != 0 {
		t.Fatalf("expected no changed items when ignored metadata differs, got %d", report.Changed)
	}
}
