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
	"os"
	"strings"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
)

var (
	FipsEnabled        bool
	FipsEnableFilePath = "/proc/sys/crypto/fips_enabled"
)

// detectFipsMode checks if FIPS mode is enabled in the system.
func init() {
	detectFipsMode(FipsEnableFilePath)
}

func detectFipsMode(filepath string) {
	contents, err := os.ReadFile(filepath)
	if err != nil {
		FipsEnabled = false
	} else {
		fipsEnabled := strings.TrimSuffix(string(contents), "\n")
		if fipsEnabled == "1" {
			FipsEnabled = true
		}
	}
}

// ApplyFipsValues sets pilot.env.COMPLIANCE_POLICY if FIPS mode is enabled in the system.
func ApplyFipsValues(values *v1.Values) {
	if !FipsEnabled || values == nil {
		return
	}
	if values.Pilot == nil {
		values.Pilot = &v1.PilotConfig{}
	}
	if values.Pilot.Env == nil {
		values.Pilot.Env = make(map[string]string)
	}
	if _, found := values.Pilot.Env["COMPLIANCE_POLICY"]; !found {
		values.Pilot.Env["COMPLIANCE_POLICY"] = "fips-140-2"
	}
}
