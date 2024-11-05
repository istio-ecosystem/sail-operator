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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var ctx = context.Background()

func TestReconcileActiveRevision(t *testing.T) {
	const version = "my-version"

	testCases := []struct {
		name                 string
		istioValues          v1alpha1.Values
		revValues            *v1alpha1.Values
		expectOwnerReference bool
	}{
		{
			name: "creates IstioRevision",
			istioValues: v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: ptr.Of("quay.io/hub"),
				},
				MeshConfig: &v1alpha1.MeshConfig{
					AccessLogFile: ptr.Of("/dev/stdout"),
				},
			},
			expectOwnerReference: true,
		},
		{
			name: "updates IstioRevision",
			istioValues: v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: ptr.Of("quay.io/new-hub"),
				},
				MeshConfig: &v1alpha1.MeshConfig{
					AccessLogFile: ptr.Of("/dev/stdout"),
				},
			},
			revValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: ptr.Of("old-image"),
				},
			},
			expectOwnerReference: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var initObjs []client.Object

			if tc.revValues != nil {
				initObjs = append(initObjs,
					&v1alpha1.IstioRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-revision",
						},
						Spec: v1alpha1.IstioRevisionSpec{
							Version: version,
							Values:  tc.revValues,
						},
					},
				)
			}

			cl := newFakeClientBuilder().WithObjects(initObjs...).Build()

			ownerRef := metav1.OwnerReference{
				APIVersion:         v1alpha1.GroupVersion.String(),
				Kind:               v1alpha1.IstioKind,
				Name:               "my-istio",
				UID:                "my-istio-UID",
				Controller:         ptr.Of(true),
				BlockOwnerDeletion: ptr.Of(true),
			}
			err := CreateOrUpdate(ctx, cl, "my-revision", version, "istio-system", &tc.istioValues, ownerRef)
			if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			revKey := types.NamespacedName{Name: "my-revision"}
			rev := &v1alpha1.IstioRevision{}
			Must(t, cl.Get(ctx, revKey, rev))

			var expectedOwnerRefs []metav1.OwnerReference
			if tc.expectOwnerReference {
				expectedOwnerRefs = []metav1.OwnerReference{ownerRef}
			}
			if diff := cmp.Diff(rev.OwnerReferences, expectedOwnerRefs); diff != "" {
				t.Errorf("invalid ownerReference; diff (-expected, +actual):\n%v", diff)
			}

			if rev.Spec.Version != version {
				t.Errorf("IstioRevision.spec.version doesn't match Istio.spec.version; expected %s, got %s", version, rev.Spec.Version)
			}

			if diff := cmp.Diff(helm.FromValues(&tc.istioValues), helm.FromValues(rev.Spec.Values)); diff != "" {
				t.Errorf("IstioRevision.spec.values don't match Istio.spec.values; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}
