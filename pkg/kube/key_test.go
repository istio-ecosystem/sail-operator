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

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestKey(t *testing.T) {
	t.Run("name-only", func(t *testing.T) {
		result := Key("foo")
		expected := client.ObjectKey{Name: "foo"}
		assert.Equal(t, expected, result)
	})

	t.Run("name-and-namespace", func(t *testing.T) {
		result := Key("bar", "ns")
		expected := client.ObjectKey{Name: "bar", Namespace: "ns"}
		assert.Equal(t, expected, result)
	})

	t.Run("multiple-namespaces", func(t *testing.T) {
		assert.Panics(t, func() {
			Key("my-name", "ns1", "ns2")
		})
	})
}
