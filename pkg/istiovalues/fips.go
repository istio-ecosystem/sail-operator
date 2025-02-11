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

package istiovalues

import (
	"fmt"
	"os"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

var FipsEnableFilePath = "/proc/sys/crypto/fips_enabled"

// ApplyFipsValues: Detect FIPS enabled mode on OpenShift and set value pilot.env.COMPLIANCE_POLICY
// To avoid changing the host machine FIPS_ENABLED_FILE_PATH configuration,
// set testMode to true when running unit tests.
func ApplyFipsValues(values helm.Values) (helm.Values, error) {
	readline, err := os.ReadFile(FipsEnableFilePath)
	contents := strings.TrimSuffix(string(readline), "\n")
	if err == nil && contents == "1" {
		if err = values.SetIfAbsent("pilot.env.COMPLIANCE_POLICY", "fips-140-2"); err != nil {
			return nil, fmt.Errorf("failed to set pilot.env.COMPLIANCE_POLICY: %w", err)
		}
	}
	return values, nil
}
