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
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/fieldignore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// defaultTestRules replicates the built-in field ignore rules used in production.
var defaultTestRules = []fieldignore.FieldIgnoreRule{
	{
		Group:        "admissionregistration.k8s.io",
		Version:      "v1",
		Kind:         "ValidatingWebhookConfiguration",
		Fields:       []string{"webhooks[*].failurePolicy"},
		OnlyOnUpdate: true,
	},
	{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "ValidatingWebhookConfiguration",
		Fields:  []string{"webhooks[*].clientConfig.caBundle"},
	},
	{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "MutatingWebhookConfiguration",
		Fields:  []string{"webhooks[*].clientConfig.caBundle"},
	},
}

func TestHelmPostRenderer(t *testing.T) {
	testCases := []struct {
		name             string
		ownerReference   *metav1.OwnerReference
		ownerNamespace   string
		isUpdate         bool
		fieldIgnoreRules []fieldignore.FieldIgnoreRule
		input            string
		expected         string
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
		{
			name:             "ValidatingWebhookConfiguration create",
			ownerReference:   nil,
			ownerNamespace:   "",
			fieldIgnoreRules: defaultTestRules,
			input: `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istio-validator
webhooks:
- name: rev.validation.istio.io
  failurePolicy: Fail
`,
			expected: `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  labels:
    managed-by: sail-operator
  name: istio-validator
webhooks:
  - failurePolicy: Fail
    name: rev.validation.istio.io
`,
		},
		{
			name:             "ValidatingWebhookConfiguration update",
			ownerReference:   nil,
			ownerNamespace:   "",
			isUpdate:         true,
			fieldIgnoreRules: defaultTestRules,
			input: `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istio-validator
webhooks:
- name: rev.validation.istio.io
  failurePolicy: Fail
`,
			expected: `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  labels:
    managed-by: sail-operator
  name: istio-validator
webhooks:
  - name: rev.validation.istio.io
`,
		},
		{
			name:             "ValidatingWebhookConfiguration strips caBundle on install",
			ownerReference:   nil,
			ownerNamespace:   "",
			fieldIgnoreRules: defaultTestRules,
			input: `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istio-validator
webhooks:
- name: rev.validation.istio.io
  failurePolicy: Fail
  clientConfig:
    caBundle: c29tZS1jYS1idW5kbGU=
    service:
      name: istiod
`,
			expected: `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  labels:
    managed-by: sail-operator
  name: istio-validator
webhooks:
  - clientConfig:
      service:
        name: istiod
    failurePolicy: Fail
    name: rev.validation.istio.io
`,
		},
		{
			name:             "MutatingWebhookConfiguration strips caBundle on update",
			ownerReference:   nil,
			ownerNamespace:   "",
			isUpdate:         true,
			fieldIgnoreRules: defaultTestRules,
			input: `apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: istio-sidecar-injector
webhooks:
- name: rev.namespace.sidecar-injector.istio.io
  clientConfig:
    caBundle: c29tZS1jYS1idW5kbGU=
    service:
      name: istiod
`,
			expected: `apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  labels:
    managed-by: sail-operator
  name: istio-sidecar-injector
webhooks:
  - clientConfig:
      service:
        name: istiod
    name: rev.namespace.sidecar-injector.istio.io
`,
		},
		{
			name: "non-ValidatingWebhookConfiguration update",
			ownerReference: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "my-configmap",
				UID:        "123",
			},
			ownerNamespace: "istio-system",
			isUpdate:       true,
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			postRenderer := HelmPostRenderer{
				ownerReference:   tc.ownerReference,
				ownerNamespace:   tc.ownerNamespace,
				isUpdate:         tc.isUpdate,
				managedByValue:   constants.ManagedByLabelValue,
				fieldIgnoreRules: tc.fieldIgnoreRules,
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
