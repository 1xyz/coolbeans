package state

import (
	"container/heap"
	"container/list"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"math"
)

// Represents a client reservation as requested by the client
type ClientResvEntry struct {
	// ID of the client making reservations
	CliID ClientID

	// Tubes are the tube names watched by this client
	// for this reservation
	WatchedTubes []TubeName

	// reservation Deadline timestamp
	ResvDeadlineAt int64

	// Indicates if this entry is waiting for a reservation
	IsWaitingForResv bool

	// clock at which the client needs some processing
	// If client IsWaitingForResv is set to true
	// - if there is a job already reserved at the lowest job's
	//   deadline is within a second of now, then send a DEADLINE_SOON
	//   and un-reserve the client
	// - If there are no jobs reserved at the current time is past
	//   the client's reservation ResvDeadlineAt, then un-reserve the client
	// Check to see if any reservations need to be cleaned
	TickAt int64

	// the request ID of the reservation
	ReqID string

	// Index of the client in the client Heap
	HeapIndex int
}

func newClientResvEntry() *ClientResvEntry {
	return &ClientResvEntry{
		CliID:          ClientID(uuid.New().URN()),
		ReqID:          uuid.New().URN(),
		TickAt:         0,
		WatchedTubes:   make([]TubeName, 0),
		ResvDeadlineAt: math.MaxInt64,
		HeapIndex:      -1,
	}
}

// A heap of client reservations indexed prioritized by
// the clientResvEntry TickAt. A lower TickAt gets a higher
// priority.
type clientResvHeap []*ClientResvEntry

// create and initialize a client resv heap
func newClientResvHeap() *clientResvHeap {
	h := make(clientResvHeap, 0)
	heap.Init(&h)
	return &h
}

// create an un-initialized heap (typically needed for snapshot restore)
func newClientResvHeapWithSize(size int) *clientResvHeap {
	h := make(clientResvHeap, size)
	return &h
}

// initialize a populated heap
func initializeHeap(h heap.Interface) {
	heap.Init(h)
}

func (h clientResvHeap) Len() int {
	return len(h)
}

func (h clientResvHeap) Less(i, j int) bool {
	// we want to pop the entry with the lowest TickAt first
	// in order to get total ordering if the two TickAt's
	// are the same then break the tie on CliID
	if h[i].TickAt == h[j].TickAt {
		return h[i].CliID < h[j].CliID
	}
	return h[i].TickAt < h[j].TickAt
}

func (h clientResvHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].HeapIndex = i
	h[j].HeapIndex = j
}

func (h *clientResvHeap) Enqueue(cli *ClientResvEntry) {
	heap.Push(h, cli)
}

func (h *clientResvHeap) Dequeue() *ClientResvEntry {
	cli, ok := heap.Pop(h).(*ClientResvEntry)
	if !ok {
		log.Panicf("cast-error, interface %T cannot be cast to *clientResvEntry", cli)
	}

	return cli
}

func (h *clientResvHeap) Remove(cli *ClientResvEntry) error {
	if cli.HeapIndex >= h.Len() {
		return ErrInvalidIndex
	}

	if cli.CliID != (*h)[cli.HeapIndex].CliID {
		return ErrMismatchJobEntry
	}

	_, ok := heap.Remove(h, cli.HeapIndex).(*ClientResvEntry)
	if !ok {
		log.Panicf("cast-error, interface  cannot be cast to *clientResvEntry")
	}

	log.WithField("client.id", cli.CliID).Debugf("clientHeap.remove success")
	return nil
}

func (h *clientResvHeap) Peek() *ClientResvEntry {
	if h.Len() == 0 {
		return nil
	} else {
		return (*h)[0]
	}
}

// update modifies the priority and value of an Item in the queue.
func (h *clientResvHeap) UpdateTickAt(cli *ClientResvEntry, newTickAt int64) {
	cli.TickAt = newTickAt
	heap.Fix(h, cli.HeapIndex)
}

func (h *clientResvHeap) Push(x interface{}) {
	n := h.Len()
	item, ok := x.(*ClientResvEntry)
	if !ok {
		log.Panicf("cast-error, interface %T cannot be cast to *client", x)
	}

	item.HeapIndex = n
	*h = append(*h, item)
}

func (h *clientResvHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil      // avoid memory leak
	item.HeapIndex = -1 // for safety
	*h = old[0 : n-1]
	return item
}

// ClientResvQueue is container that provides two functions.
// 1. queue of client reservations that supports Enqueue & Dequeue operations
// 2. An index (map) of client reservations indexed by CliID, that provides
//    O(1) operations to Find, Contains & Remove
type clientResvQueue struct {
	m map[ClientID]*list.Element

	l *list.List
}

func newClientResvQueue() *clientResvQueue {
	return &clientResvQueue{
		m: map[ClientID]*list.Element{},
		l: list.New(),
	}
}

func (c *clientResvQueue) Enqueue(entry *ClientResvEntry) error {
	if c.Contains(entry) {
		return fmt.Errorf("entry with clientID=%s exists %w", entry.CliID, ErrEntryExists)
	}

	elem := c.l.PushBack(entry)
	c.m[entry.CliID] = elem
	return nil
}

func (c *clientResvQueue) Find(clientID ClientID) (*ClientResvEntry, error) {
	elem, ok := c.m[clientID]
	if !ok {
		return nil, ErrEntryMissing
	}

	res, ok := elem.Value.(*ClientResvEntry)
	if !ok {
		log.Panicf("cast-error expected value %T to be of type *clientResvEntry", elem.Value)
	}

	return res, nil
}

func (c *clientResvQueue) Contains(entry *ClientResvEntry) bool {
	_, ok := c.m[entry.CliID]
	return ok
}

func (c *clientResvQueue) Remove(entry *ClientResvEntry) error {
	elem, ok := c.m[entry.CliID]
	if !ok {
		return ErrEntryMissing
	}

	_, ok = elem.Value.(*ClientResvEntry)
	if !ok {
		log.Panicf("cast-error expected value %T to be of type *clientResvEntry", elem.Value)
	}

	c.l.Remove(elem)
	elem = nil

	delete(c.m, entry.CliID)
	return nil
}

func (c *clientResvQueue) Len() int {
	return len(c.m)
}

func (c *clientResvQueue) Dequeue() (*ClientResvEntry, error) {
	if c.l.Len() == 0 {
		return nil, ErrContainerEmpty
	}

	firstElem := c.l.Front()

	res, ok := firstElem.Value.(*ClientResvEntry)
	if !ok {
		log.Panicf("cast-error expected value %T to be of type *clientResvEntry", firstElem.Value)
	}

	return res, nil
}

// Return a read-only channel of *clientResvEntry
func (c *clientResvQueue) Entries() <-chan *ClientResvEntry {
	entriesCh := make(chan *ClientResvEntry)
	go func(ch chan<- *ClientResvEntry) {
		defer close(ch)
		for e := c.l.Front(); e != nil; e = e.Next() {
			cli, ok := e.Value.(*ClientResvEntry)
			if !ok {
				log.WithField("method", "clientResvQueue.Entries").
					Panicf("class-cast error")
			}

			if cli.HeapIndex < 0 {
				log.WithField("method", "clientResvQueue.Entries").
					Warnf("cli %v with heapIndex = %v is skipped", cli.CliID, cli.HeapIndex)
			} else {
				ch <- cli
			}
		}
	}(entriesCh)
	return entriesCh
}

// clientResvMap is a map from ClientID to a *clientResvEntry
type clientResvMap map[ClientID]*ClientResvEntry

func (c clientResvMap) Contains(id ClientID) bool {
	_, ok := c[id]
	return ok
}

func (c clientResvMap) Put(entry *ClientResvEntry) error {
	if c.Contains(entry.CliID) {
		return fmt.Errorf("entry with clientID:%v exists. %w", entry.CliID, ErrEntryExists)
	}

	c[entry.CliID] = entry
	return nil
}

func (c clientResvMap) Get(id ClientID) (*ClientResvEntry, error) {
	entry, ok := c[id]
	if !ok {
		return nil, ErrEntryMissing
	}

	return entry, nil
}

func (c clientResvMap) Remove(id ClientID) error {
	if !c.Contains(id) {
		return ErrEntryMissing
	}

	delete(c, id)
	return nil
}

func (c clientResvMap) AddOrUpdate(id ClientID, watchedTubes []TubeName, reqID string, resvDeadlineAt int64) (*ClientResvEntry, error) {
	if entry, ok := c[id]; ok {
		entry.ReqID = reqID
		entry.ResvDeadlineAt = resvDeadlineAt

		return entry, nil
	}

	entry := &ClientResvEntry{
		CliID:            id,
		WatchedTubes:     watchedTubes,
		ResvDeadlineAt:   resvDeadlineAt,
		IsWaitingForResv: true,
		TickAt:           0,
		ReqID:            reqID,
		HeapIndex:        -1,
	}

	if err := c.Put(entry); err != nil {
		return nil, err
	}

	return entry, nil
}
