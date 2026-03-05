# AWS Discovery Engine

Arch discovery is implemented in `internal/discovery/aws`.

## Covered Services (MVP)

- EC2 (`DescribeInstances`)
- ECS (`ListClusters`, `ListServices`, `DescribeServices`)
- ELBv2 (`DescribeLoadBalancers`)
- RDS (`DescribeDBInstances`)

## Normalized Resource Model

All discoveries are normalized to:

- `id`
- `type`
- `provider`
- `region`
- `arn`
- `name`
- `state`
- `tags`
- `metadata`

## Operational Behavior

- paginated AWS APIs are traversed until completion
- throttling errors are retried with exponential backoff
- service-specific mappers produce stable, provider-normalized resources

