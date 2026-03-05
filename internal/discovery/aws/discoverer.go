package aws

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	ec2api "github.com/aws/aws-sdk-go-v2/service/ec2"
	ecsapi "github.com/aws/aws-sdk-go-v2/service/ecs"
	elbv2api "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	rdsapi "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/smithy-go"
	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

type Config struct {
	Region               string
	MaxAttempts          int
	InitialBackoff       time.Duration
	MaxBackoff           time.Duration
	MaxPagesPerOperation int
	JitterFraction       float64
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
	clients              Clients
	region               string
	maxAttempts          int
	initialBackoff       time.Duration
	maxBackoff           time.Duration
	maxPagesPerOperation int
	jitterFraction       float64
	sleep                func(time.Duration)
	randFloat64          func() float64

	metrics struct {
		throttledResponses    atomic.Uint64
		retryAttempts         atomic.Uint64
		retryExhausted        atomic.Uint64
		pagesFetched          atomic.Uint64
		maxPageDepth          atomic.Uint64
		paginationLimitErrors atomic.Uint64
	}
}

type DiscoveryMetrics struct {
	ThrottledResponses   uint64 `json:"throttled_responses"`
	RetryAttempts        uint64 `json:"retry_attempts"`
	RetryExhausted       uint64 `json:"retry_exhausted"`
	PagesFetched         uint64 `json:"pages_fetched"`
	MaxPageDepth         uint64 `json:"max_page_depth"`
	PaginationLimitError uint64 `json:"pagination_limit_errors"`
}

var ErrPaginationLimitExceeded = errors.New("discovery pagination limit exceeded")

func NewDiscoverer(clients Clients, cfg Config) *Discoverer {
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	initialBackoff := cfg.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = 250 * time.Millisecond
	}

	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 5 * time.Second
	}
	if maxBackoff < initialBackoff {
		maxBackoff = initialBackoff
	}

	maxPagesPerOperation := cfg.MaxPagesPerOperation
	if maxPagesPerOperation <= 0 {
		maxPagesPerOperation = 500
	}

	jitterFraction := cfg.JitterFraction
	if jitterFraction <= 0 {
		jitterFraction = 0.2
	}
	if jitterFraction > 1 {
		jitterFraction = 1
	}

	return &Discoverer{
		clients:              clients,
		region:               cfg.Region,
		maxAttempts:          maxAttempts,
		initialBackoff:       initialBackoff,
		maxBackoff:           maxBackoff,
		maxPagesPerOperation: maxPagesPerOperation,
		jitterFraction:       jitterFraction,
		sleep:                time.Sleep,
		randFloat64:          rand.Float64,
	}
}

func (d *Discoverer) Metrics() DiscoveryMetrics {
	return DiscoveryMetrics{
		ThrottledResponses:   d.metrics.throttledResponses.Load(),
		RetryAttempts:        d.metrics.retryAttempts.Load(),
		RetryExhausted:       d.metrics.retryExhausted.Load(),
		PagesFetched:         d.metrics.pagesFetched.Load(),
		MaxPageDepth:         d.metrics.maxPageDepth.Load(),
		PaginationLimitError: d.metrics.paginationLimitErrors.Load(),
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
	seenTokens := map[string]struct{}{}

	for pageDepth := 1; ; pageDepth++ {
		if err := d.ensurePageDepth("ec2.describe_instances", pageDepth); err != nil {
			return nil, err
		}

		output, err := d.describeInstancesWithRetry(ctx, &ec2api.DescribeInstancesInput{
			NextToken: token,
		})
		if err != nil {
			return nil, err
		}
		d.observePageDepth(pageDepth)

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				resources = append(resources, mapEC2Instance(instance, d.region))
			}
		}

		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		if _, exists := seenTokens[*output.NextToken]; exists {
			return nil, fmt.Errorf("ec2.describe_instances pagination token cycle detected at depth %d: %q", pageDepth, *output.NextToken)
		}
		seenTokens[*output.NextToken] = struct{}{}
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
	seenMarkers := map[string]struct{}{}

	for pageDepth := 1; ; pageDepth++ {
		if err := d.ensurePageDepth("elbv2.describe_load_balancers", pageDepth); err != nil {
			return nil, err
		}

		output, err := d.describeLoadBalancersWithRetry(ctx, &elbv2api.DescribeLoadBalancersInput{
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}
		d.observePageDepth(pageDepth)

		for _, loadBalancer := range output.LoadBalancers {
			resources = append(resources, mapLoadBalancer(loadBalancer, d.region))
		}

		if output.NextMarker == nil || *output.NextMarker == "" {
			break
		}
		if _, exists := seenMarkers[*output.NextMarker]; exists {
			return nil, fmt.Errorf("elbv2.describe_load_balancers pagination token cycle detected at depth %d: %q", pageDepth, *output.NextMarker)
		}
		seenMarkers[*output.NextMarker] = struct{}{}
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
	seenMarkers := map[string]struct{}{}

	for pageDepth := 1; ; pageDepth++ {
		if err := d.ensurePageDepth("rds.describe_db_instances", pageDepth); err != nil {
			return nil, err
		}

		output, err := d.describeDBInstancesWithRetry(ctx, &rdsapi.DescribeDBInstancesInput{
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}
		d.observePageDepth(pageDepth)

		for _, dbInstance := range output.DBInstances {
			resources = append(resources, mapDBInstance(dbInstance, d.region))
		}

		if output.Marker == nil || *output.Marker == "" {
			break
		}
		if _, exists := seenMarkers[*output.Marker]; exists {
			return nil, fmt.Errorf("rds.describe_db_instances pagination token cycle detected at depth %d: %q", pageDepth, *output.Marker)
		}
		seenMarkers[*output.Marker] = struct{}{}
		marker = output.Marker
	}

	return resources, nil
}

func (d *Discoverer) listAllClusterARNs(ctx context.Context) ([]string, error) {
	clusters := make([]string, 0)
	var token *string
	seenTokens := map[string]struct{}{}

	for pageDepth := 1; ; pageDepth++ {
		if err := d.ensurePageDepth("ecs.list_clusters", pageDepth); err != nil {
			return nil, err
		}

		output, err := d.listClustersWithRetry(ctx, &ecsapi.ListClustersInput{
			NextToken: token,
		})
		if err != nil {
			return nil, err
		}
		d.observePageDepth(pageDepth)

		clusters = append(clusters, output.ClusterArns...)
		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		if _, exists := seenTokens[*output.NextToken]; exists {
			return nil, fmt.Errorf("ecs.list_clusters pagination token cycle detected at depth %d: %q", pageDepth, *output.NextToken)
		}
		seenTokens[*output.NextToken] = struct{}{}
		token = output.NextToken
	}

	return clusters, nil
}

func (d *Discoverer) listAllServiceARNs(ctx context.Context, clusterARN string) ([]string, error) {
	services := make([]string, 0)
	var token *string
	seenTokens := map[string]struct{}{}

	for pageDepth := 1; ; pageDepth++ {
		if err := d.ensurePageDepth("ecs.list_services", pageDepth); err != nil {
			return nil, err
		}

		output, err := d.listServicesWithRetry(ctx, &ecsapi.ListServicesInput{
			Cluster:   awsString(clusterARN),
			NextToken: token,
		})
		if err != nil {
			return nil, err
		}
		d.observePageDepth(pageDepth)

		services = append(services, output.ServiceArns...)
		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		if _, exists := seenTokens[*output.NextToken]; exists {
			return nil, fmt.Errorf("ecs.list_services pagination token cycle detected at depth %d: %q", pageDepth, *output.NextToken)
		}
		seenTokens[*output.NextToken] = struct{}{}
		token = output.NextToken
	}

	return services, nil
}

func (d *Discoverer) describeInstancesWithRetry(ctx context.Context, input *ec2api.DescribeInstancesInput) (*ec2api.DescribeInstancesOutput, error) {
	return withRetry(ctx, d, func() (*ec2api.DescribeInstancesOutput, error) {
		return d.clients.EC2.DescribeInstances(ctx, input)
	})
}

func (d *Discoverer) listClustersWithRetry(ctx context.Context, input *ecsapi.ListClustersInput) (*ecsapi.ListClustersOutput, error) {
	return withRetry(ctx, d, func() (*ecsapi.ListClustersOutput, error) {
		return d.clients.ECS.ListClusters(ctx, input)
	})
}

func (d *Discoverer) listServicesWithRetry(ctx context.Context, input *ecsapi.ListServicesInput) (*ecsapi.ListServicesOutput, error) {
	return withRetry(ctx, d, func() (*ecsapi.ListServicesOutput, error) {
		return d.clients.ECS.ListServices(ctx, input)
	})
}

func (d *Discoverer) describeServicesWithRetry(ctx context.Context, input *ecsapi.DescribeServicesInput) (*ecsapi.DescribeServicesOutput, error) {
	return withRetry(ctx, d, func() (*ecsapi.DescribeServicesOutput, error) {
		return d.clients.ECS.DescribeServices(ctx, input)
	})
}

func (d *Discoverer) describeLoadBalancersWithRetry(ctx context.Context, input *elbv2api.DescribeLoadBalancersInput) (*elbv2api.DescribeLoadBalancersOutput, error) {
	return withRetry(ctx, d, func() (*elbv2api.DescribeLoadBalancersOutput, error) {
		return d.clients.ELBV2.DescribeLoadBalancers(ctx, input)
	})
}

func (d *Discoverer) describeDBInstancesWithRetry(ctx context.Context, input *rdsapi.DescribeDBInstancesInput) (*rdsapi.DescribeDBInstancesOutput, error) {
	return withRetry(ctx, d, func() (*rdsapi.DescribeDBInstancesOutput, error) {
		return d.clients.RDS.DescribeDBInstances(ctx, input)
	})
}

func withRetry[T any](ctx context.Context, discoverer *Discoverer, fn func() (T, error)) (T, error) {
	var zero T
	backoff := discoverer.initialBackoff

	for attempt := 1; attempt <= discoverer.maxAttempts; attempt++ {
		output, err := fn()
		if err == nil {
			return output, nil
		}

		if !isThrottlingError(err) {
			return zero, err
		}

		discoverer.metrics.throttledResponses.Add(1)
		if attempt == discoverer.maxAttempts {
			discoverer.metrics.retryExhausted.Add(1)
			return zero, err
		}

		discoverer.metrics.retryAttempts.Add(1)
		delay := discoverer.jitteredBackoff(backoff)
		if err := waitWithContext(ctx, discoverer.sleep, delay); err != nil {
			return zero, err
		}
		backoff = discoverer.nextBackoff(backoff)
	}

	return zero, errors.New("unreachable retry state")
}

func (d *Discoverer) ensurePageDepth(operation string, pageDepth int) error {
	if pageDepth <= d.maxPagesPerOperation {
		return nil
	}

	d.metrics.paginationLimitErrors.Add(1)
	return fmt.Errorf("%w: operation=%s max_pages=%d", ErrPaginationLimitExceeded, operation, d.maxPagesPerOperation)
}

func (d *Discoverer) observePageDepth(pageDepth int) {
	d.metrics.pagesFetched.Add(1)
	pageDepthUint := uint64(pageDepth)

	for {
		current := d.metrics.maxPageDepth.Load()
		if pageDepthUint <= current {
			return
		}
		if d.metrics.maxPageDepth.CompareAndSwap(current, pageDepthUint) {
			return
		}
	}
}

func (d *Discoverer) jitteredBackoff(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	if d.jitterFraction <= 0 {
		return base
	}

	jitterScale := (d.randFloat64()*2 - 1) * d.jitterFraction
	delay := time.Duration(float64(base) * (1 + jitterScale))
	if delay < 0 {
		return 0
	}
	return delay
}

func (d *Discoverer) nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next <= 0 {
		return d.maxBackoff
	}
	if next > d.maxBackoff {
		return d.maxBackoff
	}
	return next
}

func waitWithContext(ctx context.Context, sleepFn func(time.Duration), delay time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	sleepFn(delay)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
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
