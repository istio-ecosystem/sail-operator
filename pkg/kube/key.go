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

package kube

import "sigs.k8s.io/controller-runtime/pkg/client"

// Key returns the client.ObjectKey for the given name and namespace. If no namespace is provided, it returns a key cluster scoped
func Key(name string, namespace ...string) client.ObjectKey {
	if len(namespace) > 1 {
		panic("you can only provide one namespace")
	} else if len(namespace) == 1 {
		return client.ObjectKey{Name: name, Namespace: namespace[0]}
	}
	return client.ObjectKey{Name: name}
}
