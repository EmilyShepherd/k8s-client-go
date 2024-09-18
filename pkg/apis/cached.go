package apis

import (
	"fmt"
	"strconv"

	"github.com/EmilyShepherd/k8s-client-go/types"
)

type CachedAPI[T any, PT types.Object[T]] struct {
	api     types.ObjectAPI[T, PT]
	watcher types.WatchInterface[T, PT]
	items   map[string]T
	settled chan interface{}
}

func getKey(namespace, name string) string {
	key := ""
	if namespace != "" {
		key = namespace + "/"
	}

	key += name

	return key
}

func (i *CachedAPI[T, PT]) getKeyForObject(o PT) string {
	return getKey(o.GetNamespace(), o.GetName())
}

func NewCachedAPI[T any, PT types.Object[T]](rawApi types.ObjectAPI[T, PT], namespace, name string, opts types.ListOptions) (*CachedAPI[T, PT], error) {
	api := CachedAPI[T, PT]{
		items: make(map[string]T),
		settled: make(chan interface{}),
	}

	if name != "" {
		item, err := rawApi.Get(namespace, name, types.GetOptions{})
		if err != nil {
			return nil, err
		}
		api.items[api.getKeyForObject(&item)] = item
	} else {
		list, err := rawApi.List(namespace, opts)
		if err != nil {
			return nil, err
		}

		for _, item := range list.Items {
			api.items[api.getKeyForObject(&item)] = item
		}
	}

	watcher, err := rawApi.Watch(namespace, name, opts)
	if err != nil {
		return nil, err
	}

	api.watcher = watcher

	go func() {
		for result := range api.watcher.ResultChan() {
			if result.Type == types.EventTypeDeleted {
				api.processDeletedItem(result.Object)
			} else {
				api.processItem(result.Object)
			}
		}
	}()

	return &api, nil
}

func (i *CachedAPI[T, PT]) 

// Returns an item in the cached collection
func (i *CachedAPI[T, PT]) Get(namespace, name string, opts types.GetOptions) (T, error) {
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
func (i *CachedAPI[T, PT]) List(namespace string, opts types.ListOptions) (*types.List[T, PT], error) {
	list := types.List[T, PT]{}

	for _, item := range i.items {
		if namespace != "" && namespace != PT(&item).GetNamespace() {
			continue
		}
		if !matchesSelector(opts.LabelSelector, PT(&item).GetLabels()) {
			continue
		}

		list.Items = append(list.Items, item)
	}

	return &list, nil
}

func (i *CachedAPI[T, PT]) alertChange(item T) {
	for _, watcher := range i.watchers {

	}
}

func (i *CachedAPI[T, PT]) processDeletedItem(item T) {
	key := i.getKeyForObject(&item)
	if _, exists := i.items[key]; exists {
		delete(i.items, key)
		i.alertChange(item)
	}
}

func (i *CachedAPI[T, PT]) processItem(item T) {
	key := i.getKeyForObject(&item)
	current, exists := i.items[key]

	// As we update-early, based on the response after successful CRUD
	// operations, we will get this update twice - once directly from the
	// CRUD call, and once from the watcher. We therefore need to check
	// the update is newer than the state we have before doing anything
	// with it.
	currentVersion, _ := strconv.Atoi(PT(&current).GetResourceVersion())
	newVersion, _ := strconv.Atoi(PT(&item).GetResourceVersion())
	if exists && currentVersion < newVersion {
		i.items[key] = item
		i.alertChange(item)
	}
}

func (i *CachedAPI[T, PT]) Delete(namespace, name string, force bool) (T, error) {
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

func (i *CachedAPI[T, PT]) Create(namespace string, item T) (T, error) {
	return i.processItemAndErr(i.api.Create(namespace, item))
}

func (i *CachedAPI[T, PT]) Apply(namespace, name, fieldManager string, force bool, item T) (T, error) {
	return i.processItemAndErr(i.api.Apply(namespace, name, fieldManager, force, item))
}

func (i *CachedAPI[T, PT]) Patch(namespace, name, fieldManager string, item T) (T, error) {
	return i.processItemAndErr(i.api.Patch(namespace, name, fieldManager, item))
}

// Wrapper function for the return of passthru CRUD calls. If an error
// was returned, we don't want to call processItem. Otherwise just call
// that and then return the item.
func (i *CachedAPI[T, PT]) processItemAndErr(item T, err error) (T, error) {
	if err != nil {
		return item, err
	}

	i.processItem(item)

	return item, nil
}
