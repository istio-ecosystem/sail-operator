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

package relabeling

import (
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

func TestForPlatformKubernetes(t *testing.T) {
	g := NewWithT(t)

	cfg := ForPlatform(config.PlatformKubernetes, "ignored-mesh-id", false)

	g.Expect(cfg.ServiceMonitorRelabelings).To(BeEmpty())
	g.Expect(cfg.PodMonitorRelabelings).To(HaveLen(7))

	g.Expect(cfg.PodMonitorRelabelings[0].Action).To(Equal("keep"))
	g.Expect(cfg.PodMonitorRelabelings[0].Regex).To(Equal("istio-proxy"))
	g.Expect(cfg.PodMonitorRelabelings[4].Action).To(Equal("labeldrop"))
	g.Expect(cfg.PodMonitorRelabelings[6].TargetLabel).To(Equal("pod"))
}

func TestForPlatformOpenShift(t *testing.T) {
	g := NewWithT(t)

	cfg := ForPlatform(config.PlatformOpenShift, "my-mesh", false)

	g.Expect(cfg.ServiceMonitorRelabelings).To(BeEmpty())
	g.Expect(cfg.PodMonitorRelabelings).To(HaveLen(8))

	g.Expect(cfg.PodMonitorRelabelings[4].TargetLabel).To(Equal("app"))
	g.Expect(cfg.PodMonitorRelabelings[5].TargetLabel).To(Equal("version"))
	g.Expect(cfg.PodMonitorRelabelings[7].TargetLabel).To(Equal("mesh_id"))
	g.Expect(*cfg.PodMonitorRelabelings[7].Replacement).To(Equal("my-mesh"))
}

func TestForPlatformOpenShiftTuning(t *testing.T) {
	g := NewWithT(t)

	cfg := ForPlatform(config.PlatformOpenShift, "my-mesh", true)

	g.Expect(cfg.PodMonitorRelabelings[8].Action).To(Equal("drop"))
	g.Expect(cfg.PodMonitorRelabelings[9].Action).To(Equal("labeldrop"))
}

func TestForPlatformDefault(t *testing.T) {
	g := NewWithT(t)

	cfg := ForPlatform(config.PlatformUndefined, "ignored", false)

	g.Expect(cfg.PodMonitorRelabelings).To(HaveLen(7))
	g.Expect(cfg.PodMonitorRelabelings[6].TargetLabel).To(Equal("pod"))
}

func TestCloneRelabelConfigs(t *testing.T) {
	g := NewWithT(t)

	original := kubernetesPodMonitorRelabelings()
	cloned := cloneRelabelConfigs(original)

	g.Expect(cloned).To(HaveLen(len(original)))
	g.Expect(cloned[0].SourceLabels).ToNot(BeIdenticalTo(original[0].SourceLabels))

	original[0].SourceLabels[0] = "mutated"
	g.Expect(cloned[0].SourceLabels[0]).To(Equal(monitoringv1.LabelName("__meta_kubernetes_pod_container_name")))
}
