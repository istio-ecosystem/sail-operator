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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"path/filepath"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	common "github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var sailCRDs = []string{
	// TODO: Find an alternative to this list
	"authorizationpolicies.security.istio.io",
	"destinationrules.networking.istio.io",
	"envoyfilters.networking.istio.io",
	"gateways.networking.istio.io",
	"istiorevisions.operator.istio.io",
	"istios.operator.istio.io",
	"peerauthentications.security.istio.io",
	"proxyconfigs.networking.istio.io",
	"requestauthentications.security.istio.io",
	"serviceentries.networking.istio.io",
	"sidecars.networking.istio.io",
	"telemetries.telemetry.istio.io",
	"virtualservices.networking.istio.io",
	"wasmplugins.extensions.istio.io",
	"workloadentries.networking.istio.io",
	"workloadgroups.networking.istio.io",
}

var _ = Describe("Operator", Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	Describe("installation", func() {
		BeforeAll(func() {
			Expect(kubectl.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

			extraArg := ""
			if ocp {
				extraArg = "--set=platform=openshift"
			}

			if skipDeploy {
				Success("Skipping operator installation because it was deployed externally")
			} else {
				Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, extraArg)).
					To(Succeed(), "Operator failed to be deployed")
			}
		})

		It("deploys all the CRDs", func(ctx SpecContext) {
			Eventually(common.GetList).WithArguments(ctx, cl, &apiextensionsv1.CustomResourceDefinitionList{}).
				Should(WithTransform(extractCRDNames, ContainElements(sailCRDs)),
					"Not all Istio and Sail CRDs are present")
			Success("Istio CRDs are present")
		})

		It("updates the CRDs status to Established", func(ctx SpecContext) {
			for _, crdName := range sailCRDs {
				Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(crdName), &apiextensionsv1.CustomResourceDefinition{}).
					Should(HaveCondition(apiextensionsv1.Established, metav1.ConditionTrue), "Error getting Istio CRD")
			}
			Success("CRDs are Established")
		})

		Specify("istio crd is present", func(ctx SpecContext) {
			// When the operator runs in OCP cluster, the CRD is created but not available at the moment
			Eventually(cl.Get).WithArguments(ctx, kube.Key("istios.operator.istio.io"), &apiextensionsv1.CustomResourceDefinition{}).
				Should(Succeed(), "Error getting Istio CRD")
			Success("Istio CRD is present")
		})

		It("starts successfully", func(ctx SpecContext) {
			Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
		})

		AfterAll(func() {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo()
			}
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			common.LogDebugInfo()
		}

		if skipDeploy {
			Success("Skipping operator undeploy because it was deployed externally")
			return
		}

		By("Uninstalling the operator")
		Expect(helm.Uninstall("sail-operator", "--namespace "+namespace)).
			To(Succeed(), "Operator failed to be deleted")
		Success("Operator uninstalled")

		By("Deleting the CRDs")
		Expect(kubectl.DeleteCRDs(sailCRDs)).To(Succeed(), "CRDs failed to be deleted")
		Success("CRDs deleted")
	})
})

func extractCRDNames(crdList *apiextensionsv1.CustomResourceDefinitionList) []string {
	var names []string
	for _, crd := range crdList.Items {
		names = append(names, crd.ObjectMeta.Name)
	}
	return names
}
