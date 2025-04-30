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

package helm

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHelmPostRenderer(t *testing.T) {
	testCases := []struct {
		name           string
		ownerReference *metav1.OwnerReference
		ownerNamespace string
		input          string
		expected       string
	}{
		{
			name: "cluster-scoped owner",
			ownerReference: &metav1.OwnerReference{
				APIVersion: "sailoperator.io/v1",
				Kind:       "Istio",
				Name:       "my-istio",
				UID:        "123",
			},
			ownerNamespace: "istio-system",
			input: `apiVersion: v1
kind: Deployment
metadata:
  name: istiod
  namespace: istio-system
spec:
  replicas: 1
`,
			expected: `apiVersion: v1
kind: Deployment
metadata:
  labels:
    managed-by: sail-operator
  name: istiod
  namespace: istio-system
  ownerReferences:
    - apiVersion: sailoperator.io/v1
      kind: Istio
      name: my-istio
      uid: "123"
spec:
  replicas: 1
`,
		},
		{
			name: "namespace-scoped owner in same namespace",
			ownerReference: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "my-configmap",
				UID:        "123",
			},
			ownerNamespace: "istio-system",
			input: `apiVersion: v1
kind: Deployment
metadata:
  name: istiod
  namespace: istio-system
spec:
  replicas: 1
`,
			expected: `apiVersion: v1
kind: Deployment
metadata:
  labels:
    managed-by: sail-operator
  name: istiod
  namespace: istio-system
  ownerReferences:
    - apiVersion: v1
      kind: ConfigMap
      name: my-configmap
      uid: "123"
spec:
  replicas: 1
`,
		},
		{
			name: "namespace-scoped owner in different namespace",
			ownerReference: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "my-configmap",
				UID:        "123",
			},
			ownerNamespace: "istio-system",
			input: `apiVersion: v1
kind: Service
metadata:
  name: some-service
  namespace: other-namespace
  labels:
    foo: bar 
spec:
  ports:
  - port: 80
`,
			expected: `apiVersion: v1
kind: Service
metadata:
  annotations:
    operator-sdk/primary-resource: istio-system/my-configmap
    operator-sdk/primary-resource-type: ConfigMap.v1
  labels:
    foo: bar
    managed-by: sail-operator
  name: some-service
  namespace: other-namespace
spec:
  ports:
    - port: 80
`,
		},
		{
			name:           "no owner reference",
			ownerReference: nil,
			ownerNamespace: "",
			input: `apiVersion: v1
kind: Deployment
metadata:
  name: istiod
  namespace: istio-system
spec:
  replicas: 1
`,
			expected: `apiVersion: v1
kind: Deployment
metadata:
  labels:
    managed-by: sail-operator
  name: istiod
  namespace: istio-system
spec:
  replicas: 1
`,
		},
		{
			name: "multiple manifests",
			ownerReference: &metav1.OwnerReference{
				APIVersion: "sailoperator.io/v1",
				Kind:       "Istio",
				Name:       "my-istio",
				UID:        "123",
			},
			ownerNamespace: "istio-system",
			input: `
---
apiVersion: v1
kind: Deployment
metadata:
  name: istiod
  namespace: istio-system
spec:
  replicas: 1
---
# some comment
# there's no object here
---
apiVersion: v1
kind: Service
metadata:
  name: some-service
  namespace: other-namespace
  labels:
    foo: bar 
spec:
  ports:
  - port: 80
`,
			expected: `apiVersion: v1
kind: Deployment
metadata:
  labels:
    managed-by: sail-operator
  name: istiod
  namespace: istio-system
  ownerReferences:
    - apiVersion: sailoperator.io/v1
      kind: Istio
      name: my-istio
      uid: "123"
spec:
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    operator-sdk/primary-resource: istio-system/my-istio
    operator-sdk/primary-resource-type: Istio.sailoperator.io
  labels:
    foo: bar
    managed-by: sail-operator
  name: some-service
  namespace: other-namespace
spec:
  ports:
    - port: 80
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			postRenderer := HelmPostRenderer{
				ownerReference: tc.ownerReference,
				ownerNamespace: tc.ownerNamespace,
			}

			actual, err := postRenderer.Run(bytes.NewBufferString(tc.input))
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.expected, actual.String()); diff != "" {
				t.Errorf("ownerReference or managed-by label wasn't added properly; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}
