package server

import (
	"github.com/1xyz/beanstalkd/state"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseClientID(t *testing.T) {
	var entries = []struct {
		inputCliID   state.ClientID
		outClientURI *clientURI
		outErr       error
	}{
		{state.ClientID("client:alpha:beta"), &clientURI{proxyID: "alpha", clientID: "beta"}, nil},
		{state.ClientID("client:alpha:beta:omega"), &clientURI{proxyID: "alpha", clientID: "beta:omega"}, nil},
		{state.ClientID("client:alpha:"), nil, ErrValidationFailed},
		{state.ClientID("client::beta"), nil, ErrValidationFailed},
		{state.ClientID("client:alpha"), nil, ErrInvalidRequestFragment},
		{state.ClientID("client"), nil, ErrInvalidSchema},
		{state.ClientID(":::"), nil, ErrInvalidURI},
	}

	for _, e := range entries {
		c, err := ParseClientID(e.inputCliID)
		assert.Equalf(t, e.outClientURI, c, "expect clientURI to match")
		assert.Equalf(t, e.outErr, err, "expect err to match")
	}
}

func TestClientURI_ClientID(t *testing.T) {
	c := NewClientURI("foo", "bar").ToClientID()
	assert.Equalf(t, state.ClientID("client:foo:bar"), c, "expect ClientIDs to match")
}
