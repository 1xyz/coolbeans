package store

import (
	"bytes"
	"fmt"
	"github.com/1xyz/coolbeans/state"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewSnapshotFrom_EmptyJsm(t *testing.T) {
	jsm, err := state.NewJSM()
	if err != nil {
		t.Fatalf("err state.NewJSM %v", err)
	}

	ss, err := NewSnapshotFrom(jsm)
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, ss, "expect snapshot to not be nil")
	assert.Equalf(t, 0, len(ss.snap.Jobs), "expect zero jobs to be snapshot")
	assert.Equalf(t, 0, len(ss.snap.Reservations), "expect zero reservations to be snapshot")
}

func TestNewSnapshotFrom(t *testing.T) {
	jsm, _ := state.NewJSM()
	resv := putNClientResvEntries(t, jsm, 5)
	jobs := putNJobs(t, jsm, 7)

	ss, err := NewSnapshotFrom(jsm)
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, ss, "expect snapshot to not be nil")
	assert.Equalf(t, len(jobs), len(ss.snap.Jobs), "expect zero jobs to be snapshot")
	assert.Equalf(t, len(resv), len(ss.snap.Reservations), "expect zero reservations to be snapshot")
}

func TestPersist(t *testing.T) {
	jsm, _ := state.NewJSM()
	putNClientResvEntries(t, jsm, 5)
	putNJobs(t, jsm, 7)

	ss, _ := NewSnapshotFrom(jsm)
	sink := newTestSink()
	err := ss.Persist(sink)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Greaterf(t, len(sink.Bytes()), 0, "expect byte len > 0")
}

func TestRestoreSnapshotTo(t *testing.T) {
	jsm, _ := state.NewJSM()
	resv := putNClientResvEntries(t, jsm, 5)
	jobIDs := putNJobs(t, jsm, 7)

	ss, _ := NewSnapshotFrom(jsm)
	sink := newTestSink()
	ss.Persist(sink)

	// restore the snapshot to another state machine
	jsm2, _ := state.NewJSM()
	err := RestoreSnapshotTo(sink, jsm2, time.Second*30)
	assert.Nilf(t, err, "expect err to be nil")

	jsmSnap, _ := jsm2.Snapshot()
	// Verify if the jobIDs match
	if jobCh, err := jsmSnap.SnapshotJobs(); err != nil {
		t.Fatalf("jsmSnap.SnapshotJobs err=%v", err)
	} else {
		count := 0
		for range jobCh {
			count++
		}
		assert.Equalf(t, len(jobIDs), count, "expect job count to be %v", len(jobIDs))
	}

	// Verif if the clientIDs match
	if cliCh, err := jsmSnap.SnapshotClients(); err != nil {
		t.Fatalf("jsmSnap.SnapshotClients err=%v", err)
	} else {
		count := 0
		for range cliCh {
			count++
		}
		assert.Equalf(t, len(resv), count, "expect resv count to be %v", len(resv))
	}

}

func putNClientResvEntries(t *testing.T, jsm state.JSM, n int) []*state.Reservation {
	reqID := "bar"
	testResv := make([]*state.Reservation, 0)
	now := testNowSecs()
	deadlineAt := now + 100
	for i := 0; i < n; i++ {
		cliID := state.ClientID(fmt.Sprintf("foo-%d", i))
		if r, err := jsm.AppendReservation(cliID, reqID, []state.TubeName{"foo"}, now, deadlineAt); err != nil {
			t.Fatalf("error in jsm.AppendReservation. err=%v", err)
		} else {
			testResv = append(testResv, r)
		}
	}

	return testResv
}

func putNJobs(t *testing.T, jsm state.JSM, n int) []state.JobID {
	now := testNowSecs()
	body := "hello world"
	testJobs := make([]state.JobID, 0)
	for i := 0; i < n; i++ {
		jobID, err := jsm.Put(now, uint32(i), 0, 10, len(body), []byte(body), state.TubeName("foo"))
		if err != nil {
			t.Fatalf("error in jsm.Put. err=%v", err)
		} else {
			testJobs = append(testJobs, jobID)
		}
	}

	return testJobs
}

func testNowSecs() int64 {
	return time.Now().UTC().Unix()
}

type testSink struct {
	bytes.Buffer
}

func newTestSink() *testSink {
	return &testSink{
		bytes.Buffer{},
	}
}

func (t *testSink) Close() error {
	return nil
}

func (t *testSink) ID() string {
	return "id"
}

func (t *testSink) Cancel() error {
	return nil
}
