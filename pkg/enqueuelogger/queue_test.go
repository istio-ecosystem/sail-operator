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

package enqueuelogger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestAdditionNotifierQueue(t *testing.T) {
	mockQueue := new(MockQueue[reconcile.Request])

	var lastOnAddItem reconcile.Request
	queue := &AdditionNotifierQueue{
		delegate: mockQueue,
		onAdd: func(item reconcile.Request) {
			lastOnAddItem = item
		},
	}

	t.Run("Add", func(t *testing.T) {
		item := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "name1"}}
		mockQueue.On("Add", mock.Anything).Return()
		queue.Add(item)
		mockQueue.AssertCalled(t, "Add", item)
		assert.Equal(t, item, lastOnAddItem)
	})

	t.Run("AddAfter", func(t *testing.T) {
		item := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns2", Name: "name2"}}
		mockQueue.On("AddAfter", mock.Anything, mock.Anything).Return()
		queue.AddAfter(item, time.Second)
		mockQueue.AssertCalled(t, "AddAfter", item, time.Second)
		assert.Equal(t, item, lastOnAddItem)
	})

	t.Run("AddRateLimited", func(t *testing.T) {
		item := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns3", Name: "name3"}}
		mockQueue.On("AddRateLimited", mock.Anything).Return()
		queue.AddRateLimited(item)
		mockQueue.AssertCalled(t, "AddRateLimited", item)
		assert.Equal(t, item, lastOnAddItem)
	})

	t.Run("Forget", func(t *testing.T) {
		item := reconcile.Request{}
		mockQueue.On("Forget", item).Return()
		queue.Forget(item)
		mockQueue.AssertCalled(t, "Forget", item)
	})

	t.Run("Get", func(t *testing.T) {
		item := reconcile.Request{}
		mockQueue.On("Get").Return(item, false)
		returnedItem, returnedBool := queue.Get()
		assert.Equal(t, item, returnedItem)
		assert.Equal(t, false, returnedBool)
		mockQueue.AssertCalled(t, "Get")
	})

	t.Run("Done", func(t *testing.T) {
		item := reconcile.Request{}
		mockQueue.On("Done", item).Return()
		queue.Done(item)
		mockQueue.AssertCalled(t, "Done", item)
	})

	t.Run("Len", func(t *testing.T) {
		mockQueue.On("Len").Return(3)
		assert.Equal(t, 3, queue.Len())
		mockQueue.AssertCalled(t, "Len")
	})

	t.Run("NumRequeues", func(t *testing.T) {
		item := reconcile.Request{}
		mockQueue.On("NumRequeues", item).Return(1)
		assert.Equal(t, 1, queue.NumRequeues(item))
		mockQueue.AssertCalled(t, "NumRequeues", item)
	})

	t.Run("ShutDown", func(t *testing.T) {
		mockQueue.On("ShutDown").Return()
		queue.ShutDown()
		mockQueue.AssertCalled(t, "ShutDown")
	})

	t.Run("ShutDownWithDrain", func(t *testing.T) {
		mockQueue.On("ShutDownWithDrain").Return()
		queue.ShutDownWithDrain()
		mockQueue.AssertCalled(t, "ShutDownWithDrain")
	})

	t.Run("ShutDown", func(t *testing.T) {
		mockQueue.On("ShuttingDown").Return(true)
		assert.Equal(t, true, queue.ShuttingDown())
		mockQueue.AssertCalled(t, "ShuttingDown")
	})
}

type MockQueue[T comparable] struct {
	mock.Mock
	workqueue.TypedRateLimitingInterface[T]
}

func (m *MockQueue[T]) Add(item T) {
	m.Called(item)
}

func (m *MockQueue[T]) Len() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockQueue[T]) Get() (item T, shutdown bool) {
	args := m.Called()
	if v, ok := args.Get(0).(T); ok {
		return v, args.Bool(1)
	}
	panic("unexpected type")
}

func (m *MockQueue[T]) Done(item T) {
	m.Called(item)
}

func (m *MockQueue[T]) ShutDown() {
	m.Called()
}

func (m *MockQueue[T]) ShutDownWithDrain() {
	m.Called()
}

func (m *MockQueue[T]) ShuttingDown() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockQueue[T]) AddAfter(item T, duration time.Duration) {
	m.Called(item, duration)
}

func (m *MockQueue[T]) AddRateLimited(item T) {
	m.Called(item)
}

func (m *MockQueue[T]) Forget(item T) {
	m.Called(item)
}

func (m *MockQueue[T]) NumRequeues(item T) int {
	args := m.Called(item)
	return args.Int(0)
}
