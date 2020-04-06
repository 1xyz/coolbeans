package cluster_test

import (
	"fmt"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/cluster"
	"github.com/1xyz/coolbeans/cluster/clusterfakes"
	"github.com/git-lfs/git-lfs/errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestReservationsController_RunStop(t *testing.T) {
	tests := []struct {
		tr  *v1.TickResponse
		err error
	}{
		{&v1.TickResponse{
			ProxyReservations: map[string]*v1.Reservations{}}, nil},
		{nil, errors.New("hello")},
	}
	for _, test := range tests {
		jsmTick := &clusterfakes.FakeJsmTick{}
		jsmTick.TickReturns(test.tr, test.err)
		rctrl := cluster.NewReservationsController(jsmTick)

		doneCh := make(chan bool)
		go func() {
			err := rctrl.Run()
			assert.Nilf(t, err, "expect Err to be nil")
			doneCh <- true
		}()

		time.Sleep(2 * time.Second)
		assert.Truef(t, jsmTick.TickCallCount() > 0,
			"expect TickCallCount > 0, actual = %v", jsmTick.TickCallCount())
		rctrl.Stop()
		<-doneCh
	}
}

func TestReservationsController_Register(t *testing.T) {
	doneCh := make(chan bool)
	rctrl := runTestController(t, doneCh)
	defer func() {
		rctrl.Stop()
		<-doneCh
	}()

	respCh, err := rctrl.Register("foobar")
	assert.Nilf(t, err, "expect Err to be nil")
	assert.NotNilf(t, respCh, "expect respCh to not be nil")

	respCh, err = rctrl.Register("foobar")
	assert.Equalf(t, cluster.ErrProxyExists, err, "expect Err to be ErrProxyExists")
	assert.Nilf(t, respCh, "expect respCh to be nil")
}

func TestReservationsController_UnRegister(t *testing.T) {
	doneCh := make(chan bool)
	rctrl := runTestController(t, doneCh)
	defer func() {
		rctrl.Stop()
		<-doneCh
	}()

	respCh, _ := rctrl.Register("foobar")
	rctrl.UnRegister("foobar", respCh)
	rctrl.UnRegister("foobar", respCh)

	respCh, err := rctrl.Register("foobar")
	assert.Nilf(t, err, "expect Err to be nil")
	assert.NotNilf(t, respCh, "expect respCh to not be nil")

}

func TestReservationsController_Run_With_Assinged_Reservations(t *testing.T) {
	doneCh := make(chan bool)
	proxyID := "proxy1"
	nReservations := 3
	rctrl := runtestControllerWithResponses(t, doneCh, proxyID, nReservations)
	defer func() {
		rctrl.Stop()
		<-doneCh
	}()

	respCh, _ := rctrl.Register(proxyID)
	for r := range respCh {
		assert.Equalf(t, cluster.Reservation, r.RespType, "expect respType to be reservation")
		assert.Equalf(t, nReservations, len(r.Reservations), "expect count of reservation to be %v actual=%v",
			nReservations, len(r.Reservations))
		break
	}
}

func TestReservationsController_Run_With_NoAssigned_Reservations(t *testing.T) {
	doneCh := make(chan bool)
	rctrl := runtestControllerWithResponses(t, doneCh, "proxy1", 1)
	defer func() {
		rctrl.Stop()
		<-doneCh
	}()

	respCh, _ := rctrl.Register("proxy2")
	for r := range respCh {
		assert.Equalf(t, cluster.Reservation, r.RespType,
			"expect respType to be of type %v got %v", cluster.Reservation, r.RespType)
		assert.Nilf(t, r.Reservations, "expect reservations to be nil")
		break
	}

	rctrl.UnRegister("proxy2", respCh)
}

func runTestController(t *testing.T, doneCh chan<- bool) *cluster.ReservationsController {
	jsmTick := &clusterfakes.FakeJsmTick{}
	jsmTick.TickReturns(nil, errors.New("foo"))
	rctrl := cluster.NewReservationsController(jsmTick)

	go func() {
		if err := rctrl.Run(); err != nil {
			t.Fatalf("expect Err=%v to be nil", err)
		}

		doneCh <- true
	}()

	return rctrl
}

func runtestControllerWithResponses(t *testing.T, doneCh chan<- bool, proxyID string, nReservations int) *cluster.ReservationsController {
	resvn := make([]*v1.Reservation, nReservations)
	for i := 0; i < nReservations; i++ {
		resvn[i] = &v1.Reservation{
			RequestId: fmt.Sprintf("test-request-%d", i),
			ClientId:  fmt.Sprintf("client://%s/test-clientid-%d", proxyID, i),
			Status:    v1.ReservationStatus_Timeout,
			JobId:     0,
			BodySize:  0,
			Body:      nil,
			ErrorMsg:  "",
			ProxyId:   proxyID,
		}
	}

	resp := v1.TickResponse{
		ProxyReservations: map[string]*v1.Reservations{
			proxyID: &v1.Reservations{Entries: resvn},
		},
	}

	jsmTick := &clusterfakes.FakeJsmTick{}
	jsmTick.TickReturns(&resp, nil)
	rctrl := cluster.NewReservationsController(jsmTick)

	go func() {
		if err := rctrl.Run(); err != nil {
			t.Fatalf("expect Err=%v to be nil", err)
		}

		doneCh <- true
	}()

	return rctrl
}
