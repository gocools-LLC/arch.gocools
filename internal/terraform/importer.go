package terraform

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
	"github.com/gocools-LLC/arch.gocools/internal/graph"
)

type ImportResult struct {
	Resources []model.Resource
	Graph     graph.Graph
}

type stateDocument struct {
	Values *stateValues `json:"values"`
}

type stateValues struct {
	RootModule *stateModule `json:"root_module"`
}

type stateModule struct {
	Address      string          `json:"address"`
	Resources    []stateResource `json:"resources"`
	ChildModules []stateModule   `json:"child_modules"`
}

type stateResource struct {
	Address      string                 `json:"address"`
	Mode         string                 `json:"mode"`
	Type         string                 `json:"type"`
	Name         string                 `json:"name"`
	ProviderName string                 `json:"provider_name"`
	Values       map[string]interface{} `json:"values"`
}

func ImportState(stateJSON []byte, generatedAt time.Time) (ImportResult, error) {
	var doc stateDocument
	if err := json.Unmarshal(stateJSON, &doc); err != nil {
		return ImportResult{}, fmt.Errorf("parse terraform state json: %w", err)
	}

	if doc.Values == nil || doc.Values.RootModule == nil {
		return ImportResult{}, errors.New("terraform state missing values.root_module")
	}

	resources := make([]model.Resource, 0)
	resources = append(resources, collectModuleResources(*doc.Values.RootModule)...)

	return ImportResult{
		Resources: resources,
		Graph:     graph.FromResources(resources, generatedAt),
	}, nil
}

func collectModuleResources(module stateModule) []model.Resource {
	collected := make([]model.Resource, 0)

	for _, resource := range module.Resources {
		if resource.Mode != "managed" {
			continue
		}
		collected = append(collected, mapStateResource(resource, module.Address))
	}

	for _, child := range module.ChildModules {
		collected = append(collected, collectModuleResources(child)...)
	}

	return collected
}

func mapStateResource(resource stateResource, moduleAddress string) model.Resource {
	resourceID := stringValue(resource.Values["id"])
	if resourceID == "" {
		resourceID = resource.Address
	}

	resourceName := stringValue(resource.Values["name"])
	if resourceName == "" {
		resourceName = resource.Name
	}

	arn := stringValue(resource.Values["arn"])
	region := stringValue(resource.Values["region"])
	if region == "" {
		region = stringValue(resource.Values["availability_zone"])
	}

	provider := providerFromName(resource.ProviderName)
	return model.Resource{
		ID:       resourceID,
		Type:     "terraform." + resource.Type,
		Provider: provider,
		Region:   region,
		ARN:      arn,
		Name:     resourceName,
		State:    "managed",
		Tags:     mapStringString(resource.Values["tags"]),
		Metadata: map[string]string{
			"terraform_address":  resource.Address,
			"terraform_mode":     resource.Mode,
			"terraform_provider": resource.ProviderName,
			"terraform_module":   moduleAddress,
		},
	}
}

func providerFromName(providerName string) string {
	if providerName == "" {
		return "terraform"
	}

	parts := strings.Split(providerName, "/")
	if len(parts) == 0 {
		return providerName
	}
	return parts[len(parts)-1]
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func mapStringString(value interface{}) map[string]string {
	typed, ok := value.(map[string]interface{})
	if !ok {
		return map[string]string{}
	}

	mapped := make(map[string]string, len(typed))
	for key, item := range typed {
		mapped[key] = stringValue(item)
	}
	return mapped
}
