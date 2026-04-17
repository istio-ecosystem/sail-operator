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

package interceptors

import (
	"context"
	"runtime/debug"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// FailGet returns interceptor.Funcs that makes Get return err for all object types.
func FailGet(err error) interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return err
		},
	}
}

// FailGetFor returns interceptor.Funcs that makes Get return err only when the
// target object matches type T. For all other types, it passes through (returns nil).
func FailGetFor[T client.Object](err error) interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if _, ok := obj.(T); ok {
				return err
			}
			return nil
		},
	}
}

// FailList returns interceptor.Funcs that makes List return err for all list types.
func FailList(err error) interceptor.Funcs {
	return interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
			return err
		},
	}
}

// FailListFor returns interceptor.Funcs that makes List return err only when the
// target list matches type T. For all other types, it passes through (returns nil).
func FailListFor[T client.ObjectList](err error) interceptor.Funcs {
	return interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
			if _, ok := list.(T); ok {
				return err
			}
			return nil
		},
	}
}

// FailCreate returns interceptor.Funcs that makes Create return err.
func FailCreate(err error) interceptor.Funcs {
	return interceptor.Funcs{
		Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
			return err
		},
	}
}

// FailUpdate returns interceptor.Funcs that makes Update return err.
func FailUpdate(err error) interceptor.Funcs {
	return interceptor.Funcs{
		Update: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.UpdateOption) error {
			return err
		},
	}
}

// FailPatch returns interceptor.Funcs that makes Patch return err.
func FailPatch(err error) interceptor.Funcs {
	return interceptor.Funcs{
		Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
			return err
		},
	}
}

// FailDelete returns interceptor.Funcs that makes Delete return err.
func FailDelete(err error) interceptor.Funcs {
	return interceptor.Funcs{
		Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
			return err
		},
	}
}

// FailSubResourcePatch returns interceptor.Funcs that makes SubResourcePatch return err.
func FailSubResourcePatch(err error) interceptor.Funcs {
	return interceptor.Funcs{
		SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
			return err
		},
	}
}

// NoWrites returns interceptor.Funcs that calls t.Fatal on any write operation.
// Use this to assert that a code path does not modify any resources.
func NoWrites(t *testing.T) interceptor.Funcs {
	t.Helper()
	return interceptor.Funcs{
		Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
			t.Fatal("unexpected call to Create in", string(debug.Stack()))
			return nil
		},
		Update: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.UpdateOption) error {
			t.Fatal("unexpected call to Update in", string(debug.Stack()))
			return nil
		},
		Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
			t.Fatal("unexpected call to Delete in", string(debug.Stack()))
			return nil
		},
		Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
			t.Fatal("unexpected call to Patch in", string(debug.Stack()))
			return nil
		},
		DeleteAllOf: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteAllOfOption) error {
			t.Fatal("unexpected call to DeleteAllOf in", string(debug.Stack()))
			return nil
		},
		SubResourceCreate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
			t.Fatal("unexpected call to SubResourceCreate in", string(debug.Stack()))
			return nil
		},
		SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ ...client.SubResourceUpdateOption) error {
			t.Fatal("unexpected call to SubResourceUpdate in", string(debug.Stack()))
			return nil
		},
		SubResourcePatch: func(_ context.Context, _ client.Client, _ string, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
			t.Fatalf("unexpected call to SubResourcePatch for %T: %s", obj, string(debug.Stack()))
			return nil
		},
	}
}

// Merge combines multiple interceptor.Funcs into one. For each field, the last
// non-nil function wins. This allows composing multiple failure injections.
//
// Example: Merge(FailGet(someErr), FailUpdate(otherErr))
// produces Funcs with both Get and Update set.
func Merge(funcs ...interceptor.Funcs) interceptor.Funcs {
	var result interceptor.Funcs
	for _, f := range funcs {
		if f.Get != nil {
			result.Get = f.Get
		}
		if f.List != nil {
			result.List = f.List
		}
		if f.Create != nil {
			result.Create = f.Create
		}
		if f.Delete != nil {
			result.Delete = f.Delete
		}
		if f.DeleteAllOf != nil {
			result.DeleteAllOf = f.DeleteAllOf
		}
		if f.Update != nil {
			result.Update = f.Update
		}
		if f.Patch != nil {
			result.Patch = f.Patch
		}
		if f.Watch != nil {
			result.Watch = f.Watch
		}
		if f.SubResource != nil {
			result.SubResource = f.SubResource
		}
		if f.SubResourceGet != nil {
			result.SubResourceGet = f.SubResourceGet
		}
		if f.SubResourceCreate != nil {
			result.SubResourceCreate = f.SubResourceCreate
		}
		if f.SubResourceUpdate != nil {
			result.SubResourceUpdate = f.SubResourceUpdate
		}
		if f.SubResourcePatch != nil {
			result.SubResourcePatch = f.SubResourcePatch
		}
	}
	return result
}
