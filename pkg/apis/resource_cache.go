package apis

import (
	"sync"

	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type ResourceCache[T any, PT types.Object[T]] struct {
	watcher  types.WatchInterface[T, PT]
	items    map[string]T
	watchers []pipeWatcher[T, PT]
	itemLock sync.RWMutex
}

func NewResourceCache[T any, PT types.Object[T]](rawApi types.ObjectAPI[T, PT], namespace string, opts types.ListOptions) (*ResourceCache[T, PT], error) {
	api := ResourceCache[T, PT]{
		items: make(map[string]T),
	}

	list, err := rawApi.List(namespace, opts)
	if err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		api.items[util.GetKeyForObject[T, PT](&item)] = item
	}
	opts.ResourceVersion = list.ResourceVersion

	watcher, err := rawApi.Watch(namespace, "", opts)
	if err != nil {
		return nil, err
	}

	api.watcher = watcher

	go func() {
		for result := range api.watcher.ResultChan() {
			api.processEvent(result)
		}
	}()

	return &api, nil
}

func (i *ResourceCache[T, PT]) get(key string) (T, bool) {
	i.itemLock.RLock()
	defer i.itemLock.RUnlock()

	found, ok := i.items[key]
	return found, ok
}

func (i *ResourceCache[T, PT]) all() map[string]T {
	i.itemLock.RLock()
	defer i.itemLock.RUnlock()

	return i.items
}

func (i *ResourceCache[T, PT]) processEvent(e types.Event[T, PT]) {
	key := util.GetKeyForObject[T, PT](&e.Object)

	i.itemLock.Lock()
	if e.Type == types.EventTypeDeleted {
		delete(i.items, key)
	} else {
		i.items[key] = e.Object
	}
	i.itemLock.Unlock()

	for _, watcher := range i.watchers {
		watcher.input <- e
	}
}
