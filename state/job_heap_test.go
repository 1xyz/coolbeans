package state

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewPriorityJobs(t *testing.T) {
	jobs := NewPriorityJobs()
	assert.Equalf(t, jobs.Len(), 0, "Initial jobs is empty")
}

func TestPriorityJobs_Enqueue_Order(t *testing.T) {
	jobs := NewPriorityJobs()

	e1, e2 := newTestJobWithPri(10), newTestJobWithPri(3)

	a1 := jobs.Enqueue(e1)
	a2 := jobs.Enqueue(e2)

	assert.Equalf(t, 0, a2.index,
		"job entry with lower pri is at the head of queue")
	assert.Equalf(t, 1, a1.index,
		"job entry with higher pri is at not at head of queue")
}

// TestPriorityJobs_Dequeue
// Verify the dequeue operation is ordered by pri; lower pri jobEntry is de-queued first
func TestPriorityJobs_Dequeue(t *testing.T) {
	jobs := NewPriorityJobs()
	e1, e2 := newTestJobWithPri(10), newTestJobWithPri(3)

	a1 := jobs.Enqueue(e1)
	a2 := jobs.Enqueue(e2)

	assert.Equalf(t, a2.ID(), jobs.Dequeue().ID(),
		"Expect jobEntry2 to be de-queued")
	assert.Equalf(t, a1.ID(), jobs.Dequeue().ID(),
		"Expect jobEntry1 to be de-queued")
}

func TestPriorityJobs_RemoveAt(t *testing.T) {
	jobs := NewPriorityJobs()
	jobs.Enqueue(newTestJobWithPri(10))
	e := jobs.Enqueue(newTestJobWithPri(3))

	a, err := jobs.RemoveAt(e)

	assert.Nil(t, err, "Expect err to be nil")
	assert.Equalf(t, e.ID(), a.ID(),
		"Expect jobEntry2 to be removedAt")
}

var jobID uint64 = 0

func newTestJob(pri uint32, delay int64, ttr int, bodySize int,
	body []byte, tubeName TubeName) Job {

	now := time.Now().UTC().Unix()
	jobID++
	return &localJob{
		id:        jobID,
		priority:  pri,
		delay:     delay,
		ttr:       ttr,
		bodySize:  bodySize,
		body:      body,
		tubeName:  tubeName,
		createdAt: now,
		readyAt:   now + delay,
		state:     Initial,
		expiresAt: 0,
		buriedAt:  0,
	}
}

func newTestJobWithPri(pri uint32) Job {
	return newTestJob(pri, 0, 0, 0, nil, TubeName("alpha"))
}

func TestNewDelayedJobs(t *testing.T) {
	jobs := NewDelayedJobs()
	assert.Equalf(t, jobs.Len(), 0, "Initial jobs is empty")
}

func TestDelayedJobs_Enqueue_Order(t *testing.T) {
	jobs := NewDelayedJobs()

	e1, e2 := newTestJobWithDelay(0, 1, 1000),
		newTestJobWithDelay(1, 1, 1)

	a1 := jobs.Enqueue(e1)
	a2 := jobs.Enqueue(e2)

	assert.Equalf(t, 0, a2.index,
		"job entry with lower ReadyAt is at the head of queue")
	assert.Equalf(t, 1, a1.index,
		"job entry with higher ReadyAt is at not at head of queue")
}

// TestDelayedJobs_Dequeue
// Verify the dequeue operation is ordered by readyAt; lower readyAt jobEntry is de-queued first
func TestDelayedJobs_Dequeue(t *testing.T) {
	jobs := NewDelayedJobs()
	e1, e2 := newTestJobWithDelay(0, 1, 1000),
		newTestJobWithDelay(1, 1, 1)

	a1 := jobs.Enqueue(e1)
	a2 := jobs.Enqueue(e2)

	assert.Equalf(t, a2.ID(), jobs.Dequeue().ID(),
		"Expect jobEntry2 to be de-queued")
	assert.Equalf(t, a1.ID(), jobs.Dequeue().ID(),
		"Expect jobEntry1 to be de-queued")
}

// TestDelayedJobs_RemoveAt
// Verify RemoveAt operation; i.e remove a specific jobEntry
func TestDelayedJobs_RemoveAt(t *testing.T) {
	jobs := NewDelayedJobs()
	jobs.Enqueue(newTestJobWithDelay(0, 1, 1000))
	e := jobs.Enqueue(newTestJobWithDelay(1, 1, 10000))

	a, err := jobs.RemoveAt(e)

	assert.Nil(t, err, "Expect err to be nil")
	assert.Equalf(t, e.ID(), a.ID(),
		"Expect jobEntry2 to be removedAt")
}

func newTestJobWithDelay(id uint64, createdAt int64, delay int64) Job {
	return &localJob{
		id:        id,
		priority:  0,
		createdAt: createdAt,
		delay:     delay,
		readyAt:   createdAt + delay,
	}
}

func TestNewReservedJobs(t *testing.T) {
	rJobs := NewReservedJobs()
	assert.Equalf(t, rJobs.Len(), 0, "Initial job list is empty")
}

func TestReservedJobs_Enqueue(t *testing.T) {
	rJobs := NewReservedJobs()
	e := newTestJobWithPri(0)
	updateResv(t, e)

	a := rJobs.Enqueue(e)

	assert.Equalf(t, e.ID(), a.ID(),
		"Result job entry matched inserted job")
	assert.Equalf(t, 0, a.index,
		"Result job entry is at the head of queue")
}

// TestReservedJobs_Enqueue_Order
// Verify Enqueue is priority ordered by expiresAt; lower expiresAt has higher precedence
func TestReservedJobs_Enqueue_Order(t *testing.T) {
	rJobs := NewReservedJobs()
	e1, e2 := newTestJobWithTTR(t, 1000), newTestJobWithTTR(t, 1)

	a1 := rJobs.Enqueue(e1)
	a2 := rJobs.Enqueue(e2)

	assert.Equalf(t, 0, a2.index,
		"job entry with lower expiresAt is at the head of queue")
	assert.Equalf(t, 1, a1.index,
		"job entry with higher expiresAt is at not at head of queue")
}

// TestReservedJobList_Dequeue
// Verify the dequeue operation is ordered by expiresAt; lower expiresAt jobEntry is de-queued first
func TestReservedJobs_Dequeue(t *testing.T) {
	rJobs := NewReservedJobs()
	e1, e2 := newTestJobWithTTR(t, 1), newTestJobWithTTR(t, 100)

	a1 := rJobs.Enqueue(e1)
	a2 := rJobs.Enqueue(e2)

	assert.Equalf(t, a1.ID(), rJobs.Dequeue().ID(),
		"Expect jobEntry1 to be de-queued")
	assert.Equalf(t, a2.ID(), rJobs.Dequeue().ID(),
		"Expect jobEntry2 to be de-queued")
}

// TestReservedJobs_RemoveAt
// Verify RemoveAt operation; i.e remove a specific jobEntry
func TestReservedJobs_RemoveAt(t *testing.T) {
	rJobs := NewReservedJobs()
	rJobs.Enqueue(newTestJobWithTTR(t, 1))
	e := rJobs.Enqueue(newTestJobWithTTR(t, 100))

	a, err := rJobs.RemoveAt(e)

	assert.Nil(t, err, "Expect err to be nil")
	assert.Equalf(t, e.ID(), a.ID(),
		"Expect jobEntry2 to be removedAt")
}

func newTestJobWithTTR(t *testing.T, ttr int) Job {
	j := newTestJob(0, 0, ttr, 0, nil, TubeName("alpha"))
	updateResv(t, j)
	return j
}
func updateResv(t *testing.T, j Job) {
	now := time.Now().UTC().Unix()
	_, err := j.UpdateReservation(now)
	if err != nil {
		t.Fatalf("un-expected err %v", err)
	}
}

func TestNewBuriedJobs(t *testing.T) {
	bJobs := NewBuriedJobs()
	assert.Equalf(t, bJobs.Len(), 0, "Initial job list is empty")
}

// TestBuriedJobs_Enqueue_Order
// Verify Enqueue is priority ordered by buriedAt; lower buriedAt has higher precedence
func TestBuriedJobs_Enqueue_Order(t *testing.T) {
	bJobs := NewBuriedJobs()
	j0, j1, j2 := newTestJobWithBuriedAt(t, 10),
		newTestJobWithBuriedAt(t, 5),
		newTestJobWithBuriedAt(t, 20)

	je0 := bJobs.Enqueue(j0)
	je1 := bJobs.Enqueue(j1)
	je2 := bJobs.Enqueue(j2)

	assert.Equalf(t, 1, je0.index,
		"Result job entry is at the middle of queue")
	assert.Equalf(t, 0, je1.index,
		"Result job entry is at the head of queue")
	assert.Equalf(t, 2, je2.index,
		"Result job entry is at the end of queue")
}

// TestBuriedJobs_Enqueue_Order_WithEqualBuriedAt
// Verify Enqueue is priority ordered by job id when buriedAt are equal;
// lower job id  has higher precedence
func TestBuriedJobs_Enqueue_Order_WithEqualBuriedAt(t *testing.T) {
	bJobs := NewBuriedJobs()
	j0, j1 := newTestJobWithBuriedAt(t, 10),
		newTestJobWithBuriedAt(t, 10)

	je1 := bJobs.Enqueue(j1)
	je0 := bJobs.Enqueue(j0)

	assert.Equalf(t, 0, je0.index,
		"Result job entry is at the head of queue since its id is lower")
	assert.Equalf(t, 1, je1.index,
		"Result job entry is at the head of queue since its id is lower")
}

// TestBuriedJobs_Dequeue
// Verify the dequeue operation is ordered by buriedAt; lower buriedAt jobEntry is de-queued first
func TestBuriedJobs_Dequeue(t *testing.T) {
	bJobs := NewBuriedJobs()
	j0, j1, j2 := newTestJobWithBuriedAt(t, 10),
		newTestJobWithBuriedAt(t, 5),
		newTestJobWithBuriedAt(t, 20)

	bJobs.Enqueue(j0)
	bJobs.Enqueue(j1)
	bJobs.Enqueue(j2)

	assert.Equalf(t, j1.ID(), bJobs.Dequeue().ID(),
		"Expect jobEntry1 to be de-queued")
	assert.Equalf(t, j0.ID(), bJobs.Dequeue().ID(),
		"Expect jobEntry0 to be de-queued")
	assert.Equalf(t, j2.ID(), bJobs.Dequeue().ID(),
		"Expect jobEntry2 to be de-queued")
}

// TestBuriedJobs_RemoveAt
// Verify RemoveAt operation; i.e remove a specific jobEntry
func TestBuriedJobs_RemoveAt(t *testing.T) {
	bJobs := NewBuriedJobs()
	j0, j1, j2 := newTestJobWithBuriedAt(t, 10),
		newTestJobWithBuriedAt(t, 5),
		newTestJobWithBuriedAt(t, 20)

	je0 := bJobs.Enqueue(j0)
	bJobs.Enqueue(j1)
	bJobs.Enqueue(j2)

	a, err := bJobs.RemoveAt(je0)

	assert.Nil(t, err, "Expect err to be nil")
	assert.Equalf(t, je0.ID(), a.ID(),
		"Expect jobEntry0 to be removedAt")
	assert.Equalf(t, 2, bJobs.Len(), "expect length to be 2")
}

func newTestJobWithBuriedAt(t *testing.T, nowSeconds int64) Job {
	j := newTestJob(0, 0, 10, 0, nil, TubeName("alpha"))
	j.UpdateBuriedAt(nowSeconds)
	return j
}
