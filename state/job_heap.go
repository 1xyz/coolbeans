package state

import (
	"container/heap"
	log "github.com/sirupsen/logrus"
)

// JobHeap is a binary heap of jobs
type JobHeap interface {
	// Enqueue appends an entry to the job heap in priority order
	Enqueue(job Job) *JobEntry

	// Dequeue returns a from the heap in priority order
	Dequeue() Job

	// RemoveAt removes a specific job entry.
	RemoveAt(jobEntry *JobEntry) (*JobEntry, error)

	// Len returns the heap length
	Len() int

	// Peek returns the top element of the heap without dequeuing it
	Peek() *JobEntry

	// Return the number of jobs found in the specific tube
	JobCountByTube(tubename TubeName) uint64
}

// JobEntry is an entry in the JobHeap
type JobEntry struct {
	Job

	// Represents the index of this entry in the Heap
	index int
}

type jobHeap struct {
	entries []*JobEntry
	lessFn  func(i, j int) bool // Customizable Less function
}

func (jh *jobHeap) Peek() *JobEntry {
	if jh.Len() == 0 {
		return nil
	} else {
		return jh.entries[0]
	}
}

func (jh *jobHeap) Enqueue(j Job) *JobEntry {
	e := &JobEntry{
		Job:   j,
		index: 0,
	}

	heap.Push(jh, e)
	return e
}

func (jh *jobHeap) Dequeue() Job {
	e, ok := heap.Pop(jh).(*JobEntry)
	if !ok {
		log.Panicf("cast-error, interface %T cannot be cast to *JobEntry", e)
	}

	return e.Job
}

func (jh *jobHeap) RemoveAt(entry *JobEntry) (*JobEntry, error) {
	if entry.index >= jh.Len() {
		return nil, ErrInvalidIndex
	}

	if entry.ID() != jh.entries[entry.index].ID() {
		return nil, ErrMismatchJobEntry
	}

	r, ok := heap.Remove(jh, entry.index).(*JobEntry)
	if !ok {
		log.Panicf("cast-error, interface  cannot be cast to *JobEntry")
	}

	return r, nil
}

func (jh *jobHeap) Len() int {
	return len(jh.entries)
}

func (jh jobHeap) Less(i, j int) bool {
	return jh.lessFn(i, j)
}

func (jh jobHeap) Swap(i, j int) {
	jh.entries[i], jh.entries[j] = jh.entries[j], jh.entries[i]
	jh.entries[i].index = i
	jh.entries[j].index = j
}

func (jh *jobHeap) Push(x interface{}) {
	n := jh.Len()
	item, ok := x.(*JobEntry)
	if !ok {
		log.Panicf("cast-error, interface %T cannot be cast to *JobEntry", x)
	}

	item.index = n
	jh.entries = append(jh.entries, item)
}

func (jh *jobHeap) Pop() interface{} {
	old := jh.entries
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	jh.entries = old[0 : n-1]
	return item
}

func (jh *jobHeap) JobCountByTube(tubeName TubeName) uint64 {
	var count uint64 = 0
	for _, e := range jh.entries {
		if e.TubeName() == tubeName {
			count++
		}
	}
	return count
}

// PriorityJobs is a JobHeap, with jobs ordered by its
// Priority. Lower priority values takes a higher precedence.
type PriorityJobs JobHeap

func NewPriorityJobs() PriorityJobs {
	pq := jobHeap{
		entries: make([]*JobEntry, 0),
	}
	pq.lessFn = func(i, j int) bool {
		if pq.entries[i].Priority() == pq.entries[j].Priority() {
			return pq.entries[i].ID() < pq.entries[j].ID()
		} else {
			return pq.entries[i].Priority() < pq.entries[j].Priority()
		}
	}
	heap.Init(&pq)
	return &pq
}

// DelayedJobs is a JobHeap, with jobs ordered by its ReadyAt field.
// Lower (earlier) ReadyAt takes a higher precedence.
type DelayedJobs JobHeap

func NewDelayedJobs() DelayedJobs {
	pq := jobHeap{
		entries: make([]*JobEntry, 0),
	}
	pq.lessFn = func(i, j int) bool {
		if pq.entries[i].ReadyAt() == pq.entries[j].ReadyAt() {
			return pq.entries[i].ID() < pq.entries[j].ID()
		}
		return pq.entries[i].ReadyAt() < pq.entries[j].ReadyAt()
	}
	heap.Init(&pq)
	return &pq
}

// ReservedJobs is a JobHeap, with jobs ordered by its ExpiresAt
// field. Lower (earlier) ExpiresAt take a higher precedence
type ReservedJobs JobHeap

func NewReservedJobs() ReservedJobs {
	pq := jobHeap{
		entries: make([]*JobEntry, 0),
	}
	pq.lessFn = func(i, j int) bool {
		if pq.entries[i].ExpiresAt() == pq.entries[j].ExpiresAt() {
			return pq.entries[i].ID() < pq.entries[j].ID()
		}
		return pq.entries[i].ExpiresAt() < pq.entries[j].ExpiresAt()
	}
	heap.Init(&pq)
	return &pq
}

// BuriedJobs is a JobHeap, with jobs ordered by its BuriedAt
// field. Lower (earlier) BuriedAt take a higher precedence
// If two jobs have the same BuriedAt value, the lower job id gets precedence
type BuriedJobs JobHeap

func NewBuriedJobs() ReservedJobs {
	pq := jobHeap{
		entries: make([]*JobEntry, 0),
	}
	pq.lessFn = func(i, j int) bool {
		if pq.entries[i].BuriedAt() == pq.entries[j].BuriedAt() {
			return pq.entries[i].ID() < pq.entries[j].ID()
		}
		return pq.entries[i].BuriedAt() < pq.entries[j].BuriedAt()
	}
	heap.Init(&pq)
	return &pq
}
