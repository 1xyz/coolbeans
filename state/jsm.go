package state

import (
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"math"
	"time"
)

const (
	maxTime = math.MaxInt64
)

type localJSM struct {
	// maxJobID is the maximum allocated JobID
	maxJobID JobID

	// jobs is a container that indexes job Entries by jobID
	jobs *JobIndex

	// tubes is a container of delayed & reserved jobs indexed by tube name
	tubes tubeIndex

	// clientHeap is a priority queue of clientResvEntries ordered by the TickAt
	clientHeap *clientResvHeap

	// clients is an index clientResvEntries by CliID
	clients *clientResvQueue

	// reservedJobs is a container of reserved jobs heaps indexed by CliID
	reservedJobs reservedJobsIndex

	// clock indicating when a the soonest delayed job is ready
	nextDelayTickAt int64

	stat *procStats
}

func NewJSM() (JSM, error) {
	return &localJSM{
		maxJobID:        JobID(0),
		jobs:            NewJobIndex(),
		tubes:           make(tubeIndex),
		clientHeap:      newClientResvHeap(),
		clients:         newClientResvQueue(),
		reservedJobs:    make(reservedJobsIndex),
		nextDelayTickAt: maxTime,
		stat: &procStats{
			nResv:         0,
			nResvOnDemand: 0,
			nResvOnTick:   0,
		},
	}, nil
}

// Return the maximum assigned JobID
func (jsm *localJSM) NextJobID() JobID {
	jsm.maxJobID = jsm.maxJobID + 1
	return jsm.maxJobID
}

func (jsm *localJSM) Put(nowSeconds int64,
	priority uint32,
	delay int64,
	ttr int,
	bodySize int,
	body []byte,
	tubeName TubeName) (JobID, error) {

	newJob, err := jsm.NewJob(nowSeconds, priority, delay, ttr, bodySize, body, tubeName)
	if err != nil {
		return JobID(0), err
	} else {
		return newJob.ID(), nil
	}
}

func (jsm *localJSM) NewJob(nowSeconds int64,
	priority uint32,
	delay int64,
	ttr int,
	bodySize int,
	body []byte,
	tubeName TubeName) (Job, error) {

	newJob := &localJob{
		id:        uint64(jsm.NextJobID()),
		priority:  priority,
		delay:     delay,
		ttr:       ttr,
		bodySize:  bodySize,
		body:      body,
		tubeName:  tubeName,
		createdAt: nowSeconds,
		readyAt:   nowSeconds + delay,
		state:     Initial,
		expiresAt: 0,
	}

	if newJob.Delay() > 0 {
		if newJob.ReadyAt() < jsm.nextDelayTickAt {
			jsm.nextDelayTickAt = newJob.ReadyAt()
		}

		return newJob, jsm.fromInitialToDelayed(newJob)
	} else {
		return newJob, jsm.fromInitialToReady(newJob)
	}
}

func (jsm *localJSM) GetJob(jobID JobID) (Job, error) {
	e, err := jsm.jobs.Get(jobID)
	if err != nil {
		return nil, err
	}

	return e.job, nil
}

func (jsm *localJSM) Delete(jobID JobID, clientID ClientID) error {
	if e, err := jsm.jobs.Get(jobID); err != nil {
		return err
	} else {
		if e.job.State() == Reserved && e.job.ReservedBy() != clientID {
			log.WithField("method", "Delete").Errorf(
				"client: %v cannot delete reserved job %v owned by %v", clientID, e.job.ID(), e.job.ReservedBy())
			return ErrUnauthorizedOperation
		}
	}

	e, err := jsm.jobs.Remove(jobID)
	if err != nil {
		return err
	}

	switch e.job.State() {
	case Ready:
		_, err = jsm.tubes.RemoveReadyJob(e.entry)
		if err != nil {
			return err
		}
	case Delayed:
		_, err := jsm.tubes.RemoveDelayedJob(e.entry)
		if err != nil {
			return err
		}
	case Reserved:
		_, err := jsm.reservedJobs.Remove(e.entry)
		if err != nil {
			return err
		}
	}
	e.job.UpdateState(Deleted) // so that the job state is not erroneously used
	e.job.UpdateReservedBy("")
	e.entry = nil
	e.job = nil
	return nil
}

func (jsm *localJSM) NextDelayedJob(tubeName TubeName) (Job, error) {
	return jsm.tubes.NextDelayedJob(tubeName)
}

func (jsm *localJSM) NextReadyJob(tubeName TubeName) (Job, error) {
	return jsm.tubes.NextReadyJob(tubeName)
}

func (jsm *localJSM) NextReservedJob(clientID ClientID) (Job, error) {
	return jsm.reservedJobs.NextReservedJob(clientID)
}

func (jsm *localJSM) Ready(jobID JobID) error {
	return jsm.fromDelayedToReady(jobID)
}

// Reserve this job (from a ready state)
func (jsm *localJSM) Reserve(nowSeconds int64, jobID JobID, clientID ClientID) error {
	return jsm.fromReadyToReserved(nowSeconds, jobID, clientID)
}

// Release this job to the ready state (from a reserved to ready state)
func (jsm *localJSM) Release(jobID JobID, clientID ClientID) error {
	return jsm.fromReservedToReady(jobID, clientID)
}

func (jsm *localJSM) fromInitialToReady(job Job) error {
	if job.State() != Initial {
		return ErrInvalidJobTransition
	}

	e, err := jsm.jobs.Add(job)
	if err != nil {
		return err
	}

	job.UpdateState(Ready)
	if je, err := jsm.tubes.EnqueueReadyJob(job); err != nil {
		return err
	} else {
		e.entry = je
		return nil
	}
}

func (jsm *localJSM) fromInitialToDelayed(job Job) error {
	if job.State() != Initial {
		return ErrInvalidJobTransition
	}

	e, err := jsm.jobs.Add(job)
	if err != nil {
		return err
	}

	job.UpdateState(Delayed)
	if je, err := jsm.tubes.EnqueueDelayedJob(job); err != nil {
		return err
	} else {
		e.entry = je
		return nil
	}
}

func (jsm *localJSM) fromDelayedToReady(jobID JobID) error {
	e, err := jsm.jobs.Get(jobID)
	if err != nil {
		return err
	}
	if e.job.State() != Delayed {
		return ErrInvalidJobTransition
	}
	if e.entry.index != 0 {
		return ErrInvalidJobTransition
	}

	_, err = jsm.tubes.RemoveDelayedJob(e.entry)
	if err != nil {
		return err
	}

	e.job.UpdateState(Ready)
	if je, err := jsm.tubes.EnqueueReadyJob(e.job); err != nil {
		return err
	} else {
		e.entry = je
		return nil
	}
}

func (jsm *localJSM) fromReservedToReady(jobID JobID, clientID ClientID) error {
	e, err := jsm.jobs.Get(jobID)
	if err != nil {
		return err
	}
	if e.job.State() != Reserved {
		return ErrInvalidJobTransition
	}
	if len(clientID) > 0 && e.job.ReservedBy() != clientID {
		return ErrUnauthorizedOperation
	}

	_, err = jsm.reservedJobs.Remove(e.entry)
	if err != nil {
		return err
	}

	e.job.UpdateState(Ready)
	e.job.UpdateReservedBy("")
	if je, err := jsm.tubes.EnqueueReadyJob(e.job); err != nil {
		return err
	} else {
		e.entry = je
		return nil
	}
}

func (jsm *localJSM) fromReadyToReserved(nowSeconds int64, jobID JobID, clientID ClientID) error {
	e, err := jsm.jobs.Get(jobID)
	if err != nil {
		return err
	}
	if e.job.State() != Ready {
		return ErrInvalidJobTransition
	}

	_, err = jsm.tubes.RemoveReadyJob(e.entry)
	if err != nil {
		return err
	}

	_, err = e.job.UpdateReservation(nowSeconds)
	if err != nil {
		return err
	}
	e.job.UpdateState(Reserved)
	e.job.UpdateReservedBy(clientID)
	if je, err := jsm.reservedJobs.Enqueue(clientID, e.job); err != nil {
		return err
	} else {
		e.entry = je
		return nil
	}
}

const maxTimeoutSecs = 24 * 3600

func (jsm *localJSM) AppendReservation(clientId ClientID, reqID string, watchedTubes []TubeName, nowSecs, resvDeadlineAt int64) (*Reservation, error) {
	if resvDeadlineAt < nowSecs || resvDeadlineAt > nowSecs+maxTimeoutSecs {
		// reservation timeout of infinity is not accepted
		return nil, ErrInvalidResvTimeout
	}

	logc := log.WithFields(log.Fields{
		"method": "localJsm.AppendReservation",
		"CliID":  clientId,
		"ReqID":  reqID})
	cli, err := jsm.clients.Find(clientId)
	if err == ErrEntryMissing {
		cli = &ClientResvEntry{
			CliID:            clientId,
			WatchedTubes:     watchedTubes,
			ResvDeadlineAt:   resvDeadlineAt,
			IsWaitingForResv: true,
			TickAt:           0,
			ReqID:            reqID,
			HeapIndex:        -1,
		}

		if err := jsm.clients.Enqueue(cli); err != nil {
			logc.Errorf("jsm.clients.put err=%v", err)
			return nil, err
		}
	} else if err != nil {
		logc.Errorf("jsm.clients.geterr=%v", err)
		return nil, err
	} else if cli.IsWaitingForResv {
		return nil, ErrClientIsWaitingForReservation
	} else {
		cli.ReqID = reqID
		cli.WatchedTubes = watchedTubes
		cli.ResvDeadlineAt = resvDeadlineAt
		cli.IsWaitingForResv = true
	}

	// start waiting for reservations on all these tubes
	for _, t := range cli.WatchedTubes {
		err := jsm.tubes.EnqueueToWaitQ(t, cli)
		if err != nil {
			logc.Panicf("EnqueueToWaitQ tubeName:%v err:%v", t, err)
		}
	}

	for _, t := range cli.WatchedTubes {
		resv, err := jsm.tryReserveJob(nowSecs, t, cli)
		if err == ErrNoReservationFound {
			continue
		} else if err != nil {
			logc.Panicf("jsm.tryReserveJob unhandled error %v", err)
		} else {
			jsm.stat.nResv += 1
			jsm.stat.nResvOnDemand += 1
			return resv, nil
		}
	}

	jsm.updateClientTickAt(cli)

	// Respond indicating a Queued response
	return &Reservation{
		RequestId: reqID,
		ClientId:  cli.CliID,
		JobId:     JobID(0),
		Status:    Queued,
		BodySize:  0,
		Body:      nil,
		Error:     nil,
	}, nil
}

func (jsm *localJSM) Tick(nowSecs int64) ([]*Reservation, error) {
	res := make([]*Reservation, 0)
	if nowSecs > jsm.nextDelayTickAt {
		jsm.nextDelayTickAt = maxTime
		jsm.tickDelayJobs(nowSecs)
	}

	if r, err := jsm.tickReadyJobs(nowSecs); err != nil {
		log.WithField("method", "jsm.Tick").
			Errorf("jsm.tickReadyJobs err=%v", err)
		return nil, err
	} else {
		res = append(res, r...)
	}

	res = append(res, jsm.tickClients(nowSecs)...)

	log.Infof("reserved %v viaTick=%v viaDemand=%v",
		jsm.stat.nResv, jsm.stat.nResvOnTick, jsm.stat.nResvOnDemand)
	return res, nil
}

// CheckClientState queries the job state machine whether the provided list of clientIds are waiting for reservations.
//
// The response returns the ClientIDs (i) which are waiting for reservations, (ii) those which are not and (iii)
// those which are missing.
func (jsm *localJSM) CheckClientState(clientIDs []ClientID) ([]ClientID, []ClientID, []ClientID, error) {
	waitingIds, notWaitingIds, missingIds :=
		make([]ClientID, 0), make([]ClientID, 0), make([]ClientID, 0)

	for _, id := range clientIDs {
		cli, err := jsm.clients.Find(id)
		if err == ErrEntryMissing {
			missingIds = append(missingIds, id)
			continue
		} else if err != nil {
			log.Errorf("CheckClientState: jsm.clients.Find clientId=%v. err = %v", id, err)
			return nil, nil, nil, err
		}

		if cli.IsWaitingForResv {
			waitingIds = append(waitingIds, id)
			continue
		}

		notWaitingIds = append(notWaitingIds, id)
	}

	return waitingIds, notWaitingIds, missingIds, nil
}

func (jsm *localJSM) Snapshot() (JSMSnapshot, error) {
	return newLocalSnapshot(jsm), nil
}

// tickDelayJobs - dequeue DELAYED jobs which are no longer delayed
// and enqueue them to the READY queue.
func (jsm *localJSM) tickDelayJobs(now int64) {
	logc := log.WithField("method", "cmdProcessor.tickDelayJobs")
	var nextDelayedTickAt int64 = maxTime
	count := 0
	for t, _ := range jsm.tubes {
		for {
			// Query the state machine for the next delayed job
			job, err := jsm.NextDelayedJob(t)
			if err == ErrEntryMissing {
				// No delayed jobs are present
				break
			} else if err != nil {
				// Un-Expected error
				logc.Panicf("jsm.NextDelayedJob(tubename=%v) err=%v", t, err)
			}

			if job.ReadyAt() >= now {
				if job.ReadyAt() < nextDelayedTickAt {
					nextDelayedTickAt = job.ReadyAt()
				}
				// no job is ready to be de-queued
				break
			}

			// Transition this job to ready
			// Note: There is a potential of a race condition in the case where
			// the job-state-machine is a distributed implementation, however the
			// expectation is that this should return as a NonFatal error.
			err = jsm.Ready(job.ID())
			if err != nil {
				logc.Panicf("c.jsm.Ready(%v) missing entry", job.ID())
			}
			count++
		}
	}

	log.Infof("tickDelayJobs: moved %v jobs %v tubes now=%v nextDelayTickAt=%v",
		count, len(jsm.tubes), time.Unix(now, 0), time.Unix(nextDelayedTickAt, 0))

	jsm.nextDelayTickAt = nextDelayedTickAt
}

// tickReadyJobs - transition READY jobs to RESERVED, by finding a matching client
func (jsm *localJSM) tickReadyJobs(nowSecs int64) ([]*Reservation, error) {
	result := make([]*Reservation, 0)
	for t, _ := range jsm.tubes {
		for {
			r, err := jsm.tryReserveJob(nowSecs, t, nil)
			if err == ErrNoReservationFound {
				break
			} else if err != nil {
				log.WithField("method", "localJSM.tickReadyJobs").
					Panicf("unexpected err %v", err)
			} else {
				result = append(result, r)
				jsm.stat.nResv += 1
				jsm.stat.nResvOnTick += 1
			}
		}
	}

	return result, nil
}

func (jsm *localJSM) tryReserveJob(nowSecs int64, t TubeName, cli *ClientResvEntry) (*Reservation, error) {
	logc := log.WithField("method", "localJSM.tryReserveJob")
	if n, err := jsm.tubes.WaitQLen(t); err != nil {
		return nil, err
	} else if n == 0 {
		// if there are no clients waiting for a reservation on this tube
		return nil, ErrNoReservationFound
	}

	// Query the job-state-machine to check if any jobs are READY
	job, err := jsm.NextReadyJob(t)
	if err == ErrEntryMissing {
		// no entry is found
		return nil, ErrNoReservationFound
	} else if err != nil {
		logc.Panicf("jsm.NextReadyJob(%v) err=%v", t, err)
	}

	if cli == nil {
		// Dequeue the first client resv entry if
		// a specific client is not provided.
		cli, err = jsm.tubes.DequeueFromWaitQ(t)
		if err != nil {
			logc.Panicf("t.watchers.Random() tube=%v err=%v", t, err)
		}
	}

	if !cli.IsWaitingForResv {
		logc.WithField("tubeName", t).
			WithField("client", cli.CliID).
			Panic("Un-Expected IsWaitingForResv=false")
	}

	// attempt to reserve this job
	if err := jsm.Reserve(nowSecs, job.ID(), cli.CliID); err != nil {
		logc.Panicf("c.jsm.Reserve(%v, %v) err=%v", job.ID(), cli.CliID, err)
	}

	// remove client from the reserved mode
	jsm.unReserve(cli)
	resv := &Reservation{
		RequestId: cli.ReqID,
		ClientId:  cli.CliID,
		JobId:     job.ID(),
		Status:    Matched,
		BodySize:  job.BodySize(),
		Body:      job.Body(),
	}

	jsm.updateClientTickAt(cli)
	logc.Debugf("matched(reqID = %v job=%v, client=%v)", cli.ReqID, job.ID(), cli.CliID)
	return resv, nil
}

// updateClientTickAt sets the time at which this specific client
// needs to be processed again, and updates this client's
// entry within the ClientHeap
func (jsm *localJSM) updateClientTickAt(cli *ClientResvEntry) {
	logc := log.WithFields(log.Fields{
		"method": "localJSM.updateClientTickAt",
		"CliID":  cli.CliID})
	var nextTickAt int64 = maxTime
	job, err := jsm.NextReservedJob(cli.CliID)
	if err == nil {
		// Indicates this client has a reserved job
		logc.Debugf("next reserved job returned %v %v", job.ID(), job.ExpiresAt())
		// find the time (seconds) with the earliest reservation deadline
		resvDeadlineAt := job.ExpiresAt()
		if cli.IsWaitingForResv {
			// subtract by one second to notify DEADLINE_SOON
			resvDeadlineAt--
		}
		nextTickAt = minTime(nextTickAt, resvDeadlineAt)
	} else if err != ErrEntryMissing {
		log.Panicf("jsm.NextReservedJob(%v) err=%v", cli.CliID, err)
	}

	if cli.IsWaitingForResv {
		// if the client is waiting for a reservation with a timeout
		nextTickAt = minTime(nextTickAt, cli.ResvDeadlineAt)
	}

	logc.Debugf("waitForResv = %v nextTickAt = %d maxTime = %d nextTickAt < maxTime = %v",
		cli.IsWaitingForResv, nextTickAt, maxTime, nextTickAt < maxTime)
	if nextTickAt < maxTime {
		// indicates that the client either has assigned reservation(s) and/or
		// the client is waiting on a reservation with a timeout
		if cli.HeapIndex < 0 {
			// client is not a part of the heap update the TickAt
			// & enqueue this client to the heap
			cli.TickAt = nextTickAt
			jsm.clientHeap.Enqueue(cli)
		} else {
			// forces the client's position to change within the heap
			jsm.clientHeap.UpdateTickAt(cli, nextTickAt)
		}

		// update the client timer only if its on the heap
		// Disable for now this is an optimization, which we need to justify
		// c.clientTimer.Update(nextTickAt)
	} else {
		// indicates the client has no reservation(s) and/or has a
		// reservation with no timeout (i.e infinite timeout)
		if cli.HeapIndex >= 0 {
			// the client is part of the heap -- remove it
			// this client doesn't need any processing.
			if err := jsm.clientHeap.Remove(cli); err != nil {
				logc.Panicf("un-expected error %v", err)
			}

			// remove this entry from the client index as well
			if err := jsm.clients.Remove(cli); err == ErrEntryMissing {
				logc.Errorf("jsm.clients.remove(CliID=%v) err=%v", cli.CliID, err)
			} else if err != nil {
				logc.Panicf("jsm.clients.remove(CliID=%v) err=%v", cli.CliID, err)
			}
		}

		cli.TickAt = nextTickAt
	}

	logc.Debugf("nextTickAt %v", time.Unix(nextTickAt, 0))
}

// tickClients performs this operation on all the clients
// - transition client(s) RESERVED job(s) back to READY, if the reservations expire.
// - stop client(s) from a waiting for a reservation, if the reservation ttl expires
// - report DeadlineSoon for clients who have a job reserved whose reservations expire soon
//
// Retuns back appropriate Reservations
func (jsm *localJSM) tickClients(nowSecs int64) []*Reservation {
	logc := log.WithField("method", "localJSM.tickClients")
	count := 0
	result := make([]*Reservation, 0)

	for jsm.clientHeap.Len() > 0 {
		count++
		lowestTickAt := jsm.clientHeap.Peek().TickAt
		duration := time.Unix(nowSecs, 0).Sub(time.Unix(lowestTickAt, 0))
		if nowSecs <= lowestTickAt {
			logc.Infof("now=%v <= lowestTickAt=%v skip processing duration=%v", nowSecs, lowestTickAt, duration)
			break
		}

		logc.Infof("now=%v > lowestTickAt=%v continue processing", nowSecs, lowestTickAt)
		cli := jsm.clientHeap.Dequeue()
		if cli.IsWaitingForResv && nowSecs > cli.ResvDeadlineAt {
			// Timeout -The client's reservation request deadline has passed
			resv := &Reservation{
				RequestId: cli.ReqID,
				ClientId:  cli.CliID,
				JobId:     0,
				Status:    Timeout,
				Body:      nil,
				Error:     nil,
			}

			// remove client from the reserved mode
			jsm.unReserve(cli)
			logc.Debugf("UnReserve client %v", cli)
			result = append(result, resv)
		}

		// Check to see if any of the reserved jobs need to be
		// moved back to the reserved state
		// for cli.reservedJobs.Len() > 0 {
		for {
			// query the job-state-machine for the client's next reservation job
			job, err := jsm.NextReservedJob(cli.CliID)
			if err == ErrEntryMissing {
				break
			} else if err != nil {
				logc.Panicf("c.json.NextReservedJob(%v) err=%v", cli.CliID, err)
			}

			logc.Debugf("c.jsm.NextReservedJob(%v)=%v", cli.CliID, job.ID())
			lowestResvDeadlineAt := job.ExpiresAt()
			if nowSecs > lowestResvDeadlineAt {
				// the reservation deadline exceeds now. time to release
				// this job back to the tube's ready queue
				err = jsm.Release(job.ID(), cli.CliID)
				if err != nil {
					logc.Panicf("c.jsm.Release(job=%v, cli=%v), err=%v",
						job.ID(), cli.CliID, err)
				}
			} else if nowSecs > lowestResvDeadlineAt-1 && cli.IsWaitingForResv {
				// DeadlineSoon -The client's reservation request deadline is in 1 second boundary
				resv := &Reservation{
					RequestId: cli.ReqID,
					ClientId:  cli.CliID,
					JobId:     0,
					Status:    DeadlineSoon,
					Body:      nil,
					Error:     nil,
				}
				// remove client from the reserved mode
				jsm.unReserve(cli)
				logc.Debugf("UnReserve client %v", cli)
				// reply to the client
				result = append(result, resv)
			} else {
				break
			}
		}

		jsm.updateClientTickAt(cli)
	}

	logc.Infof("nClients processed = %v", count)
	return result
}

func (jsm *localJSM) unReserve(cli *ClientResvEntry) {
	ctxLog := log.WithFields(log.Fields{
		"method":    "localJSM.unReserve",
		"requestId": cli.ReqID,
		"CliID":     cli.CliID})
	cli.IsWaitingForResv = false
	cli.ResvDeadlineAt = maxTime
	// cli.ReqID = ""

	for _, t := range cli.WatchedTubes {
		if err := jsm.tubes[t].waiting.Remove(cli); err != nil {
			ctxLog.Panicf("remove waiting client id=%v from tube=%v. error=%v",
				cli.CliID, t, err)
		}
		// ToDo: GCTube
	}

	ctxLog.Debugf("unReserve complete")
}

func minTime(a, b int64) int64 {
	if a < b {
		return a
	} else {
		return b
	}
}

type localSnapshot struct {
	jsm        *localJSM
	ji         *JobIndex
	ti         tubeIndex
	ri         reservedJobsIndex
	clientHeap *clientResvHeap
	clients    *clientResvQueue
	maxJobID   JobID
}

func newLocalSnapshot(jsm *localJSM) *localSnapshot {
	return &localSnapshot{
		jsm:        jsm,
		ji:         NewJobIndex(),
		ti:         make(tubeIndex),
		ri:         make(reservedJobsIndex),
		clientHeap: newClientResvHeap(),
		clients:    newClientResvQueue(),
		maxJobID:   JobID(0),
	}
}

func (l *localSnapshot) SnapshotJobs() (<-chan Job, error) {
	return l.jsm.jobs.Jobs(), nil
}

func (l *localSnapshot) RestoreJobs(ctx context.Context, jobsCh <-chan Job) error {
	for j := range jobsCh {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ErrCancel
			default:
			}
		}
		if err := l.restoreJob(j); err != nil {
			return err
		}
	}

	return nil
}

func (l *localSnapshot) restoreJob(j Job) error {
	ie, err := l.ji.Add(j)
	if err != nil {
		return err
	}

	if j.ID() > l.maxJobID {
		l.maxJobID = j.ID()
	}

	switch j.State() {
	case Reserved:
		ie.entry, err = l.ri.Enqueue(j.ReservedBy(), j)
	case Ready:
		ie.entry, err = l.ti.EnqueueReadyJob(j)
	case Delayed:
		ie.entry, err = l.ti.EnqueueDelayedJob(j)
	}
	return err
}

func (l *localSnapshot) SnapshotClients() (<-chan *ClientResvEntry, error) {
	return l.jsm.clients.Entries(), nil
}

func (l *localSnapshot) RestoreClients(ctx context.Context, nClients int, entriesCh <-chan *ClientResvEntry) error {
	logc := log.WithField("method", "RestoreClients")
	l.clientHeap = newClientResvHeapWithSize(nClients)

	for cli := range entriesCh {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ErrCancel
			default:
			}
		}

		if err := l.restoreClient(cli); err != nil {
			logc.Panicf("l.restoreClient clientID=%v. err=%v", cli.CliID, err)
			return err
		}
	}

	initializeHeap(l.clientHeap)
	return nil
}

func (l *localSnapshot) restoreClient(cli *ClientResvEntry) error {
	if cli.HeapIndex < 0 {
		log.Errorf("restoreClient clientID=%v has an invalid heap index", cli.CliID)
		return ErrInvalidIndex
	}

	if err := l.clients.Enqueue(cli); err != nil {
		return err
	}

	if cli.IsWaitingForResv {
		for _, t := range cli.WatchedTubes {
			if err := l.ti.EnqueueToWaitQ(t, cli); err != nil {
				return err
			}
		}
	}

	(*l.clientHeap)[cli.HeapIndex] = cli
	return nil
}

func (l *localSnapshot) FinalizeRestore() {
	l.jsm.maxJobID = l.maxJobID
	l.jsm.jobs = l.ji
	l.jsm.tubes = l.ti
	l.jsm.reservedJobs = l.ri
	l.jsm.clientHeap = l.clientHeap
	l.jsm.clients = l.clients
	log.Infof("Updated state of jsm from localSnapshot")
}

type procStats struct {
	nResv         int64 // count of reservations
	nResvOnDemand int64 // count of reservations via reserve method
	nResvOnTick   int64 // count of reservation via tick
}
