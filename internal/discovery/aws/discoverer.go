package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ec2api "github.com/aws/aws-sdk-go-v2/service/ec2"
	ecsapi "github.com/aws/aws-sdk-go-v2/service/ecs"
	elbv2api "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	rdsapi "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/smithy-go"
	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

type Config struct {
	Region         string
	MaxAttempts    int
	InitialBackoff time.Duration
}

type EC2Client interface {
	DescribeInstances(ctx context.Context, params *ec2api.DescribeInstancesInput, optFns ...func(*ec2api.Options)) (*ec2api.DescribeInstancesOutput, error)
}

type ECSClient interface {
	ListClusters(ctx context.Context, params *ecsapi.ListClustersInput, optFns ...func(*ecsapi.Options)) (*ecsapi.ListClustersOutput, error)
	ListServices(ctx context.Context, params *ecsapi.ListServicesInput, optFns ...func(*ecsapi.Options)) (*ecsapi.ListServicesOutput, error)
	DescribeServices(ctx context.Context, params *ecsapi.DescribeServicesInput, optFns ...func(*ecsapi.Options)) (*ecsapi.DescribeServicesOutput, error)
}

type ELBV2Client interface {
	DescribeLoadBalancers(ctx context.Context, params *elbv2api.DescribeLoadBalancersInput, optFns ...func(*elbv2api.Options)) (*elbv2api.DescribeLoadBalancersOutput, error)
}

type RDSClient interface {
	DescribeDBInstances(ctx context.Context, params *rdsapi.DescribeDBInstancesInput, optFns ...func(*rdsapi.Options)) (*rdsapi.DescribeDBInstancesOutput, error)
}

type Clients struct {
	EC2   EC2Client
	ECS   ECSClient
	ELBV2 ELBV2Client
	RDS   RDSClient
}

type Discoverer struct {
	clients        Clients
	region         string
	maxAttempts    int
	initialBackoff time.Duration
	sleep          func(time.Duration)
}

func NewDiscoverer(clients Clients, cfg Config) *Discoverer {
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	initialBackoff := cfg.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = 250 * time.Millisecond
	}

	return &Discoverer{
		clients:        clients,
		region:         cfg.Region,
		maxAttempts:    maxAttempts,
		initialBackoff: initialBackoff,
		sleep:          time.Sleep,
	}
}

func (d *Discoverer) DiscoverAll(ctx context.Context) ([]model.Resource, error) {
	resources := make([]model.Resource, 0)

	ec2Resources, err := d.discoverEC2(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover ec2: %w", err)
	}
	resources = append(resources, ec2Resources...)

	ecsResources, err := d.discoverECS(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover ecs: %w", err)
	}
	resources = append(resources, ecsResources...)

	elbResources, err := d.discoverELBV2(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover elbv2: %w", err)
	}
	resources = append(resources, elbResources...)

	rdsResources, err := d.discoverRDS(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover rds: %w", err)
	}
	resources = append(resources, rdsResources...)

	return resources, nil
}

func (d *Discoverer) discoverEC2(ctx context.Context) ([]model.Resource, error) {
	if d.clients.EC2 == nil {
		return []model.Resource{}, nil
	}

	resources := make([]model.Resource, 0)
	var token *string

	for {
		output, err := d.describeInstancesWithRetry(ctx, &ec2api.DescribeInstancesInput{
			NextToken: token,
		})
		if err != nil {
			return nil, err
		}

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				resources = append(resources, mapEC2Instance(instance, d.region))
			}
		}

		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		token = output.NextToken
	}

	return resources, nil
}

func (d *Discoverer) discoverECS(ctx context.Context) ([]model.Resource, error) {
	if d.clients.ECS == nil {
		return []model.Resource{}, nil
	}

	resources := make([]model.Resource, 0)
	clusters, err := d.listAllClusterARNs(ctx)
	if err != nil {
		return nil, err
	}

	for _, clusterARN := range clusters {
		serviceARNs, err := d.listAllServiceARNs(ctx, clusterARN)
		if err != nil {
			return nil, err
		}
		if len(serviceARNs) == 0 {
			continue
		}

		output, err := d.describeServicesWithRetry(ctx, &ecsapi.DescribeServicesInput{
			Cluster:  awsString(clusterARN),
			Services: serviceARNs,
		})
		if err != nil {
			return nil, err
		}

		for _, service := range output.Services {
			resources = append(resources, mapECSService(service, clusterARN, d.region))
		}
	}

	return resources, nil
}

func (d *Discoverer) discoverELBV2(ctx context.Context) ([]model.Resource, error) {
	if d.clients.ELBV2 == nil {
		return []model.Resource{}, nil
	}

	resources := make([]model.Resource, 0)
	var marker *string

	for {
		output, err := d.describeLoadBalancersWithRetry(ctx, &elbv2api.DescribeLoadBalancersInput{
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}

		for _, loadBalancer := range output.LoadBalancers {
			resources = append(resources, mapLoadBalancer(loadBalancer, d.region))
		}

		if output.NextMarker == nil || *output.NextMarker == "" {
			break
		}
		marker = output.NextMarker
	}

	return resources, nil
}

func (d *Discoverer) discoverRDS(ctx context.Context) ([]model.Resource, error) {
	if d.clients.RDS == nil {
		return []model.Resource{}, nil
	}

	resources := make([]model.Resource, 0)
	var marker *string

	for {
		output, err := d.describeDBInstancesWithRetry(ctx, &rdsapi.DescribeDBInstancesInput{
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}

		for _, dbInstance := range output.DBInstances {
			resources = append(resources, mapDBInstance(dbInstance, d.region))
		}

		if output.Marker == nil || *output.Marker == "" {
			break
		}
		marker = output.Marker
	}

	return resources, nil
}

func (d *Discoverer) listAllClusterARNs(ctx context.Context) ([]string, error) {
	clusters := make([]string, 0)
	var token *string

	for {
		output, err := d.listClustersWithRetry(ctx, &ecsapi.ListClustersInput{
			NextToken: token,
		})
		if err != nil {
			return nil, err
		}

		clusters = append(clusters, output.ClusterArns...)
		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		token = output.NextToken
	}

	return clusters, nil
}

func (d *Discoverer) listAllServiceARNs(ctx context.Context, clusterARN string) ([]string, error) {
	services := make([]string, 0)
	var token *string

	for {
		output, err := d.listServicesWithRetry(ctx, &ecsapi.ListServicesInput{
			Cluster:   awsString(clusterARN),
			NextToken: token,
		})
		if err != nil {
			return nil, err
		}

		services = append(services, output.ServiceArns...)
		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		token = output.NextToken
	}

	return services, nil
}

func (d *Discoverer) describeInstancesWithRetry(ctx context.Context, input *ec2api.DescribeInstancesInput) (*ec2api.DescribeInstancesOutput, error) {
	return withRetry(d.maxAttempts, d.initialBackoff, d.sleep, func() (*ec2api.DescribeInstancesOutput, error) {
		return d.clients.EC2.DescribeInstances(ctx, input)
	})
}

func (d *Discoverer) listClustersWithRetry(ctx context.Context, input *ecsapi.ListClustersInput) (*ecsapi.ListClustersOutput, error) {
	return withRetry(d.maxAttempts, d.initialBackoff, d.sleep, func() (*ecsapi.ListClustersOutput, error) {
		return d.clients.ECS.ListClusters(ctx, input)
	})
}

func (d *Discoverer) listServicesWithRetry(ctx context.Context, input *ecsapi.ListServicesInput) (*ecsapi.ListServicesOutput, error) {
	return withRetry(d.maxAttempts, d.initialBackoff, d.sleep, func() (*ecsapi.ListServicesOutput, error) {
		return d.clients.ECS.ListServices(ctx, input)
	})
}

func (d *Discoverer) describeServicesWithRetry(ctx context.Context, input *ecsapi.DescribeServicesInput) (*ecsapi.DescribeServicesOutput, error) {
	return withRetry(d.maxAttempts, d.initialBackoff, d.sleep, func() (*ecsapi.DescribeServicesOutput, error) {
		return d.clients.ECS.DescribeServices(ctx, input)
	})
}

func (d *Discoverer) describeLoadBalancersWithRetry(ctx context.Context, input *elbv2api.DescribeLoadBalancersInput) (*elbv2api.DescribeLoadBalancersOutput, error) {
	return withRetry(d.maxAttempts, d.initialBackoff, d.sleep, func() (*elbv2api.DescribeLoadBalancersOutput, error) {
		return d.clients.ELBV2.DescribeLoadBalancers(ctx, input)
	})
}

func (d *Discoverer) describeDBInstancesWithRetry(ctx context.Context, input *rdsapi.DescribeDBInstancesInput) (*rdsapi.DescribeDBInstancesOutput, error) {
	return withRetry(d.maxAttempts, d.initialBackoff, d.sleep, func() (*rdsapi.DescribeDBInstancesOutput, error) {
		return d.clients.RDS.DescribeDBInstances(ctx, input)
	})
}

func withRetry[T any](maxAttempts int, initialBackoff time.Duration, sleepFn func(time.Duration), fn func() (T, error)) (T, error) {
	var zero T
	backoff := initialBackoff

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, err := fn()
		if err == nil {
			return output, nil
		}

		if !isThrottlingError(err) || attempt == maxAttempts {
			return zero, err
		}

		sleepFn(backoff)
		backoff *= 2
	}

	return zero, errors.New("unreachable retry state")
}

func isThrottlingError(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := strings.ToLower(apiErr.ErrorCode())
	return strings.Contains(code, "throttl")
}

func awsString(value string) *string {
	return &value
}
