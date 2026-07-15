//go:build e2e

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

package istioctl

import (
	"fmt"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
)

var istioctlBinary = env.Get("ISTIOCTL_PATH", "istioctl")

// Istioctl returns the istioctl command
// If the environment variable COMMAND is set, it will return the value of COMMAND
// Otherwise, it will return the default value "istioctl" as default
// Arguments:
// - format: format of the command without istioctl
// - args: arguments of the command
func istioctl(format string, args ...interface{}) string {
	binary := "istioctl"
	if istioctlBinary != "" {
		binary = istioctlBinary
	}

	cmd := fmt.Sprintf(format, args...)

	return fmt.Sprintf("%s %s", binary, cmd)
}

// CreateRemoteSecret creates a secret in the remote cluster
// Arguments:
// - remoteKubeconfig: kubeconfig of the remote cluster
// - secretName: name of the secret
// - internalIP: internal IP of the remote cluster
func CreateRemoteSecret(remoteKubeconfig, namespace, secretName, internalIP string, additionalFlags ...string) (string, error) {
	cmd := istioctl("create-remote-secret --kubeconfig %s --namespace %s --name %s --server=%s", remoteKubeconfig, namespace, secretName, internalIP)
	if len(additionalFlags) != 0 {
		cmd += (" " + strings.Join(additionalFlags, " "))
	}
	if env.GetBool("OCP", false) {
		cmd += " --create-service-account=false"
	}

	yaml, err := shell.ExecuteCommand(cmd)

	return yaml, err
}

// GetProxyStatus runs istioctl proxy-status command and return the output
func GetProxyStatus(additionalFlags ...string) (string, error) {
	cmd := istioctl("proxy-status")
	if len(additionalFlags) != 0 {
		cmd += (" " + strings.Join(additionalFlags, " "))
	}

	status, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get proxy status: %w", err)
	}

	return status, nil
}
