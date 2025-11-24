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

package errors

import "errors"

// SailOperatorError generics struct for different types of controllers errors
type SailOperatorError[T any] struct {
	Message       string
	originalError error
}

// Error implements an Error interface
func (e *SailOperatorError[T]) Error() string {
	var errorType T
	switch any(errorType).(type) {
	case ValidationError:
		return "validation error: " + e.Message
	case TransientError:
		return "transient error: " + e.Message
	default:
		return e.Message
	}
}

// Unwrap Implement the Unwrap convention.
func (e *SailOperatorError[T]) Unwrap() error {
	return e.originalError
}

// NewSailOperatorError creates a new contextual high level error and low-level wrapped error for the operator.
// The generic type T represents the error type.
func NewSailOperatorError[T any](message string, originalError error) *SailOperatorError[T] {
	return &SailOperatorError[T]{message, originalError}
}

// IsAs is a generic helper function that checks if an error in the error chain
// matches the specified error type T. It combines the functionality of errors.As
// with Go generics to provide type-safe error checking without requiring explicit
// type assertions.
func isAs[T error](err error) bool {
	var target T
	return errors.As(err, &target)
}

type ValidationError struct{}

func NewValidationError(message string, originalError ...error) *SailOperatorError[ValidationError] {
	return NewSailOperatorError[ValidationError](message, getFirstError(originalError))
}

func IsValidation(err error) bool {
	return isAs[*SailOperatorError[ValidationError]](err)
}

// TransientError is an error returned by a Reconciler that will usually resolve itself when retrying, e.g. some resource not yet reconciled
type TransientError struct{}

func NewTransientError(message string, originalError ...error) *SailOperatorError[TransientError] {
	return NewSailOperatorError[TransientError](message, getFirstError(originalError))
}

func IsTransient(err error) bool {
	return isAs[*SailOperatorError[TransientError]](err)
}

type NameAlreadyExistsError struct{}

func NewNameAlreadyExistsError(message string, originalError ...error) *SailOperatorError[NameAlreadyExistsError] {
	return NewSailOperatorError[NameAlreadyExistsError](message, getFirstError(originalError))
}

func IsNameAlreadyExists(err error) bool {
	return isAs[*SailOperatorError[NameAlreadyExistsError]](err)
}

type ReferenceNotFoundError struct{}

func NewReferenceNotFoundError(message string, originalError ...error) *SailOperatorError[ReferenceNotFoundError] {
	return NewSailOperatorError[ReferenceNotFoundError](message, getFirstError(originalError))
}

func IsReferenceNotFound(err error) bool {
	return isAs[*SailOperatorError[ReferenceNotFoundError]](err)
}

type WebHookProbeError struct{}

func NewWebHookProbeError(message string, originalError ...error) *SailOperatorError[WebHookProbeError] {
	return NewSailOperatorError[WebHookProbeError](message, getFirstError(originalError))
}

func IsWebHookProbe(err error) bool {
	return isAs[*SailOperatorError[WebHookProbeError]](err)
}

// getFirstError will return a first error from errors slice if exists
func getFirstError(errors []error) error {
	var originalErr error
	if len(errors) > 0 {
		originalErr = errors[0]
	}
	return originalErr
}
