# Terraform Export

Arch can export the architecture graph to deterministic Terraform configuration.

Implementation: `internal/terraform/exporter.go`

## Export Characteristics

- deterministic output ordering (stable for the same graph input)
- exported Terraform uses `terraform_data` resources to represent graph nodes
- schema version is preserved in `locals.architecture_schema_version`
- unsupported nodes are listed in `output.architecture_unsupported_nodes`

## Currently Supported Node Providers

- `aws`
- `terraform`
- empty provider (`""`) for generic nodes

## Current Unsupported Example

Resources with unsupported providers (for example `gcp`) are not exported as `terraform_data` resources and appear in:

`output "architecture_unsupported_nodes"`

## Validation

The exporter test suite runs:

- `terraform init -backend=false`
- `terraform validate`

against generated config to ensure output remains Terraform-valid.

