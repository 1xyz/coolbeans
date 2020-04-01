package server

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

type clientURI struct {
	proxyID  string
	clientID string
}

func NewClientURI(proxyID string, clientID string) *clientURI {
	return &clientURI{
		proxyID:  proxyID,
		clientID: clientID,
	}
}

func ParseClientID(clientID state.ClientID) (*clientURI, error) {
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

func (c *clientURI) Validate() error {
	if len(c.clientID) == 0 || len(c.proxyID) == 0 {
		log.Errorf("invalid clientid=%v or proxyid=%v", c.clientID, c.proxyID)
		return ErrValidationFailed
	}

	return nil
}

func (c *clientURI) ToClientID() state.ClientID {
	return state.ClientID(fmt.Sprintf("%s:%s:%s", scheme, c.proxyID, c.clientID))
}
