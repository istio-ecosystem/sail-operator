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

package update

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AwaitRevisionCount waits for Istio CR to have the expected number of revisions in use
func AwaitRevisionCount(ctx context.Context, cl client.Client, istioName string, expectedCount int32) {
	Eventually(func(g Gomega) {
		istio := &v1.Istio{}
		g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed(), "Failed to get Istio CR")
		g.Expect(istio.Status.Revisions.InUse).To(Equal(expectedCount),
			fmt.Sprintf("Expected %d revisions in use, got %d", expectedCount, istio.Status.Revisions.InUse))
	}).Should(Succeed(), fmt.Sprintf("Istio CR should have %d revisions in use", expectedCount))
}

// AwaitIstioRevisionInUse waits for IstioRevision to have the expected InUse condition status
func AwaitIstioRevisionInUse(ctx context.Context, cl client.Client, revisionName, namespace string, expectedInUse bool) {
	expectedStatus := metav1.ConditionTrue
	if !expectedInUse {
		expectedStatus = metav1.ConditionFalse
	}

	Eventually(func(g Gomega) {
		revision := &v1.IstioRevision{}
		g.Expect(cl.Get(ctx, kube.Key(revisionName, namespace), revision)).To(Succeed(), "Failed to get IstioRevision")
		g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionInUse, expectedStatus),
			fmt.Sprintf("Expected IstioRevision %s InUse=%v", revisionName, expectedInUse))
	}).Should(Succeed(), fmt.Sprintf("IstioRevision %s should have InUse=%v", revisionName, expectedInUse))
}

// ValidateComponentVersion validates that a component has the expected version
func ValidateComponentVersion(ctx context.Context, cl client.Client, componentType, name, namespace string, expectedVersion *semver.Version) error {
	switch componentType {
	case "IstioCNI":
		return validateIstioCNIVersion(ctx, cl, name, expectedVersion)
	case "ZTunnel":
		return validateZTunnelVersion(ctx, cl, name, expectedVersion)
	case "Deployment":
		return validateDeploymentVersion(ctx, cl, name, namespace, expectedVersion)
	default:
		return fmt.Errorf("unsupported component type: %s", componentType)
	}
}

// validateIstioCNIVersion validates IstioCNI DaemonSet version
func validateIstioCNIVersion(ctx context.Context, cl client.Client, name string, expectedVersion *semver.Version) error {
	cni := &v1.IstioCNI{}
	if err := cl.Get(ctx, kube.Key(name), cni); err != nil {
		return fmt.Errorf("failed to get IstioCNI CR: %w", err)
	}

	// Parse version from CR status or spec
	var versionStr string
	if cni.Status.ObservedGeneration > 0 && cni.Spec.Version != "" {
		versionStr = cni.Spec.Version
	} else {
		return fmt.Errorf("IstioCNI CR %s has no version set", name)
	}

	// Extract semver from version string (may be "v1.29.2" or just "1.29.2")
	versionStr = strings.TrimPrefix(versionStr, "v")
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return fmt.Errorf("failed to parse IstioCNI version %q: %w", versionStr, err)
	}

	if !version.Equal(expectedVersion) {
		return fmt.Errorf("IstioCNI has version %s, expected %s", version, expectedVersion)
	}
	return nil
}

// validateZTunnelVersion validates ZTunnel DaemonSet version
func validateZTunnelVersion(ctx context.Context, cl client.Client, name string, expectedVersion *semver.Version) error {
	ztunnel := &v1.ZTunnel{}
	if err := cl.Get(ctx, kube.Key(name), ztunnel); err != nil {
		return fmt.Errorf("failed to get ZTunnel CR: %w", err)
	}

	// Parse version from CR spec
	var versionStr string
	if ztunnel.Spec.Version != "" {
		versionStr = ztunnel.Spec.Version
	} else {
		return fmt.Errorf("ZTunnel CR %s has no version set", name)
	}

	// Extract semver from version string
	versionStr = strings.TrimPrefix(versionStr, "v")
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return fmt.Errorf("failed to parse ZTunnel version %q: %w", versionStr, err)
	}

	if !version.Equal(expectedVersion) {
		return fmt.Errorf("ZTunnel has version %s, expected %s", version, expectedVersion)
	}
	return nil
}

// validateDeploymentVersion validates Deployment version by reading image tag
func validateDeploymentVersion(ctx context.Context, cl client.Client, name, namespace string, expectedVersion *semver.Version) error {
	deployment := &appsv1.Deployment{}
	if err := cl.Get(ctx, kube.Key(name, namespace), deployment); err != nil {
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	// Extract version from first container image tag
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("Deployment %s has no containers", name)
	}

	image := deployment.Spec.Template.Spec.Containers[0].Image
	parts := strings.Split(image, ":")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected image format: %s", image)
	}

	tag := parts[1]
	version, err := semver.NewVersion(tag)
	if err != nil {
		return fmt.Errorf("failed to parse version from image tag %q: %w", tag, err)
	}

	if !version.Equal(expectedVersion) {
		return fmt.Errorf("Deployment has version %s, expected %s", version, expectedVersion)
	}
	return nil
}
