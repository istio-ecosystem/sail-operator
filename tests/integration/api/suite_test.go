//go:build integration

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
	"context"
	"path"
	"testing"

	"github.com/istio-ecosystem/sail-operator/controllers/istio"
	"github.com/istio-ecosystem/sail-operator/controllers/istiocni"
	"github.com/istio-ecosystem/sail-operator/controllers/istiorevision"
	"github.com/istio-ecosystem/sail-operator/controllers/istiorevisiontag"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	cfg       *rest.Config
	cancel    context.CancelFunc
)

const operatorNamespace = "sail-operator"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	testEnv, k8sClient, cfg = test.SetupEnv(GinkgoWriter, true)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: ":8080"},
		NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
			return k8sClient, nil
		},
	})
	if err != nil {
		panic(err)
	}

	chartManager := helm.NewChartManager(mgr.GetConfig(), "")

	cfg := config.ReconcilerConfig{
		ResourceDirectory: path.Join(project.RootDir, "resources"),
		Platform:          config.PlatformKubernetes,
		DefaultProfile:    "",
	}

	cl := mgr.GetClient()
	scheme := mgr.GetScheme()
	Expect(istio.NewReconciler(cfg, cl, scheme).SetupWithManager(mgr)).To(Succeed())
	Expect(istiorevision.NewReconciler(cfg, cl, scheme, chartManager).SetupWithManager(mgr)).To(Succeed())
	Expect(istiorevisiontag.NewReconciler(cfg, cl, scheme, chartManager).SetupWithManager(mgr)).To(Succeed())
	Expect(istiocni.NewReconciler(cfg, cl, scheme, chartManager).SetupWithManager(mgr)).To(Succeed())

	// create new cancellable context
	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())

	go func() {
		if err := mgr.Start(ctx); err != nil {
			panic(err)
		}
	}()

	operatorNs := &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: operatorNamespace}}
	Expect(k8sClient.Create(context.TODO(), operatorNs)).To(Succeed())

	istiovalues.SetVendorDefaults(nil)
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if cancel != nil {
		cancel()
	}
	Expect(testEnv.Stop()).To(Succeed())
})
