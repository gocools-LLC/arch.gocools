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

- AWS infrastructure discovery\n- interactive architecture graph\n- Terraform import/export workflow\n- stack lifecycle management with policy enforcement

## Guardrails

All managed cloud resources must include:

```text
gocools:stack-id
gocools:environment
gocools:owner
```

Destructive actions require stack validation and environment-aware protections.
