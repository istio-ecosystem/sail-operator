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

package client

import (
	"fmt"
	"log"
	"os"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getConfig returns the configuration of the kubernetes go-client
func getConfig(kubeconfig string) (*rest.Config, error) {
	// If kubeconfig is provided, use it
	if kubeconfig != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("error building config: %w", err)
		}

		return config, nil
	}
	// If not kubeconfig is provided use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return nil, fmt.Errorf("error building config: %w", err)
	}

	return config, nil
}

// InitK8sClient returns the kubernetes clientset
// Arguments:
// Kubeconfig: string
// Set kubeconfig to "" to use the current context in kubeconfig
func InitK8sClient(kubeconfig string) (client.Client, error) {
	config, err := getConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error getting config for k8s client: %w", err)
	}

	// create the clientset
	k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %w", err)
	}

	if err := apiextensionsv1.AddToScheme(scheme.Scheme); err != nil {
		log.Fatalf("Failed to register CRD scheme: %v", err)
	}

	return k8sClient, nil
}
