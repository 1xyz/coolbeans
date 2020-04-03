package state

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

type IndexEntry struct {
	job   Job
	entry *JobEntry
}

// An index of indexEntry indexed by a job ID
type JobIndex struct {
	m        map[JobID]*IndexEntry
	maxJobID JobID
}

func NewJobIndex() *JobIndex {
	return &JobIndex{
		m:        map[JobID]*IndexEntry{},
		maxJobID: JobID(0),
	}
}

// Return the maximum assigned JobID
func (idx *JobIndex) NextJobID() JobID {
	idx.maxJobID = idx.maxJobID + 1
	return idx.maxJobID
}

func (idx *JobIndex) Len() int {
	return len(idx.m)
}

func (idx *JobIndex) Add(job Job) (*IndexEntry, error) {
	if _, ok := idx.m[job.ID()]; ok {
		return nil, fmt.Errorf("job with id=%v exists %w", job.ID(), ErrEntryExists)
	}

	e := IndexEntry{
		job:   job,
		entry: nil,
	}
	idx.m[job.ID()] = &e
	return &e, nil
}

func (idx *JobIndex) Get(jobID JobID) (*IndexEntry, error) {
	entry, ok := idx.m[jobID]
	if !ok {
		return nil, ErrEntryMissing
	}
	return entry, nil
}

func (idx *JobIndex) Remove(jobID JobID) (*IndexEntry, error) {
	entry, ok := idx.m[jobID]
	if !ok {
		return nil, ErrEntryMissing
	}

	delete(idx.m, jobID)
	return entry, nil
}

// Return a read-only channel of jobs
func (idx *JobIndex) Jobs() <-chan Job {
	entriesCh := make(chan Job)
	go func(ch chan<- Job) {
		defer close(ch)
		for _, v := range idx.m {
			ch <- v.job
		}
	}(entriesCh)
	return entriesCh
}

// An map of tube state indexed by the tubeName
type tubeIndex map[TubeName]*tubeState
type tubeState struct {
	// Min heap of jobs (ordered by the ReadyAt)
	delayedJobs DelayedJobs

	// Min heap of jobs (ordered by job pri)
	// if two jobs are of same pri, older (by id) one is ahead
	readyJobs PriorityJobs

	// Set of clientHeap that are waiting for reservations
	waiting *clientResvQueue
}

func (ti tubeIndex) getByName(tubeName TubeName, create bool) (*tubeState, error) {
	res, ok := ti[tubeName]
	if !ok {
		if !create {
			return nil, ErrEntryMissing
		}

		log.Debugf("tubeStates.getTube created tube=%v", tubeName)
		ti[tubeName] = &tubeState{
			delayedJobs: NewDelayedJobs(),
			readyJobs:   NewPriorityJobs(),
			waiting:     newClientResvQueue(),
		}
		return ti[tubeName], nil
	} else {
		return res, nil
	}
}

func (ti tubeIndex) cleanup(tubeName TubeName) {
	if t, ok := ti[tubeName]; !ok {
		return
	} else if t.readyJobs.Len() == 0 && t.delayedJobs.Len() == 0 && t.waiting.Len() == 0 {
		delete(ti, tubeName)
		t.readyJobs = nil
		t.delayedJobs = nil
	}
}

func (ti tubeIndex) EnqueueReadyJob(job Job) (*JobEntry, error) {
	t, err := ti.getByName(job.TubeName(), true)
	if err != nil {
		return nil, err
	}
	return t.readyJobs.Enqueue(job), err
}

func (ti tubeIndex) EnqueueDelayedJob(job Job) (*JobEntry, error) {
	t, err := ti.getByName(job.TubeName(), true)
	if err != nil {
		return nil, err
	}
	return t.delayedJobs.Enqueue(job), nil
}

func (ti tubeIndex) RemoveDelayedJob(jobEntry *JobEntry) (*JobEntry, error) {
	tubeName := jobEntry.TubeName()
	t, err := ti.getByName(tubeName, false)
	if err != nil {
		return nil, err
	}

	res, err := t.delayedJobs.RemoveAt(jobEntry)
	ti.cleanup(tubeName)
	return res, err
}

func (ti tubeIndex) RemoveReadyJob(jobEntry *JobEntry) (*JobEntry, error) {
	t, err := ti.getByName(jobEntry.TubeName(), false)
	if err != nil {
		return nil, err
	}

	return t.readyJobs.RemoveAt(jobEntry)
}

func (ti tubeIndex) NextDelayedJob(tubeName TubeName) (*JobEntry, error) {
	t, err := ti.getByName(tubeName, false)
	if err != nil {
		return nil, ErrEntryMissing
	}

	if t.delayedJobs.Len() == 0 {
		return nil, ErrEntryMissing
	}

	return t.delayedJobs.Peek(), nil
}

func (ti tubeIndex) NextReadyJob(tubeName TubeName) (*JobEntry, error) {
	t, err := ti.getByName(tubeName, false)
	if err != nil {
		return nil, ErrEntryMissing
	}
	if t.readyJobs.Len() == 0 {
		return nil, ErrEntryMissing
	}
	return t.readyJobs.Peek(), nil
}

func (ti tubeIndex) EnqueueToWaitQ(tubeName TubeName, cli *ClientResvEntry) error {
	ts, err := ti.getByName(tubeName, true)
	if err != nil {
		return err
	}

	return ts.waiting.Enqueue(cli)
}

func (ti tubeIndex) WaitQLen(tubeName TubeName) (int, error) {
	if t, err := ti.getByName(tubeName, false); err != nil {
		return -1, err
	} else {
		return t.waiting.Len(), nil
	}
}

func (ti tubeIndex) DequeueFromWaitQ(tubeName TubeName) (*ClientResvEntry, error) {
	if t, err := ti.getByName(tubeName, false); err != nil {
		return nil, err
	} else {
		return t.waiting.Dequeue()
	}
}

func (ti tubeIndex) RemoveFromWaitQ(tubeName TubeName, cli *ClientResvEntry) error {
	if t, err := ti.getByName(tubeName, false); err != nil {
		return err
	} else {
		return t.waiting.Remove(cli)
	}
}

// An map of jobs reserved where the key is the CliID
type reservedJobsIndex map[ClientID]ReservedJobs

func (r reservedJobsIndex) Enqueue(clientID ClientID, job Job) (*JobEntry, error) {
	_, ok := r[clientID]
	if !ok {
		r[clientID] = NewReservedJobs()
	}
	return r[clientID].Enqueue(job), nil
}

func (r reservedJobsIndex) Remove(jobEntry *JobEntry) (Job, error) {
	clientID := jobEntry.ReservedBy()
	_, ok := r[clientID]
	if !ok {
		log.WithField("method", "RemoveReadyJob").
			Errorf("cannot find a reservation list for client=%v", clientID)
		return nil, ErrEntryMissing
	}

	defer func() {
		if r[clientID].Len() == 0 {
			delete(r, clientID)
		}
	}()

	if jobEntry.index == 0 {
		return r[clientID].Dequeue(), nil
	} else {
		return r[clientID].RemoveAt(jobEntry)
	}
}

func (r reservedJobsIndex) NextReservedJob(clientID ClientID) (*JobEntry, error) {
	resvJobs, ok := r[clientID]
	if !ok {
		log.WithField("method", "reservedJobsState.NextReservedJob").
			Debugf("no reservedJobList for CliID=%v", clientID)
		return nil, ErrEntryMissing
	}

	if resvJobs.Len() == 0 {
		log.WithField("method", "reservedJobsState.NextReservedJob").
			Debugf("reservedJobList for CliID=%v has zero entries", clientID)
		return nil, ErrEntryMissing
	}

	return r[clientID].Peek(), nil
}
