package apis

import (
	"github.com/EmilyShepherd/k8s-client-go/types"
)

type pipeWatcher[T any, PT types.Object[T]] struct {
	result    chan types.Event[T, PT]
	namespace string
	selectors []types.LabelSelector
}

func (p *pipeWatcher[T, PT]) Event(event types.Event[T, PT]) {
	if Matches(p.namespace, p.selectors, PT(&event.Object)) {
		p.result <- event
	}
}

func (p *pipeWatcher[T, PT]) Stop() {
	close(p.result)
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
