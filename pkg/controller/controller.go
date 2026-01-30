package controller

import (
	"k8s.io/client-go/util/workqueue"

	"github.com/EmilyShepherd/k8s-client-go/pkg/apis"
	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type Controller[T any, PT types.Object[T]] struct {
	resource *apis.ResourceCache[T, PT]
	queue    workqueue.TypedRateLimitingInterface[string]
}

func NewEmptyController[T any, PT types.Object[T]](root *apis.ResourceCache[T, PT]) *Controller[T, PT] {
	return &Controller[T, PT]{
		resource: root,
		queue:    workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}
}

func NewController[T any, PT types.Object[T]](root *apis.ResourceCache[T, PT]) *Controller[T, PT] {
	c := NewEmptyController[T, PT](root)

	root.RegisterListener(&Notifier[T, PT]{
		Parent:  c,
		Indexer: util.GetKeyForObject[T, PT],
	})

	return c
}

func (c *Controller[T, PT]) Notify(key string) {
	c.queue.AddRateLimited(key)
}

func (c *Controller[T, PT]) Watches(r chan string) *Controller[T, PT] {
	go func() {
		for key := range r {
			c.Notify(key)
		}

		c.Stop()
	}()

	return c
}

func (c *Controller[T, PT]) Stop() {
	c.queue.ShutDown()
}

func (c *Controller[T, PT]) Run(action RunHandler[T]) {
	c.Reconcile(&FuncHandler[T]{action})
}

func (c *Controller[T, PT]) Reconcile(r Reconciller[T]) {
	for {
		key, quit := c.queue.Get()
		if quit {
			return
		}

		element, found := c.resource.Get(key)
		if found {
			err := r.Reconcile(element)
			if err == nil {
				// When a success occurs we have to clear the item from the rate
				// limiter. This resets any failures or requeues it has previously had.
				c.queue.Forget(key)
			}
		} else {
			// If the reconciller explictly cares about element deletions, we
			// will notify it. Otherwise deletion events are ignored.
			if remover, ok := r.(RemoveReconciller); ok {
				ns, name := util.GetObjectForKey(key)
				remover.Remove(ns, name)
			}
		}

		c.queue.Done(key)
	}
}
