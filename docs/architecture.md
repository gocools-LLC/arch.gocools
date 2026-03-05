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
