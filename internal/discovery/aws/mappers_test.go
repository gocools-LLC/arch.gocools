package aws

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

func TestMapEC2Instance(t *testing.T) {
	resource := mapEC2Instance(ec2types.Instance{
		InstanceId:   awsString("i-123"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		VpcId:        awsString("vpc-1"),
		SubnetId:     awsString("subnet-1"),
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
		Tags: []ec2types.Tag{
			{
				Key:   awsString("Name"),
				Value: awsString("app-node"),
			},
		},
	}, "us-east-1")

	if resource.ID != "i-123" {
		t.Fatalf("expected resource id i-123, got %q", resource.ID)
	}
	if resource.Name != "app-node" {
		t.Fatalf("expected name app-node, got %q", resource.Name)
	}
	if resource.Type != "aws.ec2.instance" {
		t.Fatalf("expected type aws.ec2.instance, got %q", resource.Type)
	}
}

func TestMapECSService(t *testing.T) {
	resource := mapECSService(ecstypes.Service{
		ServiceArn:   awsString("arn:aws:ecs:us-east-1:123:service/cluster/service-a"),
		ServiceName:  awsString("service-a"),
		Status:       awsString("ACTIVE"),
		DesiredCount: 2,
		RunningCount: 2,
		LaunchType:   ecstypes.LaunchTypeFargate,
		Tags: []ecstypes.Tag{
			{
				Key:   awsString("gocools:owner"),
				Value: awsString("team-a"),
			},
		},
	}, "arn:aws:ecs:us-east-1:123:cluster/cluster-a", "us-east-1")

	if resource.ID == "" || resource.ARN == "" {
		t.Fatal("expected ecs service id/arn to be set")
	}
	if resource.State != "ACTIVE" {
		t.Fatalf("expected state ACTIVE, got %q", resource.State)
	}
	if resource.Metadata["desired_count"] != "2" {
		t.Fatalf("expected desired_count 2, got %q", resource.Metadata["desired_count"])
	}
}

func TestMapLoadBalancer(t *testing.T) {
	resource := mapLoadBalancer(elbv2types.LoadBalancer{
		LoadBalancerArn:  awsString("arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/app/lb-a"),
		LoadBalancerName: awsString("lb-a"),
		DNSName:          awsString("lb-a.elb.amazonaws.com"),
		Type:             elbv2types.LoadBalancerTypeEnumApplication,
		Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
		State: &elbv2types.LoadBalancerState{
			Code: elbv2types.LoadBalancerStateEnumActive,
		},
	}, "us-east-1")

	if resource.Type != "aws.elbv2.load_balancer" {
		t.Fatalf("expected elbv2 resource type, got %q", resource.Type)
	}
	if resource.State != "active" {
		t.Fatalf("expected state active, got %q", resource.State)
	}
	if resource.Metadata["dns_name"] == "" {
		t.Fatal("expected dns_name metadata to be set")
	}
}

func TestMapDBInstance(t *testing.T) {
	resource := mapDBInstance(rdstypes.DBInstance{
		DBInstanceArn:        awsString("arn:aws:rds:us-east-1:123:db:db-a"),
		DBInstanceIdentifier: awsString("db-a"),
		DBInstanceStatus:     awsString("available"),
		Engine:               awsString("postgres"),
		DBInstanceClass:      awsString("db.t3.micro"),
		DbiResourceId:        awsString("db-abc"),
		TagList: []rdstypes.Tag{
			{
				Key:   awsString("gocools:environment"),
				Value: awsString("dev"),
			},
		},
	}, "us-east-1")

	if resource.ID != "db-a" {
		t.Fatalf("expected db id db-a, got %q", resource.ID)
	}
	if resource.Metadata["engine"] != "postgres" {
		t.Fatalf("expected engine postgres, got %q", resource.Metadata["engine"])
	}
	if resource.Tags["gocools:environment"] != "dev" {
		t.Fatalf("expected env tag dev, got %q", resource.Tags["gocools:environment"])
	}
}
