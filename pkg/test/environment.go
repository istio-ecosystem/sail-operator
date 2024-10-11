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

package test

import (
	"io"
	"path"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func SetupEnv(logWriter io.Writer, installCRDs bool) (*envtest.Environment, client.Client, *rest.Config) {
	logf.SetLogger(zap.New(zap.WriteTo(logWriter), zap.UseDevMode(true)))

	var crdDirectoryPaths []string
	if installCRDs {
		crdDirectoryPaths = append(crdDirectoryPaths, path.Join(project.RootDir, "chart", "crds"))
	}

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     crdDirectoryPaths,
		ErrorIfCRDPathMissing: true,
	}

	// disabling mutatingwebhooks to avoid failing calls to the injection webhooks
	// once we implement mutatingwebhooks in the operator we might have to find another way
	testEnv.ControlPlane.GetAPIServer().Configure().Append("disable-admission-plugins", "MutatingAdmissionWebhook")

	cfg, err := testEnv.Start()
	if err != nil || cfg == nil {
		panic(err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}

	return testEnv, k8sClient, cfg
}
