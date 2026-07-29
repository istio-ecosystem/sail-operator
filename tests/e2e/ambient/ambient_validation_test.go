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

package ambient

import (
	"fmt"
	"strings"
	"time"

	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ambient API Validation", Label("ambient", "ambient-validation"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(defaultTimeout) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	// Use the latest supported ambient version for these tests
	version := getLatestAmbientVersion()

	var clr cleaner.Cleaner

	BeforeAll(func(ctx SpecContext) {
		clr = cleaner.New(cl)
		clr.Record(ctx)

		Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
		Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
		Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() {
			common.LogDebugInfo(common.Ambient, k)
			if keepOnFailure {
				return
			}
		}
		clr.Cleanup(ctx)
	})

	Context("Single Instance Name Enforcement", func() {
		When("attempting to create IstioCNI with non-default name", func() {
			It("rejects IstioCNI CR with name != 'default'", func(ctx SpecContext) {
				cniYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: custom-cni
spec:
  version: %s
  namespace: %s
  profile: ambient`, version.Name, istioCniNamespace)

				err := k.CreateFromString(cniYAML)
				Expect(err).To(HaveOccurred(), "IstioCNI creation should fail with non-default name")

				// Verify error message indicates validation failure
				errorMsg := err.Error()
				Expect(strings.Contains(errorMsg, "metadata.name") || strings.Contains(errorMsg, "default")).To(BeTrue(),
					"Error message should mention metadata.name or 'default' requirement")
				Success("IstioCNI with custom name correctly rejected")
			})
		})

		When("attempting to create ZTunnel with non-default name", func() {
			It("rejects ZTunnel CR with name != 'default'", func(ctx SpecContext) {
				ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: custom-ztunnel
spec:
  version: %s
  namespace: %s`, version.Name, ztunnelNamespace)

				err := k.CreateFromString(ztunnelYAML)
				Expect(err).To(HaveOccurred(), "ZTunnel creation should fail with non-default name")

				// Verify error message indicates validation failure
				errorMsg := err.Error()
				Expect(strings.Contains(errorMsg, "metadata.name") || strings.Contains(errorMsg, "default")).To(BeTrue(),
					"Error message should mention metadata.name or 'default' requirement")
				Success("ZTunnel with custom name correctly rejected")
			})
		})

		When("creating IstioCNI and ZTunnel with metadata.name='default'", func() {
			It("successfully creates IstioCNI with name 'default'", func(ctx SpecContext) {
				cniYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  profile: ambient`, version.Name, istioCniNamespace)

				Expect(k.CreateFromString(cniYAML)).To(Succeed())
				Success("IstioCNI with name='default' created successfully")
			})

			It("successfully creates ZTunnel with name 'default'", func(ctx SpecContext) {
				ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s`, version.Name, ztunnelNamespace)

				Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())
				Success("ZTunnel with name='default' created successfully")
			})
		})
	})
})
