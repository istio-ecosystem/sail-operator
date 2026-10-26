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

package integrations

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/perses"
	"github.com/istio-ecosystem/sail-operator/pkg/perses/testutil"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

// PersesInstaller installs and removes Perses-related dependencies for E2E tests.
type PersesInstaller struct {
	restConf          *rest.Config
	operatorNamespace string
	isOCP             bool
	crdInstalled      bool
}

// NewPersesInstaller creates a Perses dependency installer for the current platform.
func NewPersesInstaller(cfg *rest.Config) *PersesInstaller {
	return &PersesInstaller{
		restConf:          cfg,
		operatorNamespace: env.Get("NAMESPACE", "sail-operator"),
		isOCP:             env.GetBool("OCP", false),
	}
}

// InstallDashboardCRD ensures the PersesDashboard CRD exists. On OpenShift the CRD may
// already be provided by the observability stack; in that case this is a no-op.
func (p *PersesInstaller) InstallDashboardCRD(ctx context.Context) error {
	ext, err := apiextensionsclient.NewForConfig(p.restConf)
	if err != nil {
		return fmt.Errorf("create apiextensions client: %w", err)
	}

	_, err = ext.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, perses.PersesDashboardCRD, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get PersesDashboard CRD: %w", err)
	}

	crd := testutil.DashboardCRD()
	if _, err = ext.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create PersesDashboard CRD: %w", err)
	}
	p.crdInstalled = true

	return wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		got, err := ext.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, perses.PersesDashboardCRD, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, c := range got.Status.Conditions {
			if c.Type == apiextensionsv1.Established && c.Status == apiextensionsv1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

// UninstallDashboardCRD removes the PersesDashboard CRD when this installer created it.
// It does not delete cluster-wide Perses CRDs on OpenShift.
func (p *PersesInstaller) UninstallDashboardCRD(ctx context.Context) error {
	if p.isOCP || !p.crdInstalled {
		return nil
	}

	ext, err := apiextensionsclient.NewForConfig(p.restConf)
	if err != nil {
		return fmt.Errorf("create apiextensions client: %w", err)
	}

	err = ext.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, perses.PersesDashboardCRD, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete PersesDashboard CRD: %w", err)
	}

	return wait.PollUntilContextTimeout(ctx, time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := ext.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, perses.PersesDashboardCRD, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

// RolloutRestartOperator forces a new operator process so leader-elected runnables
// (for example the Perses dashboard installer) run again after optional CRDs are installed.
//
// Pods are deleted instead of `kubectl rollout restart`: under OLM the CSV owns the
// Deployment and a rollout restart often does not replace the running pod, leaving the
// Perses installer stuck after a one-shot Helm discovery failure.
func (p *PersesInstaller) RolloutRestartOperator(deploymentName string) error {
	cmd := fmt.Sprintf(
		"kubectl delete pod --namespace %q --selector=control-plane=sail-operator --wait=true --timeout=3m",
		p.operatorNamespace,
	)
	out, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("kubectl delete pod (restart %s): %w\n%s", deploymentName, err, out)
	}
	return nil
}

// SetOperatorPersesDashboards toggles operator.enablePersesDashboards via Helm upgrade.
func (p *PersesInstaller) SetOperatorPersesDashboards(enabled bool) error {
	chartPath := filepath.Join(project.RootDir, "chart")
	cmd := fmt.Sprintf(
		"helm upgrade sail-operator %q --namespace %q --reuse-values --set operator.enablePersesDashboards=%t --wait --timeout 5m",
		chartPath, p.operatorNamespace, enabled,
	)
	out, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("helm upgrade sail-operator (enablePersesDashboards=%t): %w\n%s", enabled, err, out)
	}
	return nil
}
