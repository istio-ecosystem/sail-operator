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
	"os"
	"path/filepath"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	clPrimary                     client.Client
	clRemote                      client.Client
	err                           error
	debugInfoLogged               bool
	namespace                     = env.Get("NAMESPACE", "sail-operator")
	deploymentName                = env.Get("DEPLOYMENT_NAME", "sail-operator")
	controlPlaneNamespace         = env.Get("CONTROL_PLANE_NS", "istio-system")
	externalControlPlaneNamespace = env.Get("EXTERNAL_CONTROL_PLANE_NS", "external-istiod")
	istioName                     = env.Get("ISTIO_NAME", "default")
	istioCniNamespace             = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")
	istioCniName                  = env.Get("ISTIOCNI_NAME", "default")
	skipDeploy                    = env.GetBool("SKIP_DEPLOY", false)
	multicluster                  = env.GetBool("MULTICLUSTER", false)
	keepOnFailure                 = env.GetBool("KEEP_ON_FAILURE", false)
	kubeconfig                    = env.Get("KUBECONFIG", "")
	kubeconfig2                   = env.Get("KUBECONFIG2", "")
	artifacts                     = env.Get("ARTIFACTS", "/tmp/artifacts")
	sampleNamespace               = env.Get("SAMPLE_NAMESPACE", "sample")

	controlPlaneGatewayYAML string
	eastGatewayYAML         string
	westGatewayYAML         string
	exposeServiceYAML       string
	exposeIstiodYAML        string

	k1 kubectl.Kubectl
	k2 kubectl.Kubectl

	clr1 cleaner.Cleaner
	clr2 cleaner.Cleaner
)

func TestMultiCluster(t *testing.T) {
	if !multicluster {
		t.Skip("Skipping test. Only valid for multicluster")
	}
	if kubeconfig == "" && kubeconfig2 == "" {
		t.Skip("Skipping test. Two clusters required for multicluster test")
	}
	RegisterFailHandler(Fail)
	setup(t)
	RunSpecs(t, "Multi-Cluster Test Suite")
}

func setup(t *testing.T) {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	clPrimary, err = k8sclient.InitK8sClient(kubeconfig)
	clRemote, err = k8sclient.InitK8sClient(kubeconfig2)
	if err != nil {
		t.Fatalf("Error initializing k8s client: %v", err)
	}

	err := certs.CreateIntermediateCA(artifacts)
	if err != nil {
		t.Fatalf("Error creating intermediate CA: %v", err)
	}

	// Set the path for the multicluster YAML files to be used
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting working directory: %v", err)
	}

	// Set base path
	baseRepoDir := filepath.Join(workDir, "../../..")
	controlPlaneGatewayYAML = fmt.Sprintf("%s/docs/deployment-models/resources/controlplane-gateway.yaml", baseRepoDir)
	eastGatewayYAML = fmt.Sprintf("%s/docs/deployment-models/resources/east-west-gateway-net1.yaml", baseRepoDir)
	westGatewayYAML = fmt.Sprintf("%s/docs/deployment-models/resources/east-west-gateway-net2.yaml", baseRepoDir)
	exposeServiceYAML = fmt.Sprintf("%s/docs/deployment-models/resources/expose-services.yaml", baseRepoDir)
	exposeIstiodYAML = fmt.Sprintf("%s/docs/deployment-models/resources/expose-istiod.yaml", baseRepoDir)

	// Initialize kubectl utilities, one for each cluster
	k1 = kubectl.New().WithKubeconfig(kubeconfig).WithClusterName("primary")
	k2 = kubectl.New().WithKubeconfig(kubeconfig2).WithClusterName("remote")
	clr1 = cleaner.New(clPrimary, "cluster=primary")
	clr2 = cleaner.New(clRemote, "cluster=remote")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	clr1.Record(ctx)
	clr2.Record(ctx)

	if skipDeploy {
		return
	}

	Expect(k1.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on Primary Cluster")
	Expect(k2.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on Remote Cluster")

	Expect(common.InstallOperatorViaHelm("--kubeconfig", kubeconfig)).
		To(Succeed(), "Operator failed to be deployed in Primary Cluster")

	Expect(common.InstallOperatorViaHelm("--kubeconfig", kubeconfig2)).
		To(Succeed(), "Operator failed to be deployed in Remote Cluster")

	Eventually(common.GetObject).
		WithArguments(ctx, clPrimary, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
		Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
	Success("Operator is deployed in the Primary namespace and Running")

	Eventually(common.GetObject).
		WithArguments(ctx, clRemote, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
		Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
	Success("Operator is deployed in the Remote namespace and Running")
})

var _ = ReportAfterSuite("Conditional cleanup", func(ctx SpecContext, r Report) {
	if !r.SuiteSucceeded {
		if !debugInfoLogged {
			common.LogDebugInfo(common.MultiCluster, k1, k2)
		}

		if keepOnFailure {
			return
		}
	}

	c1Deleted := clr1.CleanupNoWait(ctx)
	c2Deleted := clr2.CleanupNoWait(ctx)
	clr1.WaitForDeletion(ctx, c1Deleted)
	clr2.WaitForDeletion(ctx, c2Deleted)
})
