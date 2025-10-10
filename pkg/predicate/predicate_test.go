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

package predicate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestIgnoreUpdate(t *testing.T) {
	predicate := IgnoreUpdate()
	assert.Equal(t, false, predicate.Update(event.UpdateEvent{}))
	assert.Equal(t, true, predicate.Create(event.CreateEvent{}))
	assert.Equal(t, true, predicate.Delete(event.DeleteEvent{}))
	assert.Equal(t, true, predicate.Generic(event.GenericEvent{}))
}

func TestIgnoreUpdateWhenAnnotation(t *testing.T) {
	predicate := IgnoreUpdateWhenAnnotation()
	// Object does not contain sailoperator.io/ignore annotation
	// so reconciliation should be done and both objects should be equal
	assert.Equal(t, true, predicate.Update(event.UpdateEvent{
		ObjectOld: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		},
		ObjectNew: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
			Data: map[string]string{
				"foo": "bar",
			},
		},
	}))
	// Object has sailoperator.io/ignore annotation set with wrong value
	// so reconciliation should be done and both objects should be equal
	assert.Equal(t, true, predicate.Update(event.UpdateEvent{
		ObjectOld: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		},
		ObjectNew: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"sailoperator.io/ignore": "wrongvalue",
				},
			},
			Data: map[string]string{
				"foo": "bar",
			},
		},
	}))
	// Object has sailoperator.io/ignore annotation set to "true"
	// so reconciliation should be skipped
	assert.Equal(t, false, predicate.Update(event.UpdateEvent{
		ObjectOld: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		},
		ObjectNew: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"sailoperator.io/ignore": "true",
				},
			},
			Data: map[string]string{
				"foo": "bar",
			},
		},
	}))
	assert.Equal(t, true, predicate.Create(event.CreateEvent{}))
	assert.Equal(t, true, predicate.Delete(event.DeleteEvent{}))
	assert.Equal(t, true, predicate.Generic(event.GenericEvent{}))
}
