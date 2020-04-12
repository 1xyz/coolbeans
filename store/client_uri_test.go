package store

import (
	"github.com/1xyz/coolbeans/state"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseClientID(t *testing.T) {
	var entries = []struct {
		inputCliID   state.ClientID
		outClientURI *ClientURI
		outErr       error
	}{
		{state.ClientID("client://id?proxy=alpha&client=beta"),
			&ClientURI{proxyID: "alpha", clientID: "beta"}, nil},
		{state.ClientID("client://id?proxy=alpha&client=beta&theta=omega"),
			&ClientURI{proxyID: "alpha", clientID: "beta"}, nil},
		{state.ClientID("client://id?proxy=urn:uuid:043a76c3-903b-45ac-bd02-b03d6298b52e&client=urn:uuid:9d9eee85-bec7-4434-9559-0b8c83380033"),
			&ClientURI{
				proxyID:  "urn:uuid:043a76c3-903b-45ac-bd02-b03d6298b52e",
				clientID: "urn:uuid:9d9eee85-bec7-4434-9559-0b8c83380033"}, nil},
		{state.ClientID("client://id?proxy=alpha"), nil, ErrValidationFailed},
		{state.ClientID("client://id"), nil, ErrValidationFailed},
		{state.ClientID("client:"), nil, ErrValidationFailed},
		{state.ClientID("client"), nil, ErrInvalidSchema},
		{state.ClientID(":::"), nil, ErrInvalidURI},
	}

	for _, e := range entries {
		c, err := ParseClientURI(e.inputCliID)
		assert.Equalf(t, e.outClientURI, c, "expect clientURI to match")
		assert.Equalf(t, e.outErr, err, "expect err to match")
	}
}

func TestClientURI_ClientID(t *testing.T) {
	c := NewClientURI("foo", "bar").ToClientID()
	assert.Equalf(t, state.ClientID("client://id?client=bar&proxy=foo"), c, "expect ClientIDs to match")
}
