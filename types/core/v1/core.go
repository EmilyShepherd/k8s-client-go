package v1

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
