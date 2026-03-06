# Arch

Cloud architecture visualizer and control plane for safe stack operations.

## Features
- AWS infrastructure discovery
- interactive architecture graph
- Terraform import/export workflow
- stack lifecycle management with policy enforcement

## Quick Start

```bash
go run ./cmd/arch
curl -s localhost:8081/healthz
curl -s "localhost:8081/api/v1/graph?stack_id=dev-stack&environment=dev"
curl -s -X POST localhost:8081/api/v1/graph/diff \
  -H 'content-type: application/json' \
  -d '{"stack_id":"dev-stack","environment":"dev","before":{"schema_version":"arch.gocools/v1alpha1","generated_at":"2026-03-05T00:00:00Z","nodes":[],"edges":[]},"after":{"schema_version":"arch.gocools/v1alpha1","generated_at":"2026-03-05T00:01:00Z","nodes":[],"edges":[]}}'
curl -s -X POST localhost:8081/api/v1/stacks/operations \
  -H 'content-type: application/json' \
  -d '{"action":"create","stack_id":"dev-stack","environment":"dev","actor":"alice","tags":{"gocools:stack-id":"dev-stack","gocools:environment":"dev","gocools:owner":"alice"}}'
curl -s -X POST localhost:8081/api/v1/drift \
  -H 'content-type: application/json' \
  -d '{"desired":[{"id":"i-1","type":"aws.ec2.instance","state":"running"}],"actual":[{"id":"i-1","type":"aws.ec2.instance","state":"stopped"}]}'
make smoke-local
```

Use AWS-backed discovery (instead of static demo resources):

```bash
ARCH_DISCOVERY_MODE=aws \
ARCH_AWS_REGION=us-east-1 \
ARCH_AWS_ROLE_ARN=arn:aws:iam::123456789012:role/arch-observer \
ARCH_AWS_SESSION_NAME=arch-session \
ARCH_AWS_VALIDATE_ON_START=true \
go run ./cmd/arch
```

## Repository Layout

- `cmd/arch`: CLI entrypoint.
- `internal/`: internal application logic.
- `pkg/`: reusable packages.
- `docs/`: architecture, roadmap, and RFCs.

## Security Model

- AWS STS AssumeRole (no permanent access keys)
- least-privilege IAM roles
- tag-based ownership and safe destructive operations

## Required Resource Tags

```text
gocools:stack-id
gocools:environment
gocools:owner
```

## Documentation

- [Architecture](docs/architecture.md)
- [Graph Schema](docs/graph-schema.md)
- [Graph API](docs/api/graph.md)
- [Graph Diff API](docs/api/graph-diff.md)
- [AWS Discovery Engine](docs/discovery-engine.md)
- [Terraform State Import](docs/terraform-import.md)
- [Terraform Export](docs/terraform-export.md)
- [Stack Lifecycle](docs/stack-lifecycle.md)
- [Drift Detection](docs/drift-detection.md)
- [Policy Engine](docs/policy-engine.md)
- [Release Checklist](docs/release-checklist.md)
- [Release Notes v0.1.1](docs/releases/v0.1.1.md)
- [Release Notes v0.1.0](docs/releases/v0.1.0.md)
- [Roadmap](docs/roadmap.md)
- [RFC-0001](docs/rfc/rfc-0001-platform.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
