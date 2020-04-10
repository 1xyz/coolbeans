package cluster

import (
	"errors"
	v1 "github.com/1xyz/coolbeans/api/v1"
	log "github.com/sirupsen/logrus"
	"time"
)

// reservationsController provides the ability to stream
// reservation updates from the job state machine (jsm)
// back to the connected clients (aka. "proxy" clients)
//
// A high level overview:
//
// ┌───────────────┐            ┌───────────────┐
// │  State Proxy  │            │  State Proxy  │
// |     Client    | ......     |     Client    |
// └───────────────┘            └───────────────┘
//         ^                            ^
//         |                            |
//         |                            | (stream Reservations)
//         |                            |
// ┌───────────────────────────────────────────────┐
// │              reservationsController           │
// └───────────────────────────────────────────────┘
//         |                      ^
//         | (every 1s)           | Reservations
//         |                      |
//         V                      |
// ┌───────────────────────────────────────────────┐
// │                 JSM.Tick()                    │
// └───────────────────────────────────────────────┘
type ReservationsController struct {
	// connProxies, represents connected proxy clients which
	// can receive reservation updates.
	// the map is keyed of the proxyID and the value is
	// channel where specific proxies receive their updates
	connProxies map[string]chan *ProxyResp

	// proxyJoinCh, all proxy join requests are sent here
	proxyJoinCh chan *ProxyJoinReq

	// proxyLeaveCh, all proxy leave requests are sent here
	proxyLeaveCh chan *ProxyLeaveReq

	// interface allowing the controller to access periodic
	// tick functionality
	jsmTick JsmTick

	// Channel to signal a stop
	doneCh chan bool
}

var (
	// Returned if the same proxy client attempts to connect with the controller
	ErrProxyExists = errors.New("proxy with id exists")
	ErrNotLeader   = errors.New("current node is not a leader")
)

func NewReservationsController(jsmTick JsmTick) *ReservationsController {
	return &ReservationsController{
		connProxies:  make(map[string]chan *ProxyResp),
		proxyJoinCh:  make(chan *ProxyJoinReq),
		proxyLeaveCh: make(chan *ProxyLeaveReq),
		jsmTick:      jsmTick,
		doneCh:       make(chan bool),
	}
}

// Register makes a request to add this proxy client (identified by proxyID)
//
// Register returns back a read only channel to receive updates
func (rctrl *ReservationsController) Register(proxyID string) (<-chan *ProxyResp, error) {
	logc := log.WithField("method", "jsmController.Register")

	req := &ProxyJoinReq{
		proxyID: proxyID,
		respCh:  make(chan *ProxyResp),
	}

	rctrl.proxyJoinCh <- req
	resp := <-req.respCh
	if resp.Err != nil {
		logc.Errorf("resp.Err with proxyID %v. Err %v", proxyID, resp.Err)
		return nil, resp.Err
	}

	logc.Infof("Register for proxyID=%v done", proxyID)
	return req.respCh, nil
}

// UnRegister makes a request to remove this proxy client (identified by the proxyID)
//
// Additionally, once the unRegister is complete, it drains the response channel
func (rctrl *ReservationsController) UnRegister(proxyID string, respCh <-chan *ProxyResp) {
	logc := log.WithField("method", "jsmController.UnRegister")
	rctrl.proxyLeaveCh <- &ProxyLeaveReq{
		proxyID: proxyID,
	}

	// drain all the responses & log errors
	for resp := range respCh {
		if resp.Err != nil {
			logc.Errorf("resp.Err with proxyID %v. Err %v", proxyID, resp.Err)
		}
	}

	logc.Infof("UnRegister for proxyID=%v done", proxyID)
}

// Run, runs this controller. the control loop does not return immediately (unless there is an error)
//
// Run performs the following functions
// 1) Periodically (for every second), queries the underlying JSM (job state machine) for any
//    reservation updates. (These could include newly assigned jobs, timeouts or deadline-soon to any
//    Reservations request). These updates are dispatched to the appropriate client proxy (if they are
//    connected.
// 2) Processes any register (join) or un-register (leave) requests from proxies
func (rctrl *ReservationsController) Run() error {
	logc := log.WithField("method", "Run")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rctrl.doneCh:
			logc.Infof("Done signalled")
			return nil

		case t := <-ticker.C:
			logc.Debugf("tick-req at %v", t)
			if tickResp, err := rctrl.jsmTick.Tick(); err != nil {
				logc.Debugf("jsmTick.Tick() Err=%v", err)
				continue
			} else {
				// map of proxies responded
				responded := make(map[string]bool)
				for proxyId, reservation := range tickResp.ProxyReservations {
					if respCh, ok := rctrl.connProxies[proxyId]; !ok {
						log.Warnf("no entry with proxyID=%v discarding forwarding", proxyId)
					} else {
						respCh <- &ProxyResp{
							RespType:     Reservation,
							Reservations: reservation.Entries,
							Err:          nil,
						}
					}

					responded[proxyId] = true
				}

				// send a nil response back to proxies which don't have
				// any responses to handle.
				for proxyId, respCh := range rctrl.connProxies {
					if _, ok := responded[proxyId]; !ok {
						respCh <- &ProxyResp{
							RespType:     Reservation,
							Reservations: nil,
							Err:          nil,
						}
					}
				}
			}

		case jreq := <-rctrl.proxyJoinCh:
			logc.Debugf("join-req proxyID=%v", jreq.proxyID)
			if _, ok := rctrl.connProxies[jreq.proxyID]; ok {
				logc.Errorf("join-req proxy with id = %v exists", jreq.proxyID)
				jreq.respCh <- &ProxyResp{
					RespType:     Join,
					Reservations: nil,
					Err:          ErrProxyExists,
				}
				close(jreq.respCh)
			} else {
				rctrl.connProxies[jreq.proxyID] = jreq.respCh
				jreq.respCh <- &ProxyResp{
					RespType:     Join,
					Reservations: nil,
					Err:          nil,
				}
			}

		case lreq := <-rctrl.proxyLeaveCh:
			logc.Debugf("leave-req proxyID=%v", lreq.proxyID)
			if respCh, ok := rctrl.connProxies[lreq.proxyID]; !ok {
				logc.Errorf("leave-req proxy with id = %v does not exists", lreq.proxyID)
			} else {
				respCh <- &ProxyResp{
					RespType:     Leave,
					Reservations: nil,
					Err:          nil,
				}
				close(respCh)
				delete(rctrl.connProxies, lreq.proxyID)
				logc.Infof("leave-req deleted entry with proxyID=%v", lreq.proxyID)
			}
		}

	}
}

func (rctrl *ReservationsController) Stop() {
	rctrl.doneCh <- true
	close(rctrl.doneCh)
}

// //////////////////////////////////////////////////////////////////

type ProxyRespType int

const (
	Unknown ProxyRespType = iota
	Join
	Leave
	Reservation
)

func (p ProxyRespType) String() string {
	return [...]string{"Unknown", "Join", "Leave", "Reservation"}[p]
}

type ProxyResp struct {
	RespType     ProxyRespType
	Reservations []*v1.Reservation
	Err          error
}

type ProxyJoinReq struct {
	proxyID string
	respCh  chan *ProxyResp
}

type ProxyLeaveReq struct {
	proxyID string
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . JsmTick

type JsmTick interface {
	Tick() (*v1.TickResponse, error)
}
