package core

import (
	"fmt"
	"github.com/1xyz/coolbeans/state"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type client struct {
	// unique UUID for this client
	id state.ClientID

	// currently the tube used by this client
	useTube state.TubeName

	// Command Response channel
	responseCh chan CmdResponse

	// Tubes currently watched
	watchingTubes tubeSet
}

func (c client) String() string {
	return fmt.Sprintf("client: id = %v usetube = %v watching = %v",
		c.id, c.useTube, c.watchingTubes)
}

func NewClient(useTube state.TubeName) *client {
	id := uuid.New().URN()
	tubes := newTubeSet()
	err := tubes.Set(defaultTubeName)
	if err != nil {
		log.WithField("method", "useClient").Panicf("tubes.set %v", err)
	}

	return &client{
		id:            state.ClientID(id),
		useTube:       useTube,
		responseCh:    make(chan CmdResponse),
		watchingTubes: tubes,
	}
}

type ClientSet map[state.ClientID]*client

func (cs ClientSet) Set(c *client) error {
	if cs.Contains(c) {
		return state.ErrEntryExists
	}

	cs[c.id] = c
	return nil
}

func (cs ClientSet) Contains(c *client) bool {
	_, ok := cs[c.id]
	return ok
}

func (cs ClientSet) Remove(c *client) error {
	if !cs.Contains(c) {
		return state.ErrEntryMissing
	}

	delete(cs, c.id)
	return nil
}

func (cs ClientSet) Find(clientID state.ClientID) (*client, error) {
	cli, ok := cs[clientID]
	if !ok {
		return nil, state.ErrEntryMissing
	}

	return cli, nil
}

func (cs ClientSet) Len() int {
	return len(cs)
}

func (cs ClientSet) Random() (*client, error) {
	for _, v := range cs {
		return v, nil
	}

	return nil, state.ErrContainerEmpty
}

type tubeSet map[state.TubeName]bool

func newTubeSet() tubeSet {
	return make(tubeSet)
}

func (t tubeSet) Set(name state.TubeName) error {
	_, ok := t[name]
	if ok {
		return state.ErrEntryExists
	}
	t[name] = true
	return nil
}

func (t tubeSet) Remove(name state.TubeName) error {
	if _, ok := t[name]; !ok {
		return state.ErrEntryMissing
	}

	delete(t, name)
	return nil
}

func (t tubeSet) Len() int {
	return len(t)
}

func (t tubeSet) String() string {
	return fmt.Sprintf("tubeSet len = %v", t.Len())
}
