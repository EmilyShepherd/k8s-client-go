package token

// TokenProvider is a generic interface for a service that provides the
// auth token for the client to use
type TokenProvider interface {

	// Retrieves the current token at the time - this may return a fixed
	// or cached value, or it may go and do some work to acquire the
	// latest valid token.
	Token() string
}
