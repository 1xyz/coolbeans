package core

import (
	"github.com/1xyz/coolbeans/state"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClientSet(t *testing.T) {

}

func TestClientSet_Set(t *testing.T) {
	cs := make(ClientSet)

	cli := newTestClient()
	err := cs.Set(cli)
	assert.Nilf(t, err, "expect err to be nil")

	err = cs.Set(cli)
	assert.Equalf(t, state.ErrEntryExists, err, "expect err to be ErrEntryExists")

	err = cs.Set(newTestClient())
	assert.Nilf(t, err, "expect err to be nil")
}

func TestClientSet_Remove(t *testing.T) {
	cs := make(ClientSet)

	cli := newTestClient()
	if err := cs.Set(cli); err != nil {
		t.Fatalf("test error %v", err)
	}

	err := cs.Remove(cli)
	assert.Nilf(t, err, "expect err to be nil")

	err = cs.Remove(cli)
	assert.Equalf(t, state.ErrEntryMissing, err, "expect err to be ErrEntryMissing")
}

func TestClientSet_Contains(t *testing.T) {
	cs := make(ClientSet)

	cli := newTestClient()
	if err := cs.Set(cli); err != nil {
		t.Fatalf("test error %v", err)
	}

	b := cs.Contains(cli)
	assert.Truef(t, b, "expect result to be true")

	if err := cs.Remove(cli); err != nil {
		t.Fatalf("test error %v", err)
	}
	b = cs.Contains(cli)
	assert.Falsef(t, b, "expect result to be false")
}

func TestClientSet_Find(t *testing.T) {
	cs := make(ClientSet)

	cli, err := cs.Find("abracadabra")
	assert.Equalf(t, state.ErrEntryMissing, err, "expectErrEntryMissing")

	cli = newTestClient()
	if err := cs.Set(cli); err != nil {
		t.Fatalf("test error %v", err)
	}

	resultCli, err := cs.Find(cli.id)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, cli.id, resultCli.id, "expect client to be found")
}

func newTestClient() *client {
	cli := NewClient(state.TubeName("foo"))
	return cli
}

func TestTubeSet(t *testing.T) {
	ts := newTubeSet()

	tubeName := state.TubeName("othello")
	err := ts.Set(tubeName)
	assert.Nilf(t, err, "expect err to  be nil")

	err = ts.Set(tubeName)
	assert.Equalf(t, state.ErrEntryExists, err, "expect ErrEntryExists")

	err = ts.Remove(tubeName)
	assert.Nilf(t, err, "expect err to  be nil")

	err = ts.Remove(tubeName)
	assert.Equalf(t, state.ErrEntryMissing, err, "expect ErrEntryExists")
}
