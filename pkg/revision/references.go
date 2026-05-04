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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetReferencedRevisionFromNamespace(labels map[string]string) string {
	// istio-injection label takes precedence over istio.io/rev
	if labels[constants.IstioInjectionLabel] == constants.IstioInjectionEnabledValue {
		return v1.DefaultRevision
	}
	return labels[constants.IstioRevLabel]
	// TODO: if .Values.sidecarInjectorWebhook.enableNamespacesByDefault is true, then all namespaces except system namespaces should use the "default" revision
}

func GetReferencedRevisionFromPod(podLabels map[string]string) string {
	// we only look at pod labels to identify injection intent
	if podLabels[constants.IstioSidecarInjectLabel] != "false" {
		if rev := podLabels[constants.IstioRevLabel]; rev != "" {
			return rev
		}
		if podLabels[constants.IstioSidecarInjectLabel] == "true" {
			return v1.DefaultRevision
		}
	}

	return ""
}

func GetInjectedRevisionFromPod(podAnnotations map[string]string) string {
	// if pod was already injected, the revision that did the injection is specified in the istio.io/rev annotation
	return podAnnotations[constants.IstioRevLabel]
}

func GetIstioRevisionFromTargetReference(ctx context.Context, client client.Client, ref v1.TargetReference) (*v1.IstioRevision, error) {
	var revisionName string
	switch ref.Kind {
	case v1.IstioRevisionKind:
		revisionName = ref.Name
	case v1.IstioKind:
		i := v1.Istio{}
		err := client.Get(ctx, types.NamespacedName{Name: ref.Name}, &i)
		if err != nil {
			return nil, err
		}
		if i.Status.ActiveRevisionName == "" {
			return nil, reconciler.NewTransientError("referenced Istio has no active revision")
		}
		revisionName = i.Status.ActiveRevisionName
	default:
		return nil, reconciler.NewValidationError("unknown targetRef.kind")
	}

	rev := v1.IstioRevision{}
	err := client.Get(ctx, types.NamespacedName{Name: revisionName}, &rev)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}
