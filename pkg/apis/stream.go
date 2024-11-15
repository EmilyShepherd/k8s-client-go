package apis

import (
	"encoding/json"
	"io"

	"github.com/EmilyShepherd/k8s-client-go/pkg/client"
	"github.com/EmilyShepherd/k8s-client-go/types"
)

// ResponseDecoder allows to specify custom JSON response decoder. By default, std json decoder is used.
type ResponseDecoder interface {
	Decode(v any) error
}

// Watcher is a [Stream] wrapper for kubernetes watch events.

// In addition to processing objects as [watch.Event] structs, the
// Watcher will also keep track of the latest resourceVersion returned
// and will attempt to gracefully reconnect when watch connections
// time out.
type Watcher[T any, PT types.Object[T]] struct {
	closer          io.Closer
	decoder         ResponseDecoder
	api             *client.Client
	req             client.ResourceRequest
	resourceVersion string
}

// [Close] closes the underlying response body io.ReadCloser
func (sw *Watcher[T, PT]) Close() error {
	sw.decoder = nil

	return sw.closer.Close()
}

func (sw *Watcher[T, PT]) doWatch() error {
	if sw.resourceVersion != "" {
		sw.req.Values.Set("resourceVersion", sw.resourceVersion)
	}

	resp, err := sw.api.Do(sw.req)
	if err != nil {
		return err
	}

	sw.closer = resp.Body
	sw.decoder = json.NewDecoder(resp.Body)

	return nil
}

// receive reads result from the decoder in a loop and sends down the result channel.
func (sw *Watcher[T, PT]) Next() (types.Event[T, PT], error) {
	for {
		var evt types.Event[T, PT]
		err := sw.decoder.Decode(&evt)

		switch err {
		// Success case. Make a note of the latest resource version and then
		// return the event to the caller.
		case nil:
			sw.resourceVersion = PT(&evt.Object).GetResourceVersion()
			return evt, nil

		// Graceful closure of the underlying io.Reader, normally caused by
		// a timeout. Attempt to reconnect.
		case io.EOF:
			// If the decoder is nil, we can't restart and should just return.
			// This will either happen if you call Next() before intially
			// starting the watcher, or because it has been explictly Closed()
			if sw.decoder == nil {
				return evt, err
			}

			if err = sw.doWatch(); err != nil {
				return evt, err
			}

		// Some other error occurred, which we cannot deal with. Return it
		// to the caller.
		default:
			return evt, err
		}
	}
}
