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

const DefaultBinary = "kubectl"

// kubectl return the kubectl command
// If the environment variable COMMAND is set, it will return the value of COMMAND
// Otherwise, it will return the default value "kubectl" as default
// Arguments:
// - format: format of the command without kubeclt or oc
// - args: arguments of the command
func kubectl(format string, args ...interface{}) string {
	binary := DefaultBinary
	if cmd := os.Getenv("COMMAND"); cmd != "" {
		binary = cmd
	}

	return binary + " " + fmt.Sprintf(format, args...)
}

// CreateFromString creates a resource from the given yaml string
func CreateFromString(yamlString string) error {
	cmd := kubectl("create -f -")
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		return fmt.Errorf("error creating resource from yaml: %w", err)
	}
	return nil
}

// ApplyString applies the given yaml string to the cluster
func ApplyString(ns, yamlString string) error {
	cmd := kubectl("apply -n %s --server-side -f -", ns)
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		return fmt.Errorf("error applying yaml: %w", err)
	}

	return nil
}

// CreateNamespace creates a namespace
// If the namespace already exists, it will return nil
func CreateNamespace(ns string) error {
	cmd := kubectl("create namespace %s", ns)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		if strings.Contains(output, "AlreadyExists") {
			return nil
		}

		return fmt.Errorf("error creating namespace: %w, output: %s", err, output)
	}

	return nil
}

// DeleteNamespace deletes a namespace
func DeleteNamespace(ns string) error {
	cmd := kubectl("delete namespace %s", ns)
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting namespace: %w", err)
	}

	return nil
}

// Delete deletes a resource based on the namespace, kind and the name
func Delete(ns, kind, name string) error {
	cmd := kubectl("delete %s %s %s", kind, name, nsflag(ns))
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting deployment: %w", err)
	}

	return nil
}

// DeleteCRDs deletes the CRDs by given list of crds names
func DeleteCRDs(crds []string) error {
	for _, crd := range crds {
		cmd := kubectl("delete crd %s", crd)
		_, err := shell.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("error deleting crd %s: %w", crd, err)
		}
	}

	return nil
}

// Patch patches a resource.
func Patch(ns, kind, name, patchType, patch string) error {
	cmd := kubectl(`patch %s %s %s --type=%s -p=%q`, kind, name, prepend("-n", ns), patchType, patch)
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error patching resource: %w", err)
	}
	return nil
}

// ForceDelete deletes a resource by removing its finalizers.
func ForceDelete(ns, kind, name string) error {
	// Not all resources have finalizers, trying to remove them returns an error here.
	// We explicitly ignore the error and attempt to delete the resource anyway.
	_ = Patch(ns, kind, name, "json", `[{"op": "remove", "path": "/metadata/finalizers"}]`)
	return Delete(ns, kind, name)
}

// GetYAML returns the yaml of a resource
// Arguments:
// - ns: namespace
// - kind: type of the resource
// - name: name of the resource
func GetYAML(ns, kind, name string) (string, error) {
	cmd := kubectl("get %s %s %s -o yaml", kind, name, nsflag(ns))
	return shell.ExecuteCommand(cmd)
}

// GetPods returns the pods of a namespace
func GetPods(ns string, args ...string) (string, error) {
	cmd := kubectl("get pods %s %s", nsflag(ns), strings.Join(args, " "))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting pods: %w, output: %s", err, output)
	}

	return output, nil
}

// GetEvents returns the events of a namespace
func GetEvents(ns string) (string, error) {
	cmd := kubectl("get events %s", nsflag(ns))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting events: %w, output: %s", err, output)
	}

	return output, nil
}

// Describe returns the description of a resource
// Arguments:
// - ns: namespace
// - kind: type of the resource
// - name: name of the resource
func Describe(ns, kind, name string) (string, error) {
	cmd := kubectl("describe %s %s %s", kind, name, nsflag(ns))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error describing resource: %w, output: %s", err, output)
	}

	return output, nil
}

// Logs returns the logs of a deployment
// Arguments:
// - ns: namespace
// - pod: the pod name, "kind/name", or "-l labelselector"
// - Since: time range
func Logs(ns, pod string, since *time.Duration) (string, error) {
	cmd := kubectl("logs %s %s %s", pod, nsflag(ns), sinceFlag(since))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

func sinceFlag(since *time.Duration) string {
	if since == nil {
		return ""
	}
	return "--since=" + since.String()
}

// Exec executes a command in the pod or specific container
func Exec(ns, pod, container, command string) (string, error) {
	cmd := kubectl("exec %s %s %s -- %s", pod, containerflag(container), nsflag(ns), command)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

// prepend prepends the prefix, but only if str is not empty
func prepend(prefix, str string) string {
	if str == "" {
		return str
	}
	return prefix + str
}

func nsflag(ns string) string {
	if ns == "" {
		return "--all-namespaces"
	}
	return "-n " + ns
}

func containerflag(container string) string {
	if container == "" {
		return ""
	}
	return "-c " + container
}
