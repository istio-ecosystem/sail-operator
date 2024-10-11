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
	"strconv"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
)

type Kubectl struct {
	binary     string
	namespace  string
	kubeconfig string
}

// New creates a new kubectl.Kubectl
func New() Kubectl {
	return Kubectl{}.WithBinary(os.Getenv("COMMAND"))
}

func (k Kubectl) build(cmd string) string {
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

// WithBinary returns a new Kubectl with the binary set to the given value; if the value is "", the binary is set to "kubectl"
func (k Kubectl) WithBinary(binary string) Kubectl {
	if binary == "" {
		k.binary = "kubectl"
	} else {
		k.binary = binary
	}
	return k
}

// WithNamespace returns a new Kubectl with the namespace set to the given value
func (k Kubectl) WithNamespace(ns string) Kubectl {
	if ns == "" {
		k.namespace = "--all-namespaces"
	} else {
		k.namespace = fmt.Sprintf("-n %s", ns)
	}
	return k
}

// WithKubeconfig returns a new Kubectl with kubeconfig set to the given value
func (k Kubectl) WithKubeconfig(kubeconfig string) Kubectl {
	if kubeconfig == "" {
		k.kubeconfig = ""
	} else {
		k.kubeconfig = fmt.Sprintf("--kubeconfig %s", kubeconfig)
	}
	return k
}

// CreateNamespace creates a namespace
// If the namespace already exists, it will return nil
func (k Kubectl) CreateNamespace(ns string) error {
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
func (k Kubectl) CreateFromString(yamlString string) error {
	cmd := k.build(" create -f -")
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		return fmt.Errorf("error creating resource from yaml: %w", err)
	}
	return nil
}

// DeleteCRDs deletes the CRDs by given list of crds names
func (k Kubectl) DeleteCRDs(crds []string) error {
	for _, crd := range crds {
		cmd := k.build(" delete crd " + crd)
		_, err := shell.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("error deleting crd %s: %w", crd, err)
		}
	}

	return nil
}

// DeleteNamespaceNoWait deletes a namespace and returns immediately (without waiting for the namespace to be removed).
func (k Kubectl) DeleteNamespaceNoWait(namespaces ...string) error {
	return k.deleteNamespace(namespaces, false)
}

// DeleteNamespace deletes a namespace and waits for it to be removed completely.
func (k Kubectl) DeleteNamespace(namespaces ...string) error {
	return k.deleteNamespace(namespaces, true)
}

func (k Kubectl) deleteNamespace(namespaces []string, wait bool) error {
	cmd := k.build(" delete namespace " + strings.Join(namespaces, " ") + " --wait=" + strconv.FormatBool(wait))
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting namespace: %w", err)
	}
	return nil
}

// ApplyString applies the given yaml string to the cluster
func (k Kubectl) ApplyString(yamlString string) error {
	cmd := k.build(" apply --server-side -f -")
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		return fmt.Errorf("error applying yaml: %w", err)
	}

	return nil
}

// Apply applies the given yaml file to the cluster
func (k Kubectl) Apply(yamlFile string) error {
	err := k.ApplyWithLabels(yamlFile, "")
	return err
}

// ApplyWithLabels applies the given yaml file to the cluster with the given labels
func (k Kubectl) ApplyWithLabels(yamlFile, label string) error {
	cmd := k.build(" apply " + labelFlag(label) + " -f " + yamlFile)
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error applying yaml: %w", err)
	}

	return nil
}

// DeleteFromFile deletes a resource from the given yaml file
func (k Kubectl) DeleteFromFile(yamlFile string) error {
	cmd := k.build(" delete -f " + yamlFile)
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting resource from yaml: %w", err)
	}

	return nil
}

// Delete deletes a resource based on the namespace, kind and the name
func (k Kubectl) Delete(kind, name string) error {
	cmd := k.build(" delete " + kind + " " + name)
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting deployment: %w", err)
	}

	return nil
}

// Wait waits for a specific condition on one or many resources
func (k Kubectl) Wait(waitFor, resource string, timeout time.Duration) error {
	cmd := k.build(fmt.Sprintf("wait --for %s %s --timeout %s", waitFor, resource, timeout.String()))
	_, err := k.executeCommand(cmd)
	return err
}

// Patch patches a resource
func (k Kubectl) Patch(kind, name, patchType, patch string) error {
	cmd := k.build(fmt.Sprintf(" patch %s %s --type=%s -p=%q", kind, name, patchType, patch))
	_, err := k.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("error patching resource: %w", err)
	}
	return nil
}

// ForceDelete deletes a resource by removing its finalizers
func (k Kubectl) ForceDelete(kind, name string) error {
	// Not all resources have finalizers, trying to remove them returns an error here.
	// We explicitly ignore the error and attempt to delete the resource anyway.
	_ = k.Patch(kind, name, "json", `[{"op": "remove", "path": "/metadata/finalizers"}]`)
	return k.Delete(kind, name)
}

// GetYAML returns the yaml of a resource
func (k Kubectl) GetYAML(kind, name string) (string, error) {
	cmd := k.build(fmt.Sprintf(" get %s %s -o yaml", kind, name))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting yaml: %w, output: %s", err, output)
	}

	return output, nil
}

// GetPods returns the pods of a namespace
func (k Kubectl) GetPods(args ...string) (string, error) {
	cmd := k.build(fmt.Sprintf(" get pods %s", strings.Join(args, " ")))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting pods: %w, output: %s", err, output)
	}

	return output, nil
}

// GetInternalIP returns the internal IP of a node
func (k Kubectl) GetInternalIP(label string) (string, error) {
	cmd := k.build(fmt.Sprintf(" get nodes -l %s -o jsonpath='{.items[0].status.addresses[?(@.type==\"InternalIP\")].address}'", label))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting internal IP: %w, output: %s", err, output)
	}

	return output, nil
}

// Exec executes a command in the pod or specific container
func (k Kubectl) Exec(pod, container, command string) (string, error) {
	cmd := k.build(fmt.Sprintf(" exec %s %s -- %s", pod, containerFlag(container), command))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

// GetEvents returns the events of a namespace
func (k Kubectl) GetEvents() (string, error) {
	cmd := k.build(" get events")
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting events: %w, output: %s", err, output)
	}

	return output, nil
}

// Describe returns the description of a resource
func (k Kubectl) Describe(kind, name string) (string, error) {
	cmd := k.build(fmt.Sprintf(" describe %s %s", kind, name))
	output, err := k.executeCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error describing resource: %w, output: %s", err, output)
	}

	return output, nil
}

// Logs returns the logs of a deployment
func (k Kubectl) Logs(pod string, since *time.Duration) (string, error) {
	cmd := k.build(fmt.Sprintf(" logs %s %s", pod, sinceFlag(since)))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

// executeCommand handles running the command and then resets the namespace automatically
func (k Kubectl) executeCommand(cmd string) (string, error) {
	return shell.ExecuteCommand(cmd)
}

// WaitNamespaceDeleted waits for a namespace to be deleted
func (k Kubectl) WaitNamespaceDeleted(ns string) error {
	return k.Wait("delete", "namespace/"+ns, 2*time.Minute)
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

func containerFlag(container string) string {
	if container == "" {
		return ""
	}
	return "-c " + container
}
