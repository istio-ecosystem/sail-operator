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

package integration

import (
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

func NewOwnerReference(obj client.Object) metav1.OwnerReference {
	// TODO: find a way to get the APIVersion from the object or schema
	var apiVersion, kind string
	switch obj.(type) {
	case *v1.Istio:
		apiVersion = v1.GroupVersion.String()
		kind = v1.IstioKind
	case *v1.IstioRevision:
		apiVersion = v1.GroupVersion.String()
		kind = v1.IstioRevisionKind
	case *v1.IstioCNI:
		apiVersion = v1.GroupVersion.String()
		kind = v1.IstioCNIKind
	default:
		panic("unknown type")
	}

	return metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               obj.GetName(),
		UID:                obj.GetUID(),
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}
}
