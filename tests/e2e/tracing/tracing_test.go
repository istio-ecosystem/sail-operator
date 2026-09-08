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
	"context"
	"fmt"
	"strings"
	"time"

	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	sailv1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryapiv1 "istio.io/api/telemetry/v1"
	telemetryv1 "istio.io/client-go/pkg/apis/telemetry/v1"
)

const (
	collectorName      = "tracing"
	collectorNamespace = "tracing-collector"
	happyPathName      = "e2e-tracing-happy-path"
	sidecarNamespace   = "tracing-sidecar"
	telemetryName      = "mesh-default"
	istioName          = "default"

	// otherFieldManager stands in for a user or a third-party controller that writes to
	// the same objects as the tracing integration.
	otherFieldManager = "e2e-other-field-manager"

	// conflictSettlePeriod is how long the integration must stay conflict-free before we
	// accept that a write by another field manager didn't disturb it.
	conflictSettlePeriod = 30 * time.Second
)

// istioVersion is the version the specs install; recorded so that applies of the Istio object
// can send spec.version, which is required and therefore always serialized.
var istioVersion string

var _ = Describe("Tracing integration controller",
	Label("tracing", "integration"), Ordered, Serial, func() {
		var clr cleaner.Cleaner

		BeforeAll(func(ctx SpecContext) {
			DeferCleanup(func(ctx SpecContext) {
				Expect(installer.UninstallOtel(ctx)).To(Succeed())
			})
			Expect(installer.InstallOtel(ctx)).To(Succeed())

			By("restarting the Sail Operator with the OpenTelemetry Collector CRD available")
			_, err := k.WithNamespace(operatorNamespace).RolloutRestart("deployment/" + operatorDeployment)
			Expect(err).NotTo(HaveOccurred())
			Eventually(deploymentIsRolledOut).WithArguments(ctx, operatorNamespace, operatorDeployment).Should(BeTrue())

			clr = cleaner.New(cl, "tracing happy path")
			clr.Record(ctx)
			DeferCleanup(func(ctx SpecContext) {
				clr.Cleanup(ctx)
			})

			versions := istioversion.GetLatestPatchVersions()
			Expect(versions).NotTo(BeEmpty(), "no supported Istio versions are configured")
			istioVersion = versions[0].Name

			Expect(k.CreateNamespace(common.ControlPlaneNamespace)).To(Succeed())
			Expect(k.CreateNamespace(common.IstioCniNamespace)).To(Succeed())
			Expect(k.CreateNamespace(collectorNamespace)).To(Succeed())
			Expect(k.CreateNamespace(sidecarNamespace)).To(Succeed())

			common.CreateIstioCNI(k, istioVersion)
			common.CreateIstio(k, istioVersion)
			common.AwaitCondition(ctx, sailv1.IstioCNIConditionReady, kube.Key("default"), &sailv1.IstioCNI{}, k, cl)
			common.AwaitCondition(ctx, sailv1.IstioConditionReady, kube.Key("default"), &sailv1.Istio{}, k, cl)

			Expect(k.CreateFromString(fmt.Sprintf(`
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: %s
  namespace: %s
spec:
  mode: deployment
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
    exporters:
      debug:
        verbosity: detailed
    service:
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [debug]`, collectorName, collectorNamespace))).To(Succeed())
			Eventually(deploymentIsRolledOut).
				WithArguments(ctx, collectorNamespace, collectorName+"-collector").Should(BeTrue())

			integration := &v1alpha1.TracingIntegration{
				ObjectMeta: metav1.ObjectMeta{Name: happyPathName},
				Spec: v1alpha1.TracingIntegrationSpec{
					TargetRefs:    []v1alpha1.TargetReference{{Kind: sailv1.IstioKind, Name: istioName}},
					TelemetryName: telemetryName,
					TracingConfig: v1alpha1.TracingConfig{
						Type: v1alpha1.TracingTypeOpenTelemetry,
						OpenTelemetry: &v1alpha1.OpenTelemetryConfig{
							OTELCollectorRef: v1alpha1.NamespacedReference{
								Name: collectorName, Namespace: collectorNamespace,
							},
						},
					},
				},
			}
			Expect(cl.Create(ctx, integration)).To(Succeed())
			Eventually(func(g Gomega) {
				actual := &v1alpha1.TracingIntegration{}
				g.Expect(cl.Get(ctx, kube.Key(happyPathName), actual)).To(Succeed())
				g.Expect(actual.Status.ObservedGeneration).To(Equal(actual.Generation))
				reconciled := actual.Status.GetCondition(v1alpha1.TracingIntegrationConditionReconciled)
				g.Expect(actual.Status.State).To(Equal(v1alpha1.TracingIntegrationReasonHealthy), reconciled.Message)
				g.Expect(reconciled.Status).To(Equal(metav1.ConditionTrue), reconciled.Message)
			}).Should(Succeed())
			telemetry := &telemetryv1.Telemetry{}
			Eventually(cl.Get).
				WithArguments(ctx, client.ObjectKey{Namespace: common.ControlPlaneNamespace, Name: "mesh-default"}, telemetry).
				Should(Succeed())
			Expect(telemetry.Spec.Tracing).NotTo(BeEmpty())
			samplingPercentage := new(wrappers.DoubleValue)
			samplingPercentage.Value = 100
			patch := client.MergeFrom(telemetry.DeepCopy())
			telemetry.Spec.Tracing[0].RandomSamplingPercentage = samplingPercentage
			Expect(cl.Patch(ctx, telemetry, patch)).To(Succeed())
		})

		It("exports a trace between two sidecar-injected pods", func(ctx SpecContext) {
			Expect(k.Label("namespace", sidecarNamespace, "istio-injection", "enabled")).To(Succeed())
			Expect(k.WithNamespace(sidecarNamespace).ApplyKustomize(common.SleepContainerName)).To(Succeed())
			Expect(k.WithNamespace(sidecarNamespace).ApplyKustomize(common.HttpbinContainerName)).To(Succeed())
			Eventually(common.CheckPodsReady).WithArguments(ctx, cl, sidecarNamespace).Should(Succeed())
			Eventually(func(g Gomega) {
				pods := &corev1.PodList{}
				g.Expect(cl.List(ctx, pods, client.InNamespace(sidecarNamespace))).To(Succeed())
				g.Expect(pods.Items).To(HaveLen(2))
				for _, pod := range pods.Items {
					g.Expect(common.HasSidecarInjected(pod)).To(BeTrue(), "pod %s has no sidecar", pod.Name)
				}
			}).Should(Succeed())

			sleepPods := &corev1.PodList{}
			Expect(cl.List(ctx, sleepPods, client.InNamespace(sidecarNamespace), client.MatchingLabels{"app": "sleep"})).
				To(Succeed())
			Expect(sleepPods.Items).To(HaveLen(1))

			initialTraceCount, err := collectorTraceCount(ctx)
			Expect(err).NotTo(HaveOccurred())
			Eventually(common.CheckHTTPConnectivity).WithArguments(
				k, sidecarNamespace, sleepPods.Items[0].Name, common.SleepContainerName,
				fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", sidecarNamespace), "200", 5,
			).Should(Succeed())
			Eventually(func(g Gomega) {
				traceCount, err := collectorTraceCount(ctx)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(traceCount).To(BeNumerically(">", initialTraceCount))
			}).Should(Succeed())
		})

		When("another field manager writes to the objects the integration applies", Ordered, func() {
			BeforeAll(func(ctx SpecContext) {
				// spec.tracing is an atomic list, so the sampling percentage patch above handed the
				// whole field to the patching manager and the integration now reports a conflict on
				// it. Delete the Telemetry so these specs start from an object the integration alone
				// owns; the controller notices the deletion and applies it again from scratch.
				telemetry := &telemetryv1.Telemetry{Namespace: common.ControlPlaneNamespace, Name: telemetryName}
				Expect(client.IgnoreNotFound(cl.Delete(ctx, telemetry))).To(Succeed())
				Eventually(integrationIsHealthy).WithArguments(ctx).Should(Succeed())
			})

			It("reports a conflict when another field manager owns a Telemetry field we manage", func(ctx SpecContext) {
				DeferCleanup(func(ctx SpecContext) {
					Expect(applyAsOtherFieldManager(ctx, defaultTelemetryObject())).To(Succeed())
					Eventually(integrationIsHealthy).WithArguments(ctx).Should(Succeed())
				})

				By("taking over spec.tracing and pointing it at a different provider")
				override := defaultTelemetryObject()
				override.Spec.Tracing = []*telemetryapiv1.Tracing{{
					Providers: []*telemetryapiv1.ProviderRef{{Name: "some-other-provider"}},
				}}
				Expect(applyAsOtherFieldManager(ctx, override, client.ForceOwnership)).To(Succeed())

				Eventually(integrationHasConflict).WithArguments(ctx, ".spec.tracing").Should(Succeed())

				By("verifying the integration did not take the field back")
				telemetry := &telemetryv1.Telemetry{}
				Expect(cl.Get(ctx, client.ObjectKey{Namespace: common.ControlPlaneNamespace, Name: telemetryName}, telemetry)).
					To(Succeed())
				Expect(telemetry.Spec.Tracing).To(HaveLen(1))
				Expect(telemetry.Spec.Tracing[0].Providers[0].Name).To(Equal("some-other-provider"))
			})

			It("reports no conflict when another field manager owns a Telemetry field we don't manage", func(ctx SpecContext) {
				DeferCleanup(func(ctx SpecContext) {
					Expect(applyAsOtherFieldManager(ctx, defaultTelemetryObject())).To(Succeed())
				})

				By("setting spec.accessLogging, which the integration never applies")
				override := defaultTelemetryObject()
				override.Spec.AccessLogging = []*telemetryapiv1.AccessLogging{{
					Providers: []*telemetryapiv1.ProviderRef{{Name: "envoy"}},
				}}
				Expect(applyAsOtherFieldManager(ctx, override)).To(Succeed())

				Consistently(integrationIsHealthy).WithArguments(ctx).
					WithTimeout(conflictSettlePeriod).WithPolling(2 * time.Second).Should(Succeed())

				By("verifying both field managers kept their fields")
				telemetry := &telemetryv1.Telemetry{}
				Expect(cl.Get(ctx, client.ObjectKey{Namespace: common.ControlPlaneNamespace, Name: telemetryName}, telemetry)).
					To(Succeed())
				Expect(telemetry.Spec.AccessLogging).To(HaveLen(1))
				Expect(telemetry.Spec.Tracing).To(HaveLen(1))
				Expect(telemetry.Spec.Tracing[0].Providers[0].Name).To(Equal(collectorName))
			})

			It("reports a conflict when another field manager owns an Istio field we manage", func(ctx SpecContext) {
				DeferCleanup(func(ctx SpecContext) {
					Expect(applyAsOtherFieldManager(ctx, defaultIstioObject())).To(Succeed())
					Eventually(integrationIsHealthy).WithArguments(ctx).Should(Succeed())
					Eventually(func(g Gomega) {
						g.Expect(istioMeshConfig(ctx, g).EnableTracing).To(HaveValue(BeTrue()))
					}).Should(Succeed())
				})

				By("taking over spec.values.meshConfig.enableTracing and turning tracing off")
				override := defaultIstioObject()
				override.Spec.Values = &sailv1.Values{MeshConfig: &sailv1.MeshConfig{EnableTracing: new(false)}}
				Expect(applyAsOtherFieldManager(ctx, override, client.ForceOwnership)).To(Succeed())

				Eventually(integrationHasConflict).
					WithArguments(ctx, ".spec.values.meshConfig.enableTracing").Should(Succeed())

				By("verifying the integration did not take the field back")
				Expect(istioMeshConfig(ctx, Default).EnableTracing).To(HaveValue(BeFalse()))
			})

			It("reports no conflict when another field manager owns an Istio field we don't manage", func(ctx SpecContext) {
				DeferCleanup(func(ctx SpecContext) {
					Expect(applyAsOtherFieldManager(ctx, defaultIstioObject())).To(Succeed())
				})

				By("setting spec.values.meshConfig.accessLogFile, a sibling of the fields we manage")
				override := defaultIstioObject()
				override.Spec.Values = &sailv1.Values{MeshConfig: &sailv1.MeshConfig{AccessLogFile: new("/dev/stdout")}}
				Expect(applyAsOtherFieldManager(ctx, override)).To(Succeed())

				Consistently(integrationIsHealthy).WithArguments(ctx).
					WithTimeout(conflictSettlePeriod).WithPolling(2 * time.Second).Should(Succeed())

				By("verifying both field managers kept their fields")
				meshConfig := istioMeshConfig(ctx, Default)
				Expect(meshConfig.AccessLogFile).To(HaveValue(Equal("/dev/stdout")))
				Expect(meshConfig.EnableTracing).To(HaveValue(BeTrue()))
				Expect(meshConfig.ExtensionProviders).To(HaveLen(1))
			})
		})
	})

// applyAsOtherFieldManager server-side applies obj on behalf of otherFieldManager. Pass
// client.ForceOwnership to take fields away from the integration's own field manager.
func applyAsOtherFieldManager(ctx context.Context, obj client.Object, opts ...client.ApplyOption) error {
	gvk, err := cl.GroupVersionKindFor(obj)
	if err != nil {
		return err
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	applyConfig := &unstructured.Unstructured{Object: content}
	applyConfig.SetGroupVersionKind(gvk)
	return cl.Apply(ctx, client.ApplyConfigurationFromUnstructured(applyConfig),
		append([]client.ApplyOption{client.FieldOwner(otherFieldManager)}, opts...)...)
}

// defaultTelemetryObject returns the Telemetry the integration manages, with an empty spec.
// Callers set the fields they want otherFieldManager to own.
func defaultTelemetryObject() *telemetryv1.Telemetry {
	return &telemetryv1.Telemetry{
		Name:      telemetryName,
		Namespace: common.ControlPlaneNamespace,
	}
}

// defaultIstioObject returns the Istio the integration targets. Callers set the fields they want
// otherFieldManager to own. The required spec fields are filled in because a typed object always
// serializes them, and applying them empty would blank them on the live Istio.
func defaultIstioObject() *sailv1.Istio {
	return &sailv1.Istio{
		Name: istioName,
		Spec: sailv1.IstioSpec{
			Version:   istioVersion,
			Namespace: common.ControlPlaneNamespace,
		},
	}
}

// istioMeshConfig returns the mesh config of the Istio object targeted by the integration.
func istioMeshConfig(ctx context.Context, g Gomega) *sailv1.MeshConfig {
	istio := &sailv1.Istio{}
	g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
	g.Expect(istio.Spec.Values).NotTo(BeNil())
	g.Expect(istio.Spec.Values.MeshConfig).NotTo(BeNil())
	return istio.Spec.Values.MeshConfig
}

// integrationIsHealthy asserts that the integration reconciled without an apply conflict.
func integrationIsHealthy(g Gomega, ctx context.Context) { //nolint:revive // Gomega must be first for async assertion injection.
	integration := &v1alpha1.TracingIntegration{}
	g.Expect(cl.Get(ctx, kube.Key(happyPathName), integration)).To(Succeed())
	conflicted := integration.Status.GetCondition(v1alpha1.TracingIntegrationConditionConflicted)
	g.Expect(conflicted.Status).To(Equal(metav1.ConditionFalse), conflicted.Message)
	g.Expect(conflicted.Reason).To(Equal(string(v1alpha1.TracingIntegrationReasonNoConflict)))
	g.Expect(integration.Status.State).To(Equal(v1alpha1.TracingIntegrationReasonHealthy))
}

// integrationHasConflict asserts that the integration reports an apply conflict on
// conflictingField. The integration stays Reconciled: a conflict is surfaced to the user,
// not treated as a reconcile failure, because the controller never forces ownership back.
func integrationHasConflict(g Gomega, ctx context.Context, conflictingField string) { //nolint:revive // Gomega must be first for async assertion injection.
	integration := &v1alpha1.TracingIntegration{}
	g.Expect(cl.Get(ctx, kube.Key(happyPathName), integration)).To(Succeed())
	conflicted := integration.Status.GetCondition(v1alpha1.TracingIntegrationConditionConflicted)
	g.Expect(conflicted.Status).To(Equal(metav1.ConditionTrue), "conditions: %+v", integration.Status.Conditions)
	g.Expect(conflicted.Reason).To(Equal(string(v1alpha1.TracingIntegrationReasonApplyConflict)))
	g.Expect(conflicted.Message).To(ContainSubstring(otherFieldManager))
	g.Expect(conflicted.Message).To(ContainSubstring(conflictingField))
	g.Expect(integration.Status.State).To(Equal(v1alpha1.TracingIntegrationReasonApplyConflict))
	g.Expect(integration.Status.GetCondition(v1alpha1.TracingIntegrationConditionReconciled).Status).
		To(Equal(metav1.ConditionTrue))
}

func collectorTraceCount(ctx context.Context) (int, error) {
	pods := &corev1.PodList{}
	if err := cl.List(ctx, pods, client.InNamespace(collectorNamespace)); err != nil {
		return 0, fmt.Errorf("failed to list collector pods: %w", err)
	}
	since := 5 * time.Minute
	traceCount := 0
	for _, pod := range pods.Items {
		if !strings.HasPrefix(pod.Name, collectorName+"-collector-") {
			continue
		}
		for _, container := range pod.Spec.Containers {
			if container.Name == "istio-proxy" {
				continue
			}
			logs, err := k.WithNamespace(collectorNamespace).LogsForContainer(pod.Name, container.Name, &since)
			if err != nil {
				return 0, fmt.Errorf("failed to read logs from %s/%s: %w", pod.Name, container.Name, err)
			}
			traceCount += strings.Count(strings.ToLower(logs), "trace id")
		}
	}
	return traceCount, nil
}

func deploymentIsRolledOut(ctx context.Context, namespace, name string) bool {
	deployment := &appsv1.Deployment{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		return false
	}
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	return deployment.Status.ObservedGeneration == deployment.Generation &&
		deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.AvailableReplicas == desired
}
