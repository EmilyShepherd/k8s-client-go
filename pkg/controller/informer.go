package controller

import (
	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

func IndexChan[T any, PT types.Object[T]](watcher types.WatchInterface[T, PT], indexer Indexer[PT]) chan string {
	result := make(chan string)

	go func() {
		for item := range watcher.ResultChan() {
			key := indexer(&item.Object)
			if key != "" {
				result <- key
			}
		}
	}()

	return result
}

type Indexer[T any] func(T) string

func KeyFromLabel[T any, PT types.Object[T]](label string) Indexer[PT] {
	return func(item PT) string {
		value := item.GetLabels()[label]
		if value == "" {
			return ""
		} else {
			return util.GetKey(item.GetNamespace(), item.GetLabels()[label])
		}
	}
}
