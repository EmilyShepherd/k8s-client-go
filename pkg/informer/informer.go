package informer

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/EmilyShepherd/k8s-client-go/types"
)

type Informer[T metav1.Object] struct {
	api     types.ObjectAPI[T]
	watcher types.WatchInterface[T]
	items   map[string]T
}

func getKey(namespace, name string) string {
	key := ""
	if namespace != "" {
		key = namespace + "/"
	}

	key += name

	return key
}

func (i *Informer[T]) getKeyForObject(o T) string {
	return getKey(o.GetNamespace(), o.GetName())
}

func NewInformer[T metav1.Object](api types.ObjectAPI[T], namespace, name string, opts types.ListOptions) (*Informer[T], error) {
	informer := Informer[T]{
		items: make(map[string]T),
	}

	if name != "" {
		item, err := api.Get(namespace, name, types.GetOptions{})
		if err != nil {
			return nil, err
		}
		informer.items[informer.getKeyForObject(item)] = item
	} else {
		list, err := api.List(namespace, opts)
		if err != nil {
			return nil, err
		}

		for _, item := range list.Items {
			informer.items[informer.getKeyForObject(item)] = item
		}
	}

	watcher, err := api.Watch(namespace, name, opts)
	if err != nil {
		return nil, err
	}

	informer.watcher = watcher

	go func() {
		for result := range informer.watcher.ResultChan() {
			if result.Type == types.EventTypeDeleted {
				informer.processDeletedItem(result.Object)
			} else {
				informer.processItem(result.Object)
			}
		}
	}()

	return &informer, nil
}

// Returns an item in the cached collection
func (i *Informer[T]) Get(namespace, name string, opts types.GetOptions) (T, error) {
	key := getKey(namespace, name)
	item, found := i.items[key]
	if !found {
		return item, fmt.Errorf("Could not find object %s", key)
	}

	return item, nil
}

func matchesSelector(selectors []types.LabelSelector, labels map[string]string) bool {
	for _, selector := range selectors {
		value, exists := labels[selector.Label]
		if !exists {
			return false
		}
		switch selector.Operator {
		case types.Equals:
			if value != selector.Value {
				return false
			}
		case types.NotEquals:
			if value == selector.Value {
				return false
			}
		case types.LessThan:
			if value >= selector.Value {
				return false
			}
		case types.GreaterThan:
			if value <= selector.Value {
				return false
			}
		}
	}

	return true
}

// List the items in the cached collection.
// If a namespace or LabelSelectors are provided, these will be matched
// against client side.
func (i *Informer[T]) List(namespace string, opts types.ListOptions) (*types.List[T], error) {
	list := types.List[T]{}

	for _, item := range i.items {
		if namespace != "" && namespace != item.GetNamespace() {
			continue
		}
		if !matchesSelector(opts.LabelSelector, item.GetLabels()) {
			continue
		}

		list.Items = append(list.Items, item)
	}

	return &list, nil
}

func (i *Informer[T]) alertChange(item T) {
	//
}

func (i *Informer[T]) processDeletedItem(item T) {
	key := i.getKeyForObject(item)
	if _, exists := i.items[key]; exists {
		delete(i.items, key)
		i.alertChange(item)
	}
}

func (i *Informer[T]) processItem(item T) {
	key := i.getKeyForObject(item)
	current, exists := i.items[key]

	// As we update-early, based on the response after successful CRUD
	// operations, we will get this update twice - once directly from the
	// CRUD call, and once from the watcher. We therefore need to check
	// the update is newer than the state we have before doing anything
	// with it.
	currentVersion, _ := strconv.Atoi(current.GetResourceVersion())
	newVersion, _ := strconv.Atoi(item.GetResourceVersion())
	if exists && currentVersion < newVersion {
		i.items[key] = item
		i.alertChange(item)
	}
}

func (i *Informer[T]) Delete(namespace, name string, force bool) (T, error) {
	item, err := i.api.Delete(namespace, name, force)
	if err != nil {
		return item, err
	}

	// The "delete" verb in kubernetes can mean one of two things:
	//   1. Mark the resource as deleted, but don't actually hard delete
	//      it until the apiserver is happy it is safe to do so.
	//   2. "Force" the deletion and hard delete.
	//
	// Case 1 is effectively just a modification, as it's just the
	// addition of the "deletionTimestamp" field, so it is treated as
	// such. Case 2 is an actual deletion, and has so has to be dealt with
	// by actually removing the item from the cached list.
	if force {
		i.processDeletedItem(item)
	} else {
		i.processItem(item)
	}

	return item, nil
}

func (i *Informer[T]) Create(namespace string, item T) (T, error) {
	return i.processItemAndErr(i.api.Create(namespace, item))
}

func (i *Informer[T]) Apply(namespace, name, fieldManager string, force bool, item T) (T, error) {
	return i.processItemAndErr(i.api.Apply(namespace, name, fieldManager, force, item))
}

func (i *Informer[T]) Patch(namespace, name, fieldManager string, item T) (T, error) {
	return i.processItemAndErr(i.api.Patch(namespace, name, fieldManager, item))
}

// Wrapper function for the return of passthru CRUD calls. If an error
// was returned, we don't want to call processItem. Otherwise just call
// that and then return the item.
func (i *Informer[T]) processItemAndErr(item T, err error) (T, error) {
	if err != nil {
		return item, err
	}

	i.processItem(item)

	return item, nil
}
