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
- [AWS Discovery Engine](docs/discovery-engine.md)
- [Terraform State Import](docs/terraform-import.md)
- [Roadmap](docs/roadmap.md)
- [RFC-0001](docs/rfc/rfc-0001-platform.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
