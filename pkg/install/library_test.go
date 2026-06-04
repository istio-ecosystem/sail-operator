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
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"istio.io/istio/pkg/ptr"
)

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
