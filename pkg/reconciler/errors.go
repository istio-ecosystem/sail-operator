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

type ValidationError struct {
	message string
}

func (v ValidationError) Error() string {
	return "validation error: " + v.message
}

func NewValidationError(message string) error {
	return &ValidationError{message: message}
}

func IsValidationError(err error) bool {
	e := &ValidationError{}
	return errors.As(err, &e)
}

// TransitoryError is an error returned by a Reconciler that will usually resolve itself when retrying, e.g. some resource not yet reconciled
type TransitoryError struct {
	message string
}

func (v TransitoryError) Error() string {
	return "transitory error: " + v.message
}

func NewTransitoryError(message string) error {
	return &TransitoryError{message: message}
}

func IsTransitoryError(err error) bool {
	e := &TransitoryError{}
	return errors.As(err, &e)
}
