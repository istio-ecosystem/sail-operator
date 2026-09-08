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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	operatorhelm "github.com/istio-ecosystem/sail-operator/pkg/helm"
	"helm.sh/helm/v4/pkg/action"
	chartv2loader "helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	"helm.sh/helm/v4/pkg/storage/driver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	otelOperatorRelease = "e2e-opentelemetry-operator"

	otelOperatorHelmNamespace  = "opentelemetry-operator-system"
	otelOperatorOLMNamespace   = "openshift-opentelemetry-operator"
	otelOperatorHelmDeployment = "e2e-opentelemetry-operator"
	otelOperatorOLMDeployment  = "opentelemetry-operator-controller-manager"

	otelOperatorChart      = "opentelemetry-operator"
	otelOperatorRepository = "https://open-telemetry.github.io/opentelemetry-helm-charts"

	// TODO: Should these be configurable?
	otelOperatorSubscription = "opentelemetry-product"
	otelOperatorCSVPrefix    = "opentelemetry-operator."
	otelOperatorCatalog      = "redhat-operators"
	otelOperatorCatalogNS    = "openshift-marketplace"
)

// Installer installs and uninstalls integrations.
type Installer struct {
	client                 crclient.Client
	isOCP                  bool
	otelOperatorNamespace  string
	otelOperatorDeployment string
	restConf               *rest.Config
}

// NewInstaller creates an integration installer for the current test platform.
func NewInstaller(cl crclient.Client, kubeconfig *rest.Config) *Installer {
	installer := &Installer{
		client:                 cl,
		isOCP:                  env.GetBool("OCP", false),
		otelOperatorNamespace:  otelOperatorHelmNamespace,
		otelOperatorDeployment: otelOperatorHelmDeployment,
		restConf:               kubeconfig,
	}
	if installer.isOCP {
		installer.otelOperatorNamespace = otelOperatorOLMNamespace
		installer.otelOperatorDeployment = otelOperatorOLMDeployment
	}
	return installer
}

var otelOperatorValues = map[string]any{
	"admissionWebhooks": map[string]any{
		"certManager":      map[string]any{"enabled": false},
		"autoGenerateCert": map[string]any{"enabled": true},
	},
	"manager": map[string]any{
		"collectorImage": map[string]any{"repository": "otel/opentelemetry-collector-k8s"},
	},
}

func isReleaseUninstalled(versions []release.Releaser) bool {
	if len(versions) == 0 {
		return false
	}
	accessor, err := release.NewAccessor(versions[len(versions)-1])
	return err == nil && accessor.Status() == common.StatusUninstalled.String()
}

// InstallOtel installs or upgrades the OpenTelemetry Operator used by E2E tests.
// It uses OLM on OpenShift and Helm on other Kubernetes platforms.
func (i *Installer) InstallOtel(ctx context.Context) error {
	if i.isOCP {
		if err := i.installOtelWithOLM(ctx); err != nil {
			return err
		}
	} else {
		if err := i.installOtelWithHelm(ctx); err != nil {
			return err
		}
	}
	return i.waitForOtelOperatorDeployment(ctx)
}

func (i *Installer) installOtelWithOLM(ctx context.Context) error {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: i.otelOperatorNamespace}}
	if err := i.client.Create(ctx, namespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create OpenTelemetry Operator namespace: %w", err)
	}

	operatorGroupList := &unstructured.UnstructuredList{}
	operatorGroupList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "operators.coreos.com", Version: "v1", Kind: "OperatorGroupList",
	})
	if err := i.client.List(ctx, operatorGroupList, crclient.InNamespace(i.otelOperatorNamespace)); err != nil {
		return fmt.Errorf("failed to list OpenTelemetry OperatorGroups: %w", err)
	}
	if len(operatorGroupList.Items) == 0 {
		operatorGroup := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "operators.coreos.com/v1",
			"kind":       "OperatorGroup",
			"metadata": map[string]any{
				"generateName": i.otelOperatorNamespace + "-",
				"namespace":    i.otelOperatorNamespace,
			},
		}}
		if err := i.client.Create(ctx, operatorGroup); err != nil {
			return fmt.Errorf("failed to create OpenTelemetry OperatorGroup: %w", err)
		}
	}

	subscription := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "operators.coreos.com/v1alpha1",
		"kind":       "Subscription",
		"metadata": map[string]any{
			"name":      otelOperatorSubscription,
			"namespace": i.otelOperatorNamespace,
		},
		"spec": map[string]any{
			"installPlanApproval": "Automatic",
			"name":                otelOperatorSubscription,
			"source":              otelOperatorCatalog,
			"sourceNamespace":     otelOperatorCatalogNS,
		},
	}}
	if err := i.client.Create(ctx, subscription); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create OpenTelemetry Operator subscription: %w", err)
	}
	return nil
}

func (i *Installer) waitForOtelOperatorDeployment(ctx context.Context) error {
	deployment := &appsv1.Deployment{}
	key := crclient.ObjectKey{Namespace: i.otelOperatorNamespace, Name: i.otelOperatorDeployment}
	return wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := i.client.Get(ctx, key, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		return deployment.Status.ObservedGeneration == deployment.Generation &&
			deployment.Status.UpdatedReplicas == desired &&
			deployment.Status.AvailableReplicas == desired, nil
	})
}

func (i *Installer) installOtelWithHelm(ctx context.Context) error {
	actionConfig := action.NewConfiguration()
	if err := actionConfig.Init(operatorhelm.NewRESTClientGetter(i.restConf), i.otelOperatorNamespace, ""); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	settings := cli.New()
	chartPathOptions := action.ChartPathOptions{
		RepoURL: otelOperatorRepository,
		Version: env.Get("OTEL_OPERATOR_CHART_VERSION", ""),
	}
	chartPath, err := chartPathOptions.LocateChart(otelOperatorChart, settings)
	if err != nil {
		return fmt.Errorf("failed to locate OpenTelemetry Operator chart: %w", err)
	}
	chart, err := chartv2loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load OpenTelemetry Operator chart: %w", err)
	}

	history := action.NewHistory(actionConfig)
	history.Max = 1
	versions, err := history.Run(otelOperatorRelease)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return fmt.Errorf("failed to get OpenTelemetry Operator release history: %w", err)
	}

	if errors.Is(err, driver.ErrReleaseNotFound) || isReleaseUninstalled(versions) {
		install := action.NewInstall(actionConfig)
		install.CreateNamespace = true
		install.Namespace = i.otelOperatorNamespace
		install.ReleaseName = otelOperatorRelease
		install.Replace = isReleaseUninstalled(versions)
		install.WaitStrategy = kube.StatusWatcherStrategy
		install.WaitForJobs = true
		install.Timeout = 5 * time.Minute

		if _, err := install.RunWithContext(ctx, chart, otelOperatorValues); err != nil {
			return fmt.Errorf("failed to install OpenTelemetry Operator: %w", err)
		}
		return nil
	}

	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Namespace = i.otelOperatorNamespace
	upgrade.Install = true
	upgrade.WaitStrategy = kube.StatusWatcherStrategy
	upgrade.WaitForJobs = true
	upgrade.Timeout = 5 * time.Minute

	if _, err := upgrade.RunWithContext(ctx, otelOperatorRelease, chart, otelOperatorValues); err != nil {
		return fmt.Errorf("failed to upgrade OpenTelemetry Operator: %w", err)
	}
	return nil
}

// UninstallOtel removes the OpenTelemetry Operator installation. It uses OLM
// on OpenShift and Helm on other Kubernetes platforms, and succeeds if the
// installation is already absent.
func (i *Installer) UninstallOtel(ctx context.Context) error {
	if i.isOCP {
		if err := i.uninstallOtelWithOLM(ctx); err != nil {
			return err
		}
	} else if err := i.uninstallOtelWithHelm(); err != nil {
		return err
	}

	apiExtensionsClient, err := apiextensionsclient.NewForConfig(i.restConf)
	if err != nil {
		return fmt.Errorf("failed to create API extensions client: %w", err)
	}
	crds := apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions()
	if err := crds.Delete(ctx, constants.OpenTelemetryCollectorCRDName, metav1.DeleteOptions{}); err != nil &&
		!apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete OpenTelemetry Collector CRD: %w", err)
	}
	if err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := crds.Get(ctx, constants.OpenTelemetryCollectorCRDName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		return fmt.Errorf("failed waiting for OpenTelemetry Collector CRD deletion: %w", err)
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: i.otelOperatorNamespace}}
	if err := i.client.Delete(ctx, namespace); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete OpenTelemetry Operator namespace: %w", err)
	}
	if err := i.waitForResourceDeletion(ctx, namespace); err != nil {
		return fmt.Errorf("failed waiting for OpenTelemetry Operator namespace deletion: %w", err)
	}
	return nil
}

func (i *Installer) uninstallOtelWithOLM(ctx context.Context) error {
	subscriptionGVK := schema.GroupVersionKind{
		Group: "operators.coreos.com", Version: "v1alpha1", Kind: "Subscription",
	}
	csvGVK := schema.GroupVersionKind{
		Group: "operators.coreos.com", Version: "v1alpha1", Kind: "ClusterServiceVersion",
	}

	csvNames := map[string]struct{}{}
	subscription := &unstructured.Unstructured{}
	subscription.SetGroupVersionKind(subscriptionGVK)
	subscription.SetName(otelOperatorSubscription)
	subscription.SetNamespace(i.otelOperatorNamespace)
	err := i.client.Get(ctx, crclient.ObjectKeyFromObject(subscription), subscription)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get OpenTelemetry Operator subscription: %w", err)
	}
	subscriptionExists := err == nil
	if subscriptionExists {
		for _, field := range []string{"installedCSV", "currentCSV"} {
			if name, found, fieldErr := unstructured.NestedString(subscription.Object, "status", field); fieldErr != nil {
				return fmt.Errorf("failed to read OpenTelemetry Operator subscription status.%s: %w", field, fieldErr)
			} else if found && name != "" {
				csvNames[name] = struct{}{}
			}
		}
	}

	// Also discover an orphaned CSV left by a partially completed uninstall.
	csvList := &unstructured.UnstructuredList{}
	csvList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: csvGVK.Group, Version: csvGVK.Version, Kind: "ClusterServiceVersionList",
	})
	if err := i.client.List(ctx, csvList, crclient.InNamespace(i.otelOperatorNamespace)); err != nil &&
		!apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to list OpenTelemetry Operator CSVs: %w", err)
	}
	for _, csv := range csvList.Items {
		if strings.HasPrefix(csv.GetName(), otelOperatorCSVPrefix) {
			csvNames[csv.GetName()] = struct{}{}
		}
	}

	if subscriptionExists {
		if err := i.client.Delete(ctx, subscription); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete OpenTelemetry Operator subscription: %w", err)
		}
		if err := i.waitForResourceDeletion(ctx, subscription); err != nil {
			return fmt.Errorf("failed waiting for OpenTelemetry Operator subscription deletion: %w", err)
		}
	}

	for name := range csvNames {
		csv := &unstructured.Unstructured{}
		csv.SetGroupVersionKind(csvGVK)
		csv.SetName(name)
		csv.SetNamespace(i.otelOperatorNamespace)
		if err := i.client.Delete(ctx, csv); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete OpenTelemetry Operator CSV %q: %w", name, err)
		}
		if err := i.waitForResourceDeletion(ctx, csv); err != nil {
			return fmt.Errorf("failed waiting for OpenTelemetry Operator CSV %q deletion: %w", name, err)
		}
	}
	return nil
}

func (i *Installer) waitForResourceDeletion(ctx context.Context, obj crclient.Object) error {
	key := crclient.ObjectKeyFromObject(obj)
	return wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		err := i.client.Get(ctx, key, obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func (i *Installer) uninstallOtelWithHelm() error {
	actionConfig := action.NewConfiguration()
	if err := actionConfig.Init(operatorhelm.NewRESTClientGetter(i.restConf), i.otelOperatorNamespace, ""); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	uninstall := action.NewUninstall(actionConfig)
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
	uninstall.Timeout = 5 * time.Minute
	if _, err := uninstall.Run(otelOperatorRelease); err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return fmt.Errorf("failed to uninstall OpenTelemetry Operator: %w", err)
	}
	return nil
}
