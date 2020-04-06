package cluster

import (
	v1 "github.com/1xyz/coolbeans/api/v1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type RaftCluster interface {
	// Join, joins this node, identified by nodeID and reachable at addr,
	// to an existing Raft cluster.
	Join(nodeID, addr string) error
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

func (c *ClusterServer) Join(ctx context.Context, req *v1.JoinRequest) (*v1.Empty, error) {
	log.WithField("method", "clusterServer.Join").
		Infof("request to join cluster nodeId=%v, addr=%v", req.NodeId, req.Addr)
	if err := c.rc.Join(req.NodeId, req.Addr); err != nil {
		return nil, err
	} else {
		return &v1.Empty{}, nil
	}
}
