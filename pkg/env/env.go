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

package env

import (
	"os"
	"strconv"
)

func Get(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetBool(key string, defaultValue bool) bool {
	value := Get(key, strconv.FormatBool(defaultValue))
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}
