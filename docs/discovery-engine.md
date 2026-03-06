# AWS Discovery Engine

Arch discovery is implemented in `internal/discovery/aws`.

## Runtime Modes

Arch supports two graph-discovery modes:

- `ARCH_DISCOVERY_MODE=static` (default): serves deterministic built-in demo resources
- `ARCH_DISCOVERY_MODE=aws`: queries live AWS APIs through the discovery engine

For AWS mode:

- `ARCH_AWS_REGION` (or `AWS_REGION`) should be set
- optional role assumption is supported via:
  - `ARCH_AWS_ROLE_ARN`
  - `ARCH_AWS_SESSION_NAME`
  - `ARCH_AWS_EXTERNAL_ID`
- optional startup credential validation:
  - `ARCH_AWS_VALIDATE_ON_START=true`

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
- throttling errors are retried with adaptive jittered exponential backoff
- service-specific mappers produce stable, provider-normalized resources
- pagination loops are bounded by a deterministic max-page limit per operation

## Discovery Metrics

`Discoverer.Metrics()` exposes:

- `throttled_responses`
- `retry_attempts`
- `retry_exhausted`
- `pages_fetched`
- `max_page_depth`
- `pagination_limit_errors`
