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
	"fmt"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestListOwned(t *testing.T) {
	t.Run("happy-path", func(t *testing.T) {
		rev1 := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "rev1",
				OwnerReferences: []metav1.OwnerReference{newOwnerReference("Istio", "default", "123")},
			},
		}
		rev2 := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "rev2",
				OwnerReferences: []metav1.OwnerReference{newOwnerReference("Istio", "default", "123")},
			},
		}
		rev3 := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "rev3",
				OwnerReferences: []metav1.OwnerReference{newOwnerReference("Istio", "default", "456")},
			},
		}
		cl := newFakeClientBuilder().
			WithObjects(&rev1, &rev2, &rev3).
			Build()

		result, err := ListOwned(ctx, cl, "123")
		assert.NoError(t, err)
		assert.Equalf(t, []v1.IstioRevision{rev1, rev2}, result, "ListOwned(%v)", "123")
	})

	t.Run("list-error", func(t *testing.T) {
		cl := newFakeClientBuilder().
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
					return fmt.Errorf("simulated error")
				},
			}).
			Build()

		result, err := ListOwned(ctx, cl, "123")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func Test_isOwnedRevision(t *testing.T) {
	t.Run("no-owner", func(t *testing.T) {
		rev := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{Name: "my-rev"},
		}
		assert.Equal(t, false, isOwnedRevision(rev, "111"))
	})

	t.Run("wrong-owner", func(t *testing.T) {
		rev := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "my-rev",
				OwnerReferences: []metav1.OwnerReference{newOwnerReference("Istio", "default", "222")},
			},
		}
		assert.Equal(t, false, isOwnedRevision(rev, "111"))
	})

	t.Run("single-owner", func(t *testing.T) {
		rev := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "my-rev",
				OwnerReferences: []metav1.OwnerReference{newOwnerReference("Istio", "default", "111")},
			},
		}
		assert.Equal(t, true, isOwnedRevision(rev, "111"))
	})

	t.Run("multiple-owners", func(t *testing.T) {
		rev := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-rev",
				OwnerReferences: []metav1.OwnerReference{
					newOwnerReference("Foo", "foo", "111"),
					newOwnerReference("Istio", "default", "222"),
					newOwnerReference("Bar", "bar", "333"),
				},
			},
		}
		assert.Equal(t, true, isOwnedRevision(rev, "222"))
	})

	// since resources in tests have no uid unless it's explicitly set, the ListOwned function will panic
	// so that the test author is reminded to add the uid to the resource
	t.Run("no-uid", func(t *testing.T) {
		rev := v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{Name: "my-rev"},
		}
		assert.Panics(t, func() {
			_ = isOwnedRevision(rev, "")
		})
	})
}

func newOwnerReference(kind, name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		Kind: kind,
		Name: name,
		UID:  uid,
	}
}
