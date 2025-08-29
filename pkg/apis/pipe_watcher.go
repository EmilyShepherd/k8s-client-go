package apis

import (
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type pipeWatcher[T any, PT types.Object[T]] struct {
	input     chan types.Event[T, PT]
	result    chan types.Event[T, PT]
	parent    *CachedAPI[T, PT]
	namespace string
	selectors []types.LabelSelector
}

func (p *pipeWatcher[T, PT]) run() {
	for event := range p.input {
		if Matches(p.namespace, p.selectors, PT(&event.Object)) {
			p.result <- event
		}
	}
}

func (p *pipeWatcher[T, PT]) Stop() {
	//
}

func (p *pipeWatcher[T, PT]) ResultChan() <-chan types.Event[T, PT] {
	return p.result
}

func (p *pipeWatcher[T, PT]) Next() (types.Event[T, PT], error) {
	return <-p.result, nil
}

func (p *pipeWatcher[T, PT]) Error() error {
	return nil
}
