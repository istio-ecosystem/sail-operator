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

package perses

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DashboardCRD returns a minimal CustomResourceDefinition for PersesDashboard. It matches
// the API version and shape used by bundled dashboards and is intended for tests or clusters
// where the full Perses operator CRD is not already installed.
func DashboardCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: PersesDashboardCRD},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: persesGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   persesDashboardResource,
				Singular: "persesdashboard",
				Kind:     "PersesDashboard",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    persesVersion,
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: new(true),
					},
				},
			}},
		},
	}
}
