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
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"sync"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	managedByValue = "sail-library"
	helmDriver     = "secret"

	defaultQPS   float32 = 50
	defaultBurst int     = 100
)

// OverwriteOLMManagedCRDFunc is called when a CRD is detected with OLM
// ownership labels. If provided and returns true, the library takes
// ownership of the CRD. If nil or returns false, OLM-labeled CRDs are
// left alone.
type OverwriteOLMManagedCRDFunc func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) bool

// LibraryOption configures optional Library parameters.
type LibraryOption func(*libraryOptions)

type libraryOptions struct {
	qps   float32
	burst int
}

// WithQPS sets the maximum sustained queries-per-second to the API server.
func WithQPS(qps float32) LibraryOption {
	return func(o *libraryOptions) { o.qps = qps }
}

// WithBurst sets the maximum burst of requests to the API server.
func WithBurst(burst int) LibraryOption {
	return func(o *libraryOptions) { o.burst = burst }
}

// Options specifies the desired state for the istiod installation.
type Options struct {
	Namespace      string
	Version        string
	Revision       string
	Values         *v1.Values
	ManageCRDs     bool
	IncludeAllCRDs bool

	// OverwriteOLMManagedCRD is called when a CRD is detected with OLM
	// ownership labels. Skipped by optionsEqual since function values
	// are not comparable.
	OverwriteOLMManagedCRD OverwriteOLMManagedCRDFunc
}

// optionsEqual compares two Options for equality, skipping the
// OverwriteOLMManagedCRD function field. Used by Apply to suppress
// no-op reconciliation triggers.
func optionsEqual(a, b Options) bool {
	if a.Namespace != b.Namespace ||
		a.Version != b.Version ||
		a.Revision != b.Revision ||
		a.ManageCRDs != b.ManageCRDs ||
		a.IncludeAllCRDs != b.IncludeAllCRDs {
		return false
	}
	return reflect.DeepEqual(helm.FromValues(a.Values), helm.FromValues(b.Values))
}

// CRDManagementState represents the state of CRD management.
type CRDManagementState string

const (
	CRDManagementStateUnknown  CRDManagementState = "Unknown"
	CRDManagementStateReady    CRDManagementState = "Ready"
	CRDManagementStateNotReady CRDManagementState = "NotReady"
	CRDManagementStateError    CRDManagementState = "Error"
)

// CRDInfo contains information about a managed CRD.
type CRDInfo struct {
	Name    string
	Managed bool
	Ready   bool
}

// Status contains the current state of the installation.
type Status struct {
	Generation uint64
	CRDState   CRDManagementState
	CRDMessage string
	CRDs       []CRDInfo
	Installed  bool
	Version    string
	Error      error
}

// Library manages the lifecycle of an istiod installation without
// requiring the full Sail Operator to be deployed.
type Library struct {
	mu       sync.Mutex
	statusMu sync.RWMutex

	kubeConfig   *rest.Config
	resourceFS   fs.FS
	crdFS        fs.FS
	cl           client.Client
	chartManager *helm.ChartManager
	platform     config.Platform
	tlsConfig    *config.TLSConfig

	triggerCh chan event.GenericEvent
	notifyCh  chan struct{}
	cancel    context.CancelFunc
	done      chan struct{}

	generation    uint64
	desiredOpts   *Options
	currentStatus Status
}

// ValidateOptions checks that the provided options are valid.
func ValidateOptions(opts Options) error {
	if opts.Namespace == "" {
		return fmt.Errorf("namespace must not be empty")
	}
	return istioversion.ValidateVersion(opts.Version)
}

// New creates a new Library instance. The resourceFS should contain Helm chart
// resources (typically from resources/ directory or FromDirectory()).
// The crdFS should contain CRD YAML files (typically from chart/crds/).
func New(kubeConfig *rest.Config, resourceFS, crdFS fs.FS, opts ...LibraryOption) (*Library, error) {
	o := libraryOptions{qps: defaultQPS, burst: defaultBurst}
	for _, fn := range opts {
		fn(&o)
	}

	cfg := rest.CopyConfig(kubeConfig)
	cfg.QPS = o.qps
	cfg.Burst = o.burst

	chartManager := helm.NewChartManager(cfg, helmDriver, helm.WithManagedByValue(managedByValue))

	cl, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	setupLog := ctrl.Log.WithName("install-library")

	platform, err := config.DetectPlatform(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to detect platform: %w", err)
	}
	setupLog.Info("detected platform", "platform", platform)

	var tlsConfig *config.TLSConfig
	if platform == config.PlatformOpenShift {
		tlsConfig, err = config.NewTLSConfigForOpenShift(context.Background(), setupLog, cl)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch TLS config: %w", err)
		}
	}

	return &Library{
		kubeConfig:   cfg,
		resourceFS:   resourceFS,
		crdFS:        crdFS,
		cl:           cl,
		chartManager: chartManager,
		platform:     platform,
		tlsConfig:    tlsConfig,
		triggerCh:    make(chan event.GenericEvent, 1),
		notifyCh:     make(chan struct{}, 1),
	}, nil
}

// FromDirectory creates an fs.FS from a local directory path.
func FromDirectory(path string) fs.FS {
	return os.DirFS(path)
}

// Apply validates the desired installation state and triggers reconciliation.
// If the options are identical to the previously applied options, this is a
// no-op — no new reconciliation is triggered. Use Enqueue to force a
// reconciliation with the current options (e.g. after drift detection).
func (l *Library) Apply(opts Options) error {
	if err := ValidateOptions(opts); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}
	l.mu.Lock()
	if l.desiredOpts != nil && optionsEqual(*l.desiredOpts, opts) {
		l.mu.Unlock()
		return nil
	}
	l.generation++
	l.desiredOpts = &opts
	l.mu.Unlock()
	l.sendTrigger()
	return nil
}

// Stop cancels the reconciliation loop and waits for the manager to shut down.
func (l *Library) Stop() {
	l.mu.Lock()
	cancel := l.cancel
	done := l.done
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// Enqueue triggers a reconciliation of the current desired state.
func (l *Library) Enqueue() {
	l.sendTrigger()
}

// Status returns the current installation status.
func (l *Library) Status() Status {
	l.statusMu.RLock()
	defer l.statusMu.RUnlock()
	return l.currentStatus
}

// Uninstall removes the istiod Helm release.
func (l *Library) Uninstall(ctx context.Context, namespace, revision string) error {
	l.mu.Lock()
	inst := l.newInstaller(namespace)
	l.mu.Unlock()
	return inst.uninstall(ctx, namespace, revision)
}
