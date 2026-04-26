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

package install

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"strings"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	olmManagedLabel = "operators.coreos.com/managed-by"
)

type crdOwnership int

const (
	crdUnmanaged crdOwnership = iota
	crdManagedByOLM
	crdManagedByLibrary
)

type crdManager struct {
	cl    client.Client
	crdFS fs.FS
}

// Reconcile installs or updates CRDs based on the provided options.
func (m *crdManager) Reconcile(ctx context.Context, opts Options) ([]CRDInfo, error) {
	if !opts.ManageCRDs {
		return nil, nil
	}

	crds, err := m.loadCRDs(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load CRDs: %w", err)
	}

	var infos []CRDInfo
	for _, crd := range crds {
		info, err := m.applyCRD(ctx, crd, opts.OverwriteOLMManagedCRD)
		if err != nil {
			return infos, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (m *crdManager) applyCRD(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, overwriteOLM OverwriteOLMManagedCRDFunc) (CRDInfo, error) {
	name := crd.GetName()
	info := CRDInfo{Name: name, Managed: true}

	existing := &apiextensionsv1.CustomResourceDefinition{}
	err := m.cl.Get(ctx, types.NamespacedName{Name: name}, existing)
	if apierrors.IsNotFound(err) {
		setManagedByLabel(crd)
		if err := m.cl.Create(ctx, crd); err != nil {
			return info, fmt.Errorf("failed to create CRD %s: %w", name, err)
		}
		return info, nil
	}
	if err != nil {
		return info, fmt.Errorf("failed to get CRD %s: %w", name, err)
	}

	ownership := m.classifyCRD(existing)
	if ownership == crdManagedByOLM {
		if overwriteOLM == nil || !overwriteOLM(ctx, existing) {
			info.Managed = false
			info.Ready = m.isCRDReady(existing)
			return info, nil
		}
	}

	crd.SetResourceVersion(existing.ResourceVersion)
	mergedLabels := maps.Clone(existing.Labels)
	maps.Copy(mergedLabels, crd.GetLabels())
	crd.SetLabels(mergedLabels)
	setManagedByLabel(crd)
	mergedAnnotations := maps.Clone(existing.Annotations)
	maps.Copy(mergedAnnotations, crd.GetAnnotations())
	crd.SetAnnotations(mergedAnnotations)
	if err := m.cl.Update(ctx, crd); err != nil {
		return info, fmt.Errorf("failed to update CRD %s: %w", name, err)
	}
	info.Ready = m.isCRDReady(existing)
	return info, nil
}

func setManagedByLabel(crd *apiextensionsv1.CustomResourceDefinition) {
	labels := crd.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["app.kubernetes.io/managed-by"] = managedByValue
	crd.SetLabels(labels)
}

func (m *crdManager) classifyCRD(crd *apiextensionsv1.CustomResourceDefinition) crdOwnership {
	if _, ok := crd.Labels[olmManagedLabel]; ok {
		return crdManagedByOLM
	}
	if crd.Labels != nil && crd.Labels["app.kubernetes.io/managed-by"] == managedByValue {
		return crdManagedByLibrary
	}
	return crdUnmanaged
}

func (m *crdManager) isCRDReady(crd *apiextensionsv1.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
			return true
		}
	}
	return false
}

func (m *crdManager) loadCRDs(opts Options) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	targetKinds := targetCRDKinds(opts.IncludeAllCRDs, opts.Values)

	var crds []*apiextensionsv1.CustomResourceDefinition
	err := fs.WalkDir(m.crdFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return err
		}
		f, err := m.crdFS.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open CRD file %s: %w", path, err)
		}
		defer f.Close()

		decoder := utilyaml.NewYAMLOrJSONDecoder(f, 4096)
		for {
			var crd apiextensionsv1.CustomResourceDefinition
			if err := decoder.Decode(&crd); err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("failed to decode CRD from %s: %w", path, err)
			}
			if crd.Kind != "CustomResourceDefinition" {
				continue
			}
			if opts.IncludeAllCRDs || matchesCRDFilter(&crd, targetKinds) {
				crds = append(crds, &crd)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return crds, nil
}

// AggregateState returns the overall CRD management state from individual CRD infos.
func AggregateState(infos []CRDInfo) (CRDManagementState, string) {
	if len(infos) == 0 {
		return CRDManagementStateUnknown, "no CRDs to manage"
	}
	allReady := true
	for _, info := range infos {
		if !info.Ready {
			allReady = false
			break
		}
	}
	if allReady {
		return CRDManagementStateReady, ""
	}
	return CRDManagementStateNotReady, "not all CRDs are ready"
}

func matchesCRDFilter(crd *apiextensionsv1.CustomResourceDefinition, targetKinds map[string]bool) bool {
	if len(targetKinds) == 0 {
		return true
	}
	key := strings.ToLower(crd.Spec.Names.Kind + "." + crd.Spec.Group)
	return targetKinds[key]
}

func targetCRDKinds(includeAll bool, values *v1.Values) map[string]bool {
	if includeAll || values == nil || values.Pilot == nil {
		return nil
	}

	kinds := map[string]bool{}

	if include, ok := values.Pilot.Env["PILOT_INCLUDE_RESOURCES"]; ok {
		for _, r := range strings.Split(include, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				kinds[strings.ToLower(r)] = true
			}
		}
	}

	return kinds
}
