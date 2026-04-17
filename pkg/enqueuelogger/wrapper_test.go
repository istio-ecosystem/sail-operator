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

package enqueuelogger

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Test_determineKind(t *testing.T) {
	tests := []struct {
		name string
		arg  client.Object
		want string
	}{
		{
			name: "nil",
			arg:  nil,
			want: "",
		},
		{
			name: "with-type-meta",
			arg: &v1.Istio{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Istio123",
					APIVersion: "v1",
				},
			},
			want: "Istio123",
		},
		{
			name: "without-type-meta", // uses reflection to get the type name
			arg: &v1.IstioRevision{
				TypeMeta: metav1.TypeMeta{},
			},
			want: "IstioRevision",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineKind(tt.arg)
			assert.Equalf(t, tt.want, got, "determineKind(%v)", tt.arg)
		})
	}
}

func TestWrapIfNecessary(t *testing.T) {
	delegate := &fakeHandler{}
	logger := logr.Discard()

	t.Run("returns delegate when logging disabled", func(t *testing.T) {
		LogEnqueueEvents = false
		result := WrapIfNecessary("Istio", logger, delegate)
		assert.Equal(t, delegate, result)
	})

	t.Run("wraps delegate when logging enabled", func(t *testing.T) {
		LogEnqueueEvents = true
		defer func() { LogEnqueueEvents = false }()

		result := WrapIfNecessary("Istio", logger, delegate)
		assert.IsType(t, &EnqueueEventLogger{}, result)
		wrapper := result.(*EnqueueEventLogger)
		assert.Equal(t, "Istio", wrapper.kind)
		assert.Equal(t, delegate, wrapper.delegate)
	})
}

type fakeHandler struct{}

func (f *fakeHandler) Create(_ context.Context, _ event.TypedCreateEvent[client.Object],
	_ workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
}

func (f *fakeHandler) Update(_ context.Context, _ event.TypedUpdateEvent[client.Object],
	_ workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
}

func (f *fakeHandler) Delete(_ context.Context, _ event.TypedDeleteEvent[client.Object],
	_ workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
}

func (f *fakeHandler) Generic(_ context.Context, _ event.TypedGenericEvent[client.Object],
	_ workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
}
