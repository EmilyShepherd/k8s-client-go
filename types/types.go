package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOptions is reserved to be implemented.
type GetOptions struct {
}

type Object[T any] interface {
	*T
	GetName() string
	GetNamespace() string
	GetResourceVersion() string
	GetLabels() map[string]string
}

const (
	Equals      = "%3D"
	Exists      = "Exists"
	LessThan    = "lt"
	GreaterThan = "gt"
	NotEquals   = "%21%3D"
)

type LabelSelector struct {
	Label    string
	Value    string
	Operator string
}

// ListOptions is reserved to be implemented.
type ListOptions struct {
	LabelSelector   []LabelSelector
	ResourceVersion string
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
type Event[T any, PT Object[T]] struct {
	Type   EventType `json:"type"`
	Object T         `json:"object"`
}

type List[T any, PT Object[T]] struct {
	metav1.ListMeta `json:"metadata"`
	Items           []T `json:"items"`
}

// WatchInterface can be implemented by anything that knows how to Watch and report changes.
type WatchInterface[T any, PT Object[T]] interface {
	// Stop stops watching. Will close the channel returned by ResultChan(). Releases
	// any resources used by the Watch.
	Stop()

	// ResultChan returns a chan which will receive all the events. If an error occurs
	// or Stop() is called, this channel will be closed, in which case the
	// Watch should be completely cleaned up.
	ResultChan() <-chan Event[T, PT]
}

// ObjectAPI wraps all operations on object.
type ObjectAPI[T any, PT Object[T]] interface {
	Get(namespace, name string, _ GetOptions) (T, error)
	Watch(namespace, name string, _ ListOptions) (WatchInterface[T, PT], error)
	List(namespace string, _ ListOptions) (*List[T, PT], error)
	Apply(namespace, name, fieldManager string, force bool, item T) (T, error, EventType)
	Patch(namespace, name, fieldManager string, item T) (T, error)
	Create(namespace string, item T) (T, error)
	Delete(namespace, name string, force bool) (T, error)
	Subresource(subresource string) ObjectAPI[T, PT]
	Status() ObjectAPI[T, PT]
}
