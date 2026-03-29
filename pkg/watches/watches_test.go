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

package watches

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIgnoreStatusChanges(t *testing.T) {
	shouldReconcile := IgnoreStatusChanges()

	oldObj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1",
			Generation:      1,
			Finalizers:      []string{"finalizer1"},
			Labels:          map[string]string{"app": "test"},
			Annotations:     map[string]string{"annotation1": "value1"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "IstioRevision",
					Name:       "myrev",
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "1.1.1.1",
					},
				},
			},
			Conditions: nil,
		},
	}

	tests := []struct {
		name     string
		update   func(svc *corev1.Service)
		expected bool
	}{
		{
			name:     "No changes",
			update:   func(svc *corev1.Service) {},
			expected: false,
		},
		{
			name: "ResourceVersion changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
			},
			expected: false,
		},
		{
			name: "Spec changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Generation++
				svc.Spec.Type = corev1.ServiceTypeNodePort
			},
			expected: true,
		},
		{
			name: "Status changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Status.LoadBalancer.Ingress[0].IP = "2.2.2.2"
			},
			expected: false,
		},
		{
			name: "Spec and status changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Generation++
				svc.Spec.Type = corev1.ServiceTypeNodePort
				svc.Status.LoadBalancer.Ingress[0].IP = "2.2.2.2"
			},
			expected: true,
		},
		{
			name: "Labels changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Labels["app"] = "new-value"
			},
			expected: true,
		},
		{
			name: "Annotations changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Annotations["annotation1"] = "new-value"
			},
			expected: true,
		},
		{
			name: "OwnerReferences changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.OwnerReferences[0].Name = "new-owner"
			},
			expected: true,
		},
		{
			name: "Finalizers changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Finalizers = append(svc.Finalizers, "finalizer2")
			},
			expected: true,
		},
		{
			name: "Ignore annotation added",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Annotations[ignoreAnnotation] = "true"
			},
			// IgnoreStatusChanges sees the annotation change and returns true.
			// The ignore annotation is handled by a separate predicate (ignoreAnnotationPredicate).
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			newObj := oldObj.DeepCopy()
			tc.update(newObj)

			result := shouldReconcile(oldObj, newObj)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}
