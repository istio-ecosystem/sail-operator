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

package install

import (
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func TestManagedByWatchPredicate(t *testing.T) {
	g := NewWithT(t)

	pred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{managedByLabelKey: managedByValue},
	})
	g.Expect(err).NotTo(HaveOccurred())

	labeled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{constants.ManagedByLabelKey: managedByValue},
		},
	}
	g.Expect(pred.Create(event.CreateEvent{Object: labeled})).To(BeTrue())

	wrongKey := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{constants.KubernetesAppManagedByKey: managedByValue},
		},
	}
	g.Expect(pred.Create(event.CreateEvent{Object: wrongKey})).To(BeFalse())
}
