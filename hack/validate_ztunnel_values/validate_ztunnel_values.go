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
	"reflect"
	"sort"
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

// NestedStructMapping maps an upstream YAML section to a Go struct for validation
type NestedStructMapping struct {
	StructName string
}

// Constants holds all configurable string constants used by the script
type Constants struct {
	// Filter string to identify versions to check
	VersionFilter string

	// YAML section name in upstream Helm charts where actual values are stored
	InternalDefaultsSection string

	// Go struct name to search for in the Sail Operator types file
	StructName string

	// Maps upstream nested section names to Go struct names for separate validation.
	// e.g., "global" -> ZTunnelGlobalConfig means the fields under the upstream "global"
	// section are validated against the ZTunnelGlobalConfig struct instead of ZTunnelConfig.
	NestedStructs map[string]NestedStructMapping
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

// UpstreamFields holds parsed upstream YAML fields separated by section
type UpstreamFields struct {
	// Top-level fields belonging to the main struct
	MainFields map[string]bool
	// Fields belonging to nested sections, keyed by section name
	NestedFields map[string]map[string]bool
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
			NestedStructs: map[string]NestedStructMapping{
				"global": {StructName: "ZTunnelGlobalConfig"},
			},
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

func parseLatestZTunnelHelmValues(
	valuesPattern, versionFilter, internalSection string,
	nestedSections map[string]NestedStructMapping,
) (*UpstreamFields, error) {
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

	latestFile := filteredFiles[0]
	fmt.Printf("📖 Parsing upstream values from %s version: %s\n", versionFilter, latestFile)

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", latestFile, err)
	}

	// Parse YAML into generic map
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", latestFile, err)
	}

	result := &UpstreamFields{
		MainFields:   make(map[string]bool),
		NestedFields: make(map[string]map[string]bool),
	}

	// Extract fields from the internal defaults section
	if internalDefaults, exists := values[internalSection]; exists {
		defaultsMap := toStringMap(internalDefaults)
		for key, val := range defaultsMap {
			if _, isNested := nestedSections[key]; isNested {
				nested := make(map[string]bool)
				for subKey := range toStringMap(val) {
					nested[subKey] = true
				}
				result.NestedFields[key] = nested
			} else {
				result.MainFields[key] = true
			}
		}
	}

	// Also extract any top-level fields (excluding the internal defaults section itself)
	for key := range values {
		if key != internalSection {
			if _, isNested := nestedSections[key]; isNested {
				nested := make(map[string]bool)
				for subKey := range toStringMap(values[key]) {
					nested[subKey] = true
				}
				if existing, ok := result.NestedFields[key]; ok {
					for k := range nested {
						existing[k] = true
					}
				} else {
					result.NestedFields[key] = nested
				}
			} else {
				result.MainFields[key] = true
			}
		}
	}

	totalFields := len(result.MainFields)
	for section, fields := range result.NestedFields {
		fmt.Printf("ℹ️  Found %d fields in upstream %s.%s section\n", len(fields), versionFilter, section)
		totalFields += len(fields)
	}
	fmt.Printf("ℹ️  Found %d fields in upstream %s ztunnel chart (%d top-level + %d in nested sections)\n",
		totalFields, versionFilter, len(result.MainFields), totalFields-len(result.MainFields))

	return result, nil
}

// toStringMap converts an any value to map[string]any, handling both map[string]any and map[any]any
func toStringMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		result := make(map[string]any)
		for k, val := range m {
			if key, ok := k.(string); ok {
				result[key] = val
			}
		}
		return result
	}
	return nil
}

func parseGoStructFields(typesFilePath, fileName, structName string) (map[string]bool, error) {
	fmt.Printf("📖 Parsing Sail Operator %s struct\n", structName)

	fullFilePath := filepath.Join(typesFilePath, fileName)

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
		var jsonName string
		if field.Tag != nil {
			tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
			if jsonTag, ok := tag.Lookup("json"); ok {
				jsonName = strings.Split(jsonTag, ",")[0]
			}
		}

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

func findMissingFields(upstream, sail map[string]bool, ignored map[string]bool) []string {
	var missing []string

	// Build case-insensitive lookup of Sail Operator fields
	sailLower := make(map[string]bool)
	for field := range sail {
		sailLower[strings.ToLower(field)] = true
	}

	for field := range upstream {
		lower := strings.ToLower(field)

		if ignored[lower] {
			continue
		}

		if !sailLower[lower] {
			missing = append(missing, field)
		}
	}

	sort.Strings(missing)
	return missing
}

func validateZTunnelConfig(scriptConfig ScriptConfig) error {
	fmt.Println("🔍 Validating ztunnel values completeness...")

	config, err := loadValidationConfig(scriptConfig)
	if err != nil {
		return fmt.Errorf("failed to load validation config: %w", err)
	}

	// Build case-insensitive ignore map, supporting both "field" and "section.field" syntax
	ignored := make(map[string]bool)
	for _, field := range config.IgnoreMissingFields {
		ignored[strings.ToLower(field)] = true
	}

	upstreamFields, err := parseLatestZTunnelHelmValues(
		scriptConfig.Paths.ZTunnelValuesPattern,
		scriptConfig.Constants.VersionFilter,
		scriptConfig.Constants.InternalDefaultsSection,
		scriptConfig.Constants.NestedStructs,
	)
	if err != nil {
		return fmt.Errorf("failed to parse upstream values: %w", err)
	}

	// Parse the main struct
	sailFields, err := parseGoStructFields(
		scriptConfig.Paths.SailOperatorTypesFilePath,
		scriptConfig.Paths.TypesFileName,
		scriptConfig.Constants.StructName,
	)
	if err != nil {
		return fmt.Errorf("failed to parse Sail config: %w", err)
	}

	// Parse all nested structs and build a combined field set.
	// The ztunnel chart's zzz_profile.yaml flattens .Values.global into the top-level
	// defaults via mustMergeOverwrite, so fields are accessible at both levels.
	// A field is "covered" if it exists in ANY of the structs.
	combinedFields := make(map[string]bool)
	for field := range sailFields {
		combinedFields[field] = true
	}

	for section, mapping := range scriptConfig.Constants.NestedStructs {
		nestedSailFields, err := parseGoStructFields(
			scriptConfig.Paths.SailOperatorTypesFilePath,
			scriptConfig.Paths.TypesFileName,
			mapping.StructName,
		)
		if err != nil {
			return fmt.Errorf("failed to parse %s struct: %w", mapping.StructName, err)
		}

		for field := range nestedSailFields {
			combinedFields[field] = true
		}

		// Validate nested section fields against the combined set
		if nestedUpstream, ok := upstreamFields.NestedFields[section]; ok {
			nestedIgnored := make(map[string]bool)
			for ignoredField := range ignored {
				if strings.HasPrefix(ignoredField, strings.ToLower(section)+".") {
					nestedIgnored[strings.TrimPrefix(ignoredField, strings.ToLower(section)+".")] = true
				}
			}

			nestedMissing := findMissingFields(nestedUpstream, combinedFields, nestedIgnored)
			if len(nestedMissing) > 0 {
				sort.Strings(nestedMissing)
				fmt.Printf("❌ Fields present in upstream ztunnel %s section but missing in Sail Operator:\n", section)
				for _, field := range nestedMissing {
					fmt.Printf("   - %s.%s\n", section, field)
				}
				return fmt.Errorf("found %d missing fields in %s section. Please add them or ignore them in %s",
					len(nestedMissing), section, scriptConfig.Paths.ConfigFile)
			}
		}
	}

	// Validate top-level fields against the combined set
	mainMissing := findMissingFields(upstreamFields.MainFields, combinedFields, ignored)
	if len(mainMissing) > 0 {
		sort.Strings(mainMissing)
		fmt.Printf("❌ Fields present in upstream ztunnel but missing in Sail Operator:\n")
		for _, field := range mainMissing {
			fmt.Printf("   - %s\n", field)
		}
		return fmt.Errorf("found %d missing fields. Please add them or ignore them in %s",
			len(mainMissing), scriptConfig.Paths.ConfigFile)
	}

	fmt.Println("✅ All upstream ztunnel fields are present in Sail Operator")
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
