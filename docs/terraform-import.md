# Terraform State Import

Arch can import Terraform state into the architecture graph model.

Implementation: `internal/terraform/importer.go`

## Import Behavior

- parses Terraform state JSON (`values.root_module`)
- traverses root and nested `child_modules`
- imports `managed` resources into normalized resource model
- maps provider names (for example `.../aws`, `.../random`)
- converts imported resources into graph nodes using the versioned graph schema

## Error Model

Importer returns actionable errors:

- `parse terraform state json: ...` for invalid JSON
- `terraform state missing values.root_module` for invalid structure

## Included Sample Fixture

- `internal/terraform/testdata/state_sample.json`

This fixture covers:

- root module resources
- child module resources
- multiple providers

