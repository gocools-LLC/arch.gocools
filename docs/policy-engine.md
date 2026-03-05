# Policy Engine

Arch evaluates policy decisions before stack operations execute.

Implementation:

- `internal/policy/engine`

## Baseline Rule

Rule ID: `deny.prod.destroy.without-manual-override`

Behavior:

- denies `destroy` action in `prod` when `manual_override != true`
- returns explicit deny reason:
  `policy deny: production destroy requires manual_override=true`

## Execution Order

1. operation request is received
2. policy engine evaluates request context
3. deny result blocks operation before mutation
4. allow result proceeds to operation-specific checks

