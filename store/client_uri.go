package store

import (
	"errors"
	"fmt"
	"github.com/1xyz/beanstalkd/state"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strings"
)

const scheme = "client"

var (
	ErrInvalidURI             = errors.New("invalid uri")
	ErrInvalidRequestFragment = errors.New("invalid request fragment")
	ErrValidationFailed       = errors.New("uri validation failed")
	ErrInvalidSchema          = errors.New("uri schema did not match or not present")
)

type ClientURI struct {
	proxyID  string
	clientID string
}

func NewClientURI(proxyID string, clientID string) *ClientURI {
	return &ClientURI{
		proxyID:  proxyID,
		clientID: clientID,
	}
}

func ParseClientURI(clientID state.ClientID) (*ClientURI, error) {
	u, err := url.Parse(string(clientID))
	if err != nil {
		log.Errorf("invalid uri. err %v", err)
		return nil, ErrInvalidURI
	}

	if strings.ToLower(u.Scheme) != scheme {
		log.Errorf("invalid schema = %v", u.Scheme)
		return nil, ErrInvalidSchema
	}

	req := strings.SplitN(u.RequestURI(), ":", 2)
	if len(req) != 2 {
		log.Errorf("invalid req framgement = %v", req)
		return nil, ErrInvalidRequestFragment
	}

	c := NewClientURI(req[0], req[1])
	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ClientURI) Validate() error {
	if len(c.clientID) == 0 || len(c.proxyID) == 0 {
		log.Errorf("invalid clientid=%v or proxyid=%v", c.clientID, c.proxyID)
		return ErrValidationFailed
	}

	return nil
}

func (c *ClientURI) ToClientID() state.ClientID {
	return state.ClientID(fmt.Sprintf("%s:%s:%s", scheme, c.proxyID, c.clientID))
}
