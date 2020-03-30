package store

import (
	"fmt"
	"github.com/1xyz/beanstalkd/state"
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

type fsm Store

// Apply applies a Raft log entry to the key-value store.
func (f *fsm) Apply(l *raft.Log) interface{} {
	return nil
}

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
