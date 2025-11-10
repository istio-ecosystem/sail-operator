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
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/gomega"
)

// ClusterDeployment represents a cluster along with its sample app version.
type ClusterDeployment struct {
	Kubectl    kubectl.Kubectl
	AppVersion string
}

// deploySampleApp deploys the sample apps (helloworld and sleep) in the given cluster.
func deploySampleApp(k kubectl.Kubectl, ns string, appVersion string) {
	Expect(k.WithNamespace(ns).ApplyKustomize("helloworld", "service=helloworld")).To(Succeed(), "Sample service deploy failed on Cluster")
	Expect(k.WithNamespace(ns).ApplyKustomize("helloworld", "version="+appVersion)).To(Succeed(), "Sample service deploy failed on Cluster")
	Expect(k.WithNamespace(ns).ApplyKustomize("sleep")).To(Succeed(), "Sample sleep deploy failed on Cluster")
}

// deploySampleAppToClusters deploys the sample app to all provided clusters.
func deploySampleAppToClusters(ns string, clusters []ClusterDeployment) {
	for _, cd := range clusters {
		deploySampleApp(cd.Kubectl, ns, cd.AppVersion)
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
