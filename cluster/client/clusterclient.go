package client

import (
	v1 "github.com/1xyz/coolbeans/api/v1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"time"
)

type ClusterNodeClient struct {
	v1.ClusterClient
	timeout  time.Duration
	conn     *grpc.ClientConn
	HostAddr string
}

func NewClusterNodeClient(hostAddr string,
	connTimeout time.Duration, opts ...grpc.DialOption) (*ClusterNodeClient, error) {
	conn, err := grpc.Dial(hostAddr, opts...)
	if err != nil {
		log.Errorf("NewClusterNodeClient: grpc.Dial err=%v", err)
		return nil, err
	}
	return &ClusterNodeClient{
		HostAddr:      hostAddr,
		conn:          conn,
		ClusterClient: v1.NewClusterClient(conn),
		timeout:       connTimeout,
	}, nil
}

func (c *ClusterNodeClient) Close() error {
	return c.conn.Close()
}

func (c *ClusterNodeClient) newCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	return ctx, cancel
}

func (c *ClusterNodeClient) LeaveCluster(nodeID string) error {
	ctx, cancel := c.newCtx()
	defer cancel()
	if _, err := c.Leave(ctx, &v1.LeaveRequest{NodeId: nodeID}); err != nil {
		log.Errorf("LeaveCluster: error %v", err)
		return err
	}

	log.Debugf("LeaveCluster: nodeId:%v complete", nodeID)
	return nil
}

func (c *ClusterNodeClient) IsNodeLeader() (bool, error) {
	ctx, cancel := c.newCtx()
	defer cancel()
	b, err := c.ClusterClient.IsNodeLeader(ctx, &v1.Empty{})
	if err != nil {
		log.Errorf("IsNodeLeader: error %v", err)
		return false, err
	}

	log.Debugf("LeaveCluster: IsNodeLeader:%v ", b.IsLeader)
	return b.IsLeader, nil
}

func (c *ClusterNodeClient) JoinCluster(nodeID, raftAddr string) error {
	req := &v1.JoinRequest{NodeId: nodeID, Addr: raftAddr}
	ctx, cancel := c.newCtx()
	defer cancel()
	if _, err := c.Join(ctx, req); err != nil {
		log.Errorf("JoinCluster: error %v", err)
		return err
	}

	log.Debugf("JoinCluster: nodeId:%v raftAddr:%v complete", nodeID, raftAddr)
	return nil
}
