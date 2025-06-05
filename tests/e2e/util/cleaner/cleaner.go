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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cleaner records resources to keep, and cleans up any resources it didn't record.
type Cleaner struct {
	cl         client.Client
	ctx        []string
	namespaces map[string]struct{}
}

// New returns a Cleaner which can record resources to keep, and clean up any resources it didn't record.
// It needs an initialized client, and has optional (string) context which can be used to distinguish it's output.
func New(cl client.Client, ctx ...string) Cleaner {
	return Cleaner{
		cl:         cl,
		ctx:        ctx,
		namespaces: make(map[string]struct{}),
	}
}

// Record will save the state of all resources it wants to keep in the cluster, so they won't be cleaned up.
func (c *Cleaner) Record(ctx context.Context) {
	// Save all namespaces that exist so that we can skip them when cleaning
	namespaceList := &corev1.NamespaceList{}
	Expect(c.cl.List(ctx, namespaceList)).To(Succeed())
	for _, ns := range namespaceList.Items {
		c.namespaces[ns.Name] = struct{}{}
	}
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

func (c *Cleaner) cleanup(ctx context.Context) (deleted []client.Object) {
	// Clean up all namespaces that didn't exist before the tests
	namespaceList := &corev1.NamespaceList{}
	Expect(c.cl.List(ctx, namespaceList)).To(Succeed())
	for _, ns := range namespaceList.Items {
		if mapHasKey(c.namespaces, ns.Name) {
			continue
		}

		By(c.cleaningUpThe(&ns, "namespace"), func() {
			Expect(c.delete(ctx, &ns)).To(Succeed())
			deleted = append(deleted, &ns)
		})
	}

	return
}

func mapHasKey[V any](m map[string]V, k string) bool {
	_, exists := m[k]
	return exists
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
		s = fmt.Sprintf("on %s", s)
	}

	By(fmt.Sprintf("Waiting for resources to be deleted%s", s))
	for _, obj := range deleted {
		Expect(c.waitForDeletion(ctx, obj)).To(Succeed())
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
