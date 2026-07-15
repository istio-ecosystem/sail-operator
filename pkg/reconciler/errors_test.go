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

package reconciler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("my message")
	assert.NotNil(t, err)
	assert.Equal(t, "validation error: my message", err.Error())
	assert.IsTypef(t, &ValidationError{}, err, "expected NewValidationError to return a ValidationError")
}

func TestIsValidationError(t *testing.T) {
	err := &ValidationError{message: "error"}
	assert.True(t, IsValidationError(err), "expected IsValidationError to return true for a ValidationError")

	wrappedError := fmt.Errorf("wrapped error: %w", err)
	assert.True(t, IsValidationError(wrappedError), "expected IsValidationError to return true for a wrapped ValidationError")
}

func TestNewTransientError(t *testing.T) {
	err := NewTransientError("my message")
	assert.NotNil(t, err)
	assert.Equal(t, "transient error: my message", err.Error())
	assert.IsTypef(t, &TransientError{}, err, "expected NewTransientError to return a TransientError")
}

func TestIsTransientError(t *testing.T) {
	err := &TransientError{message: "error"}
	assert.True(t, IsTransientError(err), "expected IsTransientError to return true for a TransientError")

	wrappedError := fmt.Errorf("wrapped error: %w", err)
	assert.True(t, IsTransientError(wrappedError), "expected IsTransientError to return true for a wrapped TransientError")
}
