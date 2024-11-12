package controller

import (
	"fmt"
	"sync"

	"github.com/gammazero/deque"

	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type Controller[T any, PT types.Object[T]] struct {
	waiting    map[string]bool
	inProgress map[string]bool
	queue      *deque.Deque[string]
	lock       sync.RWMutex
	cond       *sync.Cond
	api        types.ObjectAPI[T, PT]
}

func NewEmptyController[T any, PT types.Object[T]](root types.ObjectAPI[T, PT]) *Controller[T, PT] {
	c := Controller[T, PT]{
		waiting: make(map[string]bool),
		queue:   deque.New[string](),
		api:     root,
	}
	c.cond = sync.NewCond(&c.lock)
	return &c
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
			c.lock.Lock()
			_, exists := c.waiting[key]
			if exists {
				c.lock.Unlock()
				continue
			}

			c.queue.PushBack(key)
			c.waiting[key] = true
			c.lock.Unlock()
			c.cond.Signal()
		}
	}()

	return c
}

type RunAction[T any] func(T) error

func (c *Controller[T, PT]) Run(action RunAction[T]) {
	for {
		c.lock.Lock()
		for c.queue.Len() == 0 {
			c.cond.Wait()
		}

		key := c.queue.PopFront()

		// If the key has been readded since a controller started working on
		// it, it needs processing but we need to wait for the last run to
		// complete first, so we'll just put it to the back of the line and
		// try again later.
		if _, exists := c.inProgress[key]; exists {
			c.queue.PushBack(key)
			continue
		} else {
			c.inProgress[key] = true
		}

		delete(c.waiting, key)
		c.lock.Unlock()

		ns, name := util.GetObjectForKey(key)
		element, err := c.api.Get(ns, name, types.GetOptions{})
		if err != nil {
			fmt.Printf("ERROR %s\n", err)
		}

		action(element)

		c.lock.Lock()
		delete(c.inProgress, key)
		c.lock.Unlock()
	}
}
