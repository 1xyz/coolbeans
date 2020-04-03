package state

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewClientResvHeap(t *testing.T) {
	ch := newClientResvHeap()
	assert.Equalf(t, 0, ch.Len(), "initial length is zero")
}

func TestClientResvHeap_Enqueue(t *testing.T) {
	ch := newClientResvHeap()
	c1, c2 := newTestClientResvEntry(30), newTestClientResvEntry(20)

	ch.Enqueue(c1)
	ch.Enqueue(c2)

	assert.Equalf(t, 0, c2.HeapIndex,
		"client with lower nextTickAt is at the head of queue")
	assert.Equalf(t, 1, c1.HeapIndex,
		"client with lower nextTickAt is not at head of queue")
}

func TestClientHeap_Dequeue(t *testing.T) {
	ch := newClientResvHeap()
	c1, c2 := newTestClientResvEntry(30), newTestClientResvEntry(20)

	ch.Enqueue(c1)
	ch.Enqueue(c2)

	out1 := ch.Dequeue()
	out2 := ch.Dequeue()
	assert.Equalf(t, c2.CliID, out1.CliID, "expect c2 to be dequeued first")
	assert.Equalf(t, c1.CliID, out2.CliID, "expect c1 to be dequeued next")
}

func TestClientHeap_Peek(t *testing.T) {
	ch := newClientResvHeap()
	c1, c2 := newTestClientResvEntry(30), newTestClientResvEntry(20)

	ch.Enqueue(c1)
	ch.Enqueue(c2)

	assert.Equalf(t, c2.CliID, ch.Peek().CliID, "expect c2 to be peeked")
}

func TestClientHeap_Peek_Nil(t *testing.T) {
	ch := newClientResvHeap()
	assert.Nilf(t, ch.Peek(), "expect peek of empty heap to return nil")
}

func TestClientHeap_UpdateTickAt(t *testing.T) {
	ch := newClientResvHeap()

	c0, c1, c2 := newTestClientResvEntry(30), newTestClientResvEntry(20), newTestClientResvEntry(10)
	ch.Enqueue(c0)
	ch.Enqueue(c1)
	ch.Enqueue(c2)

	ch.UpdateTickAt(c2, 100)

	assert.Equalf(t, 0, c1.HeapIndex, "expect c1 to be at head of heap")
	assert.Equalf(t, 1, c0.HeapIndex, "expect c0 to be at middle of heap")
	assert.Equalf(t, 2, c2.HeapIndex, "expect c2 to be at end of heap")
}

func TestClientHeap_Remove(t *testing.T) {
	ch := newClientResvHeap()
	c1, c2 := newTestClientResvEntry(30), newTestClientResvEntry(20)

	ch.Enqueue(c1)
	ch.Enqueue(c2)

	err := ch.Remove(c2)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, -1, c2.HeapIndex, "expect heap index to be reset to -1")
	assert.Equalf(t, ch.Len(), 1, "expect client heap's length to be 1")
}

func TestClientHeap_Remove_InvalidJobEntry(t *testing.T) {
	ch := newClientResvHeap()
	c1, c2 := newTestClientResvEntry(30), newTestClientResvEntry(20)

	ch.Enqueue(c1)
	ch.Enqueue(c2)

	c3 := newTestClientResvEntry(20)
	c3.HeapIndex = c2.HeapIndex
	err := ch.Remove(c3)
	assert.Equalf(t, err, ErrMismatchJobEntry, "expect err to be ErrMismatchJobEntry")
}

func TestClientHeap_Remove_ErrInvalidIndex(t *testing.T) {
	ch := newClientResvHeap()
	c1, c2 := newTestClientResvEntry(30), newTestClientResvEntry(20)

	ch.Enqueue(c1)
	ch.Enqueue(c2)

	c2.HeapIndex = 100
	err := ch.Remove(c2)
	assert.Equalf(t, err, ErrInvalidIndex, "expect err to be ErrInvalidIndex")
}

func TestClientResvQueue_Enqueue(t *testing.T) {
	cs := newClientResvQueue()

	cli := newTestClientResvEntry(1)
	err := cs.Enqueue(cli)
	assert.Nilf(t, err, "expect err to be nil")

	err = cs.Enqueue(cli)
	assert.Truef(t, errors.Is(err, ErrEntryExists), "expect err to contain ErrEntryExists")

	err = cs.Enqueue(newTestClientResvEntry(1))
	assert.Nilf(t, err, "expect err to be nil")
}

func TestClientResvQueue_Remove(t *testing.T) {
	cs := newClientResvQueue()

	cli := newTestClientResvEntry(1)
	if err := cs.Enqueue(cli); err != nil {
		t.Fatalf("test error %v", err)
	}

	err := cs.Remove(cli)
	assert.Nilf(t, err, "expect err to be nil")

	err = cs.Remove(cli)
	assert.Equalf(t, ErrEntryMissing, err, "expect err to be ErrEntryMissing")

	assert.Equalf(t, 0, cs.Len(), "expect len to be zero")
}

func TestClientResvQueue_Contains(t *testing.T) {
	cs := newClientResvQueue()

	cli := newTestClientResvEntry(1)
	if err := cs.Enqueue(cli); err != nil {
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

func TestClientResvQueue_Find(t *testing.T) {
	cs := newClientResvQueue()

	cli, err := cs.Find("abracadabra")
	assert.Equalf(t, ErrEntryMissing, err, "expectErrEntryMissing")

	cli = newTestClientResvEntry(1)
	if err := cs.Enqueue(cli); err != nil {
		t.Fatalf("test error %v", err)
	}

	resultCli, err := cs.Find(cli.CliID)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, cli.CliID, resultCli.CliID, "expect client to be found")
}

func TestClientResvQueue_Dequeue(t *testing.T) {
	cs := newClientResvQueue()

	cli1 := newTestClientResvEntry(1)
	cli2 := newTestClientResvEntry(1)
	cs.Enqueue(cli2)
	cs.Enqueue(cli1)

	resultCli, err := cs.Dequeue()
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, cli2.CliID, resultCli.CliID, "expect cli2 to be first result")
}

func TestClientResvQueue_Dequeue_PostRemove(t *testing.T) {
	cs := newClientResvQueue()

	cli1 := newTestClientResvEntry(1)
	cli2 := newTestClientResvEntry(1)
	cs.Enqueue(cli2)
	cs.Enqueue(cli1)

	resultCli, err := cs.Dequeue()
	cs.Remove(resultCli)

	resultCli, err = cs.Dequeue()
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, cli1.CliID, resultCli.CliID, "expect cli2 to be first result")
}

func TestClientResvQueue_Dequeue_Err(t *testing.T) {
	cs := newClientResvQueue()

	cli1 := newTestClientResvEntry(1)
	cli2 := newTestClientResvEntry(1)
	cs.Enqueue(cli2)
	cs.Enqueue(cli1)

	resultCli, err := cs.Dequeue()
	cs.Remove(resultCli)
	resultCli, err = cs.Dequeue()
	cs.Remove(resultCli)

	resultCli, err = cs.Dequeue()
	assert.Equalf(t, ErrContainerEmpty, err, "expect err to be ErrContainerEmpty")
	assert.Nilf(t, resultCli, "expect resultCli to be nil")
}

func TestClientResvMap_Put(t *testing.T) {
	c := make(clientResvMap)

	cli1 := newTestClientResvEntry(1)
	err := c.Put(cli1)
	assert.Nilf(t, err, "expect err to be nil")
}

func TestClientResvMap_Put_ReturnsErr(t *testing.T) {
	c := make(clientResvMap)

	cli1 := newTestClientResvEntry(1)
	err := c.Put(cli1)
	err = c.Put(cli1)
	assert.Truef(t, errors.Is(err, ErrEntryExists), "expect err to contain ErrEntryExists")
}

func TestClientResvMap_Get(t *testing.T) {
	c := make(clientResvMap)

	cli1 := newTestClientResvEntry(1)
	err := c.Put(cli1)

	cli2, err := c.Get(cli1.CliID)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, cli1, cli2, "expect two client resv entry to match")
}

func TestClientResvMap_Get_ReturnsErr(t *testing.T) {
	c := make(clientResvMap)
	_, err := c.Get(ClientID("abrcadabra"))
	assert.Equalf(t, ErrEntryMissing, err, "expect err to be ErrEntryMissing")
}

func TestClientResvMap_Remove(t *testing.T) {
	c := make(clientResvMap)

	cli1 := newTestClientResvEntry(1)
	c.Put(cli1)
	err := c.Remove(cli1.CliID)
	assert.Nilf(t, err, "expect err to be nil")
}

func TestClientResvMap_Remove_ReturnsErr(t *testing.T) {
	c := make(clientResvMap)
	err := c.Remove(ClientID("abrcadabra"))
	assert.Equalf(t, ErrEntryMissing, err, "expect err to be ErrEntryMissing")

	cli1 := newTestClientResvEntry(1)
	c.Put(cli1)
	c.Remove(cli1.CliID)
	err = c.Remove(cli1.CliID)
	assert.Equalf(t, ErrEntryMissing, err, "expect err to be ErrEntryMissing")
}

func newTestClientResvEntry(tickAt int64) *ClientResvEntry {
	cli := newClientResvEntry()
	cli.TickAt = tickAt
	return cli
}
