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
	"bytes"
	"fmt"
	"sort"
	"text/template"

	"github.com/istio-ecosystem/sail-operator/pkg/analyze"
)

func main() {
	metricDescriptions := analyze.ListMetrics()
	sort.Slice(metricDescriptions, func(i, j int) bool {
		return metricDescriptions[i].Name < metricDescriptions[j].Name
	})

	tmpl, err := template.New("Sail Operator Metrics").Parse("# Sail Operator Metrics\n" +
		"This document describes the custom telemetry metrics for the Sail operator and CRD usage." +
		"It aims to provide a list of those metrics that are collected and exposed by the operator.\n\n" +
		"The following section outlines the usage and limitations on metric counts and cardinality." +
		"The last section provides a development guide about how to add additional metrics for new functionalities.\n\n" +
		"## Sail Operator Custom Metrics List" +
		"{{range .}}\n" +
		"### {{.Name}}\n" +
		"{{.Help}} " +
		"Type: {{.Type}}.\n" +
		"{{end}}\n" +
		"## Developing new metrics\n" +
		"After developing new metrics or changing old ones, please run \"make generate-metricsdocs\" to regenerate this document.\n" +
		"If you feel that the new metric doesn't follow these rules, please change \"analyze/metricsdocs\" according to your needs.")
	if err != nil {
		panic(err)
	}

	// generate the template using the sorted list of metrics
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, metricDescriptions); err != nil {
		panic(err)
	}

	// print the generated metrics documentation
	fmt.Println(buf.String())
}
