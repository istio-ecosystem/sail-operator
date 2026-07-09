//go:build e2e

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

package operandimages

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	"gopkg.in/yaml.v3"
)

const (
	csvKind            = "csv"
	deploymentKind     = "deployment"
	olmOwnerAnnotation = "olm.owner"
	csvSucceededPhase  = "Succeeded"
)

// Options configures CSV operand image discovery.
type Options struct {
	Namespace      string // default: env NAMESPACE or "sail-operator"
	CSVName        string // default: env OPERATOR_CSV, else discover
	DeploymentName string // default: env DEPLOYMENT_NAME; used to pick install deployment entry
}

type csvDocument struct {
	Spec struct {
		Install struct {
			Spec struct {
				Deployments []csvInstallDeployment `json:"deployments" yaml:"deployments"`
			} `json:"spec" yaml:"spec"`
		} `json:"install" yaml:"install"`
	} `json:"spec" yaml:"spec"`
}

type csvInstallDeployment struct {
	Name string `json:"name" yaml:"name"`
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations" yaml:"annotations"`
			} `json:"metadata" yaml:"metadata"`
		} `json:"template" yaml:"template"`
	} `json:"spec" yaml:"spec"`
}

type csvList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name" yaml:"name"`
		} `json:"metadata" yaml:"metadata"`
		Status struct {
			Phase string `json:"phase" yaml:"phase"`
		} `json:"status" yaml:"status"`
	} `json:"items" yaml:"items"`
}

type deploymentDocument struct {
	Metadata struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
}

// MergeFromCSV reads operand image refs from the operator CSV install deployment
// annotations and merges them into config.Config.ImageDigests.
func MergeFromCSV(k kubectl.Kubectl, opts Options) error {
	opts.applyDefaults()
	explicitCSV := opts.CSVName != ""

	csvName, err := resolveCSVName(k, opts)
	if err != nil {
		if explicitCSV {
			return err
		}
		return nil
	}
	if csvName == "" {
		return nil
	}

	annotations, err := fetchInstallDeploymentAnnotations(k, opts.Namespace, csvName, opts.DeploymentName)
	if err != nil {
		if explicitCSV {
			return err
		}
		return nil
	}

	digests, err := config.ParseImageDigestsFromAnnotations(annotations)
	if err != nil {
		return err
	}
	if len(digests) == 0 {
		return nil
	}

	config.MergeImageDigests(digests)
	GinkgoWriter.Printf("merged %d operand image digest entries from CSV %s\n", len(digests), csvName)
	return nil
}

func (o *Options) applyDefaults() {
	if o.Namespace == "" {
		o.Namespace = env.Get("NAMESPACE", "sail-operator")
	}
	if o.DeploymentName == "" {
		o.DeploymentName = env.Get("DEPLOYMENT_NAME", "sail-operator")
	}
	if o.CSVName == "" {
		o.CSVName = os.Getenv("OPERATOR_CSV")
	}
}

func resolveCSVName(k kubectl.Kubectl, opts Options) (string, error) {
	if opts.CSVName != "" {
		return opts.CSVName, nil
	}

	fromOwner, err := csvFromOLMOwner(k, opts.Namespace, opts.DeploymentName)
	if err != nil {
		return "", err
	}
	if fromOwner != "" {
		return fromOwner, nil
	}

	return highestSucceededCSV(k, opts.Namespace)
}

func csvFromOLMOwner(k kubectl.Kubectl, namespace, deploymentName string) (string, error) {
	yamlDoc, err := k.WithNamespace(namespace).GetYAML(deploymentKind, deploymentName)
	if err != nil {
		if isNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("getting operator deployment %s/%s: %w", namespace, deploymentName, err)
	}

	var dep deploymentDocument
	if err := yaml.Unmarshal([]byte(yamlDoc), &dep); err != nil {
		return "", fmt.Errorf("parsing operator deployment %s/%s: %w", namespace, deploymentName, err)
	}

	return dep.Metadata.Annotations[olmOwnerAnnotation], nil
}

func highestSucceededCSV(k kubectl.Kubectl, namespace string) (string, error) {
	output, err := k.WithNamespace(namespace).GetYAML("csv", "")
	if err != nil {
		if isNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("listing CSVs in namespace %s: %w", namespace, err)
	}

	var list csvList
	if err := yaml.Unmarshal([]byte(output), &list); err != nil {
		return "", fmt.Errorf("parsing CSV list in namespace %s: %w", namespace, err)
	}

	var (
		bestName    string
		bestVersion *semver.Version
	)
	for _, item := range list.Items {
		if item.Status.Phase != csvSucceededPhase {
			continue
		}
		version, err := csvSemver(item.Metadata.Name)
		if err != nil {
			continue
		}
		if bestVersion == nil || version.GreaterThan(bestVersion) {
			bestVersion = version
			bestName = item.Metadata.Name
		}
	}
	return bestName, nil
}

func csvSemver(csvName string) (*semver.Version, error) {
	idx := strings.Index(csvName, ".v")
	if idx < 0 {
		return nil, fmt.Errorf("CSV name %q has no version suffix", csvName)
	}
	return semver.NewVersion(csvName[idx+1:])
}

func fetchInstallDeploymentAnnotations(k kubectl.Kubectl, namespace, csvName, deploymentName string) (map[string]string, error) {
	yamlDoc, err := k.WithNamespace(namespace).GetYAML(csvKind, csvName)
	if err != nil {
		return nil, fmt.Errorf("getting CSV %s/%s: %w", namespace, csvName, err)
	}
	annotations, err := parseInstallDeploymentAnnotations(yamlDoc, deploymentName)
	if err != nil {
		return nil, fmt.Errorf("CSV %s/%s: %w", namespace, csvName, err)
	}
	return annotations, nil
}

func isNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "NotFound") || strings.Contains(msg, "not found")
}

// parseInstallDeploymentAnnotations extracts install deployment template annotations from CSV YAML.
func parseInstallDeploymentAnnotations(yamlDoc, deploymentName string) (map[string]string, error) {
	var doc csvDocument
	if err := yaml.Unmarshal([]byte(yamlDoc), &doc); err != nil {
		return nil, err
	}
	for _, dep := range doc.Spec.Install.Spec.Deployments {
		if dep.Name == deploymentName {
			return dep.Spec.Template.Metadata.Annotations, nil
		}
	}
	names := make([]string, 0, len(doc.Spec.Install.Spec.Deployments))
	for _, dep := range doc.Spec.Install.Spec.Deployments {
		names = append(names, dep.Name)
	}
	return nil, fmt.Errorf("install deployment %q not found; available: %s", deploymentName, strings.Join(names, ", "))
}
