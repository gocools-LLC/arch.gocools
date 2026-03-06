# Arch v0.1.0 Release Checklist

## Versioning

Arch uses Semantic Versioning:

- `MAJOR`: breaking API/schema changes
- `MINOR`: backward-compatible feature additions
- `PATCH`: backward-compatible fixes

Target release: `v0.1.0`

## Pre-Release Validation

- [ ] `go test ./...`
- [ ] `make smoke-local`
- [ ] `go build ./...`
- [ ] `terraform validate` check covered by exporter tests

## Smoke Test Matrix

### API availability

- health and readiness endpoints return `200`

### Graph functions

- graph query supports `stack_id` + `environment` filters
- graph diff returns deterministic changes

### Control plane safety

- prod destroy blocked without `manual_override=true`
- required tags enforced on create/update

### Terraform workflows

- import fixture parses successfully
- export output validates with Terraform

## Release Steps

1. Verify branch `main` is green.
2. Update release notes/changelog.
3. Tag release:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

4. Confirm CI release artifacts.

## Post-Release

- [ ] publish release notes
- [ ] verify downloaded artifacts
- [ ] open patch milestone for follow-up fixes
