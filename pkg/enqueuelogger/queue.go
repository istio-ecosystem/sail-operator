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
	"time"

	"k8s.io/client-go/util/workqueue"
)

// AdditionNotifierQueue is a queue that calls an onAdd function whenever an item is added to the queue.
// It is meant to be used in conjunction with EnqueueEventLogger to log items enqueued by a handler.
type AdditionNotifierQueue struct {
	delegate workqueue.RateLimitingInterface
	onAdd    func(item any)
}

var _ workqueue.RateLimitingInterface = &AdditionNotifierQueue{}

func NewAdditionNotifierQueue(delegate workqueue.RateLimitingInterface, onAddFunc func(item any)) *AdditionNotifierQueue {
	return &AdditionNotifierQueue{delegate: delegate}
}

func (q *AdditionNotifierQueue) Add(item interface{}) {
	q.delegate.Add(item)
	q.onAdd(item)
}

func (q *AdditionNotifierQueue) Len() int {
	return q.delegate.Len()
}

func (q *AdditionNotifierQueue) Get() (item interface{}, shutdown bool) {
	return q.delegate.Get()
}

func (q *AdditionNotifierQueue) Done(item interface{}) {
	q.delegate.Done(item)
}

func (q *AdditionNotifierQueue) ShutDown() {
	q.delegate.ShutDown()
}

func (q *AdditionNotifierQueue) ShutDownWithDrain() {
	q.delegate.ShutDownWithDrain()
}

func (q *AdditionNotifierQueue) ShuttingDown() bool {
	return q.delegate.ShuttingDown()
}

func (q *AdditionNotifierQueue) AddAfter(item interface{}, duration time.Duration) {
	q.delegate.AddAfter(item, duration)
	q.onAdd(item)
}

func (q *AdditionNotifierQueue) AddRateLimited(item interface{}) {
	q.delegate.AddRateLimited(item)
	q.onAdd(item)
}

func (q *AdditionNotifierQueue) Forget(item interface{}) {
	q.delegate.Forget(item)
}

func (q *AdditionNotifierQueue) NumRequeues(item interface{}) int {
	return q.delegate.NumRequeues(item)
}
