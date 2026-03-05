# Stack Lifecycle Operations

Arch stack lifecycle service supports:

- `create`
- `update`
- `scale`
- `destroy`

Endpoint:

- `POST /api/v1/stacks/operations`

## Safety Guardrails

- destroy requires `confirm=true`
- destroy in `prod` requires `manual_override=true`
- update/destroy fail fast when `gocools:owner` is missing
- dry-run mode (`dry_run=true`) returns operation result without mutating stack state
- create/update enforce required tags:
  - `gocools:stack-id`
  - `gocools:environment`
  - `gocools:owner`

When owner-tag validation fails, API errors include remediation guidance.

## Audit Logging

Each successful operation emits an audit entry with:

- `timestamp`
- `actor`
- `stack_id`
- `environment`
- `action`
- `dry_run`
- `result`

## Example

```json
{
  "action": "destroy",
  "stack_id": "dev-stack",
  "environment": "dev",
  "actor": "alice",
  "confirm": true
}
```
