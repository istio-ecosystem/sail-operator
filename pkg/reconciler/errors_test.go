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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestIsRESTMappingError(t *testing.T) {
	assert.False(t, IsRESTMappingError(nil))
	assert.False(t, IsRESTMappingError(errors.New("generic error")))
	assert.False(t, IsRESTMappingError(apierrors.NewForbidden(schema.GroupResource{}, "foo", errors.New("access denied"))))

	// Local client-go NoKindMatchError
	noKindErr := &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "foo", Kind: "Bar"}}
	assert.True(t, IsRESTMappingError(noKindErr))
	assert.True(t, IsRESTMappingError(fmt.Errorf("wrapped: %w", noKindErr)))

	// Local client-go NoResourceMatchError
	noResErr := &meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{Resource: "bars"}}
	assert.True(t, IsRESTMappingError(noResErr))
	assert.True(t, IsRESTMappingError(fmt.Errorf("wrapped: %w", noResErr)))

	// Remote GC admission plugin error (HTTP 403 with RESTMapping in error message)
	gcAdmErr := apierrors.NewForbidden(schema.GroupResource{}, "foo",
		errors.New("cannot set blockOwnerDeletion in this case because cannot find RESTMapping for APIVersion xyz"))
	assert.True(t, IsRESTMappingError(gcAdmErr))
	assert.True(t, IsRESTMappingError(fmt.Errorf("wrapped: %w", gcAdmErr)))

	// Remote GC admission plugin error with status cause details
	gcCauseErr := &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status: metav1.StatusFailure,
			Code:   403,
			Reason: metav1.StatusReasonForbidden,
			Details: &metav1.StatusDetails{
				Causes: []metav1.StatusCause{
					{
						Type:    metav1.CauseTypeUnexpectedServerResponse,
						Message: "cannot find RESTMapping for APIVersion xyz",
					},
				},
			},
		},
	}
	assert.True(t, IsRESTMappingError(gcCauseErr))
	assert.True(t, IsRESTMappingError(fmt.Errorf("wrapped: %w", gcCauseErr)))
}
