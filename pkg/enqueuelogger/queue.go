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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AdditionNotifierQueue is a queue that calls an onAdd function whenever an item is added to the queue.
// It is meant to be used in conjunction with EnqueueEventLogger to log items enqueued by a handler.
type AdditionNotifierQueue struct {
	workqueue.TypedRateLimitingInterface[reconcile.Request]
	onAdd func(item reconcile.Request)
}

var _ workqueue.TypedRateLimitingInterface[reconcile.Request] = &AdditionNotifierQueue{}

func (q *AdditionNotifierQueue) Add(item reconcile.Request) {
	q.TypedRateLimitingInterface.Add(item)
	q.onAdd(item)
}

func (q *AdditionNotifierQueue) AddAfter(item reconcile.Request, duration time.Duration) {
	q.TypedRateLimitingInterface.AddAfter(item, duration)
	q.onAdd(item)
}

func (q *AdditionNotifierQueue) AddRateLimited(item reconcile.Request) {
	q.TypedRateLimitingInterface.AddRateLimited(item)
	q.onAdd(item)
}
