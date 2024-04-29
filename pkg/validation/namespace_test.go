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

package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/testtime"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestValidateTargetNamespace(t *testing.T) {
	testCases := []struct {
		name         string
		objects      []client.Object
		interceptors interceptor.Funcs
		expectErr    string
	}{
		{
			name: "success",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-namespace",
					},
				},
			},
			expectErr: "",
		},
		{
			name:      "namespace not found",
			objects:   []client.Object{},
			expectErr: `namespace "my-namespace" doesn't exist`,
		},
		{
			name: "namespace deleted",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "my-namespace",
						DeletionTimestamp: testtime.OneMinuteAgo(),
						Finalizers:        []string{"dummy"}, // required for fake client builder to accept a deleted object
					},
				},
			},
			expectErr: `namespace "my-namespace" is being deleted`,
		},
		{
			name: "get error",
			interceptors: interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return fmt.Errorf("simulated error")
				},
			},
			expectErr: "simulated error",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.objects...).
				WithInterceptorFuncs(tc.interceptors).
				Build()

			err := ValidateTargetNamespace(context.TODO(), cl, "my-namespace")
			if tc.expectErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			}
		})
	}
}
