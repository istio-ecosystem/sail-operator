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

package config

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type Platform string

const (
	PlatformUndefined  Platform = ""
	PlatformOpenShift  Platform = "openshift"
	PlatformKubernetes Platform = "kubernetes"
)

const (
	openshiftKind            = "OpenShiftAPIServer"
	openshiftResourceGroup   = "operator.openshift.io"
	openshiftResourceVersion = "v1"
)

func DetectPlatform(cfg *rest.Config) (Platform, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create discoveryClient: %w", err)
	}

	resources, err := dc.ServerResourcesForGroupVersion(openshiftResourceGroup + "/" + openshiftResourceVersion)
	if errors.IsNotFound(err) {
		return PlatformKubernetes, nil
	} else if err != nil {
		return "", err
	}

	for _, apiResource := range resources.APIResources {
		if apiResource.Kind == openshiftKind {
			return PlatformOpenShift, nil
		}
	}
	return PlatformKubernetes, nil
}
