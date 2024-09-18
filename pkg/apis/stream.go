package apis

import (
	"fmt"
	"io"
	"sync"

	"github.com/EmilyShepherd/k8s-client-go/types"
)

// ResponseDecoder allows to specify custom JSON response decoder. By default, std json decoder is used.
type ResponseDecoder interface {
	Decode(v any) error
}

// StreamWatcher turns any stream for which you can write a Decoder interface
// into a Watch.Interface.
type streamWatcher[T any, PT types.Object[T]] struct {
	result  chan types.Event[T, PT]
	r       io.ReadCloser
	log     Logger
	decoder ResponseDecoder
	sync.Mutex
	stopped bool
}

// NewStreamWatcher creates a StreamWatcher from the given io.ReadClosers.
func newStreamWatcher[T any, PT types.Object[T]](r io.ReadCloser, log Logger, decoder ResponseDecoder) types.WatchInterface[T, PT] {
	sw := &streamWatcher[T, PT]{
		r:       r,
		log:     log,
		decoder: decoder,
		result:  make(chan types.Event[T, PT]),
	}
	go sw.receive()
	return sw
}

// ResultChan implements Interface.
func (sw *streamWatcher[T, PT]) ResultChan() <-chan types.Event[T, PT] {
	return sw.result
}

// Stop implements Interface.
func (sw *streamWatcher[T, PT]) Stop() {
	sw.Lock()
	defer sw.Unlock()
	if !sw.stopped {
		sw.stopped = true
		sw.r.Close()
	}
}

// stopping returns true if Stop() was called previously.
func (sw *streamWatcher[T, PT]) stopping() bool {
	sw.Lock()
	defer sw.Unlock()
	return sw.stopped
}

// receive reads result from the decoder in a loop and sends down the result channel.
func (sw *streamWatcher[T, PT]) receive() {
	defer close(sw.result)
	defer sw.Stop()
	for {
		obj, err := sw.Decode()
		if err != nil {
			// Ignore expected error.
			if sw.stopping() {
				return
			}
			switch err {
			case io.EOF:
				// Watch closed normally.
			case io.ErrUnexpectedEOF:
				sw.log.Infof("k8s-client-go: unexpected EOF during Watch stream event decoding: %v", err)
			default:
				sw.log.Infof("k8s-client-go: unable to decode an event from the Watch stream: %v", err)
			}
			return
		}
		sw.result <- obj
	}
}

// Decode blocks until it can return the next object in the writer. Returns an error
// if the writer is closed or an object can't be decoded.
func (sw *streamWatcher[T, PT]) Decode() (types.Event[T, PT], error) {
	var t types.Event[T, PT]
	if err := sw.decoder.Decode(&t); err != nil {
		return t, err
	}
	switch t.Type {
	case types.EventTypeAdded, types.EventTypeModified, types.EventTypeDeleted, types.EventTypeError:
		return t, nil
	default:
		return t, fmt.Errorf("got invalid Watch event type: %v", t.Type)
	}
}
