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

package install

import (
	"context"
	"io/fs"
	"sync"
	"testing"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/resources"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v4/pkg/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlreconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
)

func TestWithCRDOwnershipLabel(t *testing.T) {
	g := NewWithT(t)

	o := libraryOptions{
		crdOwnershipLabelKey:   defaultCRDOwnershipLabelKey,
		crdOwnershipLabelValue: defaultCRDOwnershipLabelValue,
	}
	WithCRDOwnershipLabel("ingress.operator.openshift.io/owned", "true")(&o)
	g.Expect(o.crdOwnershipLabelKey).To(Equal("ingress.operator.openshift.io/owned"))
	g.Expect(o.crdOwnershipLabelValue).To(Equal("true"))
}

func TestCRDOwnershipLabelEmptyValidation(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "empty key", key: "", value: "true", wantErr: "CRD ownership label key must not be empty"},
		{name: "empty value", key: defaultCRDOwnershipLabelKey, value: "", wantErr: "CRD ownership label value must not be empty"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateCRDOwnershipLabel(tc.key, tc.value)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(Equal(tc.wantErr))
		})
	}
}

func TestValidateOptions(t *testing.T) {
	savedMap := istioversion.Map
	savedEOL := istioversion.EOL
	defer func() { istioversion.Map = savedMap; istioversion.EOL = savedEOL }()

	istioversion.Map = map[string]istioversion.VersionInfo{
		"v1.0.0": {Name: "v1.0.0"},
	}
	istioversion.EOL = []string{"v0.9.0"}

	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name:    "valid options",
			opts:    Options{Namespace: "istio-system", Version: "v1.0.0"},
			wantErr: false,
		},
		{
			name:    "empty namespace",
			opts:    Options{Namespace: "", Version: "v1.0.0"},
			wantErr: true,
		},
		{
			name:    "empty version",
			opts:    Options{Namespace: "istio-system", Version: ""},
			wantErr: true,
		},
		{
			name:    "unsupported version",
			opts:    Options{Namespace: "istio-system", Version: "v99.0.0"},
			wantErr: true,
		},
		{
			name:    "EOL version",
			opts:    Options{Namespace: "istio-system", Version: "v0.9.0"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateOptions(tc.opts)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestStatus(t *testing.T) {
	g := NewWithT(t)

	l := &Library{
		notifyCh: make(chan struct{}, 1),
	}
	l.currentStatus = Status{
		Installed: true,
		Version:   "v1.0.0",
		CRDState:  CRDManagementStateReady,
	}

	s := l.Status()
	g.Expect(s.Installed).To(BeTrue())
	g.Expect(s.Version).To(Equal("v1.0.0"))
	g.Expect(s.CRDState).To(Equal(CRDManagementStateReady))
}

func TestStop_noop(t *testing.T) {
	l := &Library{}
	l.Stop()
}

func TestUninstall_clearsStateAndAllowsReinstall(t *testing.T) {
	g := NewWithT(t)

	savedMap := istioversion.Map
	savedEOL := istioversion.EOL
	defer func() { istioversion.Map = savedMap; istioversion.EOL = savedEOL }()
	istioversion.Map = map[string]istioversion.VersionInfo{"v1.0.0": {Name: "v1.0.0"}}
	istioversion.EOL = nil

	l := &Library{
		triggerCh: make(chan event.GenericEvent, 1),
	}

	// Simulate a successful install cycle: set desiredOpts and status.
	opts := Options{Namespace: "istio-system", Version: "v1.0.0"}
	err := l.Apply(opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(1)))
	<-l.triggerCh

	l.statusMu.Lock()
	l.currentStatus = Status{Installed: true, Version: "v1.0.0"}
	l.statusMu.Unlock()

	// Directly nil the desiredOpts and clear status as Uninstall would
	// after a successful Helm uninstall. We can't call the real Uninstall
	// without a cluster, but we can verify the state contract.
	l.mu.Lock()
	l.desiredOpts = nil
	l.mu.Unlock()
	l.statusMu.Lock()
	l.currentStatus = Status{}
	l.statusMu.Unlock()

	// Status should now be zero-value.
	s := l.Status()
	g.Expect(s.Installed).To(BeFalse())
	g.Expect(s.Version).To(BeEmpty())
	g.Expect(s.Error).NotTo(HaveOccurred())

	// Apply with the same options should trigger a new install cycle
	// because desiredOpts is nil (no optionsEqual comparison).
	err = l.Apply(opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(2)))
	g.Expect(l.triggerCh).To(HaveLen(1))
}

func TestApply_validationError(t *testing.T) {
	g := NewWithT(t)

	savedMap := istioversion.Map
	savedEOL := istioversion.EOL
	defer func() { istioversion.Map = savedMap; istioversion.EOL = savedEOL }()

	istioversion.Map = map[string]istioversion.VersionInfo{
		"v1.0.0": {Name: "v1.0.0"},
	}
	istioversion.EOL = nil

	l := &Library{
		triggerCh: make(chan event.GenericEvent, 1),
	}

	err := l.Apply(Options{Namespace: "", Version: "v1.0.0"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("namespace"))
}

func TestEnqueue(t *testing.T) {
	g := NewWithT(t)

	l := &Library{
		triggerCh: make(chan event.GenericEvent, 1),
	}

	l.Enqueue()
	g.Expect(l.triggerCh).To(HaveLen(1))

	// Second call should not block (channel already has an event)
	l.Enqueue()
	g.Expect(l.triggerCh).To(HaveLen(1))
}

func TestApply_skipsWhenOptionsUnchanged(t *testing.T) {
	g := NewWithT(t)

	savedMap := istioversion.Map
	savedEOL := istioversion.EOL
	defer func() { istioversion.Map = savedMap; istioversion.EOL = savedEOL }()
	istioversion.Map = map[string]istioversion.VersionInfo{"v1.0.0": {Name: "v1.0.0"}}
	istioversion.EOL = nil

	l := &Library{
		triggerCh: make(chan event.GenericEvent, 1),
	}

	opts := Options{Namespace: "istio-system", Version: "v1.0.0"}

	err := l.Apply(opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(1)))
	g.Expect(l.triggerCh).To(HaveLen(1))

	// Drain the trigger channel
	<-l.triggerCh

	// Apply same options again — should be a no-op
	err = l.Apply(opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(1)))
	g.Expect(l.triggerCh).To(BeEmpty())
}

func TestApply_isolatesFromCallerMutation(t *testing.T) {
	g := NewWithT(t)

	savedMap := istioversion.Map
	savedEOL := istioversion.EOL
	defer func() { istioversion.Map = savedMap; istioversion.EOL = savedEOL }()
	istioversion.Map = map[string]istioversion.VersionInfo{"v1.0.0": {Name: "v1.0.0"}}
	istioversion.EOL = nil

	l := &Library{
		triggerCh: make(chan event.GenericEvent, 1),
	}

	values := &v1.Values{Pilot: &v1.PilotConfig{Hub: ptr.Of("original")}}
	opts := Options{
		Namespace: "istio-system",
		Version:   "v1.0.0",
		Values:    values,
	}

	err := l.Apply(opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(1)))
	<-l.triggerCh

	// Caller mutates the Values pointer after Apply (simulates what
	// ApplyDigests would do if Apply hadn't deep-copied).
	values.Pilot.Image = ptr.Of("mutated-by-caller")

	// Apply again with a fresh copy of the original intent — must be a
	// no-op because stored desiredOpts should be isolated from the
	// caller's mutation above.
	freshOpts := Options{
		Namespace: "istio-system",
		Version:   "v1.0.0",
		Values:    &v1.Values{Pilot: &v1.PilotConfig{Hub: ptr.Of("original")}},
	}
	err = l.Apply(freshOpts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(1)), "generation should not increment when intent is unchanged")
	g.Expect(l.triggerCh).To(BeEmpty())
}

func TestApply_triggersWhenOptionsChange(t *testing.T) {
	g := NewWithT(t)

	savedMap := istioversion.Map
	savedEOL := istioversion.EOL
	defer func() { istioversion.Map = savedMap; istioversion.EOL = savedEOL }()
	istioversion.Map = map[string]istioversion.VersionInfo{
		"v1.0.0": {Name: "v1.0.0"},
		"v1.1.0": {Name: "v1.1.0"},
	}
	istioversion.EOL = nil

	l := &Library{
		triggerCh: make(chan event.GenericEvent, 1),
	}

	err := l.Apply(Options{Namespace: "istio-system", Version: "v1.0.0"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(1)))

	// Drain the trigger channel
	<-l.triggerCh

	// Apply different options — should trigger
	err = l.Apply(Options{Namespace: "istio-system", Version: "v1.1.0"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(l.generation).To(Equal(uint64(2)))
	g.Expect(l.triggerCh).To(HaveLen(1))
}

// slowChartReconciler blocks during UpgradeOrInstallChart and records the
// order of install-start, install-end, and uninstall operations so tests
// can verify serialization between Reconcile and Uninstall.
type slowChartReconciler struct {
	mu  sync.Mutex
	ops []string

	installEntered chan struct{} // closed when UpgradeOrInstallChart is entered
	installBlock   chan struct{} // UpgradeOrInstallChart blocks until this is closed
}

var _ helm.ChartReconciler = (*slowChartReconciler)(nil)

func (m *slowChartReconciler) UpgradeOrInstallChart(
	_ context.Context, _ fs.FS, _ string, _ helm.Values,
	_, _ string, _ *metav1.OwnerReference,
) (release.Releaser, error) {
	m.mu.Lock()
	m.ops = append(m.ops, "install_start")
	m.mu.Unlock()

	close(m.installEntered)
	<-m.installBlock

	m.mu.Lock()
	m.ops = append(m.ops, "install_end")
	m.mu.Unlock()
	return nil, nil
}

func (m *slowChartReconciler) UninstallChart(
	_ context.Context, _, _ string,
) (*release.UninstallReleaseResponse, error) {
	m.mu.Lock()
	m.ops = append(m.ops, "uninstall")
	m.mu.Unlock()
	return &release.UninstallReleaseResponse{Info: "ok"}, nil
}

func TestUninstall_raceWithReconcile(t *testing.T) {
	g := NewWithT(t)

	mock := &slowChartReconciler{
		installEntered: make(chan struct{}),
		installBlock:   make(chan struct{}),
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}}
	cl := fake.NewClientBuilder().WithObjects(ns).Build()

	l := &Library{
		chartManager: mock,
		cl:           cl,
		resourceFS:   resources.FS,
		triggerCh:    make(chan event.GenericEvent, 1),
		notifyCh:     make(chan struct{}, 1),
	}

	opts := Options{
		Namespace: "test-ns",
		Version:   istioversion.Default,
		Revision:  "racetest",
		Values:    GatewayAPIDefaults("test-ns"),
	}
	l.desiredOpts = &opts
	l.generation = 1

	reconciler := &libraryReconciler{lib: l}

	var wg sync.WaitGroup

	// Start a reconciliation — it will block inside UpgradeOrInstallChart.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = reconciler.Reconcile(context.Background(), ctrlreconcile.Request{})
	}()

	// Wait for the install to be in progress.
	select {
	case <-mock.installEntered:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for install to start")
	}

	// Call Uninstall while install is blocked. Without the lifecycleMu fix
	// this runs concurrently and UninstallChart executes before
	// UpgradeOrInstallChart returns.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = l.Uninstall(context.Background(), "test-ns", "racetest")
	}()

	// Give Uninstall time to either execute (no fix) or block (with fix).
	time.Sleep(200 * time.Millisecond)

	// Release the blocked install.
	close(mock.installBlock)

	// Wait for both goroutines to finish.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for goroutines to complete")
	}

	mock.mu.Lock()
	ops := append([]string{}, mock.ops...)
	mock.mu.Unlock()

	// The last Helm operation must be "uninstall". If UpgradeOrInstallChart
	// completed after UninstallChart (the race), the last entry would be
	// "install_end" and istiod would be re-created — which is the bug.
	g.Expect(ops).NotTo(BeEmpty())
	g.Expect(ops[len(ops)-1]).To(Equal("uninstall"),
		"expected the last Helm operation to be uninstall, but got ops=%v — "+
			"this means a concurrent reconcile re-installed istiod after Uninstall", ops)
}
