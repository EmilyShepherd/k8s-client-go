package apis

import (
	"sync"

	"github.com/EmilyShepherd/k8s-client-go/pkg/util"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type EventListener[T any, PT types.Object[T]] interface {
	Event(types.Event[T, PT])
	Stop()
}

type ResourceCache[T any, PT types.Object[T]] struct {
	watcher  types.WatchInterface[T, PT]
	items    map[string]T
	watchers []EventListener[T, PT]
	itemLock sync.RWMutex
	ready    bool
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
		for {
			result, err := api.watcher.Next()
			if err == nil {
				api.processEvent(result)
			} else {
				api.ready = false

				for _, watcher := range api.watchers {
					watcher.Stop()
				}

				return
			}
		}
	}()

	api.ready = true

	return &api, nil
}

func (i *ResourceCache[T, PT]) IsReady() bool {
	return i.ready
}

func (i *ResourceCache[T, PT]) Error() error {
	return i.watcher.Error()
}

func (i *ResourceCache[T, PT]) Get(key string) (T, bool) {
	i.itemLock.RLock()
	defer i.itemLock.RUnlock()

	found, ok := i.items[key]
	return found, ok
}

func (i *ResourceCache[T, PT]) RegisterListener(listener EventListener[T, PT]) {
	i.itemLock.Lock()
	defer i.itemLock.Unlock()

	for _, item := range i.items {
		listener.Event(types.Event[T, PT]{
			Type:   types.EventTypeAdded,
			Object: item,
		})
	}
	i.watchers = append(i.watchers, listener)
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
		watcher.Event(e)
	}
}
