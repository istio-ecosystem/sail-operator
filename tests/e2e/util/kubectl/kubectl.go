// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubectl

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
)

type Builder struct {
	binary     string
	namespace  string
	kubeconfig string
}

const DefaultBinary = "kubectl"

// NewBuilder creates a new kubectl.Builder
func NewBuilder() *Builder {
	k := &Builder{}
	k.setBinary()
	return k
}

func (k *Builder) setBinary() {
	binary := DefaultBinary
	if cmd := os.Getenv("COMMAND"); cmd != "" {
		binary = cmd
	}

	k.binary = binary
}

func (k *Builder) build(cmd string) string {
	args := []string{k.binary}

	// Only append namespace if it's set
	if k.namespace != "" {
		args = append(args, k.namespace)
	}

	// Only append kubeconfig if it's set
	if k.kubeconfig != "" {
		args = append(args, k.kubeconfig)
	}

	args = append(args, cmd)

	// Join all the arguments with a space
	return strings.Join(args, " ")
}

// SetNamespace sets the namespace
func (k *Builder) SetNamespace(ns string) *Builder {
	if ns == "" {
		k.namespace = "--all-namespaces"
	} else {
		k.namespace = fmt.Sprintf("-n %s", ns)
	}
	return k
}

// SetKubeconfig sets the kubeconfig
func (k *Builder) SetKubeconfig(kubeconfig string) *Builder {
	if kubeconfig != "" {
		k.kubeconfig = fmt.Sprintf("--kubeconfig %s", kubeconfig)
	}
	return k
}

// CreateNamespace creates a namespace
// If the namespace already exists, it will return nil
func (k *Builder) CreateNamespace(ns string) error {
	cmd := k.build(" create namespace " + ns)
	output, err := k.executeCommand(cmd)
	if err != nil {
		if strings.Contains(output, "AlreadyExists") {
			return nil
		}

		return fmt.Errorf("error creating namespace: %w, output: %s", err, output)
	}

	return nil
}

// CreateFromString creates a resource from the given yaml string
func (k *Builder) CreateFromString(yamlString string) error {
	cmd := k.build(" create -f -")
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	k.ResetNamespace()
	if err != nil {
		return fmt.Errorf("error creating resource from yaml: %w", err)
	}
	return nil
}

// DeleteCRDs deletes the CRDs by given list of crds names
func (k *Builder) DeleteCRDs(crds []string) error {
	for _, crd := range crds {
		cmd := k.build(" delete crd " + crd)
		_, err := shell.ExecuteCommand(cmd)
		if err != nil {
			k.ResetNamespace()
			return fmt.Errorf("error deleting crd %s: %w", crd, err)
		}
	}

	k.ResetNamespace()
	return nil
}

// DeleteNamespace deletes a namespace
func (k *Builder) DeleteNamespace(ns string) error {
	cmd := k.build(" delete namespace " + ns)
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting namespace: %w", err)
	}

	return nil
}

// ApplyString applies the given yaml string to the cluster
func (k *Builder) ApplyString(yamlString string) error {
	cmd := k.build(" apply --server-side -f -")
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	k.ResetNamespace()
	if err != nil {
		return fmt.Errorf("error applying yaml: %w", err)
	}

	return nil
}

// Apply applies the given yaml file to the cluster
func (k *Builder) Apply(yamlFile string) error {
	err := k.ApplyWithLabels(yamlFile, "")
	return err
}

// ApplyWithLabels applies the given yaml file to the cluster with the given labels
func (k *Builder) ApplyWithLabels(yamlFile, label string) error {
	cmd := k.build(" apply " + labelFlag(label) + " -f " + yamlFile)
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error applying yaml: %w", err)
	}

	return nil
}

// DeleteFromFile deletes a resource from the given yaml file
func (k *Builder) DeleteFromFile(yamlFile string) error {
	cmd := k.build(" delete -f " + yamlFile)
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting resource from yaml: %w", err)
	}

	return nil
}

// Delete deletes a resource based on the namespace, kind and the name
func (k *Builder) Delete(kind, name string) error {
	cmd := k.build(" delete " + kind + " " + name)
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting deployment: %w", err)
	}

	return nil
}

// Patch patches a resource
func (k *Builder) Patch(kind, name, patchType, patch string) error {
	cmd := k.build(fmt.Sprintf(" patch %s %s --type=%s -p=%q", kind, name, patchType, patch))
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error patching resource: %w", err)
	}
	return nil
}

// ForceDelete deletes a resource by removing its finalizers
func (k *Builder) ForceDelete(kind, name string) error {
	// Not all resources have finalizers, trying to remove them returns an error here.
	// We explicitly ignore the error and attempt to delete the resource anyway.
	_ = k.Patch(kind, name, "json", `[{"op": "remove", "path": "/metadata/finalizers"}]`)
	return k.Delete(kind, name)
}

// GetYAML returns the yaml of a resource
func (k *Builder) GetYAML(kind, name string) (string, error) {
	cmd := k.build(fmt.Sprintf(" get %s %s -o yaml", kind, name))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting yaml: %w, output: %s", err, output)
	}

	return output, nil
}

// GetPods returns the pods of a namespace
func (k *Builder) GetPods(args ...string) (string, error) {
	cmd := k.build(fmt.Sprintf(" get pods %s", strings.Join(args, " ")))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting pods: %w, output: %s", err, output)
	}

	return output, nil
}

// GetInternalIP returns the internal IP of a node
func (k *Builder) GetInternalIP(label string) (string, error) {
	cmd := k.build(fmt.Sprintf(" get nodes -l %s -o jsonpath='{.items[0].status.addresses[?(@.type==\"InternalIP\")].address}'", label))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting internal IP: %w, output: %s", err, output)
	}

	return output, nil
}

// Exec executes a command in the pod or specific container
func (k *Builder) Exec(pod, container, command string) (string, error) {
	cmd := k.build(fmt.Sprintf(" exec %s %s -- %s", pod, containerflag(container), command))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

// GetEvents returns the events of a namespace
func (k *Builder) GetEvents() (string, error) {
	cmd := k.build(" get events")
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting events: %w, output: %s", err, output)
	}

	return output, nil
}

// Describe returns the description of a resource
func (k *Builder) Describe(kind, name string) (string, error) {
	cmd := k.build(fmt.Sprintf(" describe %s %s", kind, name))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error describing resource: %w, output: %s", err, output)
	}

	return output, nil
}

// Logs returns the logs of a deployment
func (k *Builder) Logs(pod string, since *time.Duration) (string, error) {
	cmd := k.build(fmt.Sprintf(" logs %s %s", pod, sinceFlag(since)))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

// executeCommand handles running the command and then resets the namespace automatically
func (k *Builder) executeCommand(cmd string) (string, error) {
	result, err := shell.ExecuteCommand(cmd)
	k.ResetNamespace()
	return result, err
}

// ResetNamespace resets the namespace
func (k *Builder) ResetNamespace() {
	k.namespace = ""
}

func sinceFlag(since *time.Duration) string {
	if since == nil {
		return ""
	}
	return "--since=" + since.String()
}

func labelFlag(label string) string {
	if label == "" {
		return ""
	}
	return "-l " + label
}

func containerflag(container string) string {
	if container == "" {
		return ""
	}
	return "-c " + container
}
