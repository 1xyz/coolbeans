package server

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
}

type JSMServer struct {
	v1.UnimplementedJobStateMachineServer
	r ReplicatedJsm
}

func NewJSMServer(r ReplicatedJsm) *JSMServer {
	return &JSMServer{
		r: r,
	}
}

func (j *JSMServer) Put(ctx context.Context, req *v1.PutRequest) (*v1.PutResponse, error) {
	var resp v1.PutResponse
	if err := j.performApply(v1.OpType_PUT, req, &resp); err != nil {
		log.WithField("method", "Put").Errorf("performApply. err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) Reserve(ctx context.Context, req *v1.ReserveRequest) (*v1.ReserveResponse, error) {
	var resp v1.ReserveResponse
	if err := j.performApply(v1.OpType_RESERVE, req, &resp); err != nil {
		log.WithField("method", "Reserve").Errorf("performApply. err=%v", err)
		return nil, err
	}

	return &resp, nil
}

func (j *JSMServer) performApply(opType v1.OpType, req pb.Message, resp pb.Message) error {
	logc := log.WithField("method", "performApply")

	b, err := pb.Marshal(req)
	if err != nil {
		logc.Errorf("pb.marshal. err=%v", err)
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
		logc.Errorf("applyOp. err = %v %v",
			applyResp.ErrorCode, applyResp.ErrorMessage)
		return status.Errorf(codes.Code(applyResp.ErrorCode),
			"applyOp err = %v", applyResp.ErrorMessage)
	}

	if err := pb.Unmarshal(applyResp.Body, resp); err != nil {
		logc.Errorf("pb.Unmarshal. err=%v", err)
		return status.Errorf(codes.Internal,
			"error un-marshalling resp %v", err)
	}

	return nil
}
