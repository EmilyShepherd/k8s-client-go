package controller

import (
	"fmt"
	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type Controller[T any, PT types.Object[T]] struct {
}

func NewEmptyController[T any, PT types.Object[T]]() *Controller[T, PT] {
	return &Controller[T, PT]{}
}

func NewController[T any, PT types.Object[T]](root types.ObjectAPI[T, PT]) (*Controller[T, PT], error) {
	c := NewEmptyController[T, PT]()

	watcher, err := root.Watch("", "", types.ListOptions{})
	if err != nil {
		return nil, err
	}

	return c.Watches(watcher, util.GetKeyForObject[T, PT]), nil
}

type Indexer[T any] func(T) string

func (c *Controller[T, PT]) Watches(watcher types.WatchInterface[T, PT], indexFunc Indexer[PT]) *Controller[T, PT] {
	go func() {
		for item := range watcher.ResultChan() {
			key := indexFunc(&item.Object)
			fmt.Printf("Enqueue %s\n", key)
		}
	}()

	return c
}

func KeyFromLabel[T any, PT types.Object[T]](label string) Indexer[PT] {
	return func(item PT) string {
		return item.GetLabels()[label]
	}
}

func (c *Controller[T, PT]) Run() {
	//
}
