# Architecture Graph Schema

## Schema Version

Current schema identifier:

`arch.gocools/v1alpha1`

Every graph payload includes:

- `schema_version`
- `generated_at`
- `nodes[]`
- `edges[]`

## Node Schema

Node fields:

- `id`
- `type`
- `provider`
- `region`
- `name` (optional)
- `state` (optional)
- `arn` (optional)
- `tags` (optional map)
- `metadata` (optional map)

## Edge Schema

Edge fields:

- `from`
- `to`
- `type`
- `metadata` (optional map)

## Compatibility Strategy

- additive changes only within `v1alpha1`
- removal/renaming requires a new schema version
- compatibility tests enforce preserved core fields for existing payloads

