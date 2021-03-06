package state

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"testing"
	"time"
)

func TestNewJSM(t *testing.T) {
	jsm, err := NewJSM()
	assert.Nilf(t, err, "expect err to be nil")
	ljsm, ok := jsm.(*localJSM)
	assert.Truef(t, ok, "expect jsm to be instance of localJsm")
	assert.Equalf(t, 0, len(ljsm.tubes), "expect tubes len to be zero")
	assert.Equalf(t, 0, ljsm.jobs.Len(), "expect jobs len to be zero")
	assert.Equalf(t, 0, len(ljsm.reservedJobs), "expect reservedjobs len to be zero")

}

func TestLocalJSM_Put(t *testing.T) {
	var entries = []struct {
		delay         int64
		expectedState JobState
	}{
		{0, Ready},
		{10, Delayed},
	}

	tubeName := TubeName("foo")
	jsm := newTestJsm(t)
	for i, e := range entries {
		expectedID := JobID(i + 1)
		pri := uint32(1)
		ttr := 2
		body := []byte("hello")

		jID, err := jsm.Put(testNowSecs(), pri, e.delay, ttr, len(body), body, tubeName)

		assert.Nilf(t, err, "expect err to be nil")
		assert.Equalf(t, expectedID, jID, "expect id to match")
	}
}

func TestLocalJSM_NextDelayedJob(t *testing.T) {
	tubeName := TubeName("foo")
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, tubeName, true)

	j2, err := jsm.PeekDelayedJob(tubeName)
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, j2, "expect job to be not nil")
	assert.Equalf(t, j.ID(), j2.ID(), "expect the id's to match")
}

func TestLocalJSM_NextDelayedJob_ErrMissing(t *testing.T) {
	jsm := newTestJsm(t)
	_, err := jsm.PeekDelayedJob(TubeName("foo"))
	assert.Equalf(t, ErrEntryMissing, err, "expected ErrEntryMissing")
}

func TestLocalJSM_NextReadyJob(t *testing.T) {
	tubeName := TubeName("foo")
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, tubeName, false)

	j2, err := jsm.PeekReadyJob(tubeName)
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, j2, "expect job to be not nil")
	assert.Equalf(t, j.ID(), j2.ID(), "expect the id's to match")
}

func TestLocalJSM_NextReadyJob_ErrMissing(t *testing.T) {
	jsm := newTestJsm(t)
	_, err := jsm.PeekReadyJob(TubeName("foo"))
	assert.Equalf(t, ErrEntryMissing, err, "expected ErrEntryMissing")
}

func TestLocalJSM_Ready(t *testing.T) {
	tubeName := TubeName("foo")
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, tubeName, true)

	err := jsm.Ready(j.ID())
	assert.Nilf(t, err, "expect err to be nil")

	j2, err := jsm.PeekReadyJob(tubeName)
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, j2, "expect job to be not nil")
	assert.Equalf(t, j.ID(), j2.ID(), "expect the id's to match")
	assert.Equalf(t, Ready, j2.State(), "expect the job to be transitioned")
}

func TestLocalJSM_Ready_ErrInvalidJobTransition(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)

	err := jsm.Ready(j.ID())
	assert.Equalf(t, ErrInvalidJobTransition, err,
		"expect err to be ErrInvalidJobTransition")
}

func TestLocalJSM_Reserve(t *testing.T) {
	jsm := newTestJsm(t)
	clientId := ClientID("foobar")
	j := putTestJob(t, jsm, TubeName("foo"), false)

	err := jsm.Reserve(testNowSecs(), j.ID(), clientId)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Reserved, j.State(), "Expect job state to be reserved")
	assert.Equalf(t, clientId, j.ReservedBy(), "Expect job to be reserved by foobar")
}

func TestLocalJSM_Reserve_ErrInvalidJobTransition(t *testing.T) {
	jsm := newTestJsm(t)
	clientId := ClientID("foobar")
	j := putTestJob(t, jsm, TubeName("foo"), true)

	err := jsm.Reserve(testNowSecs(), j.ID(), clientId)
	assert.Equalf(t, ErrInvalidJobTransition, err,
		"expect err to be ErrInvalidJobTransition")
}

func TestLocalJSM_Reserve_ErrMissing(t *testing.T) {
	jsm := newTestJsm(t)
	err := jsm.Reserve(testNowSecs(), JobID(213), ClientID("foobar"))
	assert.Equalf(t, ErrEntryMissing, err,
		"expect err to be ErrInvalidJobTransition")
}

func TestLocalJSM_Release(t *testing.T) {
	jsm := newTestJsm(t)
	clientId := ClientID("foobar")
	j := putTestJob(t, jsm, TubeName("foo"), false)
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientId); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	err := jsm.Release(j.ID(), clientId)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Ready, j.State(), "Expect job state to be ready")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to be reserved by foobar")
}

func TestLocalJSM_Release_WithNoClientID(t *testing.T) {
	jsm := newTestJsm(t)
	clientId := ClientID("foobar")
	j := putTestJob(t, jsm, TubeName("foo"), false)
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientId); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	err := jsm.Release(j.ID(), ClientID(""))
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Ready, j.State(), "Expect job state to be ready")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to be reserved by foobar")
}

func TestLocalJSM_Release_ErrUnauthorizedOperation(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)
	if err := jsm.Reserve(testNowSecs(), j.ID(), ClientID("foobar")); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	err := jsm.Release(j.ID(), ClientID("foofoo"))
	assert.Equalf(t, ErrUnauthorizedOperation, err,
		"expect err to be ErrUnauthorizedOperation")
}

func TestLocalJSM_ReleaseWith_ToReady(t *testing.T) {
	jsm := newTestJsm(t)
	clientId := ClientID("foobar")
	j := putTestJob(t, jsm, TubeName("foo"), false)
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientId); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	newPri := j.Priority() + 10
	err := jsm.ReleaseWith(testNowSecs(), j.ID(), clientId, newPri, 0)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Ready, j.State(), "Expect job state to be ready")
	assert.Equalf(t, newPri, j.Priority(), "Expect job Priority to be updated")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to not be reserved by foobar")
}

func TestLocalJSM_ReleaseWith_ToDelayed(t *testing.T) {
	jsm := newTestJsm(t)
	clientId := ClientID("foobar")
	j := putTestJob(t, jsm, TubeName("foo"), false)
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientId); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	now := testNowSecs()
	newPri := j.Priority() + 10
	newDelay := int64(30)
	err := jsm.ReleaseWith(now, j.ID(), clientId, newPri, newDelay)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Delayed, j.State(), "Expect job state to be ready")
	assert.Equalf(t, newPri, j.Priority(), "Expect job Priority to be updated")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to not be reserved by foobar")
	assert.Equalf(t, now+newDelay, j.ReadyAt(), "Expect job readyAt updated")
	assert.Equalf(t, newDelay, j.Delay(), "Expect job Delay to be updated")
}

func TestLocalJSM_Release_ErrInvalidJobTransition(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)
	clientID := ClientID("foobar")
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientID); err != nil {
		t.Fatalf("reserve error %v", err)
	}
	if err := jsm.Release(j.ID(), clientID); err != nil {
		t.Fatalf("release error %v", err)
	}

	err := jsm.Release(j.ID(), clientID)
	assert.Equalf(t, ErrInvalidJobTransition, err,
		"expect err to be ErrInvalidJobTransition")
}

func TestLocalJSM_Delete_NonReadyJob(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)

	err := jsm.Delete(j.ID(), ClientID("foobar"))
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Deleted, j.State(), "Expect job state to be ready")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to be reserved by foobar")
}

func TestLocalJSM_Delete_DelayedJob(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), true)

	err := jsm.Delete(j.ID(), ClientID("foobar"))
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Deleted, j.State(), "Expect job state to be ready")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to be reserved by foobar")
}

func TestLocalJSM_Delete_ReservedJob(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)
	clientID := ClientID("foobar")
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientID); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	err := jsm.Delete(j.ID(), clientID)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Deleted, j.State(), "Expect job state to be Deleted")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to be reserved by none")
}

func TestLocalJSM_Delete_ReservedJob_ErrUnauthorizedOperation(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)
	if err := jsm.Reserve(testNowSecs(), j.ID(), ClientID("foobar")); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	err := jsm.Delete(j.ID(), ClientID("foofoo"))
	assert.Equalf(t, ErrUnauthorizedOperation, err,
		"expect err to be ErrUnauthorizedOperation")
}

func TestLocalJSM_Bury_ReservedJob(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)
	clientID := ClientID("foobar")
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientID); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	newPri := j.Priority() + 1
	buriedAt := testNowSecs()
	err := jsm.Bury(buriedAt, j.ID(), newPri, clientID)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Buried, j.State(), "Expect job state to be Buried")
	assert.Equalf(t, ClientID(""), j.ReservedBy(), "Expect job to be reserved by foobar")
	assert.Equalf(t, buriedAt, j.BuriedAt(), "Expect job to be buriedAt at expected value")
	assert.Equalf(t, newPri, j.Priority(), "Expect job to be Priority to be updated")
}

func TestLocalJSM_Bury_ReservedJob_ErrUnauthorizedOperation(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)
	if err := jsm.Reserve(testNowSecs(), j.ID(), ClientID("foobar")); err != nil {
		t.Fatalf("reserve error %v", err)
	}

	err := jsm.Bury(testNowSecs(), j.ID(), 1, ClientID("foofoo"))
	assert.Equalf(t, ErrUnauthorizedOperation, err,
		"expect err to be ErrUnauthorizedOperation")
}

func TestLocalJSM_Bury_ReadyJob_ErrInvalidJobTransition(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)

	newPri := j.Priority() + 1
	buriedAt := testNowSecs()
	err := jsm.Bury(buriedAt, j.ID(), newPri, ClientID("foofoo"))
	assert.Equalf(t, ErrInvalidJobTransition, err,
		"expect err to be ErrInvalidJobTransition")
}

func TestLocalJSM_Bury_UnknownJob_ErrEntryMissing(t *testing.T) {
	jsm := newTestJsm(t)
	buriedAt := testNowSecs()
	err := jsm.Bury(buriedAt, JobID(1234), 255, ClientID("foofoo"))
	assert.Equalf(t, ErrEntryMissing, err,
		"expect err to be ErrEntryMissing")
}

func TestLocalJSM_Kick_BuriedJob(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)
	clientID := ClientID("foobar")
	if err := jsm.Reserve(testNowSecs(), j.ID(), clientID); err != nil {
		t.Fatalf("reserve error %v", err)
	}
	if err := jsm.Bury(testNowSecs(), j.ID(), j.Priority()+1, clientID); err != nil {
		t.Fatalf("bury error %v", err)
	}

	err := jsm.Kick(j.ID())
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, Ready, j.State(), "Expect job state to be Rwady")
	assert.Equalf(t, int64(0), j.BuriedAt(), "Expect job to be buriedAt at expected value")
}

func TestLocalJSM_Kick_UnknownJob_ErrEntryMissing(t *testing.T) {
	jsm := newTestJsm(t)
	err := jsm.Kick(JobID(1234))
	assert.Equalf(t, ErrEntryMissing, err,
		"expect err to be ErrEntryMissing")
}

func TestLocalJSM_Kick_ReadyJob_ErrInvalidJobTransition(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)

	err := jsm.Kick(j.ID())
	assert.Equalf(t, ErrInvalidJobTransition, err,
		"expect err to be ErrInvalidJobTransition")
}

func TestLocalJSM_KickN(t *testing.T) {
	tubeName := TubeName("foo")
	var tc = []struct {
		tube                            TubeName
		n, kickCount, expectedKickCount int
	}{
		{tubeName, 10, 3, 3},
		{tubeName, 0, 30, 0},
		{tubeName, 10, 30, 10},
		{TubeName("bar"), 10, 30, 0},
	}
	for _, tt := range tc {
		jsm := newTestJsm(t)
		jobs := buryNJobs(t, jsm, tt.n, tubeName)

		actualKickCount, err := jsm.KickN(tt.tube, tt.kickCount)
		assert.Nilf(t, err, "expect err to be nil")
		assert.Equalf(t, tt.expectedKickCount, actualKickCount, "Expect KickedCount to match")
		for i := 0; i < actualKickCount; i++ {
			assert.Equalf(t, Ready, jobs[i].State(), "Expect job state to be Ready")
		}
		for i := actualKickCount; i < tt.n; i++ {
			assert.Equalf(t, Buried, jobs[i].State(), "Expect job state to be Buried")
		}
	}
}

func TestLocalJSM_Touch_ReservedJob(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	clientID := ClientID("client-123")
	j := putTestJob(t, jsm, tubes[0], false)
	r := createTestResv(t, jsm, clientID, tubes, 30)
	if r.Status != Matched {
		t.Fatalf("expect reservation to be matched")
	}

	oldExpiresAt := j.ExpiresAt()
	var delta int64 = 10
	err := jsm.Touch(testNowSecs()+delta, j.ID(), clientID)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Greaterf(t, j.ExpiresAt(), oldExpiresAt, "Expect expiresAt to be updated")
	assert.Equalf(t, j.State(), Reserved, "Expecte state to be Reserved")
}

func TestLocalJSM_Touch_NonReservedJob(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	clientID := ClientID("client-123")
	j := putTestJob(t, jsm, tubes[0], false)

	err := jsm.Touch(testNowSecs(), j.ID(), clientID)
	assert.Equalf(t, ErrInvalidOperation, err, "expect err to be ErrInvalidOperation")
}

func TestLocalJSM_Touch_UnknownJob(t *testing.T) {
	jsm := newTestJsm(t)
	err := jsm.Touch(testNowSecs(), JobID(123), ClientID("client-123"))
	assert.Equalf(t, ErrEntryMissing, err, "expect err to be ErrEntryMissing")
}

func TestLocalJSM_Touch_UnauthorizedClient(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	clientID := ClientID("client-123")
	j := putTestJob(t, jsm, tubes[0], false)
	r := createTestResv(t, jsm, clientID, tubes, 30)
	if r.Status != Matched {
		t.Fatalf("expect reservation to be matched")
	}

	err := jsm.Touch(testNowSecs()+10, j.ID(), ClientID("foo"))
	assert.Equalf(t, ErrUnauthorizedOperation, err, "expect err to be ErrUnauthorizedOperation")
}

func buryNJobs(t *testing.T, jsm *localJSM, n int, tubeName TubeName) []Job {
	jobs := make([]Job, n)
	clientID := ClientID("foobar")
	for i := 0; i < n; i++ {
		j := putTestJob(t, jsm, tubeName, false)
		if err := jsm.Reserve(testNowSecs(), j.ID(), clientID); err != nil {
			t.Fatalf("reserve error %v", err)
		}
		if err := jsm.Bury(testNowSecs(), j.ID(), j.Priority(), clientID); err != nil {
			t.Fatalf("bury error %v", err)
		}
		jobs[i] = j
	}

	return jobs
}

func TestLocalJSM_GetJob(t *testing.T) {
	jsm := newTestJsm(t)
	j := putTestJob(t, jsm, TubeName("foo"), false)

	j2, err := jsm.GetJob(j.ID())
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, j.ID(), j2.ID(), "Expect Id to match")

	_, err = jsm.GetJob(JobID(123456))
	assert.Equalf(t, ErrEntryMissing, err,
		"expect err to be ErrEntryMissing")
}

func TestLocalJSM_AppendReservation_Matched(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	cliID := ClientID("client-123")
	j := putTestJob(t, jsm, tubes[0], false)
	r := createTestResv(t, jsm, cliID, tubes, 30)

	assert.Equalf(t, Matched, r.Status, "expect resvn to be matched")
	assert.Equalf(t, j.ID(), r.JobId, "expect job id to be matched")
	assert.Equalf(t, j.BodySize(), r.BodySize, "expect BodySize to be equal")
	assert.Equalf(t, j.Body(), r.Body, "expect BodySize to be equal")
	assert.Equalf(t, cliID, r.ClientId, "expect ClientId to be set")
	assert.NotEqualf(t, "", r.RequestId, "expect ReqID to be set")
}

func TestLocalJSM_AppendReservation_Queued(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	cliID := ClientID("client-123")
	putTestJob(t, jsm, tubes[0], true)
	r := createTestResv(t, jsm, cliID, tubes, 30)

	assert.Equalf(t, Queued, r.Status, "expect resvn to be matched")
	assert.Equalf(t, cliID, r.ClientId, "expect ClientId to be set")
	assert.NotEqualf(t, "", r.RequestId, "expect ReqID to be set")
}

func TestLocalJSM_Tick_Returns_Empty(t *testing.T) {
	jsm := newTestJsm(t)

	r, err := jsm.Tick(testNowSecs())
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, 0, len(r), "expect r to have 0 entries")
}

func TestLocalJSM_Tick_Returns_MatchedResvn(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	cliID := ClientID("client-123")
	if r := createTestResv(t, jsm, cliID, tubes, 30); r.Status != Queued {
		t.Fatalf("expect r.Status = %v to be Queued", r.Status)
	}

	j := putTestJob(t, jsm, tubes[0], false)

	r, err := jsm.Tick(testNowSecs())
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, 1, len(r), "expect r to have 1 entries")
	assert.Equalf(t, Matched, r[0].Status, "expect resvn to be matched")
	assert.Equalf(t, j.ID(), r[0].JobId, "expect job id to be matched")
	assert.Equalf(t, j.BodySize(), r[0].BodySize, "expect BodySize to be equal")
	assert.Equalf(t, j.Body(), r[0].Body, "expect BodySize to be equal")
	assert.Equalf(t, cliID, r[0].ClientId, "expect ClientId to be set")
	assert.NotEqualf(t, "", r[0].RequestId, "expect ReqID to be set")
}

func TestLocalJSM_Tick_TransitionsDelayedJob_Returns_MatchedResvn(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	cliID := ClientID("client-123")
	j := putTestJob(t, jsm, tubes[0], true)
	if r := createTestResv(t, jsm, cliID, tubes, 300); r.Status != Queued {
		t.Fatalf("expect r.Status = %v to be Queued", r.Status)
	}

	r, err := jsm.Tick(testNowSecs() + 100)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, 1, len(r), "expect r to have 1 entries")
	assert.Equalf(t, Matched, r[0].Status, "expect resvn to be matched")
	assert.Equalf(t, j.ID(), r[0].JobId, "expect job id to be matched")
	assert.Equalf(t, j.BodySize(), r[0].BodySize, "expect BodySize to be equal")
	assert.Equalf(t, j.Body(), r[0].Body, "expect BodySize to be equal")
	assert.Equalf(t, cliID, r[0].ClientId, "expect ClientId to be set")
	assert.NotEqualf(t, "", r[0].RequestId, "expect ReqID to be set")
}

func TestLocalJSM_Tick_Returns_TimeOutResvn(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	cliID := ClientID("client-123")
	if r := createTestResv(t, jsm, cliID, tubes, 10); r.Status != Queued {
		t.Fatalf("expect r.Status = %v to be Queued", r.Status)
	}

	r, err := jsm.Tick(testNowSecs() + 15)
	assert.Nilf(t, err, "expect err to be nil")
	assert.Equalf(t, 1, len(r), "expect r to have 1 entries")
	assert.Equalf(t, Timeout, r[0].Status, "expect resvn to be timedout")
	assert.NotEqualf(t, "", r[0].RequestId, "expect ReqID to be set")
}

func TestLocalJSM_CheckClientState(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	cliIDs := []ClientID{
		ClientID("client-123"),
		ClientID("foobar-123"),
		ClientID("wooyoo-123"),
	}

	if r := createTestResv(t, jsm, cliIDs[0], tubes, 10); r.Status != Queued {
		t.Fatalf("expect r.Status = %v to be Queued", r.Status)
	}

	_, err := jsm.Tick(testNowSecs() + 15)
	if r := createTestResv(t, jsm, cliIDs[1], tubes, 10); r.Status != Queued {
		t.Fatalf("expect r.Status = %v to be Queued", r.Status)
	}

	waiting, notWaiting, missing, err := jsm.CheckClientState(cliIDs)
	assert.Nilf(t, err, "expect err to be nil")

	assert.Equalf(t, 1, len(waiting), "expect waiting to be 1")
	assert.Equalf(t, []ClientID{cliIDs[1]}, waiting, "expect cliIDs[0] to be in waiting")

	assert.Equalf(t, 1, len(notWaiting), "expect notWaiting to be one")
	assert.Equalf(t, []ClientID{cliIDs[0]}, notWaiting, "expect cliIDs[1] to be in notWaiting state")

	assert.Equalf(t, 1, len(missing), "expect missing to be one")
	assert.Equalf(t, []ClientID{cliIDs[2]}, missing, "expect cliIDs[1] to be in missing state")
}

func TestLocalJSM_GetStatsJobAsYaml(t *testing.T) {
	jsm := newTestJsm(t)
	tubeName := TubeName("garbanzo-beans")
	tubes := []TubeName{tubeName}
	j := putTestJob(t, jsm, tubes[0], true)

	data, err := jsm.GetStatsJobAsYaml(testNowSecs(), j.ID())
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, data, "expect bytes to be not nil")

	// De-Serialize this Yaml => map
	m := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(data), &m)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	assert.Equalf(t, map[string]interface{}{
		"id":        1,
		"tube":      string(tubeName),
		"state":     "delayed",
		"pri":       0x1,
		"age":       0,
		"delay":     defaultTestDelay,
		"ttr":       defaultTestTTR,
		"time-left": defaultTestDelay,
		"file":      0,
		"reserves":  0,
		"timeouts":  0,
		"releases":  0,
		"buries":    0,
		"kicks":     0,
	}, m, "expect maps to match")
}

func TestLocalJSM_GetStatsJobAsYaml_NotFound(t *testing.T) {
	jsm := newTestJsm(t)
	_, err := jsm.GetStatsJobAsYaml(testNowSecs(), JobID(1334))
	assert.Equalf(t, ErrEntryMissing, err, "expect err to be ErrEntryMissing")
}

func TestLocalJSM_GetStatsTubeAsYaml(t *testing.T) {
	jsm := newTestJsm(t)
	tubes := []TubeName{"foo"}
	cliID := ClientID("client-123")
	for i := 0; i < 3; i++ {
		putTestJob(t, jsm, tubes[0], false)
	}
	for i := 0; i < 3; i++ {
		putTestJob(t, jsm, tubes[0], true)
	}
	if r := createTestResv(t, jsm, cliID, tubes, 30); r.Status != Matched {
		t.Fatalf("expect r.Status = %v to be Queued", r.Status)
	}

	data, err := jsm.GetStatsTubeAsYaml(testNowSecs(), tubes[0])
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, data, "expect bytes to be not nil")

	// De-Serialize this Yaml => map
	m := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(data), &m)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	assert.Equalf(t, map[string]interface{}{
		"cmd-delete":            0,
		"cmd-pause-tube":        0,
		"current-jobs-buried":   0,
		"current-jobs-delayed":  3,
		"current-jobs-ready":    2,
		"current-jobs-reserved": 1,
		"current-jobs-urgent":   0,
		"current-using":         0,
		"current-waiting":       0,
		"current-watching":      0,
		"name":                  string(tubes[0]),
		"pause":                 0,
		"pause-time-left":       0,
		"total-jobs":            0,
	}, m, "expect maps to match")
}

func TestLocalJSM_GetStatsTubeAsYaml_NotFound(t *testing.T) {
	jsm := newTestJsm(t)
	_, err := jsm.GetStatsTubeAsYaml(testNowSecs(), TubeName("foobar"))
	assert.Equalf(t, ErrEntryMissing, err, "expect err to be ErrEntryMissing")
}

func TestLocalJSM_SnapshotReturnsNotNil(t *testing.T) {
	snap, err := newTestJsm(t).Snapshot()
	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, snap, "expect snap to not be nil")

	entries, err := snap.SnapshotJobs()

	assert.Nilf(t, err, "expect err to be nil")
	assert.NotNilf(t, entries, "expect entries to not be nil")
}

func TestLocalSnapshot_SnapshotJobs(t *testing.T) {
	n := 10
	jsm, m := createNTestJobs(t, 10, TubeName("foo"), false)
	entries := snapshotEntries(t, jsm)

	count := 0
	for entry := range entries {
		e, ok := m[entry.ID()]
		assert.Truef(t, ok, "expect ok to be true")
		assert.Equalf(t, e, entry, "expect job to be equal")
		delete(m, e.ID())
		count++
	}

	assert.Equalf(t, n, count, "expect count to equal n")
}

func TestLocalSnapshot_RestoreJobs(t *testing.T) {
	nReady := 10
	nDelayed := 3
	nReserved := 5
	nBuried := 4
	n := nReady + nDelayed + nReserved + nBuried
	tube := TubeName("foo")
	reservedBy := ClientID("foofoo")
	jsm := createMixTestJobs(t, tube, nReady, nDelayed, nReserved, nBuried, reservedBy)
	entries := snapshotEntries(t, jsm)

	jsm2 := newTestJsm(t)
	snap2, _ := jsm2.Snapshot()

	restoreCh := make(chan Job)
	doneCh := make(chan bool)
	go func() {
		err := snap2.RestoreJobs(context.Background(), restoreCh)
		assert.Nilf(t, err, "expect to not be nil")
		doneCh <- true
		close(doneCh)
	}()

	for entry := range entries {
		restoreCh <- entry
	}
	close(restoreCh)
	<-doneCh
	snap2.FinalizeRestore()

	assert.Equalf(t, n, jsm2.jobs.Len(), "expect n=%v jobs to be restored", n)
	assert.Equalf(t, nReady, jsm2.tubes[tube].readyJobs.Len(),
		"expect n=%v jobs to be restored as ready in tube foo", nReady)
	assert.Equalf(t, nDelayed, jsm2.tubes[tube].delayedJobs.Len(),
		"expect n=%v jobs to be restored as delayed in tube foo", nDelayed)
	assert.Equalf(t, nBuried, jsm2.tubes[tube].buriedJobs.Len(),
		"expect n=%v jobs to be restored as buriedJobs in tube foo", nReserved)
	assert.Equalf(t, nReserved, jsm2.reservedJobs[reservedBy].Len(),
		"expect n=%v jobs to be restored as reserved in tube foo", nReserved)
}

func snapshotEntries(t *testing.T, jsm JSM) <-chan Job {
	snap, err := jsm.Snapshot()
	if err != nil {
		t.Fatalf("jsm.Snapshot() err to not be nil err=%v", err)
	}
	entries, err := snap.SnapshotJobs()
	if err != nil {
		t.Fatalf("snap.SnapshotJobs err to not be nil err=%v", err)
	}
	return entries
}

func createNTestJobs(t *testing.T, n int, tube TubeName, hasDelay bool) (*localJSM, map[JobID]Job) {
	jsm := newTestJsm(t)
	m := make(map[JobID]Job)
	for i := 0; i < n; i++ {
		j := putTestJob(t, jsm, tube, hasDelay)
		m[j.ID()] = j
	}
	return jsm, m
}

func createMixTestJobs(t *testing.T, tube TubeName, nReady, nDelayed, nReserved, nBuried int, clientID ClientID) *localJSM {
	jsm := newTestJsm(t)
	for i := 0; i < nReady; i++ {
		putTestJob(t, jsm, tube, false)
	}
	for i := 0; i < nDelayed; i++ {
		putTestJob(t, jsm, tube, true)
	}
	now := testNowSecs()
	for i := 0; i < nReserved; i++ {
		j := putTestJob(t, jsm, tube, false)
		j.UpdateReservedBy(clientID)
		j.UpdateReservation(now)
		j.UpdateState(Reserved)
	}
	buryNJobs(t, jsm, nBuried, tube)
	return jsm
}

const (
	defaultTestPri   = uint32(1)
	defaultTestTTR   = 2
	defaultTestDelay = 10
)

func putTestJob(t *testing.T, jsm *localJSM, tubeName TubeName, hasDelay bool) Job {
	pri := defaultTestPri
	ttr := defaultTestTTR
	body := []byte("hello")
	var delay int64 = 0
	if hasDelay {
		delay = defaultTestDelay
	}

	j, err := jsm.NewJob(testNowSecs(), pri, delay, ttr, len(body), body, tubeName)
	if err != nil {
		t.Fatalf("expected err to not be nil")
	}
	return j
}

func createTestResv(t *testing.T, jsm *localJSM, cliID ClientID, tubes []TubeName,
	timeout int64) *Reservation {

	nowSecs := testNowSecs()
	if r, err := jsm.AppendReservation(cliID, "random-request",
		tubes, nowSecs, nowSecs+timeout); err != nil {
		t.Fatalf("appendReservation err=%v", err)
		return nil
	} else {
		return r
	}
}

func newTestJsm(t *testing.T) *localJSM {
	jsm, err := NewJSM()
	if err != nil {
		t.Fatalf("expect err to be nil")
	}

	var ljsm *localJSM = nil
	ljsm, ok := jsm.(*localJSM)
	if !ok {
		t.Fatalf("expect jsm to cast to *localJSM")
	}
	return ljsm
}

func testNowSecs() int64 {
	return time.Now().UTC().Unix()
}

func TestLocalSnapshot_SnapshotClients(t *testing.T) {
	n := 10
	tubes := []TubeName{"foo", "bar"}
	jsm := newTestJsm(t)
	jsm, _ = createNTestClients(t, jsm, n, tubes)
	entries := snapshotClient(t, jsm)

	count := 0
	for entry := range entries {
		expectedCliID := ClientID(fmt.Sprintf("client-%v", count))
		assert.Equalf(t, expectedCliID, entry.CliID, "expecte client id to match")
		count += 1
	}

	assert.Equalf(t, n, count, "expect count to equal n")
}

func TestLocalSnapshot_RestoreClients(t *testing.T) {
	n := 10
	tubes := []TubeName{"foo", "bar"}
	jsm := newTestJsm(t)
	jsm, _ = createNTestClients(t, jsm, n, tubes)
	entries := snapshotClient(t, jsm)

	jsm2 := newTestJsm(t)
	snap2, _ := jsm2.Snapshot()

	restoreCh := make(chan *ClientResvEntry)
	doneCh := make(chan bool)
	go func() {
		err := snap2.RestoreClients(context.Background(), n, restoreCh)
		assert.Nilf(t, err, "expect to not be nil")
		doneCh <- true
		close(doneCh)
	}()

	for entry := range entries {
		restoreCh <- entry
	}
	close(restoreCh)
	<-doneCh
	snap2.FinalizeRestore()

	assert.Equalf(t, n, jsm2.clients.Len(), "expect n=%v clients to be restored", n)
	for _, tname := range tubes {
		qlen, _ := jsm2.tubes.WaitQLen(tname)
		assert.Equalf(t, n, qlen, "expect qlen=%v to be = %v on tube foo", qlen, n)
	}
	assert.Equalf(t, n, jsm2.clientHeap.Len(), "expect client heap to be unpopulated")
}

func createNTestClients(t *testing.T, jsm *localJSM, n int,
	tubes []TubeName) (*localJSM, []*Reservation) {

	resvn := make([]*Reservation, 0)
	for i := 0; i < n; i++ {
		r := createTestResv(t, jsm, ClientID(fmt.Sprintf("client-%d", i)),
			tubes, 30)
		resvn = append(resvn, r)
	}
	return jsm, resvn
}

func snapshotClient(t *testing.T, jsm JSM) <-chan *ClientResvEntry {
	snap, err := jsm.Snapshot()
	if err != nil {
		t.Fatalf("jsm.Snapshot() err to not be nil err=%v", err)
	}
	entries, err := snap.SnapshotClients()
	if err != nil {
		t.Fatalf("snap.SnapshotClients err to not be nil err=%v", err)
	}
	return entries
}
