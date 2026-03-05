# Drift Detection

Arch drift detection compares desired and actual resource inventories.

Implementation: `internal/drift/detector.go`

## Drift Categories

- `added`: present in actual but not desired
- `missing`: present in desired but not actual
- `changed`: present in both with field differences

## False Positive Reduction

Default ignored metadata keys:

- `last_updated`
- `updated_at`
- `timestamp`
- `created_at`

Additional ignored keys can be passed per request.

## API Format

Endpoint:

`POST /api/v1/drift`

Request:

```json
{
  "desired": [],
  "actual": [],
  "ignored_metadata_keys": ["last_seen"]
}
```

Response:

```json
{
  "generated_at": "2026-03-05T00:00:00Z",
  "added": 1,
  "missing": 1,
  "changed": 1,
  "items": []
}
```

Report shape is designed for both API consumers and UI visualization.

