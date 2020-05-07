/*
 * Package store provides an implementation for a accessing a Raft
 * backed job state machine store
 */
package store

import (
	"errors"
	"fmt"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/state"
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

var (
	// ErrRaftConfig is returned when an error is encountered retrieving
	// the raft configuration.
	ErrRaftConfig = errors.New("raft configuration error")

	// ErrNodeNotLeader is returned, when the request requires the current
	// node to be a leader to execute, but is a not a raft leader.
	ErrNotRaftLeader = errors.New("node is not a raft leader")

	// ErrNodeNotFound is returned. when the specified node is not found in raft configuration
	ErrNodeNotFound = errors.New("node is not found in raft configuration")
)

type Config struct {
	// retainSnapshotCount indicates the max, number of snapshots to retain
	RetainSnasphotCount int

	// The MaxPool controls how many connections we will pool. The
	MaxPool int

	// SnapshotThreshold controls how many outstanding logs there must be before
	// we perform a snapshot. This is to prevent excessive snapshots when we can
	// just replay a small set of logs.
	SnapshotThreshold uint64

	// TrailingLogs controls how many logs we leave after a snapshot. This is
	// used so that we can quickly replay logs on a follower instead of being
	// forced to send an entire snapshot.
	TrailingLogs uint64

	// SnapshotInterval controls how often we check if we should perform a snapshot.
	// We randomly stagger between this value and 2x this value to avoid the entire
	// cluster from performing a snapshot at once.
	SnapshotInterval time.Duration

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

	// local node id of this node
	LocalNodeID string
}

type Store struct {
	jsm       state.JSM
	c         *Config
	raft      *raft.Raft
	transport *raft.NetworkTransport
}

func NewStore(c *Config) (*Store, error) {
	jsm, err := state.NewJSM()
	if err != nil {
		return nil, err
	}

	return &Store{
		jsm: jsm,
		c:   c,

		raft: nil,
	}, nil
}

// //////////////////////////////////////////////////////////////////////

// Open opens the store. If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (s *Store) Open() error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(s.c.LocalNodeID)
	config.SnapshotThreshold = s.c.SnapshotThreshold
	config.TrailingLogs = s.c.TrailingLogs
	config.SnapshotInterval = s.c.SnapshotInterval
	log.Infof("Open: config localID %v, SnapshotThreshold=%v TrailingLogs=%v SnapshotInterval=%v",
		config.LocalID, config.SnapshotThreshold, config.TrailingLogs, config.SnapshotInterval)

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
	s.transport = transport
	return nil
}

// BootstrapCluster attempts to do a one-time bootstrap of the cluster
// the input is a map of nodeID & corresponding raft address entries
func (s *Store) BootstrapCluster(nc map[string]string) error {
	servers := make([]raft.Server, 0)
	for id, addr := range nc {
		servers = append(servers, raft.Server{
			ID:      raft.ServerID(id),
			Address: raft.ServerAddress(addr),
		})
	}
	f := s.raft.BootstrapCluster(raft.Configuration{
		Servers: servers,
	})
	if f.Error() != nil {
		return f.Error()
	}
	return nil
}

// Join joins a node, identified by nodeID and located at addr, to this store.
// The node must be ready to respond to Raft communications at that address.
//
// It is required that the node that this is called into is a leader node.
func (s *Store) Join(nodeID, addr string) error {
	logc := log.WithFields(log.Fields{"method": "Join", "localID": s.c.LocalNodeID,
		"nodeID": nodeID, "addr": addr})

	rCfg, err := s.GetRaftConfiguration()
	if err != nil {
		return err
	}

	for _, srv := range rCfg.Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				logc.Warnf("node member of cluster, ignoring join request")
				return nil
			}

			f := s.raft.RemoveServer(srv.ID, 0, s.c.RaftTimeout)
			if err := f.Error(); err != nil {
				if errors.Is(err, raft.ErrNotLeader) {
					return fmt.Errorf("s.raft.RemoveServer(nodeId=%v, addr=%v); err = %v %w",
						nodeID, addr, err, ErrNotRaftLeader)
				}

				return fmt.Errorf("s.raft.RemoveServer(nodeId=%v, addr=%v): err = %w",
					nodeID, addr, err)
			}
		}
	}

	f := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, s.c.RaftTimeout)
	if err := f.Error(); err != nil {
		if errors.Is(err, raft.ErrNotLeader) {
			return fmt.Errorf("s.raft.AddVoter(nodeId=%v, addr=%v); err = %v %w",
				nodeID, addr, err, ErrNotRaftLeader)
		}

		return fmt.Errorf("s.raft.AddVoter(nodeId=%v, addr=%v): err = %w", nodeID, addr, err)
	}

	logc.Infof("joined successfully")
	return nil
}

func (s *Store) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

func (s *Store) Ready() bool {
	return s.IsLeader()
}

// Leave, allows a node (specified by nodeID_ to leave the cluster.
//
// It is required that the node that this is called into is a leader node.
func (s *Store) Leave(nodeID string) error {
	logc := log.WithFields(log.Fields{"method": "Leave", "localID": s.c.LocalNodeID, "nodeID": nodeID})

	rCfg, err := s.GetRaftConfiguration()
	if err != nil {
		return err
	}

	serverID := raft.ServerID(nodeID)
	logc = logc.WithField("serverID", serverID)
	for _, srv := range rCfg.Servers {
		if srv.ID == serverID {
			logc.Infof("found a match. serverAddr=%v", srv.Address)
			f := s.raft.RemoveServer(srv.ID, 0, s.c.RaftTimeout)
			if err := f.Error(); err != nil {
				if errors.Is(err, raft.ErrNotLeader) {
					return fmt.Errorf("s.raft.RemoveServer(nodeId=%v, addr=%v); err = %v %w",
						nodeID, srv.Address, err, ErrNotRaftLeader)
				}

				return fmt.Errorf("s.raft.RemoveServer(nodeId=%v, addr=%v); err = %w",
					nodeID, srv.Address, err)
			}

			logc.Infof("s.raft.RemoveServer success")
			return nil
		}
	}

	return fmt.Errorf("nodeId=%v, err = %w", serverID, ErrNodeNotFound)
}

func (s *Store) GetRaftConfiguration() (*raft.Configuration, error) {
	f := s.raft.GetConfiguration()
	if err := f.Error(); err != nil {
		log.Errorf("s.raft.GetConfiguration(). err=%v", err)
		return nil, fmt.Errorf("s.raft.GetConfiguration reason = %v %w", err, ErrRaftConfig)
	}

	cfg := f.Configuration()
	return &cfg, nil
}

func (s *Store) Snapshot() error {
	logc := log.WithField("method", "Snapshot")

	sf := s.raft.Snapshot()
	if err := sf.Error(); err != nil {
		logc.Errorf("s.raft.Snapshot(). err=%v", err)
		return err
	}

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
	logc.Infof("op-type %v now %v", applyReq.Op, applyReq.NowSecs)

	switch applyReq.Op {
	case v1.OpType_BURY:
		var bReq v1.BuryRequest
		unmarshalP(applyReq.Body, &bReq)
		bResp, err := f.ApplyBury(applyReq.NowSecs, &bReq)
		return newApplyRespBytes(bResp, err)

	case v1.OpType_DELETE:
		var dReq v1.DeleteRequest
		unmarshalP(applyReq.Body, &dReq)
		dResp, err := f.ApplyDelete(&dReq)
		return newApplyRespBytes(dResp, err)

	case v1.OpType_KICK:
		var kReq v1.KickRequest
		unmarshalP(applyReq.Body, &kReq)
		kResp, err := f.ApplyKick(&kReq)
		return newApplyRespBytes(kResp, err)

	case v1.OpType_KICKN:
		var knReq v1.KickNRequest
		unmarshalP(applyReq.Body, &knReq)
		kResp, err := f.ApplyKickN(&knReq)
		return newApplyRespBytes(kResp, err)

	case v1.OpType_PEEK_BURIED:
		var pReq v1.PeekRequest
		unmarshalP(applyReq.Body, &pReq)
		pResp, err := f.ApplyPeekBuried(&pReq)
		return newApplyRespBytes(pResp, err)

	case v1.OpType_PEEK_READY:
		var pReq v1.PeekRequest
		unmarshalP(applyReq.Body, &pReq)
		pResp, err := f.ApplyPeekReady(&pReq)
		return newApplyRespBytes(pResp, err)

	case v1.OpType_PEEK_DELAYED:
		var pReq v1.PeekRequest
		unmarshalP(applyReq.Body, &pReq)
		pResp, err := f.ApplyPeekDelayed(&pReq)
		return newApplyRespBytes(pResp, err)

	case v1.OpType_PUT:
		var pReq v1.PutRequest
		unmarshalP(applyReq.Body, &pReq)
		pResp, err := f.ApplyPut(applyReq.NowSecs, &pReq)
		return newApplyRespBytes(pResp, err)

	case v1.OpType_GET_JOB:
		var gReq v1.GetJobRequest
		unmarshalP(applyReq.Body, &gReq)
		gResp, err := f.ApplyGetJob(&gReq)
		return newApplyRespBytes(gResp, err)

	case v1.OpType_RELEASE:
		var rReq v1.ReleaseRequest
		unmarshalP(applyReq.Body, &rReq)
		rResp, err := f.ApplyRelease(applyReq.NowSecs, &rReq)
		return newApplyRespBytes(rResp, err)

	case v1.OpType_RESERVE:
		var rReq v1.ReserveRequest
		unmarshalP(applyReq.Body, &rReq)
		rResp, err := f.ApplyReserve(applyReq.NowSecs, &rReq)
		return newApplyRespBytes(rResp, err)

	case v1.OpType_TICK:
		tResp, err := f.ApplyTick(applyReq.NowSecs)
		return newApplyRespBytes(tResp, err)

	case v1.OpType_TOUCH:
		var tReq v1.TouchRequest
		unmarshalP(applyReq.Body, &tReq)
		rResp, err := f.ApplyTouch(applyReq.NowSecs, &tReq)
		return newApplyRespBytes(rResp, err)

	case v1.OpType_CHECK_CLIENT_STATE:
		var chkReq v1.CheckClientStateRequest
		unmarshalP(applyReq.Body, &chkReq)
		chkResp, err := f.ApplyCheckState(applyReq.NowSecs, &chkReq)
		return newApplyRespBytes(chkResp, err)

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

func (f *fsm) ApplyBury(nowSecs int64, req *v1.BuryRequest) (*v1.Empty, error) {
	cu, err := newClientUri(req)
	if err != nil {
		return nil, err
	}

	if err := f.jsm.Bury(nowSecs, state.JobID(req.JobId), req.Priority,
		cu.ToClientID()); err != nil {
		return nil, err
	}

	return &v1.Empty{}, nil
}

func (f *fsm) ApplyDelete(req *v1.DeleteRequest) (*v1.Empty, error) {
	cu, err := newClientUri(req)
	if err != nil {
		return nil, err
	}

	if err := f.jsm.Delete(state.JobID(req.JobId), cu.ToClientID()); err != nil {
		return nil, err
	}

	return &v1.Empty{}, nil
}

func (f *fsm) ApplyGetJob(gReq *v1.GetJobRequest) (*v1.GetJobResponse, error) {
	j, err := f.jsm.GetJob(state.JobID(gReq.JobId))
	if err != nil {
		return nil, err
	}
	return &v1.GetJobResponse{
		Job: JobToJobProto(j),
	}, nil
}

func (f *fsm) ApplyPut(nowSecs int64, pReq *v1.PutRequest) (*v1.PutResponse, error) {
	jobID, err := f.jsm.Put(nowSecs, pReq.Priority, pReq.Delay, int(pReq.Ttr),
		int(pReq.BodySize), pReq.Body, state.TubeName(pReq.TubeName))
	if err != nil {
		return nil, err
	}

	return &v1.PutResponse{
		JobId: int64(jobID),
	}, nil
}

func (f *fsm) ApplyPeek(pfunc func(state.TubeName) (state.Job, error), pReq *v1.PeekRequest) (*v1.PeekResponse, error) {
	j, err := pfunc(state.TubeName(pReq.TubeName))
	if err != nil {
		return nil, err
	}
	return &v1.PeekResponse{
		Job: JobToJobProto(j),
	}, nil
}

func (f *fsm) ApplyPeekBuried(pReq *v1.PeekRequest) (*v1.PeekResponse, error) {
	return f.ApplyPeek(f.jsm.PeekBuriedJob, pReq)
}

func (f *fsm) ApplyPeekDelayed(pReq *v1.PeekRequest) (*v1.PeekResponse, error) {
	return f.ApplyPeek(f.jsm.PeekDelayedJob, pReq)
}

func (f *fsm) ApplyPeekReady(pReq *v1.PeekRequest) (*v1.PeekResponse, error) {
	return f.ApplyPeek(f.jsm.PeekReadyJob, pReq)
}

func (f *fsm) ApplyKick(req *v1.KickRequest) (*v1.Empty, error) {
	err := f.jsm.Kick(state.JobID(req.JobId))
	if err != nil {
		return nil, err
	}

	return &v1.Empty{}, nil
}

func (f *fsm) ApplyKickN(req *v1.KickNRequest) (*v1.KickNResponse, error) {
	jobsKicked, err := f.jsm.KickN(state.TubeName(req.TubeName), int(req.Bound))
	if err != nil {
		return nil, err
	}

	return &v1.KickNResponse{
		JobsKicked: int32(jobsKicked),
	}, nil
}

func (f *fsm) ApplyRelease(nowSecs int64, req *v1.ReleaseRequest) (*v1.Empty, error) {
	cu := NewClientURI(req.ProxyId, req.ClientId)
	cu, err := newClientUri(req)
	if err != nil {
		return nil, err
	}

	if err := f.jsm.ReleaseWith(nowSecs, state.JobID(req.JobId), cu.ToClientID(),
		req.Priority, req.Delay); err != nil {
		return nil, err
	}

	return &v1.Empty{}, nil
}

func (f *fsm) ApplyReserve(nowSecs int64, req *v1.ReserveRequest) (*v1.ReserveResponse, error) {
	watchedTubes := make([]state.TubeName, 0)
	for _, t := range req.WatchedTubes {
		watchedTubes = append(watchedTubes, state.TubeName(t))
	}

	cu, err := newClientUri(req)
	if err != nil {
		return nil, err
	}

	r, err := f.jsm.AppendReservation(
		cu.ToClientID(),
		req.RequestId,
		watchedTubes,
		nowSecs,
		nowSecs+int64(req.TimeoutSecs))
	if err != nil {
		log.WithField("method", "ApplyReserve").
			Errorf("jsm.AppendReservation. err=%v", err)
		return nil, err
	}

	v1r, err := toV1Reservation(r)
	if err != nil {
		log.WithField("method", "ApplyReserve").
			Errorf("toV1Reservation  r=%v. err=%v", r, err)
		return nil, err
	}

	return &v1.ReserveResponse{
		Reservation: v1r,
	}, nil
}

func (f *fsm) ApplyTick(nowSecs int64) (*v1.TickResponse, error) {
	logc := log.WithField("method", "ApplyTick")
	rs, err := f.jsm.Tick(nowSecs)
	if err != nil {
		logc.Errorf("jsm.ApplyTick. err=%v", err)
		return nil, err
	}

	// group the reservations by proxy id
	proxyReservations := map[string]*v1.Reservations{}
	logc.Infof("reservations == %v", len(rs))
	for _, r := range rs {
		logc.Infof("r = %v", r)
		v1r, err := toV1Reservation(r)
		if err != nil {
			log.WithField("method", "ApplyTick").
				Errorf("toV1Reservation r=%v. err=%v", r, err)
			return nil, err
		}

		log.Infof("ApplyTick: proxyId = %v, clientId=%v status=%v",
			v1r.ProxyId, v1r.ClientId, v1r.Status)

		if resvns, ok := proxyReservations[v1r.ProxyId]; !ok {
			proxyReservations[v1r.ProxyId] = &v1.Reservations{
				Entries: []*v1.Reservation{v1r},
			}
		} else {
			resvns.Entries = append(resvns.Entries, v1r)
		}
	}

	return &v1.TickResponse{
		ProxyReservations: proxyReservations,
	}, nil
}

func (f *fsm) ApplyTouch(nowSecs int64, req *v1.TouchRequest) (*v1.Empty, error) {
	cu, err := newClientUri(req)
	if err != nil {
		return nil, err
	}

	if err := f.jsm.Touch(nowSecs, state.JobID(req.JobId), cu.ToClientID()); err != nil {
		return nil, err
	}

	return &v1.Empty{}, nil
}

type clientProxyUri interface {
	GetClientId() string
	GetProxyId() string
}

func newClientUri(uri clientProxyUri) (*ClientURI, error) {
	cu := NewClientURI(uri.GetProxyId(), uri.GetClientId())
	if err := cu.Validate(); err != nil {
		log.Errorf("store.newClientUri: cu.Validate err = %v", err)
		return nil, err
	}

	return cu, nil
}

func toV1Reservation(r *state.Reservation) (*v1.Reservation, error) {
	errMsg := ""
	if r.Error != nil {
		errMsg = r.Error.Error()
	}

	cu, err := ParseClientURI(r.ClientId)
	if err != nil {
		log.WithField("method", "applyReserve").
			Errorf("ParseClientURI clientID=%v err=%v", r.ClientId, err)
		return nil, err
	}

	log.Infof("toV1Reservation: proxyID = %v clientID=%v origclientID=%v status = %v",
		cu.proxyID, cu.clientID, r.ClientId, r.Status)

	return &v1.Reservation{
		RequestId: r.RequestId,
		ClientId:  cu.clientID,
		ProxyId:   cu.proxyID,
		Status:    v1.ReservationStatus(r.Status),
		JobId:     int64(r.JobId),
		BodySize:  int32(r.BodySize),
		Body:      r.Body,
		ErrorMsg:  errMsg,
	}, nil
}

func (f *fsm) ApplyCheckState(nowSecs int64, req *v1.CheckClientStateRequest) (*v1.CheckClientStateResponse, error) {
	logc := log.WithField("method", "ApplyCheckState")
	clientIDs := make([]state.ClientID, 0)
	for _, id := range req.ClientIds {
		cu := NewClientURI(req.ProxyId, id)
		if err := cu.Validate(); err != nil {
			logc.Errorf("clientUri.validate %v", err)
			return nil, err
		}

		clientIDs = append(clientIDs, cu.ToClientID())
	}

	w, nw, m, err := f.jsm.CheckClientState(clientIDs)
	if err != nil {
		logc.Errorf("f.jsm.CheckClisntState err = %v", err)
		return nil, err
	}

	missing, err := ToStrings(m)
	if err != nil {
		return nil, err
	}

	waiting, err := ToStrings(w)
	if err != nil {
		return nil, err
	}

	notWaiting, err := ToStrings(nw)
	if err != nil {
		return nil, err
	}

	return &v1.CheckClientStateResponse{
		ProxyId:             req.ProxyId,
		WaitingClientIds:    waiting,
		NotWaitingClientIds: notWaiting,
		MissingClientIds:    missing,
	}, nil
}

func ToStrings(clientIds []state.ClientID) ([]string, error) {
	s := make([]string, 0)
	for _, c := range clientIds {
		cu, err := ParseClientURI(c)
		if err != nil {
			log.Errorf("ToString: clientID=%v err=%v", c, err)
			return nil, err
		}

		s = append(s, cu.clientID)
	}

	return s, nil
}
