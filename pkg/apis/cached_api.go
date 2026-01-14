package apis

import (
	"fmt"

	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

var start int64

type CachedAPI[T any, PT types.Object[T]] struct {
	api   types.ObjectAPI[T, PT]
	cache *ResourceCache[T, PT]
}

func NewCachedAPI[T any, PT types.Object[T]](rawApi types.ObjectAPI[T, PT], namespace string, opts types.ListOptions) (*CachedAPI[T, PT], error) {
	cache, err := NewResourceCache(rawApi, namespace, opts)
	return &CachedAPI[T, PT]{
		api:   rawApi,
		cache: cache,
	}, err
}

func (i *CachedAPI[T, PT]) Cache() *ResourceCache[T, PT] {
	return i.cache
}

func (i *CachedAPI[T, PT]) Watch(name, namespace string, opts types.ListOptions) (types.WatchInterface[T, PT], error) {
	p := pipeWatcher[T, PT]{
		result:    make(chan types.Event[T, PT]),
		namespace: namespace,
		selectors: opts.LabelSelector,
	}

	go i.cache.RegisterListener(&p)

	return &p, nil
}

// Returns an item in the cached collection
func (i *CachedAPI[T, PT]) Get(namespace, name string, opts types.GetOptions) (T, error) {
	key := util.GetKey(namespace, name)
	item, found := i.cache.Get(key)
	if !found {
		return item, fmt.Errorf("Could not find object %s", key)
	}

	return item, nil
}

// List the items in the cached collection.
// If a namespace or LabelSelectors are provided, these will be matched
// against client side.
func (i *CachedAPI[T, PT]) List(namespace string, opts types.ListOptions) (*types.List[T, PT], error) {
	list := types.List[T, PT]{}

	i.cache.itemLock.RLock()
	for _, item := range i.cache.items {
		if Matches(namespace, opts.LabelSelector, PT(&item)) {
			list.Items = append(list.Items, item)
		}
	}
	i.cache.itemLock.RUnlock()

	return &list, nil
}

func (i *CachedAPI[T, PT]) Delete(namespace, name string, force bool) (T, error) {
	return i.api.Delete(namespace, name, force)
}

func (i *CachedAPI[T, PT]) Create(namespace string, item T) (T, error) {
	return i.api.Create(namespace, item)
}

func (i *CachedAPI[T, PT]) Apply(namespace, name, fieldManager string, force bool, item T) (T, error, types.EventType) {
	return i.api.Apply(namespace, name, fieldManager, force, item)
}

func (i *CachedAPI[T, PT]) Patch(namespace, name, fieldManager string, item T) (T, error) {
	return i.api.Patch(namespace, name, fieldManager, item)
}

func (o *CachedAPI[T, PT]) Subresource(subresource string) types.ObjectAPI[T, PT] {
	return &CachedAPI[T, PT]{
		api:   o.api.Subresource(subresource),
		cache: o.cache,
	}
}

func (o *CachedAPI[T, PT]) Status() types.ObjectAPI[T, PT] {
	return o.Subresource("status")
}
