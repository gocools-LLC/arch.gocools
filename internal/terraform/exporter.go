package terraform

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/gocools-LLC/arch.gocools/internal/graph"
)

type ExportResult struct {
	Config      string
	Unsupported []string
}

func ExportGraph(g graph.Graph) (ExportResult, error) {
	nodes := append([]graph.Node{}, g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	var builder strings.Builder
	builder.WriteString("terraform {\n")
	builder.WriteString("  required_version = \">= 1.6.0\"\n")
	builder.WriteString("}\n\n")

	builder.WriteString("locals {\n")
	builder.WriteString(fmt.Sprintf("  architecture_schema_version = %q\n", g.SchemaVersion))
	builder.WriteString("}\n\n")

	unsupported := make([]string, 0)

	for _, node := range nodes {
		if !isSupportedNode(node) {
			unsupported = append(unsupported, node.ID)
			continue
		}

		resourceName := sanitizeTerraformName(node.ID)

		builder.WriteString(fmt.Sprintf("resource \"terraform_data\" %q {\n", resourceName))
		builder.WriteString("  input = {\n")
		builder.WriteString(fmt.Sprintf("    id       = %q\n", node.ID))
		builder.WriteString(fmt.Sprintf("    type     = %q\n", node.Type))
		builder.WriteString(fmt.Sprintf("    provider = %q\n", node.Provider))
		builder.WriteString(fmt.Sprintf("    region   = %q\n", node.Region))
		if node.Name != "" {
			builder.WriteString(fmt.Sprintf("    name     = %q\n", node.Name))
		}
		if node.State != "" {
			builder.WriteString(fmt.Sprintf("    state    = %q\n", node.State))
		}
		if node.ARN != "" {
			builder.WriteString(fmt.Sprintf("    arn      = %q\n", node.ARN))
		}

		writeMapAsNestedHCL(&builder, "tags", node.Tags)
		writeMapAsNestedHCL(&builder, "metadata", node.Metadata)

		builder.WriteString("  }\n")
		builder.WriteString("}\n\n")
	}

	sort.Strings(unsupported)

	builder.WriteString("output \"architecture_unsupported_nodes\" {\n")
	builder.WriteString("  value = [\n")
	for _, item := range unsupported {
		builder.WriteString(fmt.Sprintf("    %q,\n", item))
	}
	builder.WriteString("  ]\n")
	builder.WriteString("}\n")

	return ExportResult{
		Config:      builder.String(),
		Unsupported: unsupported,
	}, nil
}

func isSupportedNode(node graph.Node) bool {
	if node.ID == "" || node.Type == "" {
		return false
	}

	switch node.Provider {
	case "", "aws", "terraform":
		return true
	default:
		return false
	}
}

func writeMapAsNestedHCL(builder *strings.Builder, key string, value map[string]string) {
	if len(value) == 0 {
		return
	}

	keys := make([]string, 0, len(value))
	for item := range value {
		keys = append(keys, item)
	}
	sort.Strings(keys)

	builder.WriteString(fmt.Sprintf("    %s = {\n", key))
	for _, item := range keys {
		builder.WriteString(fmt.Sprintf("      %q = %q\n", item, value[item]))
	}
	builder.WriteString("    }\n")
}

func sanitizeTerraformName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "node"
	}

	var builder strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' {
			builder.WriteRune(unicode.ToLower(char))
			continue
		}
		builder.WriteRune('_')
	}

	sanitized := strings.Trim(builder.String(), "_")
	if sanitized == "" {
		sanitized = "node"
	}
	if unicode.IsDigit(rune(sanitized[0])) {
		sanitized = "node_" + sanitized
	}
	return sanitized
}
