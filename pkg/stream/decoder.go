package stream

type decoderStream[T any] struct {
	decoder Decoder
}

// FromDecoder returns a Stream[T] based on the given [Decoder].
func FromDecoder[T any](decoder Decoder) Stream[T] {
	return &decoderStream[T]{
		decoder: decoder,
	}
}

// Next() blocks until it can return the next object in the writer.
// Returns an error if the writer is closed or an object can't be
// decoded.
func (sd *decoderStream[T]) Next() (T, error) {
	var t T

	return t, sd.decoder.Decode(&t)
}
