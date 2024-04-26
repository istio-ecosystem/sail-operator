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

package project

import (
	"path/filepath"
	"runtime"
)

// RootDir is the path to the project's root directory
var RootDir string

func init() {
	_, b, _, _ := runtime.Caller(0)
	// This relies on the fact this file is 3 levels down from the root; if this changes, adjust the path below.
	RootDir = filepath.Join(filepath.Dir(b), "../../../")
}
