package client

import (
	"context"

	metav1 "github.com/EmilyShepherd/k8s-client-go/types/meta/v1"
)

// ObjectGetter is generic object getter.
type ObjectGetter[T interface{}] interface {
	Get(ctx context.Context, namespace, name string, _ metav1.GetOptions) (*T, error)
}

// ObjectWatcher is generic object watcher.
type ObjectWatcher[T interface{}] interface {
	Watch(ctx context.Context, namespace, name string, _ metav1.ListOptions) (WatchInterface[T], error)
}

// ObjectAPI wraps all operations on object.
type ObjectAPI[T interface{}] interface {
	ObjectGetter[T]
	ObjectWatcher[T]
}
