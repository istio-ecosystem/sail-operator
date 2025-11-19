//go:build e2e

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

package common

import (
	"context"
	"fmt"
	"reflect"

	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AwaitCondition to be True. A key and a pointer to the object struct must be supplied. Extra arguments to pass to `Eventually` can be optionally supplied.
func AwaitCondition[T ~string](ctx context.Context, condition T, key client.ObjectKey, obj client.Object, k kubectl.Kubectl, cl client.Client, args ...any) {
	kind := reflect.TypeOf(obj).Elem().Name()
	cluster := "cluster"
	if k.ClusterName != "" {
		cluster = k.ClusterName
	}

	Eventually(GetObject, args...).
		WithArguments(ctx, cl, key, obj).
		Should(HaveConditionStatus(condition, metav1.ConditionTrue),
			fmt.Sprintf("%s %q is not %s on %s; unexpected Condition", kind, key.Name, condition, cluster))
	Success(fmt.Sprintf("%s %q is %s on %s", kind, key.Name, condition, cluster))
}
