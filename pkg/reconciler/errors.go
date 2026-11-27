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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
)

type ValidationError struct {
	message string
}

func (v *ValidationError) Error() string {
	return "validation error: " + v.message
}

func NewValidationError(message string) error {
	return &ValidationError{message: message}
}

func IsValidationError(err error) bool {
	e := &ValidationError{}
	return errors.As(err, &e)
}

// TransientError is an error returned by a Reconciler that will usually resolve itself when retrying, e.g. some resource not yet reconciled
type TransientError struct {
	message string
}

func (v *TransientError) Error() string {
	return "transient error: " + v.message
}

func NewTransientError(message string) error {
	return &TransientError{message: message}
}

func IsTransientError(err error) bool {
	e := &TransientError{}
	return errors.As(err, &e)
}

type NameAlreadyExistsError struct {
	Message       string
	originalError error
}

func (err NameAlreadyExistsError) Error() string {
	return err.Message
}

func (err NameAlreadyExistsError) Unwrap() error {
	return err.originalError
}

func NewNameAlreadyExistsError(message string, originalError error) NameAlreadyExistsError {
	return NameAlreadyExistsError{
		Message:       message,
		originalError: originalError,
	}
}

func IsNameAlreadyExistsError(err error) bool {
	var e NameAlreadyExistsError
	return errors.As(err, &e)
}

// ReferenceNotFoundError indicates that a referenced resource was not found.
type ReferenceNotFoundError struct {
	Message       string
	originalError error
}

func (err ReferenceNotFoundError) Error() string {
	return err.Message
}

func (err ReferenceNotFoundError) Unwrap() error {
	return err.originalError
}

func NewReferenceNotFoundError(message string, originalError error) ReferenceNotFoundError {
	return ReferenceNotFoundError{
		Message:       message,
		originalError: originalError,
	}
}

func IsReferenceNotFoundError(err error) bool {
	var e ReferenceNotFoundError
	return errors.As(err, &e)
}

// IsRESTMappingError returns true if the error is caused by a missing REST mapping,
// either from client-go's local RESTMapper (NoResourceMatchError, NoKindMatchError) or from
// the API server's GC admission plugin rejecting an owner reference when its RESTMapper
// has not yet synced a newly registered CRD.
func IsRESTMappingError(err error) bool {
	if err == nil {
		return false
	}
	if meta.IsNoMatchError(err) {
		return true
	}
	if apierrors.IsForbidden(err) {
		var statusErr *apierrors.StatusError
		if errors.As(err, &statusErr) && statusErr.ErrStatus.Details != nil {
			for _, cause := range statusErr.ErrStatus.Details.Causes {
				if strings.Contains(cause.Message, "RESTMapping") {
					return true
				}
			}
		}
		return strings.Contains(err.Error(), "RESTMapping")
	}
	return false
}
