package cluster

import (
	v1 "github.com/1xyz/coolbeans/api/v1"
	pb "github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReplicatedJsm interface {
	// Apply the provided request
	ApplyOp(req *v1.ApplyOpRequest) *v1.ApplyOpResponse

	// Ask the server for the current clock (now in secs)
	NowSeconds() int64

	// Returns true if this node is a leader
	IsLeader() bool
}

type JSMServer struct {
	v1.UnimplementedJobStateMachineServer
	r    ReplicatedJsm
	ctrl *ReservationsController
}

func NewJSMServer(r ReplicatedJsm) *JSMServer {
	s := &JSMServer{
		r: r,
	}
	s.ctrl = NewReservationsController(s)
	return s
}

func (j *JSMServer) RunController() {
	if err := j.ctrl.Run(); err != nil {
		log.Panicf("j.ctrl.Run() Err.=%v", err)
	}
}

func (j *JSMServer) Put(ctx context.Context, req *v1.PutRequest) (*v1.PutResponse, error) {
	var resp v1.PutResponse
	if err := j.performApply(v1.OpType_PUT, req, &resp); err != nil {
		log.WithField("method", "Put").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) Delete(ctx context.Context, req *v1.DeleteRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_DELETE, req, &resp); err != nil {
		log.WithField("method", "Delete").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) Release(ctx context.Context, req *v1.ReleaseRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_RELEASE, req, &resp); err != nil {
		log.WithField("method", "Release").Errorf("performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) Bury(ctx context.Context, req *v1.BuryRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_BURY, req, &resp); err != nil {
		log.Errorf("jsmServer.Bury: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) Kick(ctx context.Context, req *v1.KickRequest) (*v1.Empty, error) {
	var resp v1.Empty
	if err := j.performApply(v1.OpType_KICK, req, &resp); err != nil {
		log.Errorf("jsmServer.Kick: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) KickN(ctx context.Context, req *v1.KickNRequest) (*v1.KickNResponse, error) {
	var resp v1.KickNResponse
	if err := j.performApply(v1.OpType_KICKN, req, &resp); err != nil {
		log.Errorf("jsmServer.Kick: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

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

func (j *JSMServer) CheckClientState(ctx context.Context, req *v1.CheckClientStateRequest) (*v1.CheckClientStateResponse, error) {
	var resp v1.CheckClientStateResponse
	log.Infof("CheckClientState: proxyID=%v", req.ProxyId)
	if err := j.performApply(v1.OpType_CHECK_CLIENT_STATE, req, &resp); err != nil {
		log.Errorf("CheckClientState: performApply. Err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) performApply(opType v1.OpType, req pb.Message, resp pb.Message) error {
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
		logc.Errorf("applyOp. Err = %v msg = %v",
			applyResp.ErrorCode, applyResp.ErrorMessage)
		return status.Errorf(codes.Code(applyResp.ErrorCode),
			"applyOp Err = %v", applyResp.ErrorMessage)
	}

	if err := pb.Unmarshal(applyResp.Body, resp); err != nil {
		logc.Errorf("pb.Unmarshal. Err=%v", err)
		return status.Errorf(codes.Internal,
			"error un-marshalling resp %v", err)
	}

	return nil
}
