package aws

import (
	"context"
	"errors"
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

	metrics := discoverer.Metrics()
	if metrics.PagesFetched != 2 || metrics.MaxPageDepth != 2 {
		t.Fatalf("unexpected pagination metrics: %+v", metrics)
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

	metrics := discoverer.Metrics()
	if metrics.ThrottledResponses != 1 || metrics.RetryAttempts != 1 {
		t.Fatalf("unexpected retry metrics: %+v", metrics)
	}
}

func TestDiscoverAll_AdaptiveBackoffUsesJitter(t *testing.T) {
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
		failuresBeforeSuccess: 2,
	}

	discoverer := NewDiscoverer(Clients{
		EC2: client,
	}, Config{
		Region:         "us-east-1",
		MaxAttempts:    5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     400 * time.Millisecond,
		JitterFraction: 0.5,
	})

	randValues := []float64{1, 0}
	randIndex := 0
	discoverer.randFloat64 = func() float64 {
		value := randValues[randIndex]
		randIndex++
		return value
	}

	delays := make([]time.Duration, 0, 2)
	discoverer.sleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}

	_, err := discoverer.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	if len(delays) != 2 {
		t.Fatalf("expected 2 retry delays, got %d", len(delays))
	}
	if delays[0] != 150*time.Millisecond {
		t.Fatalf("expected first jittered delay 150ms, got %s", delays[0])
	}
	if delays[1] != 100*time.Millisecond {
		t.Fatalf("expected second jittered delay 100ms, got %s", delays[1])
	}
}

func TestDiscoverAll_StopsAtPaginationLimit(t *testing.T) {
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
				NextToken: awsString("page-3"),
			},
		},
	}

	discoverer := NewDiscoverer(Clients{
		EC2: client,
	}, Config{
		Region:               "us-east-1",
		MaxAttempts:          3,
		InitialBackoff:       time.Millisecond,
		MaxPagesPerOperation: 2,
	})
	discoverer.sleep = func(time.Duration) {}

	_, err := discoverer.DiscoverAll(context.Background())
	if !errors.Is(err, ErrPaginationLimitExceeded) {
		t.Fatalf("expected ErrPaginationLimitExceeded, got %v", err)
	}

	if client.calls != 2 {
		t.Fatalf("expected 2 page calls before limit trigger, got %d", client.calls)
	}

	metrics := discoverer.Metrics()
	if metrics.PagesFetched != 2 || metrics.MaxPageDepth != 2 || metrics.PaginationLimitError != 1 {
		t.Fatalf("unexpected pagination guard metrics: %+v", metrics)
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
