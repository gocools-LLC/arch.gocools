# AWS Discovery Graph API

## Endpoint

`POST /api/v1/discovery/aws/graph`

## Purpose

Use UI-provided AWS credentials to discover existing infrastructure and return it as an Arch graph snapshot.

This is designed for runtime use from ARCH UI (for example ARCH running on OCI VMs while discovering AWS resources).

## Request

```json
{
  "region": "us-east-1",
  "access_key_id": "AKIA...",
  "secret_access_key": "...",
  "session_token": "...",
  "role_arn": "arn:aws:iam::123456789012:role/arch-readonly",
  "session_name": "arch-ui-session",
  "external_id": "",
  "stack_id": "dev-stack",
  "environment": "dev",
  "validate_on_start": true
}
```

## Response

```json
{
  "connected": true,
  "provider": "aws",
  "region": "us-east-1",
  "identity": {
    "account_id": "123456789012",
    "arn": "arn:aws:sts::123456789012:assumed-role/arch-readonly/arch-ui-session",
    "user_id": "..."
  },
  "graph": {
    "schema_version": "arch.gocools/v1alpha1",
    "generated_at": "2026-03-06T09:00:00Z",
    "nodes": [],
    "edges": []
  }
}
```
