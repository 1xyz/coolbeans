package proxy

import (
	"fmt"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/state"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/health"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"sync"
	"time"
)

var (
	serviceConfig = `{
		"loadBalancingPolicy": "round_robin",
		"healthCheckConfig": {
			"serviceName": ""
		}
	}`
)

type Client struct {
	ProxyID     string
	ServerAddrs []string
	ConnTimeout time.Duration
	jsmClient   v1.JobStateMachineClient
	conn        *grpc.ClientConn
	rq          *reservationsQueue
	doneCh      chan bool

	// waiting is a set of clients that are waiting for a
	// reservation.
	waiting waitingClients

	// Boolean value to indicate if the proxy client needs
	// to an explicit recovery of the client.
	// This value can be accessed across two go-routines
	recoverClients *AtomicBool
}

func NewClient(proxyID string, serverAddrs []string, connTimeout time.Duration) *Client {
	return &Client{
		ProxyID:     proxyID,
		ServerAddrs: serverAddrs,
		ConnTimeout: connTimeout,
		rq: &reservationsQueue{
			mu: sync.Mutex{},
			q:  nil,
		},
		doneCh:         make(chan bool),
		waiting:        make(waitingClients),
		recoverClients: NewAtomicBool(),
	}
}

func (c *Client) Open() error {
	// Set up a connection to the server.
	conn, err := c.connect()
	if err != nil {
		log.Errorf("proxy.client.open: connect err = %v", err)
		return err
	}

	c.conn = conn
	c.jsmClient = v1.NewJobStateMachineClient(conn)

	go func() {
		for {
			if err := c.GetReservations(c.doneCh); err != nil {
				log.Errorf("proxy.client.Open: c.GetReservations: err = %v", err)
				// ToDo: Consider using a exponential backoff strategy if c.getReservations fails.
				// Refer: https://github.com/1xyz/coolbeans/issues/26
				b := c.recoverClients.SetIfFalse()
				log.Infof("proxy.client.Open: c.GetReservations: recoverClients.SetIfFalse result = %v", b)
			}
		}
	}()

	return nil
}

func (c *Client) Close() error {
	c.doneCh <- true
	close(c.doneCh)
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *Client) connect() (*grpc.ClientConn, error) {
	r, cleanup := manual.GenerateAndRegisterManualResolver()
	defer cleanup()

	addresses := make([]resolver.Address, 0)
	for _, s := range c.ServerAddrs {
		addresses = append(addresses, resolver.Address{Addr: s})
	}

	log.Debugf("proxy.client.connect: Addresses: %v\n", addresses)
	r.InitialState(resolver.State{
		Addresses: addresses,
	})

	address := fmt.Sprintf("%s:///unused", r.Scheme())
	options := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithDefaultServiceConfig(serviceConfig),
	}

	conn, err := grpc.Dial(address, options...)
	return conn, err
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Infof("%s took %s", name, elapsed)
}

func (c *Client) Put(nowSeconds int64, priority uint32, delay int64, ttr int,
	bodySize int, body []byte, tubeName state.TubeName) (state.JobID, error) {
	s := time.Now()
	defer timeTrack(s, "client.Put")

	ctx, cancel := context.WithTimeout(context.Background(), c.ConnTimeout)
	defer cancel()

	req := v1.PutRequest{
		Priority: priority,
		Delay:    delay,
		Ttr:      int32(ttr),
		TubeName: string(tubeName),
		BodySize: int32(bodySize),
		Body:     body,
	}

	r, err := c.jsmClient.Put(ctx, &req)
	if err != nil {
		log.Errorf("proxy.client.Put: c.jsmclient.put err: %v", err)
		return state.JobID(0), err
	}

	return state.JobID(r.JobId), err
}

func (c *Client) AppendReservation(clientID state.ClientID, reqID string, watchedTubes []state.TubeName,
	nowSecs, deadlineAt int64) (*state.Reservation, error) {
	s := time.Now()
	defer timeTrack(s, "proxy.client.AppendReservation")

	ctx, cancel := context.WithTimeout(context.Background(), c.ConnTimeout)
	defer cancel()

	wtubes := make([]string, 0)
	for _, t := range watchedTubes {
		wtubes = append(wtubes, string(t))
	}

	req := v1.ReserveRequest{
		ClientId:     string(clientID),
		ProxyId:      c.ProxyID,
		RequestId:    reqID,
		TimeoutSecs:  int32(deadlineAt - nowSecs),
		WatchedTubes: wtubes,
	}

	r, err := c.jsmClient.Reserve(ctx, &req)
	if err != nil {
		log.Errorf("proxy.client.AppendReservation: c.jsmclient.reserve err: %v", err)
		return nil, err
	}

	resv := r.Reservation
	if resv.Status == v1.ReservationStatus_Queued {
		cli := state.ClientID(resv.ClientId)
		if err := c.waiting.Add(cli); err != nil {
			log.Errorf("proxy.client.AppendReservation: c.waiting.Add clientID=%v err = %v", cli, err)
			return nil, err
		} else {
			log.Errorf("proxy.client.AppendReservation: clientID=%v added to waiting", cli)
		}
	}

	return &state.Reservation{
		RequestId: resv.RequestId,
		ClientId:  state.ClientID(resv.ClientId),
		JobId:     state.JobID(resv.JobId),
		Status:    state.ReservationStatus(resv.Status),
		BodySize:  int(resv.BodySize),
		Body:      resv.Body,
		Error:     fmt.Errorf(resv.ErrorMsg),
	}, nil
}

func (c *Client) Delete(jobID state.JobID, clientID state.ClientID) error {
	s := time.Now()
	defer timeTrack(s, "client.Delete")

	ctx, cancel := context.WithTimeout(context.Background(), c.ConnTimeout)
	defer cancel()

	_, err := c.jsmClient.Delete(ctx, &v1.DeleteRequest{
		JobId:    int64(jobID),
		ProxyId:  c.ProxyID,
		ClientId: string(clientID),
	})

	if err != nil {
		log.Errorf("c.jsmClient.delete err: %v", err)
		return err
	}

	return nil
}

func (c *Client) Bury(nowSeconds int64, jobID state.JobID, priority uint32, clientID state.ClientID) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.ConnTimeout)
	defer cancel()

	_, err := c.jsmClient.Bury(ctx, &v1.BuryRequest{
		JobId:    int64(jobID),
		Priority: priority,
		ClientId: string(clientID),
		ProxyId:  c.ProxyID,
	})

	if err != nil {
		log.Errorf("proxy.client: c.jsmClient.Bury err=%v", err)
	}

	return err
}

func (c *Client) Kick(jobID state.JobID) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.ConnTimeout)
	defer cancel()

	_, err := c.jsmClient.Kick(ctx, &v1.KickRequest{
		JobId: int64(jobID),
	})

	if err != nil {
		log.Errorf("proxy.client: c.jsmClient.Kick err=%v", err)
	}

	return err
}

func (c *Client) KickN(name state.TubeName, n int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.ConnTimeout)
	defer cancel()

	resp, err := c.jsmClient.KickN(ctx, &v1.KickNRequest{
		TubeName: string(name),
		Bound:    int32(n),
	})

	if err != nil {
		log.Errorf("proxy.client: c.jsmClient.KickN err=%v", err)
		return 0, err
	}

	return int(resp.JobsKicked), err
}

func (c *Client) Touch(nowSeconds int64, jobID state.JobID, clientID state.ClientID) error {
	return nil
}

func (c *Client) PeekDelayedJob(tubeName state.TubeName) (state.Job, error) {
	return nil, nil
}

func (c *Client) PeekReadyJob(tubeName state.TubeName) (state.Job, error) {
	return nil, nil
}

func (c *Client) PeekBuriedJob(tubeName state.TubeName) (state.Job, error) {
	return nil, nil
}

func (c *Client) GetJob(id state.JobID) (state.Job, error) {
	return nil, nil
}

func (c *Client) ReleaseWith(nowSeconds int64, jobID state.JobID, clientID state.ClientID, pri uint32, delay int64) error {
	return nil
}

func (c *Client) Tick(nowSeconds int64) ([]*state.Reservation, error) {
	result := make([]*state.Reservation, 0)
	entries := c.rq.Drain()
	if entries != nil {
		for _, r := range entries {
			if err := c.waiting.Remove(r.ClientId); err != nil {
				log.Errorf("proxy.client.Tick(): c.waiting.Remove. clientId=%v err=%v. Discarding entry",
					r.ClientId, err)
				continue
			}

			result = append(result, r)
		}
	}

	// Check to see if we need to recover any clients explicitly
	if !c.recoverClients.ResetIfTrue() {
		return result, nil
	}

	// ToDo: this approach is unclean, figure out a better way to address this
	// https://github.com/1xyz/coolbeans/issues/27
	log.Warnf("proxy.client.Tick(): attempting to recover client state")
	waiting := c.waiting.asSlice()
	if len(waiting) == 0 {
		log.Warnf("proxy.client.Tick(): no clients waiting for reservations.")
		return result, nil
	}

	_, notWaiting, missing, err := c.CheckClientState(waiting)
	if err != nil {
		log.Errorf("client.Tick(): c.CheckClientState err = %v", err)
		return result, nil
	}

	for _, nw := range notWaiting {
		log.Warnf("proxy.client.Tick(): entry for reservation in not waiting state %v", nw)
		result = append(result, &state.Reservation{
			RequestId: "00000000-0000-0000-0000-000000000000",
			ClientId:  nw,
			JobId:     0,
			Status:    state.Error,
			BodySize:  0,
			Body:      nil,
			Error:     fmt.Errorf("entry for reservation in not waiting state %v", nw),
		})

		c.waiting.Remove(nw)
	}

	for _, m := range missing {
		log.Warnf("proxy.client.Tick(): entry for reservation is not found %v", m)
		result = append(result, &state.Reservation{
			RequestId: "00000000-0000-0000-0000-000000000000",
			ClientId:  m,
			JobId:     0,
			Status:    state.Error,
			BodySize:  0,
			Body:      nil,
			Error:     fmt.Errorf("entry for reservation is not found %v", m),
		})

		c.waiting.Remove(m)
	}

	log.Warnf("proxy.client.Tick(): completed reconciling, set chkClient to false")
	return result, nil
}

func (c *Client) CheckClientState(clientIDs []state.ClientID) ([]state.ClientID,
	[]state.ClientID, []state.ClientID, error) {

	ctx, cancel := context.WithTimeout(context.Background(), c.ConnTimeout)
	defer cancel()

	s := make([]string, 0)
	for _, id := range clientIDs {
		s = append(s, string(id))
	}

	resp, err := c.jsmClient.CheckClientState(ctx,
		&v1.CheckClientStateRequest{ProxyId: c.ProxyID, ClientIds: s})
	if err != nil {
		return nil, nil, nil, err
	}

	return toClientIds(resp.WaitingClientIds),
		toClientIds(resp.NotWaitingClientIds),
		toClientIds(resp.MissingClientIds),
		err
}

func toClientIds(s []string) []state.ClientID {
	res := make([]state.ClientID, 0)
	for _, e := range s {
		res = append(res, state.ClientID(e))
	}

	return res
}

func (c *Client) GetReservations(done <-chan bool) error {
	ctx := context.Background()
	stream, err := c.jsmClient.StreamReserveUpdates(ctx,
		&v1.ReserveUpdateRequest{ProxyId: c.ProxyID})
	if err != nil {
		log.Errorf("GetReservations: err %v", err)
		return err
	}

	for {
		select {
		case <-done:
			log.Warnf("Done signalled")
			return nil
		default:
		}

		v1r, err := stream.Recv()
		if err != nil {
			log.Errorf("proxy.client.GetReservations: err %v", err)
			return err
		}

		r := state.Reservation{
			RequestId: v1r.Reservation.RequestId,
			ClientId:  state.ClientID(v1r.Reservation.ClientId),
			JobId:     state.JobID(v1r.Reservation.JobId),
			Status:    state.ReservationStatus(v1r.Reservation.Status),
			BodySize:  int(v1r.Reservation.BodySize),
			Body:      v1r.Reservation.Body,
			Error:     fmt.Errorf(v1r.Reservation.ErrorMsg),
		}

		c.rq.Enqueue(&r)
	}
}

func (c *Client) Release(jobID state.JobID, clientID state.ClientID) error {
	return fmt.Errorf("Unimplemented")
}

func (c *Client) Snapshot() (state.JSMSnapshot, error) {
	return nil, fmt.Errorf("Unimplemented")
}

type reservationsQueue struct {
	mu sync.Mutex

	q []*state.Reservation
}

func (rq *reservationsQueue) Enqueue(r *state.Reservation) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if rq.q == nil {
		rq.q = make([]*state.Reservation, 0)
	}

	rq.q = append(rq.q, r)
}

func (rq *reservationsQueue) Drain() []*state.Reservation {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if rq.q == nil {
		return nil
	}

	result := rq.q
	rq.q = nil
	return result
}

// Set of waiting clients
type waitingClients map[state.ClientID]bool

func (w waitingClients) Add(cli state.ClientID) error {
	if w.Contains(cli) {
		return state.ErrEntryExists
	}

	w[cli] = true
	return nil
}

func (w waitingClients) Contains(cli state.ClientID) bool {
	_, ok := w[cli]
	return ok
}

func (w waitingClients) Remove(cli state.ClientID) error {
	if !w.Contains(cli) {
		return state.ErrEntryMissing
	}

	delete(w, cli)
	return nil
}

func (w waitingClients) asSlice() []state.ClientID {
	result := make([]state.ClientID, 0)
	for k, _ := range w {
		result = append(result, k)
	}
	return result
}
