# Graph Diff API

## Endpoint

`POST /api/v1/graph/diff`

## Purpose

Preview architecture changes between two graph snapshots.

Supports stack-scoped previews with:

- `stack_id`
- `environment`

## Request

```json
{
  "stack_id": "dev-stack",
  "environment": "dev",
  "before": {
    "schema_version": "arch.gocools/v1alpha1",
    "generated_at": "2026-03-05T00:00:00Z",
    "nodes": [],
    "edges": []
  },
  "after": {
    "schema_version": "arch.gocools/v1alpha1",
    "generated_at": "2026-03-05T00:01:00Z",
    "nodes": [],
    "edges": []
  }
}
```

## Response

```json
{
  "added": 1,
  "removed": 0,
  "modified": 1,
  "changes": [
    {
      "kind": "modified",
      "node_id": "dev-node",
      "resource_type": "aws.ec2.instance",
      "changes": [
        {
          "field": "state",
          "before": "running",
          "after": "stopped"
        }
      ]
    }
  ]
}
```

## Ordering

Diff results are deterministic:

- sorted by `node_id`
- then by change kind for ties

## Large Snapshot Validation

Stress test:

```bash
go test ./internal/graph -run TestDiffGraphsLargeSnapshotStress -count=1
```

Allocation benchmark:

```bash
go test ./internal/graph -bench BenchmarkDiffGraphsLarge -benchmem -run '^$'
```
