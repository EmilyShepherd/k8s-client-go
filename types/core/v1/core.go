package v1

import (
	metav1 "github.com/EmilyShepherd/k8s-client-go/types/meta/v1"
)

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

type Endpoints struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Subsets           []Subset `json:"subsets"`
}

func (o Endpoints) GVR() metav1.GroupVersionResource {
	return metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "endpoints",
	}
}

func (o Endpoints) GetObjectMeta() metav1.ObjectMeta {
	return o.ObjectMeta
}

func (o Endpoints) GetTypeMeta() metav1.TypeMeta {
	return o.TypeMeta
}

type Subset struct {
	Addresses []Address `json:"addresses"`
	Ports     []Port    `json:"ports"`
}

type Address struct {
	IP        string           `json:"ip"`
	TargetRef *ObjectReference `json:"targetRef,omitempty"`
}

type ObjectReference struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
type Port struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}
