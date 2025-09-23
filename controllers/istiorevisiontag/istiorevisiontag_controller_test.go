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

package istiorevisiontag

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

const revName = "istio-revision"

func TestDetermineInUseCondition(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	testCases := []struct {
		podLabels           map[string]string
		podAnnotations      map[string]string
		nsLabels            map[string]string
		enableAllNamespaces bool
		interceptors        interceptor.Funcs
		matchesTag          string
		expectUnknownState  bool
	}{
		// no labels on namespace or pod
		{
			nsLabels:   map[string]string{},
			podLabels:  map[string]string{},
			matchesTag: "",
		},

		// pod annotations only
		{
			podAnnotations: map[string]string{"istio.io/rev": "default"},
		},

		// namespace labels only
		{
			nsLabels:   map[string]string{"istio-injection": "enabled"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "default"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "my-rev"},
			matchesTag: "my-rev",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			matchesTag: "default",
		},

		// pod labels only
		{
			podLabels:  map[string]string{"istio.io/rev": "default"},
			matchesTag: "default",
		},
		{
			podLabels:  map[string]string{"istio.io/rev": "my-rev"},
			matchesTag: "my-rev",
		},
		{
			podLabels:  map[string]string{"sidecar.istio.io/inject": "true"},
			matchesTag: "default",
		},
		{
			podLabels:  map[string]string{"sidecar.istio.io/inject": "true", "istio.io/rev": "my-rev"},
			matchesTag: "my-rev",
		},

		// ns and pod labels
		{
			nsLabels:   map[string]string{"istio.io/rev": "my-rev"},
			podLabels:  map[string]string{"sidecar.istio.io/inject": "true"},
			matchesTag: "my-rev",
		},
		{
			nsLabels:   map[string]string{"istio-injection": "enabled"},
			podLabels:  map[string]string{"istio.io/rev": "default"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio-injection": "enabled"},
			podLabels:  map[string]string{"istio.io/rev": "my-rev"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "default"},
			podLabels:  map[string]string{"istio.io/rev": "default"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "default"},
			podLabels:  map[string]string{"istio.io/rev": "my-rev"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "my-rev"},
			podLabels:  map[string]string{"istio.io/rev": "default"},
			matchesTag: "my-rev",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "my-rev"},
			podLabels:  map[string]string{"istio.io/rev": "my-rev"},
			matchesTag: "my-rev",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:  map[string]string{"istio.io/rev": "default"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:  map[string]string{"istio.io/rev": "my-rev"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:  map[string]string{"istio.io/rev": "default"},
			matchesTag: "default",
		},
		{
			nsLabels:   map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:  map[string]string{"istio.io/rev": "my-rev"},
			matchesTag: "default",
		},

		// special case: mismatch between pod annotation and label. revision tag controller should only look at label
		{
			podLabels:      map[string]string{"istio.io/rev": revName},
			podAnnotations: map[string]string{"istio.io/rev": "default"},
		},

		// special case: when Values.sidecarInjectorWebhook.enableNamespacesByDefault is true, all pods should match the default revision
		// unless they are in one of the system namespaces ("kube-system","kube-public","kube-node-lease","local-path-storage")
		{
			enableAllNamespaces: true,
			matchesTag:          "default",
		},
		{
			interceptors: interceptor.Funcs{
				List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					return fmt.Errorf("simulated error")
				},
			},
			expectUnknownState: true,
		},
	}

	for _, tagName := range []string{"default", "my-rev"} {
		for _, tc := range testCases {
			nameBuilder := strings.Builder{}
			nameBuilder.WriteString(tagName + ":")
			if len(tc.nsLabels) == 0 && len(tc.podLabels) == 0 {
				nameBuilder.WriteString("no labels")
			}
			if len(tc.nsLabels) > 0 {
				nameBuilder.WriteString("NS:")
				for k, v := range tc.nsLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			if len(tc.podLabels) > 0 {
				nameBuilder.WriteString("POD:")
				for k, v := range tc.podLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			name := strings.TrimSuffix(nameBuilder.String(), ",")

			t.Run(name, func(t *testing.T) {
				g := NewWithT(t)
				rev := &v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: revName,
					},
				}
				if tc.enableAllNamespaces {
					rev.Spec.Values = &v1.Values{
						SidecarInjectorWebhook: &v1.SidecarInjectorConfig{
							EnableNamespacesByDefault: ptr.Of(true),
						},
					}
				}
				tag := &v1.IstioRevisionTag{
					ObjectMeta: metav1.ObjectMeta{
						Name: tagName,
					},
					Spec: v1.IstioRevisionTagSpec{
						TargetRef: v1.IstioRevisionTagTargetReference{
							Kind: "IstioRevision",
							Name: rev.Name,
						},
					},
				}

				namespace := "bookinfo"
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   namespace,
						Labels: tc.nsLabels,
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "some-pod",
						Namespace:   namespace,
						Labels:      tc.podLabels,
						Annotations: tc.podAnnotations,
					},
				}

				cl := fake.NewClientBuilder().
					WithScheme(scheme.Scheme).
					WithObjects(rev, tag, ns, pod).
					WithInterceptorFuncs(tc.interceptors).
					Build()

				r := NewReconciler(cfg, cl, scheme.Scheme, nil)

				result, _ := r.determineInUseCondition(context.TODO(), tag)
				g.Expect(result.Type).To(Equal(v1.IstioRevisionTagConditionInUse))

				if tc.expectUnknownState {
					g.Expect(result.Status).To(Equal(metav1.ConditionUnknown))
					g.Expect(result.Reason).To(Equal(v1.IstioRevisionTagReasonUsageCheckFailed))
				} else {
					if tagName == tc.matchesTag {
						g.Expect(result.Status).To(Equal(metav1.ConditionTrue),
							fmt.Sprintf("RevisionTag %s should be in use, but isn't\n"+
								"revisiontag: %s\nexpected revisiontag: %s\nnamespace labels: %+v\npod labels: %+v",
								tagName, tagName, tc.matchesTag, tc.nsLabels, tc.podLabels))
					} else {
						g.Expect(result.Status).To(Equal(metav1.ConditionFalse),
							fmt.Sprintf("RevisionTag %s should not be in use\n"+
								"revisiontag: %s\nexpected revisiontag: %s\nnamespace labels: %+v\npod labels: %+v\n"+
								"message: %s",
								tagName, tagName, tc.matchesTag, tc.nsLabels, tc.podLabels, result.Message))
					}
				}
			})
		}
	}
}

func newReconcilerTestConfig(t *testing.T) config.ReconcilerConfig {
	return config.ReconcilerConfig{
		ResourceDirectory:       t.TempDir(),
		Platform:                config.PlatformKubernetes,
		DefaultProfile:          "",
		MaxConcurrentReconciles: 1,
	}
}

func TestValidation(t *testing.T) {
	testCases := []struct {
		name string
		tag  *v1.IstioRevisionTag
		objs []client.Object

		expectedErrMessage string
	}{
		{
			name: "targetRef not set",
			tag: &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{},
				},
			},
			expectedErrMessage: "spec.targetRef not set",
		},
		// TODO: add other validation tests
		{
			name: "remote IstioRevision",
			tag: &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "IstioRevision",
						Name: revName,
					},
				},
			},
			objs: []client.Object{
				&v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: revName,
					},
					Spec: v1.IstioRevisionSpec{
						Values: &v1.Values{
							Profile: ptr.Of("remote"),
						},
					},
				},
			},
			expectedErrMessage: "IstioRevisionTag cannot reference a remote IstioRevision",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			cfg := newReconcilerTestConfig(t)

			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(append(tc.objs, tc.tag)...).
				Build()

			r := NewReconciler(cfg, cl, scheme.Scheme, nil)

			ctx := context.TODO()
			_, err := r.doReconcile(ctx, tc.tag)
			if tc.expectedErrMessage != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
