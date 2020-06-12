package state

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

// IndexEntry is an entry within the job Index
type IndexEntry struct {
	job   Job
	entry *JobEntry
}

// JobIndex is an index of indexEntry indexed by a job ID
type JobIndex struct {
	m        map[JobID]*IndexEntry
	maxJobID JobID
}

// NewJobIndex returns a  pointer to a new JobIndex.
func NewJobIndex() *JobIndex {
	return &JobIndex{
		m:        map[JobID]*IndexEntry{},
		maxJobID: JobID(0),
	}
}

// NextJobID returns the maximum assigned JobID
func (idx *JobIndex) NextJobID() JobID {
	idx.maxJobID = idx.maxJobID + 1
	return idx.maxJobID
}

// Len returns the number of jobs in this index
func (idx *JobIndex) Len() int {
	return len(idx.m)
}

// Add a job to the index
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

// Get returns the job specified by the jobID
func (idx *JobIndex) Get(jobID JobID) (*IndexEntry, error) {
	entry, ok := idx.m[jobID]
	if !ok {
		return nil, ErrEntryMissing
	}
	return entry, nil
}

// Remove deletes the job from the index
func (idx *JobIndex) Remove(jobID JobID) (*IndexEntry, error) {
	entry, ok := idx.m[jobID]
	if !ok {
		return nil, ErrEntryMissing
	}

	delete(idx.m, jobID)
	return entry, nil
}

// Jobs returns a read-only channel of jobs
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

	// Min heap of jobs (ordered by buriedAt)
	buriedJobs BuriedJobs
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
			buriedJobs:  NewBuriedJobs(),
		}
		return ti[tubeName], nil
	} else {
		return res, nil
	}
}

func (ti tubeIndex) cleanup(tubeName TubeName) {
	if t, ok := ti[tubeName]; !ok {
		return
	} else if t.readyJobs.Len() == 0 && t.delayedJobs.Len() == 0 && t.waiting.Len() == 0 && t.buriedJobs.Len() == 0 {
		delete(ti, tubeName)
		t.readyJobs = nil
		t.delayedJobs = nil
		t.buriedJobs = nil
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

func (ti tubeIndex) EnqueueBuriedJob(job Job) (*JobEntry, error) {
	t, err := ti.getByName(job.TubeName(), true)
	if err != nil {
		return nil, err
	}
	return t.buriedJobs.Enqueue(job), nil
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

func (ti tubeIndex) RemoveBuriedJob(jobEntry *JobEntry) (*JobEntry, error) {
	t, err := ti.getByName(jobEntry.TubeName(), false)
	if err != nil {
		return nil, err
	}

	return t.buriedJobs.RemoveAt(jobEntry)
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

func (ti tubeIndex) NextBuriedJob(tubeName TubeName) (*JobEntry, error) {
	t, err := ti.getByName(tubeName, false)
	if err != nil {
		return nil, ErrEntryMissing
	}
	if t.buriedJobs.Len() == 0 {
		return nil, ErrEntryMissing
	}
	return t.buriedJobs.Peek(), nil
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

func (ti tubeIndex) GetTubeNames() []TubeName {
	result := make([]TubeName, 0)
	for n := range ti {
		result = append(result, n)
	}
	return result
}

func (ti tubeIndex) GetStatistics(tubeName TubeName) (map[string]interface{}, error) {
	t, err := ti.getByName(tubeName, false /*create*/)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"name":                  string(tubeName),
		"current-jobs-urgent":   0,
		"current-jobs-ready":    t.readyJobs.Len(),
		"current-jobs-reserved": 0,
		"current-jobs-delayed":  t.delayedJobs.Len(),
		"current-jobs-buried":   t.buriedJobs.Len(),
		"total-jobs":            0,
		"current-using":         0,
		"current-waiting":       t.waiting.Len(),
		"current-watching":      0,
		"pause":                 0,
		"cmd-delete":            0,
		"cmd-pause-tube":        0,
		"pause-time-left":       0,
	}, nil
}

func (ti tubeIndex) TotalJobCounts() map[string]uint64 {
	s := map[string]uint64{
		"current-jobs-urgent":   0,
		"current-jobs-ready":    0,
		"current-jobs-reserved": 0,
		"current-jobs-delayed":  0,
		"current-jobs-buried":   0,
	}
	for _, t := range ti {
		s["current-jobs-ready"] += uint64(t.readyJobs.Len())
		s["current-jobs-delayed"] += uint64(t.delayedJobs.Len())
		s["current-jobs-buried"] += uint64(t.buriedJobs.Len())
	}
	return s
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

func (r reservedJobsIndex) JobCount(tubeName TubeName) uint64 {
	var count uint64 = 0
	for _, rj := range r {
		count += rj.JobCountByTube(tubeName)
	}
	return count
}

func (r reservedJobsIndex) TotalJobs() uint64 {
	var count uint64 = 0
	for _, rj := range r {
		count += uint64(rj.Len())
	}
	return count
}
