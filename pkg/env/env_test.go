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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	key := "ENV_TEST_TEST_GET"
	value := "test value"
	assert.NoError(t, os.Setenv(key, value))

	tests := []struct {
		name         string
		key          string
		defaultValue string
		want         string
	}{
		{
			name:         "empty-key",
			key:          "",
			defaultValue: "my default",
			want:         "my default",
		},
		{
			name:         "missing-env-var",
			key:          "NONEXISTENT_ENV_VAR",
			defaultValue: "my default 2",
			want:         "my default 2",
		},
		{
			name:         "env-var-exists",
			key:          key,
			defaultValue: "",
			want:         value,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Get(tt.key, tt.defaultValue); got != tt.want {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	boolKey := "ENV_TEST_TEST_GET_BOOL_VALID"
	boolValue := "true"
	assert.NoError(t, os.Setenv(boolKey, boolValue))

	notBoolKey := "ENV_TEST_TEST_GET_BOOL_INVALID"
	notBoolValue := "not a bool"
	assert.NoError(t, os.Setenv(notBoolKey, notBoolValue))

	tests := []struct {
		name         string
		key          string
		defaultValue bool
		want         bool
	}{
		{
			name:         "empty-key",
			key:          "",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "missing-env-var",
			key:          "NONEXISTENT_ENV_VAR",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "missing-env-var2",
			key:          "NONEXISTENT_ENV_VAR",
			defaultValue: false,
			want:         false,
		},
		{
			name:         "bool-value",
			key:          boolKey,
			defaultValue: false,
			want:         true,
		},
		{
			name:         "non-bool-value-returns-default",
			key:          notBoolKey,
			defaultValue: true,
			want:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBool(tt.key, tt.defaultValue)
			assert.Equalf(t, tt.want, got, "GetBool(%v, %v)", tt.key, tt.defaultValue)
		})
	}
}
