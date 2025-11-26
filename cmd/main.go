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

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/istio-ecosystem/sail-operator/controllers/istio"
	"github.com/istio-ecosystem/sail-operator/controllers/istiocni"
	"github.com/istio-ecosystem/sail-operator/controllers/istiorevision"
	"github.com/istio-ecosystem/sail-operator/controllers/istiorevisiontag"
	"github.com/istio-ecosystem/sail-operator/controllers/webhook"
	"github.com/istio-ecosystem/sail-operator/controllers/ztunnel"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/version"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var setupLog = ctrl.Log.WithName("setup")

func main() {
	var metricsAddr string
	var probeAddr string
	var configFile string
	var logAPIRequests bool
	var printVersion bool
	var leaderElectionEnabled bool
	var reconcilerCfg config.ReconcilerConfig

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&configFile, "config-file", "/etc/sail-operator/config.properties", "Location of the config file, propagated by k8s downward APIs")
	flag.StringVar(&reconcilerCfg.ResourceDirectory, "resource-directory", "/var/lib/sail-operator/resources", "Where to find resources (e.g. charts)")
	flag.IntVar(&reconcilerCfg.MaxConcurrentReconciles, "max-concurrent-reconciles", 1,
		"MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run.")
	flag.BoolVar(&logAPIRequests, "log-api-requests", false, "Whether to log each request sent to the Kubernetes API server")
	flag.BoolVar(&printVersion, "version", printVersion, "Prints version information and exits")
	flag.BoolVar(&leaderElectionEnabled, "leader-elect", true,
		"Enable leader election for this operator. Enabling this will ensure there is only one active controller manager.")

	flag.BoolVar(&enqueuelogger.LogEnqueueEvents, "log-enqueue-events", false, "Whether to log events that cause an object to be enqueued for reconciliation")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if printVersion || flag.NArg() > 0 && flag.Arg(0) == "version" {
		fmt.Println(version.Info)
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	reconcilerCfg.OperatorNamespace = os.Getenv("POD_NAMESPACE")
	if reconcilerCfg.OperatorNamespace == "" {
		contents, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		reconcilerCfg.OperatorNamespace = string(contents)
		if err != nil || reconcilerCfg.OperatorNamespace == "" {
			setupLog.Error(err, "can't determine namespace this operator is running in; if running outside of a pod, please set the POD_NAMESPACE environment variable")
			os.Exit(1)
		}
	}

	setupLog.Info(version.Info.String())
	setupLog.Info("reading config")
	err := config.Read(configFile)
	if err != nil {
		setupLog.Error(err, "unable to read config file at "+configFile)
		os.Exit(1)
	}
	setupLog.Info("config loaded", "config", config.Config)

	cfg := ctrl.GetConfigOrDie()
	if logAPIRequests {
		cfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return requestLogger{rt: rt}
		})
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){
		// disable http/2 because of https://github.com/kubernetes/kubernetes/issues/121197
		disableHTTP2,
	}

	metricsServerOptions := metricsserver.Options{
		BindAddress:    metricsAddr,
		SecureServing:  true,
		FilterProvider: filters.WithAuthenticationAndAuthorization,
		TLSOpts:        tlsOpts,
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme.Scheme,
		Metrics:                 metricsServerOptions,
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          leaderElectionEnabled,
		LeaderElectionID:        "sail-operator-lock",
		LeaderElectionNamespace: reconcilerCfg.OperatorNamespace,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start sail-operator manager")
		os.Exit(1)
	}

	chartManager := helm.NewChartManager(mgr.GetConfig(), os.Getenv("HELM_DRIVER"))

	reconcilerCfg.Platform, err = config.DetectPlatform(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to detect platform")
		os.Exit(1)
	}

	if reconcilerCfg.Platform == config.PlatformOpenShift {
		reconcilerCfg.DefaultProfile = "openshift"
	} else {
		reconcilerCfg.DefaultProfile = "default"
	}

	err = istio.NewReconciler(reconcilerCfg, mgr.GetClient(), mgr.GetScheme()).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Istio")
		os.Exit(1)
	}

	err = istiorevision.NewReconciler(reconcilerCfg, mgr.GetClient(), mgr.GetScheme(), chartManager).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IstioRevision")
		os.Exit(1)
	}

	err = istiorevisiontag.NewReconciler(reconcilerCfg, mgr.GetClient(), mgr.GetScheme(), chartManager).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IstioRevisionTag")
		os.Exit(1)
	}

	err = istiocni.NewReconciler(reconcilerCfg, mgr.GetClient(), mgr.GetScheme(), chartManager).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IstioCNI")
		os.Exit(1)
	}

	err = ztunnel.NewReconciler(reconcilerCfg, mgr.GetClient(), mgr.GetScheme(), chartManager).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ZTunnel")
		os.Exit(1)
	}

	err = webhook.NewReconciler(reconcilerCfg, mgr.GetClient(), mgr.GetScheme()).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Webhook")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting sail-operator manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running sail-operator manager")
		os.Exit(1)
	}
}

type requestLogger struct {
	rt http.RoundTripper
}

func (rl requestLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	log := logf.FromContext(req.Context())
	log.Info("Performing API request", "method", req.Method, "URL", req.URL)
	return rl.rt.RoundTrip(req)
}

var _ http.RoundTripper = requestLogger{}
