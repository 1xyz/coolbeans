package store

import (
	"fmt"
	"github.com/1xyz/beanstalkd/state"
	v1 "github.com/1xyz/coolbeans/api/v1"
	pb "github.com/golang/protobuf/proto"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	// retainSnapshotCount indicates the max, number of snapshots to retain
	RetainSnasphotCount int

	// The MaxPool controls how many connections we will pool. The
	MaxPool int

	// RaftTimeout is the max. duration for a raft apply op.
	// timeout is used to apply I/O deadlines. For InstallSnapshot, we multiply
	// the timeout by (SnapshotSize / TimeoutScale).
	RaftTimeout time.Duration

	// RestoreTimeout is the max. duration for a restore operation
	RestoreTimeout time.Duration

	// RootDir is the root directory where store data is persisted
	RootDir string

	// RaftBindAddr is the bind address for raft tcp conn.
	RaftBindAddr string

	// Inmem is a boolean, controls if the data is persisted
	Inmem bool
}

type Store struct {
	jsm     state.JSM
	c       *Config
	raft    *raft.Raft
	localID string
}

func NewStore(c *Config) (*Store, error) {
	jsm, err := state.NewJSM()
	if err != nil {
		return nil, err
	}

	return &Store{
		jsm:  jsm,
		c:    c,
		raft: nil,
	}, nil
}

// //////////////////////////////////////////////////////////////////////

// Open opens the store. If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (s *Store) Open(enableSingle bool, localID string) error {
	s.localID = localID
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.c.RaftBindAddr)
	if err != nil {
		return err
	}

	transport, err := raft.NewTCPTransport(s.c.RaftBindAddr, addr,
		s.c.MaxPool, s.c.RaftTimeout, os.Stderr)
	if err != nil {
		return err
	}

	// Create the snapshot store. This allows the Raft to truncate the log
	snapshots, err := raft.NewFileSnapshotStore(s.c.RootDir, s.c.RetainSnasphotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	var logStore raft.LogStore
	var stableStore raft.StableStore
	if s.c.Inmem {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
	} else {
		boltDB, err := raftboltdb.NewBoltStore(filepath.Join(s.c.RootDir, "raft.db"))
		if err != nil {
			return fmt.Errorf("new bolt store: %s", err)
		}

		logStore = boltDB
		stableStore = boltDB
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, (*fsm)(s), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}

	s.raft = ra
	if enableSingle {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		ra.BootstrapCluster(configuration)
	}

	return nil
}

// Join joins a node, identified by nodeID and located at addr, to this store.
// The node must be ready to respond to Raft communications at that address.
func (s *Store) Join(nodeID, addr string) error {
	logc := log.WithFields(log.Fields{
		"method":  "Join",
		"localID": s.localID})

	logc.Infof("received join request for remote nodeID %s at addr:%s", nodeID, addr)
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		logc.Errorf("failed to get raft configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				logc.Warnf("node %s at %s already member of cluster, ignoring join request",
					nodeID, addr)
				return nil
			}

			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeID, addr, err)
			}
		}
	}

	f := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		logc.Errorf("s.raft.AddVoter. err = %v", f.Error())
		return f.Error()
	}

	logc.Infof("node %s at %s joined successfully", nodeID, addr)
	return nil
}

// //////////////////////////////////////////////////////////////////////

func (s *Store) ApplyOp(req *v1.ApplyOpRequest) *v1.ApplyOpResponse {
	logc := log.WithField("method", "ApplyOp")

	if s.raft.State() != raft.Leader {
		logc.Errorf("current node is not a leader")
		return &v1.ApplyOpResponse{
			ErrorCode:    v1.ResultCode_FailedPrecondition,
			ErrorMessage: "current node is not a leader",
		}
	}

	bIn, err := pb.Marshal(req)
	if err != nil {
		logc.Errorf("pb.marshal. err=%v", err)
		return &v1.ApplyOpResponse{
			ErrorCode:    v1.ResultCode_InvalidArgument,
			ErrorMessage: fmt.Sprintf("marshal err=%v", err),
		}
	}

	f := s.raft.Apply(bIn, s.c.RaftTimeout)
	if e := f.(raft.Future); e.Error() != nil {
		if e.Error() == raft.ErrNotLeader {
			return &v1.ApplyOpResponse{
				ErrorCode:    v1.ResultCode_FailedPrecondition,
				ErrorMessage: "current node is not a leader",
			}
		}

		return &v1.ApplyOpResponse{
			ErrorCode:    v1.ResultCode_Unknown,
			ErrorMessage: fmt.Sprintf("apply op err=%v", err),
		}
	}

	bOut, ok := f.Response().([]byte)
	if !ok {
		logc.Errorf("de-serialize resp to []byte error")
		return &v1.ApplyOpResponse{
			ErrorCode:    v1.ResultCode_Internal,
			ErrorMessage: "de-serialize resp to []byte error",
		}
	}

	var resp v1.ApplyOpResponse
	if err := pb.Unmarshal(bOut, &resp); err != nil {
		logc.Errorf("pb.Unmarshal. err=%v", err)
		return &v1.ApplyOpResponse{
			ErrorCode:    v1.ResultCode_Internal,
			ErrorMessage: fmt.Sprintf("pb.Unmarshal. err=%v", err),
		}
	}

	return &resp
}

func (s *Store) NowSeconds() int64 {
	return time.Now().UTC().Unix()
}

// //////////////////////////////////////////////////////////////////////

type fsm Store

// Snapshot is used to support log compaction. This call should
// return an FSMSnapshot which can be used to save a point-in-time
// snapshot of the FSM. Apply and Snapshot are not called in multiple
// threads, but Apply will be called concurrently with Persist. This means
// the FSM should be implemented in a fashion that allows for concurrent
// updates while a snapshot is happening.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return NewSnapshotFrom(f.jsm)
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
func (f *fsm) Restore(r io.ReadCloser) error {
	return RestoreSnapshotTo(r, f.jsm, f.c.RestoreTimeout)
}

// Apply log is invoked once a log entry is committed. It returns a value
// which will be made available in the  ApplyFuture returned by Raft.Apply
// method if that method was called on the same Raft node as the FSM.
func (f *fsm) Apply(l *raft.Log) interface{} {
	logc := log.WithFields(log.Fields{"method": "Apply", "index": l.Index,
		"term": l.Term, "type": l.Type})

	applyReq := newApplyReq(l.Data)
	logc.Debugf("op-type %v", applyReq.Op)

	switch applyReq.Op {
	case v1.OpType_PUT:
		var pReq v1.PutRequest
		unmarshalP(applyReq.Body, &pReq)
		pResp, err := f.ApplyPut(applyReq.NowSecs, &pReq)
		return newApplyRespBytes(pResp, err)
	case v1.OpType_RESERVE:
		var rReq v1.ReserveRequest
		unmarshalP(applyReq.Body, &rReq)
		rResp, err := f.ApplyReserve(applyReq.NowSecs, &rReq)
		return newApplyRespBytes(rResp, err)
	default:
		return marshalP(&v1.ApplyOpResponse{
			ErrorCode:    v1.ResultCode_Unimplemented,
			ErrorMessage: fmt.Sprintf("Unsupported optype=%v", applyReq.Op),
		})
	}
}

func newApplyReq(b []byte) *v1.ApplyOpRequest {
	var req v1.ApplyOpRequest
	unmarshalP(b, &req)
	return &req
}

func newApplyRespBytes(m pb.Message, err error) []byte {
	if err != nil {
		log.Errorf("applyResponse err=%v", err)
		return marshalP(&v1.ApplyOpResponse{
			ErrorCode:    v1.ResultCode_Unknown,
			ErrorMessage: fmt.Sprintf("applyResponse err=%v", err),
		})
	}

	return marshalP(&v1.ApplyOpResponse{
		ErrorCode:    v1.ResultCode_OK,
		ErrorMessage: "",
		Body:         marshalP(m),
	})
}

func unmarshalP(b []byte, m pb.Message) {
	if err := pb.Unmarshal(b, m); err != nil {
		log.WithField("method", "unmarshalP").
			Panicf("pb.unmarshal err=%v", err)
	}
}

func marshalP(m pb.Message) []byte {
	if b, err := pb.Marshal(m); err != nil {
		log.WithField("method", "marshalP").
			Panicf("failed to marshal m=%T to bytes err=%v", m, err)
		return nil
	} else {
		return b
	}
}

// //////////////////////////////////////////////////////////////////////

func (f *fsm) ApplyPut(nowSecs int64, pReq *v1.PutRequest) (*v1.PutResponse, error) {
	j, err := f.jsm.Put(nowSecs, pReq.Priority, pReq.Delay, int(pReq.Ttr),
		int(pReq.BodySize), pReq.Body, state.TubeName(pReq.TubeName))
	if err != nil {
		return nil, err
	}

	return &v1.PutResponse{
		JobId: int64(j.ID()),
	}, nil
}

// //////////////////////////////////////////////////////////////////////

func (f *fsm) ApplyReserve(nowSecs int64, req *v1.ReserveRequest) (*v1.ReserveResponse, error) {
	watchedTubes := make([]state.TubeName, 0)
	for _, t := range req.WatchedTubes {
		watchedTubes = append(watchedTubes, state.TubeName(t))
	}

	r, err := f.jsm.AppendReservation(
		state.ClientID(req.ClientId),
		req.RequestId,
		watchedTubes,
		nowSecs,
		nowSecs+int64(req.TimeoutSecs))
	if err != nil {
		log.WithField("method", "applyReservation").
			Errorf("jsm.AppendReservation. err=%v", err)
		return nil, err
	}

	errMsg := ""
	if r.Error != nil {
		errMsg = r.Error.Error()
	}

	return &v1.ReserveResponse{
		Reservation: &v1.Reservation{
			RequestId: r.RequestId,
			ClientId:  string(r.ClientId),
			Status:    v1.ReservationStatus(r.Status),
			JobId:     int64(r.JobId),
			BodySize:  int32(r.BodySize),
			Body:      r.Body,
			ErrorMsg:  errMsg,
		},
	}, nil
}
