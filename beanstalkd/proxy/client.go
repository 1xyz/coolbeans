package proxy

import (
	"fmt"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/state"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io"
	"sync"
	"time"
)

type Client struct {
	ProxyID     string
	ServerAddr  []string
	ConnTimeout time.Duration
	jsmClient   v1.JobStateMachineClient
	conn        *grpc.ClientConn
	rq          *reservationsQueue
	doneCh      chan bool
}

func NewClient(proxyID string, serverAddr []string, connTimeout time.Duration) *Client {
	return &Client{
		ProxyID:     proxyID,
		ServerAddr:  serverAddr,
		ConnTimeout: connTimeout,
		rq: &reservationsQueue{
			mu: sync.Mutex{},
			q:  nil,
		},
		doneCh: make(chan bool),
	}
}

func (c *Client) Open() error {
	// Set up a connection to the server.
	conn, err := grpc.Dial(c.ServerAddr[0], grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Errorf("did not connect: %v", err)
	}

	c.conn = conn
	c.jsmClient = v1.NewJobStateMachineClient(conn)

	go func() {
		err := c.RecvUpdates(c.doneCh)
		if err != nil {
			log.Errorf("c.RecvUpdates(..) %v", err)
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
		log.Errorf("c.jsmclient.put err: %v", err)
		return state.JobID(0), err
	}

	return state.JobID(r.JobId), err
}

func (c *Client) AppendReservation(clientID state.ClientID, reqID string, watchedTubes []state.TubeName,
	nowSecs, deadlineAt int64) (*state.Reservation, error) {
	s := time.Now()
	defer timeTrack(s, "client.AppendReservation")

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
		log.Errorf("c.jsmclient.reserve err: %v", err)
		return nil, err
	}

	resv := r.Reservation
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

func (c *Client) Tick(nowSeconds int64) []*state.Reservation {
	if result := c.rq.Drain(); result == nil {
		return make([]*state.Reservation, 0)
	} else {
		return result
	}
}

func (c *Client) RecvUpdates(done <-chan bool) error {
	ctx := context.Background()

	stream, err := c.jsmClient.StreamReserveUpdates(ctx,
		&v1.ReserveUpdateRequest{
			ProxyId: c.ProxyID,
		})

	if err != nil {
		log.Errorf("%v.StreamReserveUpdates(_) = _, %v", c, err)
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
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Errorf("%v.StreamReserveUpdates(_) = _, %v", c, err)
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

	return nil
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
