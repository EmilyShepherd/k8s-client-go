package controller

import (
	"fmt"

	"k8s.io/client-go/util/workqueue"

	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type Controller[T any, PT types.Object[T]] struct {
	api   types.ObjectAPI[T, PT]
	queue workqueue.TypedRateLimitingInterface[string]
}

func NewEmptyController[T any, PT types.Object[T]](root types.ObjectAPI[T, PT]) *Controller[T, PT] {
	return &Controller[T, PT]{
		api:   root,
		queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}
}

func NewController[T any, PT types.Object[T]](root types.ObjectAPI[T, PT]) (*Controller[T, PT], error) {
	c := NewEmptyController[T, PT](root)

	watcher, err := root.Watch("", "", types.ListOptions{})
	if err != nil {
		return nil, err
	}

	return c.Watches(IndexChan[T](watcher, util.GetKeyForObject[T])), nil
}

func (c *Controller[T, PT]) Watches(r chan string) *Controller[T, PT] {
	go func() {
		for key := range r {
			c.queue.Add(key)
		}
	}()

	return c
}

type RunAction[T any] func(T) error

func (c *Controller[T, PT]) Run(action RunAction[T]) {
	for {
		key, quit := c.queue.Get()
		if quit {
			return
		}

		ns, name := util.GetObjectForKey(key)
		element, err := c.api.Get(ns, name, types.GetOptions{})
		if err != nil {
			fmt.Printf("ERROR %s\n", err)
		}

		action(element)

		c.queue.Done(key)
	}
}
