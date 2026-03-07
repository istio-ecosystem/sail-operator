// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Script to validate ztunnel configuration completeness
// Automatically detects missing fields between upstream Istio ztunnel Helm chart values and Sail Operator ZTunnelConfig
//
// Configuration:
// - All file paths and patterns are configurable via the ScriptConfig struct below
// - To modify paths or constants, edit the getDefaultConfig() function
// - User-specific ignored fields are configured via config.yaml file

// Paths holds all configurable file paths and patterns used by the script
type Paths struct {
	// Configuration file path
	ConfigFile string

	// Pattern to find ztunnel values.yaml files in resources directory
	ZTunnelValuesPattern string

	// Directory path containing the Go types file (e.g., "api/v1/")
	SailOperatorTypesFilePath string

	// Filename of the Go types file (e.g., "values_types_extra.go")
	TypesFileName string
}

// Constants holds all configurable string constants used by the script
type Constants struct {
	// Filter string to identify versions to check
	VersionFilter string

	// YAML section name in upstream Helm charts where actual values are stored
	InternalDefaultsSection string

	// Go struct name to search for in the Sail Operator types file
	StructName string
}

// ScriptConfig holds all configuration for the validation script
type ScriptConfig struct {
	Paths     Paths
	Constants Constants
}

// ValidationConfig holds the user configuration for validation (loaded from YAML)
type ValidationConfig struct {
	IgnoreMissingFields []string `yaml:"ignore_missing_fields"`
}

// getDefaultConfig returns the default configuration for the script
func getDefaultConfig() ScriptConfig {
	return ScriptConfig{
		Paths: Paths{
			ConfigFile:                "hack/validate_ztunnel_values/config.yaml",
			ZTunnelValuesPattern:      "resources/*/charts/ztunnel/values.yaml",
			SailOperatorTypesFilePath: "api/v1/",
			TypesFileName:             "values_types_extra.go",
		},
		Constants: Constants{
			VersionFilter:           "alpha",
			InternalDefaultsSection: "_internal_defaults_do_not_set",
			StructName:              "ZTunnelConfig",
		},
	}
}

func loadValidationConfig(scriptConfig ScriptConfig) (*ValidationConfig, error) {
	configFile := scriptConfig.Paths.ConfigFile

	data, err := os.ReadFile(configFile)
	if err != nil {
		// If config file doesn't exist, print message but continue
		if os.IsNotExist(err) {
			fmt.Printf("⚠️  Validation config file missing at %s, will identify all missing fields\n", configFile)
			return &ValidationConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ValidationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if len(config.IgnoreMissingFields) > 0 {
		fmt.Printf("ℹ️  Loaded validation config: ignoring %d user-defined field(s)\n", len(config.IgnoreMissingFields))
	} else {
		fmt.Printf("ℹ️  Validation config loaded with no fields to ignore\n")
	}

	return &config, nil
}

func parseLatestZTunnelHelmValues(valuesPattern, versionFilter, internalSection string) (map[string]bool, error) {
	// Find all ztunnel values files
	valuesFiles, err := filepath.Glob(valuesPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob values files: %w", err)
	}

	if len(valuesFiles) == 0 {
		return nil, fmt.Errorf("no ztunnel values.yaml files found")
	}

	// Filter to only specified versions (e.g., alpha)
	var filteredFiles []string
	for _, file := range valuesFiles {
		if strings.Contains(file, versionFilter) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	if len(filteredFiles) == 0 {
		return nil, fmt.Errorf("no ztunnel values.yaml files found in %s versions", versionFilter)
	}

	// Use the first filtered file found (in a real implementation, you'd want to find the latest version)
	latestFile := filteredFiles[0]
	fmt.Printf("📖 Parsing upstream values from %s version: %s\n", versionFilter, latestFile)

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", latestFile, err)
	}

	// Parse YAML into generic map - this will capture ALL fields dynamically
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Extract field names dynamically from the map
	fields := make(map[string]bool)

	// The upstream ztunnel values.yaml has most fields under the internal defaults section
	// We need to extract those fields and also any top-level fields
	if internalDefaults, exists := values[internalSection]; exists {
		// Handle both map[string]any and map[any]any
		switch v := internalDefaults.(type) {
		case map[string]any:
			extractFieldNamesFromMap(v, "", fields)
		case map[any]any:
			// Convert map[any]any to map[string]any
			stringMap := make(map[string]any)
			for k, val := range v {
				if key, ok := k.(string); ok {
					stringMap[key] = val
				}
			}
			extractFieldNamesFromMap(stringMap, "", fields)
		}
	}

	// Also extract any top-level fields (excluding the internal defaults section itself)
	for key, value := range values {
		if key != internalSection {
			fields[key] = true
			if nestedMap, ok := value.(map[string]any); ok {
				extractFieldNamesFromMap(nestedMap, key, fields)
			}
		}
	}

	fmt.Printf("ℹ️  Found %d fields in upstream %s ztunnel chart\n", len(fields), versionFilter)

	return fields, nil
}

// extractFieldNamesFromMap extracts only top-level field names from a map[string]any
// We only need top-level fields since those correspond to Go struct fields in ZTunnelConfig
func extractFieldNamesFromMap(data map[string]any, prefix string, fields map[string]bool) {
	for key := range data {
		fullName := key
		if prefix != "" {
			fullName = prefix + "." + key
		}

		if prefix == "" {
			fields[fullName] = true
		}
	}
}

func parseZTunnelConfigStruct(typesFilePath, fileName, structName string) (map[string]bool, error) {
	fmt.Printf("📖 Parsing Sail Operator %s struct\n", structName)

	// Construct full file path from directory and filename
	fullFilePath := filepath.Join(typesFilePath, fileName)

	// Parse the Go file containing the target struct
	fset := token.NewFileSet()
	src, err := os.ReadFile(fullFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", fullFilePath, err)
	}

	file, err := parser.ParseFile(fset, fileName, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	fields := make(map[string]bool)

	// Find the target struct
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if x.Name.Name == structName {
				if structType, ok := x.Type.(*ast.StructType); ok {
					extractGoStructFields(structType, "", fields)
				}
			}
		}
		return true
	})

	return fields, nil
}

func extractGoStructFields(structType *ast.StructType, prefix string, fields map[string]bool) {
	for _, field := range structType.Fields.List {
		// Get JSON tag to determine field name
		var jsonName string
		if field.Tag != nil {
			tag := strings.Trim(field.Tag.Value, "`")
			for part := range strings.FieldsSeq(tag) {
				if jsonPart, found := strings.CutPrefix(part, "json:"); found {
					jsonTag := strings.Trim(jsonPart, "\"")
					jsonName = strings.Split(jsonTag, ",")[0]
					break
				}
			}
		}

		// Use field name if no JSON tag
		if jsonName == "" && len(field.Names) > 0 {
			jsonName = strings.ToLower(field.Names[0].Name)
		}

		if jsonName != "" && jsonName != "-" {
			fullName := jsonName
			if prefix != "" {
				fullName = prefix + "." + jsonName
			}
			fields[fullName] = true
		}
	}
}

func findMissingFields(upstream, sail map[string]bool, ignoreFields []string, internalSection string) []string {
	var missing []string

	// Create a map for faster lookup of ignored fields
	ignored := make(map[string]bool)
	for _, field := range ignoreFields {
		ignored[field] = true
	}

	// Find fields in upstream but not in Sail Operator
	for field := range upstream {
		// Always skip internal helm fields
		if strings.HasPrefix(field, internalSection) {
			continue
		}

		// Skip fields that are explicitly ignored by user configuration
		if ignored[field] {
			continue
		}

		if !sail[field] {
			missing = append(missing, field)
		}
	}

	return missing
}

func validateZTunnelConfig(scriptConfig ScriptConfig) error {
	fmt.Println("🔍 Validating ztunnel values completeness...")

	// 1. Load validation configuration
	config, err := loadValidationConfig(scriptConfig)
	if err != nil {
		return fmt.Errorf("failed to load validation config: %w", err)
	}

	// 2. Parse upstream ztunnel values
	upstreamFields, err := parseLatestZTunnelHelmValues(
		scriptConfig.Paths.ZTunnelValuesPattern,
		scriptConfig.Constants.VersionFilter,
		scriptConfig.Constants.InternalDefaultsSection,
	)
	if err != nil {
		return fmt.Errorf("failed to parse upstream values: %w", err)
	}

	// 3. Parse Sail Operator target struct
	sailFields, err := parseZTunnelConfigStruct(
		scriptConfig.Paths.SailOperatorTypesFilePath,
		scriptConfig.Paths.TypesFileName,
		scriptConfig.Constants.StructName,
	)
	if err != nil {
		return fmt.Errorf("failed to parse Sail config: %w", err)
	}

	// 4. Compare and report missing fields
	missing := findMissingFields(upstreamFields, sailFields, config.IgnoreMissingFields, scriptConfig.Constants.InternalDefaultsSection)

	if len(missing) > 0 {
		fmt.Printf("❌ Fields present in upstream ztunnel but missing in Sail Operator:\n")
		for _, field := range missing {
			fmt.Printf("   - %s\n", field)
		}
		return fmt.Errorf("found %d missing fields in %s. Please add them or ignore them in %s",
			len(missing), scriptConfig.Constants.StructName, scriptConfig.Paths.ConfigFile)
	}

	fmt.Printf("✅ All upstream ztunnel fields are present in Sail Operator %s\n", scriptConfig.Constants.StructName)
	return nil
}

func main() {
	config := getDefaultConfig()
	if err := validateZTunnelConfig(config); err != nil {
		fmt.Printf("❌ ZTunnel values validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("🎉 ZTunnel values validation completed successfully")
}
