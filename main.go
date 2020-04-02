package main

import (
	"encoding/json"
	"fmt"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/server"
	"github.com/1xyz/coolbeans/store"
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	usage := `coolbeans.

Usage:
  coolbeans [--node-id=<id>]  [--config-file=<file>] [--join-id=<id>] 
  coolbeans -h | --help

Options:
  -h --help                 Show this screen.
  --node-id=<id>            unique identifier for this node.
  --join-id=<id>            unique identifier of the node to join with the cluster [default: ].
  --config-file=<config>  	configuration json file [default: cb-config.json].`

	opts, err := docopt.ParseDoc(usage)
	if err != nil {
		log.Fatalf("error parsing arguments. err=%v", err)
	}

	fmt.Printf("opts=%v\n", opts)

	localNodeID, err := opts.String("--node-id")
	if err != nil {
		log.Fatalf("error reading --node-id %v", err)
	}

	joinNodeID, err := opts.String("--join-id")
	if err != nil {
		log.Fatalf("error reading --join-id %v", err)
	}

	file, err := opts.String("--config-file")
	if err != nil {
		log.Fatalf("error reading --config-file variable %v", err)
	}

	c, err := LoadFrom(file)
	if err != nil {
		log.Fatalf("cannot load config from file=%v err=%v", file, err)
	}

	log.Infof("NodeID = %v config-%v JoinNodeID=%v", localNodeID, c, joinNodeID)

	if err := runCoolbeans(c, localNodeID, joinNodeID); err != nil {
		log.Fatalf("runcoolbeans err = %v", err)
	}
}

type Config struct {
	// Cluster wide configuration
	Cluster struct {
		Name                string   `json:"name"`
		InMem               bool     `json:"in_mem"`
		RaftTimeout         Duration `json:"raft_timeout"`
		RestoreTimeout      Duration `json:"restore_timeout"`
		RetainSnasphotCount int      `json:"retain_snasphot_count"`
		MaxPool             int      `json:"max_pool"`
	} `json:"cluster"`
	// Node(s) specific configuration
	Nodes []NodeConfig `json:"nodes"`
}

type NodeConfig struct {
	ID         string `json:"id"`
	ListenAddr string `json:"listen_addr"`
	RaftAddr   string `json:"raft_addr"`
	RootDir    string `json:"root_dir"`
}

func (c *Config) getNode(nodeID string) (*NodeConfig, error) {
	for _, nc := range c.Nodes {
		if nc.ID == nodeID {
			return &nc, nil
		}
	}

	return nil, fmt.Errorf("node with id=%v not found", nodeID)
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return fmt.Errorf("invalid duration")
	}
}

func (d Duration) String() string {
	return fmt.Sprintf("%v", time.Duration(d))
}

func ReadFrom(r io.Reader) (*Config, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	} else {
		return &c, nil
	}
}

func LoadFrom(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Errorf("error closing file %v", err)
		}
	}()
	return ReadFrom(f)
}

func runCoolbeans(c *Config, nodeID string, joinID string) error {
	logc := log.WithField("method", "runCoolBeans")
	nc, err := c.getNode(nodeID)
	if err != nil {
		return err
	}

	logc.Infof("creating directory for node=%v at %v", nc.ID, nc.RootDir)
	if err := os.MkdirAll(nc.RootDir, 0700); err != nil {
		return err
	}

	s, err := store.NewStore(&store.Config{
		RetainSnasphotCount: c.Cluster.RetainSnasphotCount,
		MaxPool:             c.Cluster.MaxPool,
		RaftTimeout:         time.Duration(c.Cluster.RaftTimeout),
		RestoreTimeout:      time.Duration(c.Cluster.RestoreTimeout),
		RootDir:             nc.RootDir,
		RaftBindAddr:        nc.RaftAddr,
		Inmem:               c.Cluster.InMem,
	})
	if err != nil {
		return err
	}

	enableSingle := false
	if joinID == "" {
		enableSingle = true
	}

	if err := s.Open(enableSingle, nodeID); err != nil {
		logc.Errorf("store.Open enableSingle=%v nodeId=%v err=%v",
			enableSingle, nodeID, err)
		return err
	}

	if joinID != "" {
		remoteNC, err := c.getNode(joinID)
		if err != nil {
			logc.Errorf("error getNode joinID=%v. err=%v", joinID, err)
			return err
		}

		if err := joinNode(nc, remoteNC, time.Duration(c.Cluster.RaftTimeout)); err != nil {
			return err
		}
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	v1.RegisterClusterServer(grpcServer,
		server.NewClusterServer(s))
	jsmServer := server.NewJSMServer(s)
	v1.RegisterJobStateMachineServer(grpcServer,
		jsmServer)
	go jsmServer.RunController()

	logc.Infof("tcp server listen on %v", nc.ListenAddr)
	lis, err := net.Listen("tcp", nc.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	return grpcServer.Serve(lis)
}

func joinNode(LocalNC, remoteNC *NodeConfig, timeout time.Duration) error {
	conn, err := grpc.Dial(remoteNC.ListenAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("err grpc.Dial remote: %v. err: %v", remoteNC.ListenAddr, err)
	}
	defer conn.Close()

	c := v1.NewClusterClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req := &v1.JoinRequest{
		NodeId: LocalNC.ID,
		Addr:   LocalNC.RaftAddr}
	if _, err := c.Join(ctx, req); err != nil {
		return err
	}

	log.Infof("Join completed successfully")
	return nil
}
