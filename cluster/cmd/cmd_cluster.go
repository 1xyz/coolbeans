package cmd

import (
	"fmt"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/cluster/client"
	"github.com/1xyz/coolbeans/cluster/server"
	"github.com/1xyz/coolbeans/store"
	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/davecgh/go-spew/spew"
	"github.com/docopt/docopt-go"
	"github.com/hashicorp/raft"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type ClusterNodeConfig struct {
	NodeId          string
	NodeListenAddr  string
	RootDir         string
	NodePeerAddrs   string
	BootstrapNodeId string
	PeerTimeoutSecs int

	MaxClusterJoinAttempts       int
	ClusterJoinRetryIntervalSecs int

	RaftListenAddr       string
	RaftAdvertizedAddr   string
	RaftTimeoutSecs      int
	MaxPoolSize          int
	NoDiskLog            bool
	NoFsync              bool
	RestoreTimeoutSecs   int
	RetainSnapshotCount  int
	SnapshotThreshold    int
	TrailingLogCount     int
	SnapshotIntervalSecs int
	PrometheusAddr       string
}

func CmdClusterNode(argv []string, version string) {
	usage := `usage: cluster-node --node-id=<id> --root-dir=<dir> --bootstrap-node-id=<id> [options]
   
options:
    -h, --help
    --node-id=<id>               A unique identifier for this node.
    --root-dir=<dir>             Root Directory where the snapshot is stored.
    --node-listen-addr=<addr>    Listen/Bind Address for the node cluster service [default: 127.0.0.1:11000].
    --bootstrap-node-id=<id>     Identifier of the node that should perform bootstrapping.
    --node-peer-addrs=<addrs>    Comma separated node addresses of all cluster node service addresses [default: ].
    --peer-timeout-secs=<secs>   Peer grpc timeout in seconds [default: 10].
   
Cluster join options:
    --max-cluster-join-attempts=<n>            The maximum number of join attempts. Defaults to 0, 
                                               which will retry indefinitely [default: 0]
    --cluster-join-retry-interval-secs=<secs>  Time in seconds to wait between retrying to join the cluster [default: 1].
    
Raft options:
    --raft-listen-addr=<addr>        Listen/Bind Address for Raft service [default: 127.0.0.1:21000].
    --raft-advertized-addr=<addr>    Advertized address for Raft service [default: 127.0.0.1:21000].
    --raft-timeout-secs=<secs>       Raft peer network connection timeout in seconds [default: 30].
    --max-pool-size=<n>              Maximum number of raft connections pooled [default: 3].
    --no-disk-log                    If set, raft log is not persisted to disk [default: false].
    --no-fsync                       If set, raft log is not flushed to disk for every log commit. This option
                                     is only used when raft logs are persisted to disk [default: false].  
    --restore-timeout-secs=<secs>    Timeout in seconds for a snapshot restore operation [default: 60].
    --retain-snapshot-count=<n>      The maximum number of file snapshots to retain on disk [default: 3].
    --snapshot-threshold=<n>         Controls how many outstanding logs there must be before a 
                                     Raft snapshot is taken [default: 8192].
    --trailing-log-count=<n>         Controls how many logs are left after a snapshot. This is used so 
                                     that we can quickly replay logs on a follower instead of being forced 
                                     to send an entire snapshot [default: 10240].
    --snapshot-interval-secs=<secs>  Controls how often the raft library checks to perform a snapshot.
                                     The library randomly staggers between this value and 2x this value to 
                                     avoid all the nodes in the entire cluster from performing a snapshot 
                                     at once [default: 120].
    
Metrics options:
    --prometheus-addr=<addr>         Start a prometheus server to expose metrics at this address. By default no server
                                     is started. Example value is ":2122" [default: ]
`

	opts, err := docopt.ParseArgs(usage, argv[1:], version)
	if err != nil {
		log.Fatalf("cmdClusterNode: error parsing arguments. err=%v", err)
	}
	var cfg ClusterNodeConfig
	if err := opts.Bind(&cfg); err != nil {
		log.Fatalf("cmdClusterNode: error in opts.bind. err=%v", err)
	}
	spew.Dump(cfg)
	if err := RunCoolbeans(&cfg); err != nil {
		log.Fatalf("cmdClusterNode: runCoolbeans: err = %v", err)
	}
	log.Infof("gracefully terminated")
}

// waitForShutdown waits for a terminate or interrupt signal
// terminates the server once a signal is received.
func waitForShutdown(s *store.Store, c *ClusterNodeConfig, gs *grpc.Server) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-done
	log.Infof("Shutdown signal received")
	shutdown(s, c, gs)
}

func shutdown(s *store.Store, c *ClusterNodeConfig, gs *grpc.Server) {
	if err := LeaveCluster(c, parsePeerAddrs(c.NodePeerAddrs)); err != nil {
		log.Errorf("shutdown: LeaveCluster err = %v", err)
	}
	log.Infof("shutdown: stop to the grpc server")
	gs.Stop()
	log.Infof("shutdown: complete")
}

func RunCoolbeans(c *ClusterNodeConfig) error {
	log.Infof("RunCoolbeans: creating directory for node=%v at %v", c.NodeId, c.RootDir)
	if err := os.MkdirAll(c.RootDir, 0700); err != nil {
		return err
	}
	peersAddrs := parsePeerAddrs(c.NodePeerAddrs)
	s, err := store.NewStore(&store.Config{
		RetainSnasphotCount: c.RetainSnapshotCount,
		MaxPool:             c.MaxPoolSize,
		RaftTimeout:         time.Duration(c.RaftTimeoutSecs) * time.Second,
		RestoreTimeout:      time.Duration(c.RestoreTimeoutSecs) * time.Second,
		RootDir:             c.RootDir,
		RaftBindAddr:        c.RaftListenAddr,
		Inmem:               c.NoDiskLog,
		LogNoSync:           c.NoFsync,
		SnapshotInterval:    time.Duration(c.SnapshotIntervalSecs) * time.Second,
		TrailingLogs:        uint64(c.TrailingLogCount),
		SnapshotThreshold:   uint64(c.SnapshotThreshold),
		LocalNodeID:         c.NodeId,
	})
	if err != nil {
		return err
	}
	if err := InitializeMetrics("jellybeans", c.PrometheusAddr); err != nil {
		log.Errorf("RunCoolbeans: InitializeMetrics: %v", err)
		return err
	}
	if err := s.Open(); err != nil {
		log.Errorf("RunCoolbeans: store.Open err=%v", err)
		return err
	}
	if err := BootstrapCluster(c, s, peersAddrs); err != nil {
		log.Errorf("RunCoolbeans: BootstrapCluster err=%v", err)
		if err != raft.ErrCantBootstrap {
			if err := JoinCluster(c, peersAddrs); err != nil {
				log.Errorf("RunCoolbeans: BootstrapCluster JoinCluster failed! %v", err)
				return fmt.Errorf("bootstrapCluster JoinCluster failed err %w", err)
			}
		}
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	v1.RegisterClusterServer(grpcServer, server.NewClusterServer(s))
	jsmServer := server.NewJSMServer(s)
	v1.RegisterJobStateMachineServer(grpcServer, jsmServer)
	go jsmServer.RunController()
	healthgrpc.RegisterHealthServer(grpcServer,
		server.NewHealthCheckServer(s))
	log.Infof("RunCoolbeans: Cluster node server listen on addr=%v", c.NodeListenAddr)
	lis, err := net.Listen("tcp", c.NodeListenAddr)
	if err != nil {
		return fmt.Errorf("RunCoolbeans: failed to listen: %w", err)
	}
	go waitForShutdown(s, c, grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Errorf("RunCoolbeans: grpcServer.Serve. err = %v", err)
		shutdown(s, c, grpcServer)
		return err
	}
	return nil
}

func BootstrapCluster(c *ClusterNodeConfig, s *store.Store, peerAddrs []string) error {
	b, err := CanBootstrapCluster(c, peerAddrs)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("canBootstrapCluster=false. cluster can't be bootstrapped")
	}

	bootstrapAddr := c.RaftAdvertizedAddr
	if len(bootstrapAddr) == 0 {
		bootstrapAddr = c.RaftListenAddr
	}
	log.Infof("BootstrapCluster: Bootstrap cluster from node %v using addr=%v", c.NodeId, bootstrapAddr)
	return s.BootstrapCluster(map[string]string{c.NodeId: bootstrapAddr})
}

func CanBootstrapCluster(c *ClusterNodeConfig, peerAddrs []string) (bool, error) {
	if c.BootstrapNodeId != c.NodeId {
		log.Infof("CanBootstrapCluster: cannot bootstrap on this node BootstrapNodeId: %s localNodeId: %s",
			c.BootstrapNodeId, c.NodeId)
		return false, nil
	}
	if len(peerAddrs) == 0 {
		log.Infof("No peers defined assuming a single node cluster")
		return true, nil
	}
	for _, addr := range peerAddrs {
		log.Infof("CanBootstrapCluster: Connect addr = %v", addr)
		nodeCli, err := client.NewClusterNodeClient(addr, time.Duration(c.PeerTimeoutSecs)*time.Second)
		if err != nil {
			log.Errorf("CanBootstrapCluster: client.NewClusterNodeClient err %v", err)
			nodeCli.Close()
			return false, err
		}

		if _, err := nodeCli.IsNodeLeader(); err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Unavailable {
				log.Warnf("GRPC unavailable error failed at node addr=%v err=%v", addr, st.Message())
				nodeCli.Close()
				continue
			}

			log.Warnf("CanBootstrapCluster: Node is available err = %v", err)
			nodeCli.Close()
			return false, nil
		} else {
			log.Infof("CanBootstrapCluster: Request returned successfully")
			nodeCli.Close()
			return false, nil
		}
	}

	return true, nil
}

func parsePeerAddrs(addr string) []string {
	peersAddrs := strings.Split(addr, ",")
	if len(peersAddrs) == 1 && peersAddrs[0] == "" {
		log.Infof("CanBootstrapCluster: peerAddrs is empty")
		return make([]string, 0)
	}
	return peersAddrs
}

func LeaveCluster(c *ClusterNodeConfig, peerAddrs []string) error {
	if len(peerAddrs) == 0 {
		return fmt.Errorf("peerAddrs is empty")
	}
	log.Infof("LeaveCluster: try to leave the cluster nodeId: %v", c.NodeId)
	nc, err := client.NewClusterNodeClientWithLB(peerAddrs,
		time.Duration(c.PeerTimeoutSecs)*time.Second)
	if err != nil {
		return err
	}

	defer nc.Close()
	return nc.LeaveCluster(c.NodeId)
}

func JoinCluster(c *ClusterNodeConfig, peerAddrs []string) error {
	connTimeout := time.Duration(c.PeerTimeoutSecs) * time.Second
	nodeClients := make([]*client.ClusterNodeClient, len(peerAddrs))
	for i, addr := range peerAddrs {
		if nc, err := client.NewClusterNodeClient(addr, connTimeout); err != nil {
			return fmt.Errorf("joinCluster: client.NewClusterNodeClient addr=%v %w", addr, err)
		} else {
			nodeClients[i] = nc
		}
	}

	nRetry := c.MaxClusterJoinAttempts
	waitDuration := time.Duration(c.ClusterJoinRetryIntervalSecs) * time.Second
	i := 0
	for {
		if nRetry > 0 && i >= nRetry {
			break
		}
		for _, nc := range nodeClients {
			log.Infof("Joincluster (attempt: %d): trying to connect to addr = %v ...", (i + 1), nc.DispHostAddr)
			err := nc.JoinCluster(c.NodeId, c.RaftListenAddr)
			if err != nil {
				st, ok := status.FromError(err)
				if !ok {
					log.Warnf("JoinCluster: Unhandled err = %v", err)
					return err
				}
				if st.Code() == codes.Unavailable {
					log.Warnf("JoinCluster: GRPC unavailable error failed at node addr=%v err=%v", nc.DispHostAddr, st.Message())
					continue
				}
				if st.Code() == codes.FailedPrecondition && st.Message() == store.ErrNotRaftLeader.Error() {
					log.Warnf("JoinCluster: join failed at node err=%v. retrying attempt = %d", st.Message(), (i + 1))
					continue
				}
			}
			log.Infof("JoinCluster: joined cluster successfully")
			return nil
		}

		time.Sleep(waitDuration)
		i++
	}
	return fmt.Errorf("joinCluster: unable to join cluster")
}

func InitializeMetrics(serviceName, metricsAddr string) error {
	var sink metrics.MetricSink = nil
	var err error = nil
	if metricsAddr != "" {
		sink, err = prometheus.NewPrometheusSink()
		if err != nil {
			return err
		}
	} else {
		sink = &metrics.BlackholeSink{}
	}

	m, err := metrics.NewGlobal(metrics.DefaultConfig(serviceName), sink)
	if err != nil {
		return err
	}
	m.EnableHostname = false
	spew.Dump(m)

	if metricsAddr != "" {
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(metricsAddr, nil); err != nil {
				log.Fatalf("Unable to start prometheus server err = %v", err)
			}
		}()
	}
	return nil
}
