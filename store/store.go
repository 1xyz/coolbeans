package store

import (
	"github.com/1xyz/beanstalkd/state"
	"github.com/hashicorp/raft"
	"io"
	"time"
)

type store struct {
	jsm state.JSM

	// Timeout for a restore operation
	restoreTimeout time.Duration
}

// Snapshot is used to support log compaction. This call should
// return an FSMSnapshot which can be used to save a point-in-time
// snapshot of the FSM. Apply and Snapshot are not called in multiple
// threads, but Apply will be called concurrently with Persist. This means
// the FSM should be implemented in a fashion that allows for concurrent
// updates while a snapshot is happening.
func (s *store) Snapshot() (raft.FSMSnapshot, error) {
	return NewSnapshotFrom(s.jsm)
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
func (s *store) Restore(r io.ReadCloser) error {
	return RestoreSnapshotTo(r, s.jsm, s.restoreTimeout)
}
