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
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ctx = context.Background()

func TestReconcileActiveRevision(t *testing.T) {
	const version = "my-version"

	testCases := []struct {
		name                 string
		istioValues          v1.Values
		revValues            *v1.Values
		existingOwnerRef     *metav1.OwnerReference
		expectOwnerReference bool
	}{
		{
			name: "creates IstioRevision",
			istioValues: v1.Values{
				Pilot: &v1.PilotConfig{
					Hub: new("quay.io/hub"),
				},
				MeshConfig: &v1.MeshConfig{
					AccessLogFile: new("/dev/stdout"),
				},
			},
			expectOwnerReference: true,
		},
		{
			name: "updates IstioRevision",
			istioValues: v1.Values{
				Pilot: &v1.PilotConfig{
					Hub: new("quay.io/new-hub"),
				},
				MeshConfig: &v1.MeshConfig{
					AccessLogFile: new("/dev/stdout"),
				},
			},
			revValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Image: new("old-image"),
				},
			},
			expectOwnerReference: true,
		},
		{
			name: "heals stale ownerReference on update",
			istioValues: v1.Values{
				Pilot: &v1.PilotConfig{
					Hub: new("quay.io/hub"),
				},
			},
			revValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Image: new("old-image"),
				},
			},
			existingOwnerRef: &metav1.OwnerReference{
				APIVersion:         v1.GroupVersion.String(),
				Kind:               v1.IstioKind,
				Name:               "my-istio",
				UID:                "stale-UID",
				Controller:         new(true),
				BlockOwnerDeletion: new(true),
			},
			expectOwnerReference: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var initObjs []client.Object

			if tc.revValues != nil {
				rev := &v1.IstioRevision{
					Name: "my-revision",
					Spec: v1.IstioRevisionSpec{
						Version: version,
						Values:  tc.revValues,
					},
				}
				if tc.existingOwnerRef != nil {
					rev.OwnerReferences = []metav1.OwnerReference{*tc.existingOwnerRef}
				}
				initObjs = append(initObjs, rev)
			}

			owner := &v1.Istio{
				Name: "my-istio",
				UID:  "my-istio-UID",
			}

			cl := newFakeClientBuilder().WithObjects(initObjs...).Build()

			err := CreateOrUpdate(ctx, cl, scheme.Scheme, "my-revision", version, "istio-system", &tc.istioValues, owner)
			if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			revKey := types.NamespacedName{Name: "my-revision"}
			rev := &v1.IstioRevision{}
			Must(t, cl.Get(ctx, revKey, rev))

			if tc.expectOwnerReference {
				if len(rev.OwnerReferences) != 1 {
					t.Fatalf("expected 1 ownerReference, got %d", len(rev.OwnerReferences))
				}
				ref := rev.OwnerReferences[0]
				if ref.Name != owner.Name {
					t.Errorf("ownerReference.Name = %q, want %q", ref.Name, owner.Name)
				}
				if ref.UID != owner.UID {
					t.Errorf("ownerReference.UID = %q, want %q", ref.UID, owner.UID)
				}
				if ref.Controller == nil || !*ref.Controller {
					t.Errorf("ownerReference.Controller should be true")
				}
			} else if len(rev.OwnerReferences) != 0 {
				t.Errorf("expected no ownerReferences, got %v", rev.OwnerReferences)
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
