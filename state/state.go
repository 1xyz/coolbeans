/*
 * Package state provides an interface for a beanstalkd job, state for
 * job, tube and a connected client. and job state machine with the
 * states and transitions as defined in the protocol.
 * https://github.com/beanstalkd/beanstalkd/blob/master/doc/protocol.txt
 *
 * Implementations include an in-memory implementation of the interfaces
 */
package state

import (
	"fmt"
	"golang.org/x/net/context"
)

// JobState refers to the state of a job in beanstalkd context

//go:generate stringer -type=JobState --output state_string.go

type JobState int

const (
	Initial JobState = iota
	Ready
	Reserved
	Buried
	Delayed
	Deleted
)

type JobID uint64
type TubeName string
type ClientID string

type Job interface {
	// ID is a unique identifier integer for this job (generated by the server)
	ID() JobID

	// Priority is an integer < 2**32. Jobs with smaller priority values will be
	// scheduled before jobs with larger priorities. The most urgent priority is 0;
	// the least urgent priority is 4,294,967,295.
	Priority() uint32

	// Delay is an integer number of seconds to wait before putting the job in
	// the ready queue. The job will be in the "delayed" state during this time.
	// Maximum delay is 2**32-1.
	Delay() int64

	// TTR/time to run -- is an integer number of seconds to allow a worker
	// to run this job. This time is counted from the moment a worker reserves
	// this job. If the worker does not delete, release, or bury the job within
	// <ttr> seconds, the job will time out and the server will release the job.
	// The minimum ttr is 1. If the client sends 0, the server will silently
	// increase the ttr to 1. Maximum ttr is 2**32-1.
	TTR() int

	// BodySize is an integer indicating the size of the job body, not including the
	// trailing "\r\n". This value must be less than max-job-size (default: 2**16)
	BodySize() int

	// Body is the job body -- a sequence of bytes of length BodySize
	Body() []byte

	// TubeName the name of the tube associated with this job
	TubeName() TubeName

	// CreatedAt - Indicates the time, when this job is created
	CreatedAt() int64

	// ReadyAt - Indicates the time when the job is ready
	ReadyAt() int64

	// Retrieve the current job state
	State() JobState

	// Update the job state
	UpdateState(newState JobState)

	// Return the time at which the reservation expires
	ExpiresAt() int64

	// Reset the reservation time taking the current in reference
	// Return back the new reservation time
	UpdateReservation(nowSeconds int64) (int64, error)

	// ReservedBy returns the name of the client which has
	// reserved this job. Empty string if this job is not reserved
	ReservedBy() ClientID

	// Update the reservedBy client
	UpdateReservedBy(clientID ClientID)

	// Serialze the content of this job into a byte slice
	// Serialize() ([]byte, error)

	// De-Serialize the input byte slice and populate the Job object
	// DeSerialize(bytes []byte) error
}

type localJob struct {
	nowSecs func() (int64, error)

	id         uint64
	priority   uint32
	delay      int64
	ttr        int
	bodySize   int
	body       []byte
	tubeName   TubeName
	createdAt  int64
	readyAt    int64
	state      JobState
	expiresAt  int64
	reservedBy ClientID
}

func (j *localJob) ID() JobID {
	return JobID(j.id)
}

func (j *localJob) Priority() uint32 {
	return j.priority
}

func (j *localJob) Delay() int64 {
	return j.delay
}

func (j *localJob) TTR() int {
	return j.ttr
}

func (j *localJob) BodySize() int {
	return j.bodySize
}

func (j *localJob) Body() []byte {
	return j.body
}

func (j *localJob) TubeName() TubeName {
	return j.tubeName
}

func (j *localJob) CreatedAt() int64 {
	return j.createdAt
}

func (j *localJob) ReadyAt() int64 {
	return j.readyAt
}

func (j *localJob) State() JobState {
	return j.state
}

func (j *localJob) UpdateState(newState JobState) {
	j.state = newState
}

func (j *localJob) UpdateReservedBy(clientID ClientID) {
	j.reservedBy = clientID
}

func (j *localJob) ExpiresAt() int64 {
	return j.expiresAt
}

func (j *localJob) ReservedBy() ClientID {
	return j.reservedBy
}

func (j *localJob) UpdateReservation(nowSeconds int64) (int64, error) {
	j.expiresAt = nowSeconds + int64(j.ttr)
	return j.expiresAt, nil
}

// JSM provides methods for the beanstalkd job state machine.
// put with delay               release with delay
//  ----------------> [DELAYED] <------------.
//                        |                   |  touch (extend ttr)
//                        | timeout/ttr       | .----.
//                        |                   | |    |
//   put                  v     reserve       | |    v   delete
//  -----------------> [READY] ------------> [RESERVED] --------> *poof*
//                       ^  ^                | | |
//                       |  ^\  release      | | |
//                       |   \ `-------------' | |
//                       |    \                | |
//                       |     \  timeout/ttr  , |
//                       |      `--------------  |
//                       |                       |
//                       | kick                  |
//                       |                       |
//                       |       bury            |
//                    [BURIED] <-----------------'
//                       |
//                       |  delete
//                        `--------> *poof*
type JSM interface {
	// Put makes a new job. It initializes the job with
	// a unique identifier, sets state to Ready or Delayed based
	// on the delay parameter.
	Put(nowSeconds int64,
		priority uint32,
		delay int64,
		ttr int,
		bodySize int,
		body []byte,
		tubeName TubeName) (JobID, error)

	// Delete, removes a job specified by the id by a specific client
	Delete(jobID JobID, clientID ClientID) error

	// NextDelayedJob returns the job in the Delay state in order of
	// priority for this tube. A job with and earlier (lower) delay
	// takes higher precedence.
	// NextDelayedJob(tubeName TubeName) (Job, error)

	// NextReadyJob returns the job in the Ready state in order of
	// priority for this tube. A job with a lower priority value
	// takes higher precedence.
	// NextReadyJob(tubeName TubeName) (Job, error)

	// NextReservedJob returns the job in a Reserved state in order of
	// priority for this client. A job with an earlier ExpiresAt takes
	// higher precedence.
	// NextReservedJob(clientID ClientID) (Job, error)

	// Ready transitions this Delayed/Buried job to  Ready
	// Ready(jobID JobID) error

	// Reserve transitions this Ready job to be Reserved by this client
	// Reserve(nowSeconds int64, jobID JobID, clientID ClientID) error

	// Release transitioned this reserved job to a Ready one
	Release(jobID JobID, clientID ClientID) error

	// // Release this job into the denowSeconds() + int64(delaySeconds)layed state (from a reserved to delay state)
	// ReleaseWithDelay(jobID uint64, CliID string, delaySeconds int) error
	//
	// // Extend a reserved job's reservation TTL by its TTR (time-to-run)
	// Touch(jobID uint64) error
	//
	// // Bury this job (from a reserved state)
	// Bury(jobID uint64) error
	//
	// // Kick this job from buried state to a ready state
	// Kick(jobID uint64) error

	// Returns an interface that allows a caller to snapshot the current
	// state of the JSM. Callers of the interface should not be done across
	// go-routines.
	Snapshot() (JSMSnapshot, error)

	// AppendReservation Appends a new Reservation Request for a client, and the specified set of tubes
	// if the timeoutSecs is zero, then an infinite timeout is assumed.
	//
	// A pointer to a Reservation struct is returned which encapsulates a result if a reservation
	// was handled or not
	AppendReservation(clientId ClientID, reqID string, watchedTubes []TubeName, nowSecs, deadlineAt int64) (*Reservation, error)

	// Tick runs a step of the job state machine with the current time.
	//
	// This call returns all the allocated reservations in this tick call
	Tick(nowSecs int64) ([]*Reservation, error)

	// CheckClientState queries the job state machine whether the provided list of clientIds are waiting for reservations.
	//
	// The response returns the ClientIDs (i) which are waiting for reservations, (ii) those which are not waiting and (iii)
	// those which are missing or an error.
	CheckClientState(clientIDs []ClientID) ([]ClientID, []ClientID, []ClientID, error)
}

// JSMSnapshot provides methods allowing a caller to read & restore
// jobs  out of the job state machine.
type JSMSnapshot interface {
	// SnapshotJobs returns a read-only job channel. This allows a caller
	// to iterate through the jobs sent on the channel, when all the jobs
	// in the state machine are returned, this method closes the channel,
	// signaling the end of this snapshot.
	//
	// SnapshotJobs is used to support log compaction. This call should
	// an be used to save a point-in-time snapshot of the FSM.
	//
	// SnapshotJobs should not be called to the JSM across go-routines,
	// this is the default behavior (unless an implementation forces to override this)
	// A caller is recommended to clone this job
	SnapshotJobs() (<-chan Job, error)

	// SnapshotClients returns a read-only job channel. This allows a caller
	// to iterate through the clientResvEntries sent on the channel, when
	// all the entries in the state machine are returned, this method closes
	// the channel, signaling the end of this snapshot.
	//
	// SnapshotClients is used to support log compaction. This call should
	// an be used to save a point-in-time snapshot of the FSM.
	//
	// SnapshotClients should not be called to the JSM across go-routines,
	// this is the default behavior (unless an implementation forces to override this)
	// A caller is recommended to clone this job
	SnapshotClients() (<-chan *ClientResvEntry, error)

	// RestoreJobs takes jobsCh, a write-only job channel, that allow a caller
	// to send jobs to be added to the job state machine (JSM).
	//
	// Once RestoreJobs and RestoreClients are complete. call FinalizeRestore
	// which replaces the current state of JSM with the jobs provided in the
	// channel.
	//
	// RestoreJobs takes an additional context which can be used to signal
	// a cancellation. In this case, the method discards the jobs that were
	// provided on the channel, after the cancel is called
	//
	// Note: It is the responsibility of the caller to close the channels
	// and drain the jobsCh
	RestoreJobs(ctx context.Context, jobsCh <-chan Job) error

	// RestoreClients takes entriesCh, a write-only job channel, that allow
	// a caller to send clientResvEntry structs to be added to the job state
	// machine (JSM).
	//
	// Once RestoreJobs and RestoreClients are complete. call FinalizeRestore
	// which replaces the current state of JSM with the jobs provided in the
	// channel.
	//
	// RestoreClients takes an additional context which can be used to signal
	// a cancellation. In this case, the method discards the clientResvEntries
	// that were provided on the channel, after the cancel is called
	//
	// Note: It is the responsibility of the caller to close the channels
	// and drain the jobsCh
	RestoreClients(ctx context.Context, nClients int, entriesCh <-chan *ClientResvEntry) error

	// Finalize Restore overwrites the state of the job-state-machine
	// with the current state of the snapshot
	//
	// Once RestoreJobs and RestoreClients are complete. call FinalizeRestore
	// which replaces the current state of JSM with the jobs provided in the
	// channel.
	FinalizeRestore()
}

// Refers to the Reservation Status of a Reservation

// go:generate stringer -type=ReservationStatus --output state_string.go
type ReservationStatus int

const (
	Unknown ReservationStatus = iota
	Queued
	DeadlineSoon
	Matched
	Timeout
	Error
)

type Reservation struct {
	RequestId string
	ClientId  ClientID
	JobId     JobID
	Status    ReservationStatus
	BodySize  int
	Body      []byte
	Error     error
}

func (r Reservation) String() string {
	return fmt.Sprintf("Reservation ReqID=%s ClientID=%v Status=%v JobID=%v Error=%v BodySize=%v",
		r.RequestId, r.ClientId, r.Status, r.JobId, r.Error, r.BodySize)
}
