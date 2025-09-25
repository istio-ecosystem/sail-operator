//go:build e2e

// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cleaner

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cleaner records resources to keep, and cleans up any resources it didn't record.
type Cleaner struct {
	cl        client.Client
	ctx       []string
	recorded  bool
	resources map[resource]bool
	crds      map[string]struct{}
	crs       map[string]map[resource]struct{}
}

type resource struct {
	kind      string
	name      string
	namespace string
}

// New returns a Cleaner which can record resources to keep, and clean up any resources it didn't record.
// It needs an initialized client, and has optional (string) context which can be used to distinguish it's output.
func New(cl client.Client, ctx ...string) Cleaner {
	return Cleaner{
		cl:        cl,
		ctx:       ctx,
		resources: make(map[resource]bool),
		crds:      make(map[string]struct{}),
		crs:       make(map[string]map[resource]struct{}),
	}
}

// Record will save the state of all resources it wants to keep in the cluster, so they won't be cleaned up.
func (c *Cleaner) Record(ctx context.Context) {
	c.recorded = true

	// Save all namespaces that exist so that we can skip them when cleaning
	namespaceList := &corev1.NamespaceList{}
	Expect(c.cl.List(ctx, namespaceList)).To(Succeed())
	for _, ns := range namespaceList.Items {
		c.resources[c.resourceFromObj(&ns, ns.Name)] = true
	}

	crList := &rbacv1.ClusterRoleList{}
	Expect(c.cl.List(ctx, crList)).To(Succeed())
	for _, cr := range crList.Items {
		c.resources[c.resourceFromObj(&cr, cr.Name)] = true
	}

	crbList := &rbacv1.ClusterRoleBindingList{}
	Expect(c.cl.List(ctx, crbList)).To(Succeed())
	for _, crb := range crbList.Items {
		c.resources[c.resourceFromObj(&crb, crb.Name)] = true
	}

	allCRDs := &apiextensionsv1.CustomResourceDefinitionList{}
	Expect(c.cl.List(ctx, allCRDs)).To(Succeed())

	// Save all existing custom resources so we can skip them when cleaning
	for _, crd := range allCRDs.Items {
		if !c.trackedCRD(&crd) {
			continue
		}

		c.crds[crd.Name] = struct{}{}
		c.crs[crd.Name] = make(map[resource]struct{})
		customResources := &unstructured.UnstructuredList{}
		customResources.SetGroupVersionKind(extractGVK(&crd))

		Expect(c.cl.List(ctx, customResources)).To(Succeed())
		for _, cr := range customResources.Items {
			c.crs[crd.Name][crKey(cr)] = struct{}{}
		}
	}
}

func (c *Cleaner) resourceFromObj(obj client.Object, name string) resource {
	kinds, _, _ := c.cl.Scheme().ObjectKinds(obj)
	return resource{kind: kinds[0].Kind, name: name}
}

func extractGVK(crd *apiextensionsv1.CustomResourceDefinition) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crd.Spec.Versions[0].Name,
		Kind:    crd.Spec.Names.Kind,
	}
}

func crKey(cr unstructured.Unstructured) resource {
	return resource{name: cr.GetName(), namespace: cr.GetNamespace()}
}

// CleanupNoWait will cleanup any resources not recorded, while not waiting for their deletion to complete.
// Use Cleaner.WaitForDeletion to wait for their deletion at a later point.
func (c *Cleaner) CleanupNoWait(ctx context.Context) (deleted []client.Object) {
	return c.cleanup(ctx)
}

// Cleanup will cleanup any resources not recorded, and wait for their deletion.
func (c *Cleaner) Cleanup(ctx context.Context) {
	c.WaitForDeletion(ctx, c.cleanup(ctx))
}

type resObj struct {
	key resource
	obj client.Object
}

func (c *Cleaner) cleanup(ctx context.Context) (deleted []client.Object) {
	if !c.recorded {
		Fail("Running the Cleaner without recording first is unsafe and will lead to problems.")
	}

	allCRDs := &apiextensionsv1.CustomResourceDefinitionList{}
	Expect(c.cl.List(ctx, allCRDs)).To(Succeed())

	// First, clean up all custom resources that didn't exist, to give the operator a shot at finalizing them
	for _, crd := range allCRDs.Items {
		if !c.trackedCRD(&crd) {
			continue
		}

		gvk := extractGVK(&crd)
		customResources := &unstructured.UnstructuredList{}
		customResources.SetGroupVersionKind(gvk)

		Expect(c.cl.List(ctx, customResources)).To(Succeed())
		for _, cr := range customResources.Items {
			// Skip any recorded custom resource.
			if mapHasKey(c.crs, crd.Name) && mapHasKey(c.crs[crd.Name], crKey(cr)) {
				continue
			}

			By(c.cleaningUpThe(&cr, gvk.Kind), func() {
				Expect(c.delete(ctx, &cr)).To(Succeed())
				deleted = append(deleted, &cr)
			})
		}
	}

	// At this point we have to wait for cleanup to finish, since some CRs might get stuck "finalizing" if we later delete the operator namespace.
	c.WaitForDeletion(ctx, deleted)
	deleted = make([]client.Object, 0)

	// Clean up any resources we didn't record.
	var resources []resObj
	namespaceList := &corev1.NamespaceList{}
	Expect(c.cl.List(ctx, namespaceList)).To(Succeed())
	for _, ns := range namespaceList.Items {
		resources = append(resources, resObj{c.resourceFromObj(&ns, ns.Name), &ns})
	}

	crList := &rbacv1.ClusterRoleList{}
	Expect(c.cl.List(ctx, crList)).To(Succeed())
	for _, cr := range crList.Items {
		resources = append(resources, resObj{c.resourceFromObj(&cr, cr.Name), &cr})
	}

	crbList := &rbacv1.ClusterRoleBindingList{}
	Expect(c.cl.List(ctx, crbList)).To(Succeed())
	for _, crb := range crbList.Items {
		resources = append(resources, resObj{c.resourceFromObj(&crb, crb.Name), &crb})
	}

	// Clean up all resources that didn't exist before the tests
	for _, res := range resources {
		// Skip any resource we recorded previously.
		// Also, skip any OLM managed resource as it'll be recreated upon deletion.
		if mapHasKey(c.resources, res.key) ||
			mapHasKey(res.obj.GetLabels(), "olm.managed") {
			continue
		}

		By(c.cleaningUpThe(res.obj, res.key.kind), func() {
			Expect(c.delete(ctx, res.obj)).To(Succeed())
			deleted = append(deleted, res.obj)
		})
	}

	// Finally, remove any CRD that didn't exist at time of recording.
	for _, crd := range allCRDs.Items {
		// Skip any recorded CRD, and any CRD we don't track.
		if mapHasKey(c.crds, crd.Name) || !c.trackedCRD(&crd) {
			continue
		}

		By(c.cleaningUpThe(&crd, "CRD"), func() {
			Expect(c.delete(ctx, &crd)).To(Succeed())
			deleted = append(deleted, &crd)
		})
	}

	return deleted
}

func mapHasKey[K comparable, V any](m map[K]V, k K) bool {
	_, exists := m[k]
	return exists
}

func (c *Cleaner) trackedCRD(crd *apiextensionsv1.CustomResourceDefinition) bool {
	return crd.Spec.Group == "sailoperator.io" || strings.HasSuffix(crd.Spec.Group, "istio.io")
}

func (c *Cleaner) delete(ctx context.Context, obj client.Object) error {
	if err := c.cl.Delete(ctx, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	return nil
}

func (c *Cleaner) cleaningUpThe(obj client.Object, kind string) (s string) {
	ctx := []string{}
	ns := obj.GetNamespace()
	if ns != "" {
		ctx = append(ctx, fmt.Sprintf("namespace=%s", ns))
	}

	ctx = append(ctx, c.ctx...)
	s = strings.Join(ctx, ", ")
	if s != "" {
		s = fmt.Sprintf(" on %s", s)
	}

	return fmt.Sprintf("Cleaning up the %s %s%s", obj.GetName(), kind, s)
}

// WaitForDeletion receives a slice of resources marked for deletion, and waits until they're all delete.
// It will fail the test suite if any resource hasn't been deleted in sufficient time.
func (c *Cleaner) WaitForDeletion(ctx context.Context, deleted []client.Object) {
	if len(deleted) == 0 {
		return
	}

	s := strings.Join(c.ctx, ", ")
	if s != "" {
		s = fmt.Sprintf(" on %s", s)
	}

	By(fmt.Sprintf("Waiting for resources to be deleted%s", s))
	for _, obj := range deleted {
		Expect(c.waitForDeletion(ctx, obj)).To(Succeed(),
			fmt.Sprintf("Failed while waiting for %s to delete", obj.GetName()))
	}

	Success(fmt.Sprintf("Finished cleaning up resources%s", s))
}

func (c *Cleaner) waitForDeletion(ctx context.Context, obj client.Object) error {
	objKey := client.ObjectKeyFromObject(obj)

	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		gotObj := obj.DeepCopyObject().(client.Object)

		if err := c.cl.Get(ctx, objKey, gotObj); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}

			return false, err
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}
