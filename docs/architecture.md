# Arch Architecture

## Purpose

Cloud architecture visualizer and control plane for safe stack operations.

## High-Level Model

GoCools platform layers:

```text
nard.gocools
   -> arch.gocools
      -> flow.gocools
```

This repository focuses on **Arch** and integrates with the other layers through APIs and shared stack metadata.

## Core Capabilities

- AWS infrastructure discovery
- interactive architecture graph
- Terraform import/export workflow
- stack lifecycle management with policy enforcement

## Guardrails

All managed cloud resources must include:

```text
gocools:stack-id
gocools:environment
gocools:owner
```

Destructive actions require stack validation and environment-aware protections.

## Discovery Engine

Arch includes a normalized AWS discovery engine in `internal/discovery/aws` for:

- EC2 instances
- ECS services
- ELBv2 load balancers
- RDS DB instances

The engine returns stable resource identifiers and supports paginated API traversal with throttling retries.

## Graph API

Arch exposes a versioned graph schema via API:

- endpoint: `GET /api/v1/graph`
- filter support: `stack_id`, `environment`
- schema version: `arch.gocools/v1alpha1`

See:

- [graph-schema.md](graph-schema.md)
- [api/graph.md](api/graph.md)

## Terraform Import

Arch includes a Terraform state importer that converts Terraform state resources into normalized architecture graph resources.

See:

- [terraform-import.md](terraform-import.md)

## Terraform Export

Arch includes graph-to-Terraform export that emits deterministic configuration and reports unsupported resources.

See:

- [terraform-export.md](terraform-export.md)

## Stack Lifecycle Control

Arch includes guarded stack operations with audit logging:

- `create`, `update`, `scale`, `destroy`
- `confirm` enforcement for destroy
- production destroy protection via `manual_override`
- dry-run execution mode

See:

- [stack-lifecycle.md](stack-lifecycle.md)

## Drift Detection

Arch includes a drift detection prototype that reports:

- added resources
- missing resources
- changed resources

with severity and field-level change details.

See:

- [drift-detection.md](drift-detection.md)
