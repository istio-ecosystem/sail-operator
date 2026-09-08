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

package tracing

import (
	"testing"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/integrations"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cl        client.Client
	k         kubectl.Kubectl
	installer *integrations.Installer

	operatorNamespace  = common.OperatorNamespace
	operatorDeployment = env.Get("DEPLOYMENT_NAME", "sail-operator")
	multicluster       = env.GetBool("MULTICLUSTER", false)
)

func TestTracing(t *testing.T) {
	if multicluster {
		t.Skip("Skipping the tracing integration tests in a multicluster environment")
	}

	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(time.Duration(env.GetInt("DEFAULT_TEST_TIMEOUT", 180)) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	var err error
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())
	kubeConfig, err := k8sclient.GetConfig("")
	Expect(err).NotTo(HaveOccurred())
	installer = integrations.NewInstaller(cl, kubeConfig)
	k = kubectl.New()

	RunSpecs(t, "Tracing Integration Test Suite")
}
