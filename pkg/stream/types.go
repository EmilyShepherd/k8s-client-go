// Package stream implements a set of generic interfaces and classes
// designed to allow streams of atomic objects to be pipelined, much
// line one might do with an [io.Reader]
package stream

// A Decoder is able to hydrade an arbitrary variable.
type Decoder interface {
	Decode(v any) error
}

// A stream is able to provide a source of atomic data values.
//
// The source of a Stream's data is implementating specific - an example
// may be reading JSON objects from a long running HTTP response.
type Stream[T any] interface {
	Next() (T, error)
}
