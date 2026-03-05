package aws

import (
	"context"
	"testing"
	"time"

	ec2api "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

func TestDiscoverAllEC2Pagination(t *testing.T) {
	client := &fakeEC2Client{
		pages: []*ec2api.DescribeInstancesOutput{
			{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{InstanceId: awsString("i-1")},
						},
					},
				},
				NextToken: awsString("page-2"),
			},
			{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{InstanceId: awsString("i-2")},
						},
					},
				},
			},
		},
	}

	discoverer := NewDiscoverer(Clients{
		EC2: client,
	}, Config{
		Region:         "us-east-1",
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
	})
	discoverer.sleep = func(time.Duration) {}

	resources, err := discoverer.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources from paginated result, got %d", len(resources))
	}
	if client.calls != 2 {
		t.Fatalf("expected 2 API calls, got %d", client.calls)
	}
}

func TestDiscoverAllRetriesOnThrottling(t *testing.T) {
	client := &fakeEC2Client{
		pages: []*ec2api.DescribeInstancesOutput{
			{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{InstanceId: awsString("i-1")},
						},
					},
				},
			},
		},
		failuresBeforeSuccess: 1,
	}

	discoverer := NewDiscoverer(Clients{
		EC2: client,
	}, Config{
		Region:         "us-east-1",
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
	})
	discoverer.sleep = func(time.Duration) {}

	resources, err := discoverer.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if client.calls != 2 {
		t.Fatalf("expected 2 calls (retry path), got %d", client.calls)
	}
}

type fakeEC2Client struct {
	pages                 []*ec2api.DescribeInstancesOutput
	calls                 int
	failuresBeforeSuccess int
}

func (f *fakeEC2Client) DescribeInstances(_ context.Context, _ *ec2api.DescribeInstancesInput, _ ...func(*ec2api.Options)) (*ec2api.DescribeInstancesOutput, error) {
	f.calls++
	if f.calls <= f.failuresBeforeSuccess {
		return nil, &smithy.GenericAPIError{
			Code:    "ThrottlingException",
			Message: "rate exceeded",
		}
	}

	pageIndex := f.calls - f.failuresBeforeSuccess - 1
	if pageIndex < 0 || pageIndex >= len(f.pages) {
		return &ec2api.DescribeInstancesOutput{}, nil
	}
	return f.pages[pageIndex], nil
}
