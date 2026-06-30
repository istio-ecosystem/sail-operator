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

package config

import (
	"fmt"
	"strings"

	"github.com/magiconair/properties"
)

const imageAnnotationPrefix = "images.v"

// ParseImageDigestsFromAnnotations extracts operand image refs from CSV install
// deployment annotations (keys prefixed with images.v).
func ParseImageDigestsFromAnnotations(ann map[string]string) (map[string]IstioImageConfig, error) {
	var lines []string
	for k, v := range ann {
		if !strings.HasPrefix(k, imageAnnotationPrefix) {
			continue
		}
		if strings.Contains(v, "${") {
			return nil, fmt.Errorf("unresolved placeholder in annotation %s: %s", k, v)
		}
		lines = append(lines, k+"="+v)
	}
	if len(lines) == 0 {
		return map[string]IstioImageConfig{}, nil
	}

	p, err := properties.Load([]byte(strings.Join(lines, "\n")), properties.UTF8)
	if err != nil {
		return nil, err
	}

	var cfg OperatorConfig
	if err := decodePropertiesInto(p, &cfg); err != nil {
		return nil, err
	}
	return cfg.ImageDigests, nil
}

// MergeImageDigests overlays parsed image digests into the global operator config.
func MergeImageDigests(digests map[string]IstioImageConfig) {
	if len(digests) == 0 {
		return
	}
	if Config.ImageDigests == nil {
		Config.ImageDigests = make(map[string]IstioImageConfig, len(digests))
	}
	for k, v := range digests {
		Config.ImageDigests[k] = v
	}
}
