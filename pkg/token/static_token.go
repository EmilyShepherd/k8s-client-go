package token

// StaticToken is a TokenProvider wrapper for a static token
type StaticToken struct {
	token string
}

func NewStaticToken(token string) (*StaticToken, error) {
	return &StaticToken{token: token}, nil
}

func (t *StaticToken) Token() string {
	return t.token
}
