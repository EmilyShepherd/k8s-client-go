package controller

type Reconciller[T any] interface {
	Reconcile(T) error
}

type RemoveReconciller interface {
	Remove(namespace, name string) error
}

type RunHandler[T any] func(T) error

type FuncHandler[T any] struct {
	action RunHandler[T]
}

func (h *FuncHandler[T]) Reconcile(el T) error {
	return h.action(el)
}
