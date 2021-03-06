package server

import (
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/armon/go-metrics"
	pb "github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"time"
)

// ReplicatedJsm represents a JobStateMachine that is replicated via RAFT
type ReplicatedJsm interface {
	// Apply the provided request
	ApplyOp(req *v1.ApplyOpRequest) *v1.ApplyOpResponse

	// Ask the server for the current clock (now in secs)
	NowSeconds() int64

	// Returns true if this node is a leader
	IsLeader() bool
}

// JSMServer exports the ReplicatedJSM as a callable server.
type JSMServer struct {
	v1.UnimplementedJobStateMachineServer
	r    ReplicatedJsm
	ctrl *ReservationsController
}

// NewJSMServer returns a pointer to a new JSMServer struct.
func NewJSMServer(r ReplicatedJsm) *JSMServer {
	s := &JSMServer{
		r: r,
	}
	s.ctrl = NewReservationsController(s)
	return s
}

// RunController runs the ReservationsController in a separate go-routine.
func (j *JSMServer) RunController() {
	if err := j.ctrl.Run(); err != nil {
		log.Panicf("j.ctrl.Run() Err.=%v", err)
	}
}

// Bury allows a specific job to be buried
func (j *JSMServer) Bury(ctx context.Context, req *v1.BuryRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_BURY, req, &resp); err != nil {
		log.Errorf("jsmServer.Bury: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// CheckClientState returns the current state of a specific proxy client.
func (j *JSMServer) CheckClientState(ctx context.Context, req *v1.CheckClientStateRequest) (*v1.CheckClientStateResponse, error) {
	var resp v1.CheckClientStateResponse
	log.Infof("CheckClientState: proxyID=%v", req.ProxyId)
	if err := j.performApply(v1.OpType_CHECK_CLIENT_STATE, req, &resp); err != nil {
		log.Errorf("CheckClientState: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// Delete removes a job
func (j *JSMServer) Delete(ctx context.Context, req *v1.DeleteRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_DELETE, req, &resp); err != nil {
		log.WithField("method", "Delete").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// GetJob returns a job
func (j *JSMServer) GetJob(ctx context.Context, req *v1.GetJobRequest) (*v1.GetJobResponse, error) {
	var resp v1.GetJobResponse
	if err := j.performApply(v1.OpType_GET_JOB, req, &resp); err != nil {
		log.Errorf("jsmServer.GetJob: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) peek(req *v1.PeekRequest, opType v1.OpType) (*v1.PeekResponse, error) {
	var resp v1.PeekResponse
	if err := j.performApply(opType, req, &resp); err != nil {
		log.Errorf("jsmServer.Peek: opTyoe=%v performApply. Err=%v", opType, err)
		return nil, err
	}
	return &resp, nil
}

// PeekBuried peeks and returns the first buried job in the specified tube
func (j *JSMServer) PeekBuried(ctx context.Context, req *v1.PeekRequest) (*v1.PeekResponse, error) {
	return j.peek(req, v1.OpType_PEEK_BURIED)
}

// PeekDelayed peeks and returns the first delayed job in the specified tube
func (j *JSMServer) PeekDelayed(ctx context.Context, req *v1.PeekRequest) (*v1.PeekResponse, error) {
	return j.peek(req, v1.OpType_PEEK_DELAYED)
}

// PeekReady peeks and returns the first ready job in the specified tube
func (j *JSMServer) PeekReady(ctx context.Context, req *v1.PeekRequest) (*v1.PeekResponse, error) {
	return j.peek(req, v1.OpType_PEEK_READY)
}

// Kick moves a specific buried job to back to the ready queue
func (j *JSMServer) Kick(ctx context.Context, req *v1.KickRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_KICK, req, &resp); err != nil {
		log.Errorf("jsmServer.Kick: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// KickN moves N jobs from the top of the buried queue to the ready queue.
func (j *JSMServer) KickN(ctx context.Context, req *v1.KickNRequest) (*v1.KickNResponse, error) {
	var resp v1.KickNResponse
	if err := j.performApply(v1.OpType_KICKN, req, &resp); err != nil {
		log.Errorf("jsmServer.Kick: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// ListTubes returns a list of tubes currently available.
func (j *JSMServer) ListTubes(ctx context.Context, req *v1.Empty) (*v1.ListTubesResponse, error) {
	var resp v1.ListTubesResponse
	if err := j.performApply(v1.OpType_LIST_TUBES, req, &resp); err != nil {
		log.Errorf("jsmServer.ListTubes: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// Put creates a new job.
func (j *JSMServer) Put(ctx context.Context, req *v1.PutRequest) (*v1.PutResponse, error) {
	t := time.Now()

	var resp v1.PutResponse
	if err := j.performApply(v1.OpType_PUT, req, &resp); err != nil {
		log.WithField("method", "Put").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	log.Infof("jsm.Put took %v sec", time.Since(t))
	return &resp, nil
}

// Reserve allows a client to reserve a job for processing.
func (j *JSMServer) Reserve(ctx context.Context, req *v1.ReserveRequest) (*v1.ReserveResponse, error) {
	var resp v1.ReserveResponse
	log.Infof("Reserve: proxyID=%v clientID=%v timeout=%v watchedTubes=%v",
		req.ProxyId, req.ClientId, req.TimeoutSecs, req.WatchedTubes)
	if err := j.performApply(v1.OpType_RESERVE, req, &resp); err != nil {
		log.WithField("method", "Reserve").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	log.Infof("Reserve: proxyID=%v clientID=%v timeout=%v watchedTubes=%v status=%v",
		req.ProxyId, req.ClientId, req.TimeoutSecs, req.WatchedTubes, resp.Reservation.Status)
	return &resp, nil
}

// Release allows a client to return a job from being reserved.
func (j *JSMServer) Release(ctx context.Context, req *v1.ReleaseRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_RELEASE, req, &resp); err != nil {
		log.WithField("method", "Release").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// GetStatsJobYaml returns a specific job's statistics as YAML formatted response.
func (j *JSMServer) GetStatsJobYaml(ctx context.Context, req *v1.GetStatsJobYamlRequest) (*v1.GetStatsJobYamlResponse, error) {
	var resp v1.GetStatsJobYamlResponse
	if err := j.performApply(v1.OpType_STATS_JOB_YAML, req, &resp); err != nil {
		log.Errorf("jsmServer.GetStatsJobYaml: performApply: err = %v", err)
		return nil, err
	}
	return &resp, nil
}

// GetStatsTubeYaml returns a specific tube's statistics as a YAML formatted response.
func (j *JSMServer) GetStatsTubeYaml(ctx context.Context, req *v1.GetStatsTubeYamlRequest) (*v1.GetStatsTubeYamlResponse, error) {
	var resp v1.GetStatsTubeYamlResponse
	if err := j.performApply(v1.OpType_STATS_TUBE_YAML, req, &resp); err != nil {
		log.Errorf("jsmServer.GetStatsTubeYaml: performApply: err = %v", err)
		return nil, err
	}
	return &resp, nil
}

// GetStatsYaml returns the server statistics as a YAML formatted response.
func (j *JSMServer) GetStatsYaml(ctx context.Context, req *v1.Empty) (*v1.GetStatsYamlResponse, error) {
	var resp v1.GetStatsYamlResponse
	if err := j.performApply(v1.OpType_STATS_YAML, req, &resp); err != nil {
		log.Errorf("jsmServer.GetStatsYaml: performApply: err = %v", err)
		return nil, err
	}
	return &resp, nil
}

// Tick progresses the underlying job state machine
func (j *JSMServer) Tick() (*v1.TickResponse, error) {
	if !j.r.IsLeader() {
		return nil, ErrNotLeader
	}

	var resp v1.TickResponse
	if err := j.performApply(v1.OpType_TICK, &v1.Empty{}, &resp); err != nil {
		log.WithField("method", "Reserve").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

// Touch allows a client to continue to its reservation by the job's TTR
func (j *JSMServer) Touch(ctx context.Context, req *v1.TouchRequest) (*v1.Empty, error) {
	var resp v1.Empty
	err := j.performApply(v1.OpType_TOUCH, req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// StreamReserveUpdates returns back a continuous stream of reservation updates from the cluster node
// back to a specific proxy.
func (j *JSMServer) StreamReserveUpdates(req *v1.ReserveUpdateRequest,
	stream v1.JobStateMachine_StreamReserveUpdatesServer) error {

	log.Infof("StreamReserveUpdates: proxyID=%v", req.ProxyId)
	logc := log.WithField("method", "StreamReserveUpdates")
	respCh, err := j.ctrl.Register(req.ProxyId)
	if err != nil {
		logc.Errorf("ctrl.Register, Err-%v", err)
		if err == ErrProxyExists {
			return status.Errorf(codes.InvalidArgument, "proxy with id=%v exists", req.ProxyId)
		}

		return status.Errorf(codes.Internal, "Err = %v", err)
	}

	defer j.ctrl.UnRegister(req.ProxyId, respCh)

	for r := range respCh {
		ctx := stream.Context()
		if ctx != nil {
			select {
			case <-ctx.Done():
				return status.Errorf(codes.Canceled, "Operation cancelled")
			default:
			}
		}

		if r.RespType != Reservation {
			return status.Errorf(codes.Internal, "inconsistent state = %v", r.RespType)
		} else if r.Err != nil {
			return status.Errorf(codes.Internal, "Err = %v", err)
		} else if r.Reservations == nil || len(r.Reservations) == 0 {
			log.Debugf("no Reservations found")
		}

		for _, resv := range r.Reservations {
			if err := stream.Send(&v1.ReserveResponse{Reservation: resv}); err != nil {
				logc.Errorf("stream.Send(..) Err=%v", err)
				return err
			}
		}
	}

	return nil
}

func (j *JSMServer) performApply(opType v1.OpType, req pb.Message, resp pb.Message) error {
	name := strings.ToLower(opType.String())
	metrics.IncrCounter([]string{"jsm", name}, float32(1))
	defer metrics.MeasureSince([]string{"jsm", name, "latency_ms"}, time.Now())
	logc := log.WithField("method", "performApply")
	b, err := pb.Marshal(req)
	if err != nil {
		logc.Errorf("pb.marshal. Err=%v", err)
		return status.Errorf(codes.InvalidArgument,
			"error marshalling req %v", err)
	}

	applyReq := v1.ApplyOpRequest{
		Op:      opType,
		NowSecs: j.r.NowSeconds(),
		Body:    b,
	}

	applyResp := j.r.ApplyOp(&applyReq)
	if applyResp.ErrorCode > v1.ResultCode_OK {
		metrics.IncrCounter([]string{"jsm", name, "errors"}, float32(1))
		logc.Errorf("applyOp. Err = %v msg = %v",
			applyResp.ErrorCode, applyResp.ErrorMessage)
		return status.Errorf(codes.Code(applyResp.ErrorCode),
			"applyOp Err = %v", applyResp.ErrorMessage)
	}

	if err := pb.Unmarshal(applyResp.Body, resp); err != nil {
		metrics.IncrCounter([]string{"jsm", name, "errors"}, float32(1))
		logc.Errorf("pb.Unmarshal. Err=%v", err)
		return status.Errorf(codes.Internal,
			"error un-marshalling resp %v", err)
	}

	return nil
}
