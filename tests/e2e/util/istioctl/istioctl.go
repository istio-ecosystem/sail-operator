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

	env "github.com/istio-ecosystem/sail-operator/tests/e2e/util/env"
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
func CreateRemoteSecret(remoteKubeconfig string, secretName string, internalIP string) (string, error) {
	cmd := istioctl("create-remote-secret --kubeconfig %s --name %s --server=`https://%s:6443`", remoteKubeconfig, secretName, internalIP)
	yaml, err := shell.ExecuteCommand(cmd)

	return yaml, err
}
