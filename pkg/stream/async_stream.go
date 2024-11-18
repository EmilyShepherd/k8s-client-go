package stream

import (
	"io"
	"sync"
)

// AsyncStream acts as a wrapper for any Stream and allows objects to be
// read from it asynchronously.
//
// Most streams are synchronous by their nature, because the underlying
// source needs to be read sequentially, however once parsed its common
// that items can be processed independently.
type AsyncStream[T any] struct {
	stream  Stream[T]
	result  chan T
	lock    sync.RWMutex
	stopped bool
	err     error
}

func NewAsyncStream[T any](stream Stream[T]) *AsyncStream[T] {
	sd := &AsyncStream[T]{
		stream: stream,
		result: make(chan T),
	}

	go sd.run()

	return sd
}

func (sd *AsyncStream[T]) Stopped() bool {
	sd.lock.RLock()
	defer sd.lock.RUnlock()

	return sd.stopped
}

func (sd *AsyncStream[T]) run() {
	for {
		result, err := sd.stream.Next()

		if sd.stopped {
			return
		}

		if err != nil {
			sd.err = err
			sd.Stop()
		} else {
			sd.result <- result
		}
	}
}

func (sd *AsyncStream[T]) Stop() {
	sd.lock.Lock()
	defer sd.lock.Unlock()

	if sd.stopped {
		return
	}

	close(sd.result)

	// If the stream we've been given can be closed, we'll call that as
	// part of the shutdown.
	if closer, ok := sd.stream.(io.Closer); ok {
		closer.Close()
	}

	// Once this is set, the main run loop will ignore any further events
	// and will exit.
	sd.stopped = true
}

func (sd *AsyncStream[T]) Next() (T, error) {
	return <-sd.result, sd.err
}

func (sd *AsyncStream[T]) ResultChan() <-chan T {
	return sd.result
}

func (sd *AsyncStream[T]) Error() error {
	return sd.err
}
