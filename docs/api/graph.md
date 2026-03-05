# Graph API

## Endpoint

`GET /api/v1/graph`

## Query Parameters

- `stack_id` (optional): filter resources by `gocools:stack-id`
- `environment` (optional): filter resources by `gocools:environment`

## Response

Status `200 OK`:

```json
{
  "schema_version": "arch.gocools/v1alpha1",
  "generated_at": "2026-03-05T00:00:00Z",
  "nodes": [
    {
      "id": "resource-dev",
      "type": "aws.ec2.instance",
      "provider": "aws",
      "region": "us-east-1",
      "tags": {
        "gocools:stack-id": "dev-stack",
        "gocools:environment": "dev"
      }
    }
  ],
  "edges": []
}
```

## Notes

- if no filters are provided, the full graph is returned
- stack/environment filters can be combined

