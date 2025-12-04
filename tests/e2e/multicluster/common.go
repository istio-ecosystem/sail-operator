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

package multicluster

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterDeployment represents a cluster along with its sample app version.
type ClusterDeployment struct {
	Kubectl    kubectl.Kubectl
	AppVersion string
}

// deploySampleApp deploys the sample apps (helloworld and sleep) in the given cluster.
func deploySampleApp(k kubectl.Kubectl, ns, appVersion, profile string) {
	Expect(k.WithNamespace(ns).ApplyKustomize("helloworld", "service=helloworld")).To(Succeed(), "Sample service deploy failed on Cluster")
	Expect(k.WithNamespace(ns).ApplyKustomize("helloworld", "version="+appVersion)).To(Succeed(), "Sample service deploy failed on Cluster")
	Expect(k.WithNamespace(ns).ApplyKustomize("sleep")).To(Succeed(), "Sample sleep deploy failed on Cluster")

	// In Ambient mode, services need to be marked as "global" in order to load balance requests among clusters
	if profile == "ambient" {
		Expect(k.LabelNamespaced("service", ns, "helloworld", "istio.io/global", "true")).To(Succeed(), "Error labeling sample namespace")
	}
}

// deploySampleAppToClusters deploys the sample app to all provided clusters.
func deploySampleAppToClusters(ns, profile string, clusters []ClusterDeployment) {
	for _, cd := range clusters {
		k := cd.Kubectl
		Expect(k.CreateNamespace(ns)).To(Succeed(), fmt.Sprintf("Namespace failed to be created on Cluster %s", k.ClusterName))
		if profile == "ambient" {
			Expect(k.Label("namespace", ns, "istio.io/dataplane-mode", "ambient")).To(Succeed(), "Error labeling sample namespace")
		} else {
			Expect(k.Label("namespace", ns, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
		}

		deploySampleApp(k, ns, cd.AppVersion, profile)
	}
}

// verifyResponsesAreReceivedFromBothClusters checks that when the sleep pod in the sample namespace
// sends a request to the helloworld service, it receives responses from expectedVersions,
// which can be either "v1" or "v2" on on different clusters.
func verifyResponsesAreReceivedFromExpectedVersions(k kubectl.Kubectl, expectedVersions ...string) {
	if len(expectedVersions) == 0 {
		expectedVersions = []string{"v1", "v2"}
	}
	for _, v := range expectedVersions {
		Eventually(k.WithNamespace("sample").Exec, 10*time.Minute, 2*time.Second).
			WithArguments("deploy/sleep", "sleep", "curl -sS helloworld.sample:5000/hello").
			Should(ContainSubstring(fmt.Sprintf("Hello version: %s", v)),
				fmt.Sprintf("sleep pod in %s did not receive any response from %s", k.ClusterName, v))
	}
}

// genTemplate takes a YAML string with go template annotations and a structure
// that fills in those annotations and outputs a YAML string with that structure
// applied to the template.
// Example: version: {{ .Version }} | {Version: "1.2.3"} --> version: Version: "1.2.3"
// Any errors will fail the test.
func genTemplate(manifestTmpl string, values any) string {
	tmpl, err := template.New("manifest-template").Parse(manifestTmpl)
	Expect(err).ToNot(HaveOccurred(),
		"template is likely either malformed YAML or the values do not match what is expected")

	var b strings.Builder
	Expect(tmpl.Execute(&b, values)).To(Succeed())
	return b.String()
}

func createIstioNamespaces(k kubectl.Kubectl, network, profile string) {
	Expect(k.CreateNamespace(common.ControlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
	Expect(k.CreateNamespace(common.IstioCniNamespace)).To(Succeed(), "Istio CNI namespace failed to be created")

	if profile == "ambient" {
		Expect(k.Label("namespace", common.ControlPlaneNamespace, "topology.istio.io/network", network)).To(Succeed(), "Error labeling istio namespace")
		Expect(k.CreateNamespace(common.ZtunnelNamespace)).To(Succeed(), "Ztunnel namespace failed to be created")
	}
}

func createIstioResources(k kubectl.Kubectl, version, cluster, network, profile string, values ...string) {
	cniSpec := fmt.Sprintf(`
profile: %s`, profile)
	common.CreateIstioCNI(k, version, cniSpec)

	if profile == "ambient" {
		spec := fmt.Sprintf(`
values:
  ztunnel:
    multiCluster:
      clusterName: %s
    network: %s`, cluster, network)
		common.CreateZTunnel(k, version, spec)
	}

	spec := fmt.Sprintf(`
profile: %s
values:
  global:
    meshID: mesh1
    multiCluster:
      clusterName: %s
    network: %s`, profile, cluster, network)
	for _, value := range values {
		spec += common.Indent(value)
	}

	if profile == "ambient" {
		spec += fmt.Sprintf(`
  pilot:
    trustedZtunnelNamespace: %s
    env:
      AMBIENT_ENABLE_MULTI_NETWORK: "true"`, common.ZtunnelNamespace)
	}

	common.CreateIstio(k, version, spec)
}

func createIntermediateCA(k kubectl.Kubectl, zone, network, artifacts string, cl client.Client) {
	Expect(certs.PushIntermediateCA(k, common.ControlPlaneNamespace, zone, network, artifacts, cl)).
		To(Succeed(), fmt.Sprintf("Error pushing intermediate CA to %s Cluster", k.ClusterName))
}

func awaitSecretCreation(cluster string, cl client.Client) {
	Eventually(func() error {
		_, err := common.GetObject(context.Background(), cl, kube.Key("cacerts", common.ControlPlaneNamespace), &corev1.Secret{})
		return err
	}).ShouldNot(HaveOccurred(), fmt.Sprintf("Secret is not created on %s cluster", cluster))
}
