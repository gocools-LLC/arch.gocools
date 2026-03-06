package aws

import (
	"strconv"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

func mapEC2Instance(instance ec2types.Instance, region string) model.Resource {
	state := ""
	if instance.State != nil {
		state = string(instance.State.Name)
	}

	resource := model.Resource{
		ID:       derefString(instance.InstanceId),
		Type:     "aws.ec2.instance",
		Provider: "aws",
		Region:   region,
		Name:     ec2NameTag(instance.Tags),
		State:    state,
		Tags:     ec2Tags(instance.Tags),
		Metadata: map[string]string{
			"instance_type": string(instance.InstanceType),
			"vpc_id":        derefString(instance.VpcId),
			"subnet_id":     derefString(instance.SubnetId),
		},
	}

	return resource
}

func mapECSService(service ecstypes.Service, clusterARN string, region string) model.Resource {
	return model.Resource{
		ID:       derefString(service.ServiceArn),
		ARN:      derefString(service.ServiceArn),
		Type:     "aws.ecs.service",
		Provider: "aws",
		Region:   region,
		Name:     derefString(service.ServiceName),
		State:    derefString(service.Status),
		Tags:     ecsTags(service.Tags),
		Metadata: map[string]string{
			"cluster_arn":   clusterARN,
			"desired_count": intToString(service.DesiredCount),
			"running_count": intToString(service.RunningCount),
			"launch_type":   string(service.LaunchType),
			"scheduling":    string(service.SchedulingStrategy),
			"platform_vers": derefString(service.PlatformVersion),
		},
	}
}

func mapLoadBalancer(loadBalancer elbv2types.LoadBalancer, region string) model.Resource {
	state := ""
	if loadBalancer.State != nil {
		state = string(loadBalancer.State.Code)
	}

	return model.Resource{
		ID:       derefString(loadBalancer.LoadBalancerArn),
		ARN:      derefString(loadBalancer.LoadBalancerArn),
		Type:     "aws.elbv2.load_balancer",
		Provider: "aws",
		Region:   region,
		Name:     derefString(loadBalancer.LoadBalancerName),
		State:    state,
		Tags:     map[string]string{},
		Metadata: map[string]string{
			"dns_name": derefString(loadBalancer.DNSName),
			"scheme":   string(loadBalancer.Scheme),
			"lb_type":  string(loadBalancer.Type),
			"vpc_id":   derefString(loadBalancer.VpcId),
		},
	}
}

func mapDBInstance(instance rdstypes.DBInstance, region string) model.Resource {
	vpcID := ""
	if instance.DBSubnetGroup != nil {
		vpcID = derefString(instance.DBSubnetGroup.VpcId)
	}

	return model.Resource{
		ID:       derefString(instance.DBInstanceIdentifier),
		ARN:      derefString(instance.DBInstanceArn),
		Type:     "aws.rds.db_instance",
		Provider: "aws",
		Region:   region,
		Name:     derefString(instance.DBInstanceIdentifier),
		State:    derefString(instance.DBInstanceStatus),
		Tags:     rdsTags(instance.TagList),
		Metadata: map[string]string{
			"engine":         derefString(instance.Engine),
			"instance_class": derefString(instance.DBInstanceClass),
			"resource_id":    derefString(instance.DbiResourceId),
			"vpc_id":         vpcID,
		},
	}
}

func ec2NameTag(tags []ec2types.Tag) string {
	for _, tag := range tags {
		if strings.EqualFold(derefString(tag.Key), "Name") {
			return derefString(tag.Value)
		}
	}
	return ""
}

func ec2Tags(tags []ec2types.Tag) map[string]string {
	mapped := make(map[string]string, len(tags))
	for _, tag := range tags {
		mapped[derefString(tag.Key)] = derefString(tag.Value)
	}
	return mapped
}

func ecsTags(tags []ecstypes.Tag) map[string]string {
	mapped := make(map[string]string, len(tags))
	for _, tag := range tags {
		mapped[derefString(tag.Key)] = derefString(tag.Value)
	}
	return mapped
}

func rdsTags(tags []rdstypes.Tag) map[string]string {
	mapped := make(map[string]string, len(tags))
	for _, tag := range tags {
		mapped[derefString(tag.Key)] = derefString(tag.Value)
	}
	return mapped
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intToString(value int32) string {
	return strconv.FormatInt(int64(value), 10)
}
