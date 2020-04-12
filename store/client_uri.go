package store

import (
	"errors"
	"github.com/1xyz/coolbeans/state"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strings"
)

const scheme = "client"

var (
	ErrInvalidURI       = errors.New("invalid uri")
	ErrValidationFailed = errors.New("uri validation failed")
	ErrInvalidSchema    = errors.New("uri schema did not match or not present")
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

	q := u.Query()
	c := NewClientURI(q.Get("proxy"), q.Get("client"))
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
	baseUrl, err := url.Parse("client://id")
	if err != nil {
		log.Panicf("malformed url %v", err)
	}

	params := url.Values{}
	params.Add("proxy", c.proxyID)
	params.Add("client", c.clientID)
	baseUrl.RawQuery = params.Encode()
	return state.ClientID(baseUrl.String())
}
