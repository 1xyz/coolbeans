package server

import (
	"errors"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/store"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RaftCluster interface {
	// Join, joins this node, identified by nodeID and reachable at addr,
	// to an existing Raft cluster.
	Join(nodeID, addr string) error

	// Leave, leave this specified node, identified by nodeID from
	// an existing Raft cluster.
	Leave(nodeID string) error

	// Returns true if this specified node is a Leader
	IsLeader() bool

	// Ask the node to take a snapshot
	Snapshot() error
}

type ClusterServer struct {
	v1.UnimplementedClusterServer
	rc RaftCluster
}

func NewClusterServer(rc RaftCluster) *ClusterServer {
	return &ClusterServer{
		rc: rc,
	}
}

// Join joins a node, identified by nodeID and located at addr, to this cluster.
// The node must be ready to respond to Raft communications at that address.
//
// It is required that the node that this is called into is a leader node.
func (c *ClusterServer) Join(ctx context.Context, req *v1.JoinRequest) (*v1.Empty, error) {
	logc := log.WithFields(log.Fields{"method": "Join", "nodeID": req.NodeId, "addr": req.Addr})
	if err := c.rc.Join(req.NodeId, req.Addr); err != nil {
		logc.Errorf("c.rc.Join. err = %v", err)
		if errors.Is(err, store.ErrNotRaftLeader) {
			return nil, status.Errorf(codes.FailedPrecondition, "%s", store.ErrNotRaftLeader)
		}

		return nil, err
	} else {
		logc.Infof("join completed success")
		return &v1.Empty{}, nil
	}
}

// Leave leaves a node, identified by nodeID and located at addr, to this store.
//
// It is required that the node that this is called into is a leader node.
func (c *ClusterServer) Leave(ctx context.Context, req *v1.LeaveRequest) (*v1.Empty, error) {
	logc := log.WithFields(log.Fields{"method": "Leave", "nodeID": req.NodeId})
	if err := c.rc.Leave(req.NodeId); err != nil {
		logc.Errorf("c.rc.Leave. err = %v", err)
		if errors.Is(err, store.ErrNotRaftLeader) {
			return nil, status.Errorf(codes.FailedPrecondition, "%s", store.ErrNotRaftLeader)
		}

		return nil, err
	} else {
		logc.Infof("leaved completed success")
		return &v1.Empty{}, nil
	}
}

func (c *ClusterServer) IsNodeLeader(ctx context.Context, req *v1.Empty) (*v1.IsNodeLeaderResponse, error) {
	return &v1.IsNodeLeaderResponse{
		IsLeader: c.rc.IsLeader(),
	}, nil
}

func (c *ClusterServer) Snapshot(ctx context.Context, req *v1.Empty) (*v1.Empty, error) {
	if err := c.rc.Snapshot(); err != nil {
		return nil, err
	}

	return &v1.Empty{}, nil
}
