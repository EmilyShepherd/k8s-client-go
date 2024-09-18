package apis

type pipeWatcher[T any, PT types.Object[T]] struct {
	result chan types.Event[T, PT]
	parent cachedAPI[T, PT]
}

func (p *pipeWatcher[T, PT]) Stop() {
	//
}

func (p *pipeWatcher[T, PT]) ResultChan() <-chan types.Event[T, PT] {
	return p.result
}
