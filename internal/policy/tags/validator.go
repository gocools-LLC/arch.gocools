package tags

import (
	"fmt"
	"strings"
)

var requiredTagKeys = []string{
	"gocools:stack-id",
	"gocools:environment",
	"gocools:owner",
}

func Validate(tags map[string]string, stackID string, environment string) error {
	if tags == nil {
		return fmt.Errorf("missing required tags: %s; remediation: include tags %s", strings.Join(requiredTagKeys, ", "), strings.Join(requiredTagKeys, ", "))
	}

	missing := make([]string, 0)
	for _, key := range requiredTagKeys {
		if strings.TrimSpace(tags[key]) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required tags: %s; remediation: include tags %s", strings.Join(missing, ", "), strings.Join(requiredTagKeys, ", "))
	}

	if tags["gocools:stack-id"] != stackID {
		return fmt.Errorf("tag gocools:stack-id must equal stack_id; remediation: set gocools:stack-id=%s", stackID)
	}
	if tags["gocools:environment"] != environment {
		return fmt.Errorf("tag gocools:environment must equal environment; remediation: set gocools:environment=%s", environment)
	}

	return nil
}
