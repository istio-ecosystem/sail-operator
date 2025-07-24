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
func NewSailOperatorError[T any](message string, origErr error) *SailOperatorError[T] {
	return &SailOperatorError[T]{message, origErr}
}

func IsAs[T error](err error) bool {
	var target T
	return errors.As(err, &target)
}

type ValidationError struct{}

// TransientError is an error returned by a Reconciler that will usually resolve itself when retrying, e.g. some resource not yet reconciled
type TransientError struct{}

type NameAlreadyExistsError struct{}

type ReferenceNotFoundError struct{}

type WebHookProbeError struct{}
