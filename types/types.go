package types

import (
	"context"
)

// GetOptions is reserved to be implemented.
type GetOptions struct {
}

// ListOptions is reserved to be implemented.
type ListOptions struct {
}

type GroupVersionResource struct {
	Group    string
	Version  string
	Resource string
}

type EventType string

const (
	EventTypeAdded    EventType = "ADDED"
	EventTypeModified EventType = "MODIFIED"
	EventTypeDeleted  EventType = "DELETED"
	EventTypeError    EventType = "ERROR"
)

// Event represents a single event to a watched resource.
type Event[T interface{}] struct {
	Type   EventType `json:"type"`
	Object *T        `json:"object"`
}

// WatchInterface can be implemented by anything that knows how to Watch and report changes.
type WatchInterface[T interface{}] interface {
	// Stop stops watching. Will close the channel returned by ResultChan(). Releases
	// any resources used by the Watch.
	Stop()

	// ResultChan returns a chan which will receive all the events. If an error occurs
	// or Stop() is called, this channel will be closed, in which case the
	// Watch should be completely cleaned up.
	ResultChan() <-chan Event[T]
}

// ObjectGetter is generic object getter.
type ObjectGetter[T interface{}] interface {
	Get(ctx context.Context, namespace, name string, _ GetOptions) (*T, error)
}

// ObjectWatcher is generic object watcher.
type ObjectWatcher[T interface{}] interface {
	Watch(ctx context.Context, namespace, name string, _ ListOptions) (WatchInterface[T], error)
}

// ObjectAPI wraps all operations on object.
type ObjectAPI[T interface{}] interface {
	ObjectGetter[T]
	ObjectWatcher[T]
}
