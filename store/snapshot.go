package store

import (
	"encoding/binary"
	"fmt"
	v1 "github.com/1xyz/coolbeans/api/v1"
	"github.com/1xyz/coolbeans/state"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/raft"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"time"
)

//

type SnapshotWriter struct {
	js state.JSMSnapshot
}

func NewSnapshotWriter(jsm state.JSM) (*SnapshotWriter, error) {
	js, err := jsm.Snapshot()
	if err != nil {
		return nil, err
	}
	return &SnapshotWriter{js: js}, nil
}

func (sw *SnapshotWriter) Persist(sink raft.SnapshotSink) error {
	snkID := sink.ID()
	err := func(sinkID string) error {
		var nBytesWritten, nCRWritten, nJobWritten uint64 = 0, 0, 0

		crEntries, err := sw.js.SnapshotClients()
		if err != nil {
			return err
		}
		for cr := range crEntries {
			crProto := toV1ClientResvEntryProto(cr)
			if n, err := WriteProto(sink, crProto); err != nil {
				return fmt.Errorf("persist: WriteProto(cr): err: %w", err)
			} else {
				nBytesWritten += uint64(n)
				nCRWritten++
			}
		}

		jEntries, err := sw.js.SnapshotJobs()
		if err != nil {
			return err
		}
		for j := range jEntries {
			jProto := JobToJobProto(j)
			if n, err := WriteProto(sink, jProto); err != nil {
				return fmt.Errorf("persist: WriteProto(j): err: %w", err)
			} else {
				nBytesWritten += uint64(n)
				nJobWritten++
			}
		}

		log.Infof("Persist: bytesWritten=%d clientResvns=%d jobs=%v",
			nBytesWritten, nCRWritten, nJobWritten)
		return sink.Close()
	}(snkID)

	if err != nil {
		log.Errorf("Persist: write. err: %v", err)
		if err := sink.Cancel(); err != nil {
			log.Errorf("Persist: sink.cancel. err=%v", err)
		}
	}
	return err
}

func (sw *SnapshotWriter) Release() {
	log.Info("SnapshotWriter: Release called")
}

func toV1ClientResvEntryProto(cr *state.ClientResvEntry) *v1.ClientResvEntryProto {
	cliProto := &v1.ClientResvEntryProto{
		ClientId:         string(cr.CliID),
		ResvDeadlineAt:   cr.ResvDeadlineAt,
		IsWaitingForResv: cr.IsWaitingForResv,
		TickAt:           cr.TickAt,
		ReqId:            cr.ReqID,
		HeapIndex:        int32(cr.HeapIndex),
		WatchedTube:      make([]string, 0),
	}
	for _, t := range cr.WatchedTubes {
		cliProto.WatchedTube = append(cliProto.WatchedTube, string(t))
	}
	return cliProto
}

func WriteProto(w io.Writer, m proto.Message) (int, error) {
	bytes, err := proto.Marshal(m)
	if err != nil {
		return 0, fmt.Errorf("WriteProto: proto.Marshal: err: %w", err)
	}
	return WriteBytes(w, bytes)
}

func WriteBytes(w io.Writer, b []byte) (int, error) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(b)))
	nBuf, err := w.Write(buf)
	if err != nil {
		return 0, err
	}
	nMsg, err := w.Write(b)
	if err != nil {
		return 0, err
	}
	return nBuf + nMsg, nil
}

// ////////////// ////////////// ////////////// ////////////// ////////////

type SnapshotReader struct {
	js        state.JSMSnapshot
	crEntries []*state.ClientResvEntry
	jEntries  []state.Job
}

func NewSnapshotReader(jsm state.JSM) (*SnapshotReader, error) {
	js, err := jsm.Snapshot()
	if err != nil {
		return nil, err
	}
	return &SnapshotReader{
		js:        js,
		crEntries: make([]*state.ClientResvEntry, 0),
	}, nil
}

func (sr *SnapshotReader) Restore(rdr io.Reader, timeout time.Duration) error {
	for {
		m, err := ReadProto(rdr)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Errorf("Restore: ReadProto: err = %v", err)
			return err
		}

		if crProto, ok := m.(*v1.ClientResvEntryProto); ok {
			sr.crEntries = append(sr.crEntries, toClientResvEntry(crProto))
			continue
		}
		if jProto, ok := m.(*v1.JobProto); ok {
			sr.jEntries = append(sr.jEntries, NewJobFromJobProto(jProto))
			continue
		}

		return fmt.Errorf("unknown message cannot decode %v %T", m, m)
	}

	if err := sr.restoreCREntries(timeout); err != nil {
		log.Errorf("Restore: restoreCREntries: err = %v", err)
		return err
	}
	if err := sr.restoreJobs(timeout); err != nil {
		log.Errorf("Restore: restoreJobs err = %v", err)
		return err
	}
	// commit the entries to the new JSM
	sr.js.FinalizeRestore()
	return nil
}

func (sr *SnapshotReader) restoreCREntries(timeout time.Duration) error {
	crCh := make(chan *state.ClientResvEntry)
	errCh := make(chan error)

	go func(restoreCh <-chan *state.ClientResvEntry, errCh chan<- error) {
		defer close(errCh)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		n := len(sr.crEntries)
		log.Debugf("restoreCREntries: restore n=%v clients to job state machine", n)
		errCh <- sr.js.RestoreClients(ctx, n, crCh)
	}(crCh, errCh)

	for _, r := range sr.crEntries {
		crCh <- r
	}

	close(crCh)
	err := <-errCh
	return err
}

func (sr *SnapshotReader) restoreJobs(timeout time.Duration) error {
	jobCh := make(chan state.Job)
	errCh := make(chan error)

	go func(restoreCh <-chan state.Job, errCh chan<- error) {
		defer close(errCh)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		log.Debugf("restoreJobs: restore jobs to job state machine")
		errCh <- sr.js.RestoreJobs(ctx, restoreCh)
	}(jobCh, errCh)

	for _, job := range sr.jEntries {
		jobCh <- job
	}

	close(jobCh)
	err := <-errCh
	return err
}

func ReadProto(r io.Reader) (proto.Message, error) {
	b, err := ReadBytes(r)
	if err != nil {
		return nil, err
	}

	var msg proto.Message
	if err := proto.Unmarshal(b, msg); err != nil {
		return nil, err
	} else {
		return msg, nil
	}
}

func ReadBytes(r io.Reader) ([]byte, error) {
	buf := make([]byte, 4)
	if n, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	} else if n != 4 {
		return nil, fmt.Errorf("readBytes: expected 4 hdr bytes to be read")
	}

	size := binary.LittleEndian.Uint32(buf)
	b := make([]byte, size)
	if n, err := io.ReadFull(r, b); err != nil {
		return nil, err
	} else if uint32(n) != size {
		return nil, fmt.Errorf("readBytes expected size=%d payload bytes to be read", size)
	}
	return b, nil
}

func toClientResvEntry(crProto *v1.ClientResvEntryProto) *state.ClientResvEntry {
	return &state.ClientResvEntry{
		CliID:            state.ClientID(crProto.ClientId),
		WatchedTubes:     []state.TubeName{},
		ResvDeadlineAt:   crProto.ResvDeadlineAt,
		IsWaitingForResv: crProto.IsWaitingForResv,
		TickAt:           crProto.TickAt,
		ReqID:            crProto.ReqId,
		HeapIndex:        int(crProto.HeapIndex),
	}
}

// ////////////// ////////////// ////////////// ////////////
type snapshot struct {
	snap *v1.SnapshotProto
}

func NewSnapshotFrom(jsm state.JSM) (*snapshot, error) {
	jsmSnap, err := jsm.Snapshot()
	if err != nil {
		return nil, err
	}

	ss := newSnapshot()
	if err := ss.createFrom(jsmSnap); err != nil {
		return nil, err
	}

	return ss, nil
}

func RestoreSnapshotTo(rdr io.Reader, jsm state.JSM, timeout time.Duration) error {
	logc := log.WithField("method", "RestoreSnapshotTo")

	jsmSnap, err := jsm.Snapshot()
	if err != nil {
		return err
	}

	ss := newSnapshot()
	if err := ss.readFull(rdr); err != nil {
		logc.Errorf("ss.readFull. err=%v", err)
		return err
	}

	return ss.restoreTo(jsmSnap, timeout)
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	logc := log.WithField("method", "snapshot.Persist")
	snkID := sink.ID()
	err := func(sinkID string) error {
		bytes, err := proto.Marshal(s.snap)
		if err != nil {
			return err
		}

		// Write data to sink.
		if n, err := sink.Write(bytes); err != nil {
			return err
		} else {
			logc.Debugf("sinkID=%v, wrote %v bytes to sink", sinkID, n)
		}

		// Close the sink.
		return sink.Close()
	}(snkID)

	if err != nil {
		logc.Errorf("marshal. err=%v", err)
		if err := sink.Cancel(); err != nil {
			logc.Errorf("sink.cancel. err=%v", err)
		}
	}

	return err
}

func (s *snapshot) Release() {
	log.Info("Release called")
}

func newSnapshot() *snapshot {
	return &snapshot{
		snap: &v1.SnapshotProto{
			Jobs:         make([]*v1.JobProto, 0),
			Reservations: make([]*v1.ClientResvEntryProto, 0),
		}}
}

func (s *snapshot) readFull(r io.Reader) error {
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	return proto.Unmarshal(bytes, s.snap)
}

func (s *snapshot) appendClientRsvEntry(cli *state.ClientResvEntry) {
	cliProto := &v1.ClientResvEntryProto{
		ClientId:         string(cli.CliID),
		ResvDeadlineAt:   cli.ResvDeadlineAt,
		IsWaitingForResv: cli.IsWaitingForResv,
		TickAt:           cli.TickAt,
		ReqId:            cli.ReqID,
		HeapIndex:        int32(cli.HeapIndex),
		WatchedTube:      make([]string, 0),
	}

	for _, t := range cli.WatchedTubes {
		cliProto.WatchedTube = append(cliProto.WatchedTube, string(t))
	}

	s.snap.Reservations = append(s.snap.Reservations, cliProto)
}

func (s *snapshot) appendJob(j state.Job) {
	s.snap.Jobs = append(s.snap.Jobs, JobToJobProto(j))
}

func JobToJobProto(j state.Job) *v1.JobProto {
	return &v1.JobProto{
		Id:         int64(j.ID()),
		Priority:   j.Priority(),
		Delay:      j.Delay(),
		Ttr:        int32(j.TTR()),
		TubeName:   string(j.TubeName()),
		CreatedAt:  j.CreatedAt(),
		ReadyAt:    j.ReadyAt(),
		ExpiresAt:  j.ExpiresAt(),
		State:      v1.JobStateProto(j.State()),
		ReservedBy: string(j.ReservedBy()),
		BodySize:   int32(j.BodySize()),
		Body:       j.Body(),
		BuriedAt:   j.BuriedAt(),

		ReserveCount: j.ReserveCount(),
		TimeoutCount: j.TimeoutCount(),
		ReleaseCount: j.ReleaseCount(),
		BuryCount:    j.BuryCount(),
		KickCount:    j.KickCount(),
	}
}

func (s *snapshot) ReadFromFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.readFull(f)
}

func (s *snapshot) createFrom(jsmSnap state.JSMSnapshot) error {
	logc := log.WithField("method", "snapshot.createFrom")

	cliEntries, err := jsmSnap.SnapshotClients()
	if err != nil {
		logc.Errorf("jsmSnap.SnapshotClients. err=%v", err)
		return err
	}

	for cli := range cliEntries {
		s.appendClientRsvEntry(cli)
	}

	jobs, err := jsmSnap.SnapshotJobs()
	if err != nil {
		logc.Errorf("jsmSnap.SnapshotJobs. err=%v", err)
		return err
	}

	for job := range jobs {
		s.appendJob(job)
	}

	return nil
}

func (s *snapshot) restoreTo(jsmSnap state.JSMSnapshot, timeout time.Duration) error {
	logc := log.WithField("method", "snapshot.restoreTo")

	if err := s.restoreCliRevEntries(jsmSnap, timeout); err != nil {
		logc.Errorf("s.restoreCliRevEntries. err=%v", err)
		return err
	}

	if err := s.restoreJobs(jsmSnap, timeout); err != nil {
		logc.Errorf("s.restoreJobs. err=%v", err)
		return err
	}

	jsmSnap.FinalizeRestore()
	return nil
}

func (s *snapshot) restoreCliRevEntries(jsmSnap state.JSMSnapshot, timeout time.Duration) error {
	logc := log.WithField("method", "snapshot.restoreCliRevEntries")
	cliCh := make(chan *state.ClientResvEntry)
	errCliCh := make(chan error)
	go func(restoreCh <-chan *state.ClientResvEntry, errCh chan<- error) {
		defer close(errCh)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		n := len(s.snap.Reservations)
		logc.Debugf("restore n=%v clients to job state machine", n)
		errCh <- jsmSnap.RestoreClients(ctx, n, cliCh)
	}(cliCh, errCliCh)

	for _, r := range s.snap.Reservations {
		cliCh <- &state.ClientResvEntry{
			CliID:            state.ClientID(r.ClientId),
			WatchedTubes:     []state.TubeName{},
			ResvDeadlineAt:   r.ResvDeadlineAt,
			IsWaitingForResv: r.IsWaitingForResv,
			TickAt:           r.TickAt,
			ReqID:            r.ReqId,
			HeapIndex:        int(r.HeapIndex),
		}
	}

	close(cliCh)
	err := <-errCliCh
	return err
}

func (s *snapshot) restoreJobs(jsmSnap state.JSMSnapshot, timeout time.Duration) error {
	logc := log.WithField("method", "snapshot.restoreJobs")
	jobCh := make(chan state.Job)
	errCh := make(chan error)

	go func(restoreCh <-chan state.Job, errCh chan<- error) {
		defer close(errCh)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		logc.Debugf("restore jobs to job state machine")
		errCh <- jsmSnap.RestoreJobs(ctx, restoreCh)
	}(jobCh, errCh)

	for _, job := range s.snap.Jobs {
		jobCh <- NewJobFromJobProto(job)
	}

	close(jobCh)
	err := <-errCh
	return err
}

type wrapJob struct {
	jp *v1.JobProto
}

func NewJobFromJobProto(jp *v1.JobProto) state.Job {
	return &wrapJob{jp: jp}
}

func (j *wrapJob) ID() state.JobID {
	return state.JobID(j.jp.Id)
}
func (j *wrapJob) Priority() uint32 {
	return j.jp.Priority
}
func (j *wrapJob) UpdatePriority(newPriority uint32) uint32 {
	j.jp.Priority = newPriority
	return j.jp.Priority
}
func (j *wrapJob) Delay() int64 {
	return j.jp.Delay
}
func (j *wrapJob) UpdateDelay(newDelay int64) int64 {
	j.jp.Delay = newDelay
	return j.jp.Delay
}
func (j *wrapJob) TTR() int {
	return int(j.jp.Ttr)
}
func (j *wrapJob) BodySize() int {
	return int(j.jp.BodySize)
}
func (j *wrapJob) Body() []byte {
	return j.jp.Body
}
func (j *wrapJob) TubeName() state.TubeName {
	return state.TubeName(j.jp.TubeName)
}
func (j *wrapJob) CreatedAt() int64 {
	return j.jp.CreatedAt
}
func (j *wrapJob) ReadyAt() int64 {
	return j.jp.ReadyAt
}
func (j *wrapJob) UpdateReadyAt(nowSeconds int64) (int64, error) {
	j.jp.ReadyAt = nowSeconds + j.jp.Delay
	return j.jp.ReadyAt, nil
}
func (j *wrapJob) State() state.JobState {
	return state.JobState(j.jp.State)
}
func (j *wrapJob) UpdateState(newState state.JobState) {
	j.jp.State = v1.JobStateProto(newState)
}
func (j *wrapJob) UpdateReservedBy(clientID state.ClientID) {
	j.jp.ReservedBy = string(clientID)
}
func (j *wrapJob) ExpiresAt() int64 {
	return j.jp.ExpiresAt
}
func (j *wrapJob) ReservedBy() state.ClientID {
	return state.ClientID(j.jp.ReservedBy)
}
func (j *wrapJob) UpdateReservation(nowSeconds int64) (int64, error) {
	j.jp.ExpiresAt = nowSeconds + int64(j.jp.Ttr)
	return j.jp.ExpiresAt, nil
}
func (j *wrapJob) ResetBuriedAt() {
	j.jp.BuriedAt = 0
}
func (j *wrapJob) UpdateBuriedAt(nowSeconds int64) int64 {
	j.jp.BuriedAt = nowSeconds
	return j.jp.BuriedAt
}
func (j *wrapJob) BuriedAt() int64 {
	return j.jp.BuriedAt
}
func (j *wrapJob) ReserveCount() uint32 {
	return j.jp.GetReserveCount()
}
func (j *wrapJob) IncReserveCount() {
	j.jp.ReserveCount++
}
func (j *wrapJob) TimeoutCount() uint32 {
	return j.jp.GetTimeoutCount()
}
func (j *wrapJob) IncTimeoutCount() {
	j.jp.TimeoutCount++
}
func (j *wrapJob) ReleaseCount() uint32 {
	return j.jp.GetReleaseCount()
}
func (j *wrapJob) IncReleaseCount() {
	j.jp.ReleaseCount++
}
func (j *wrapJob) BuryCount() uint32 {
	return j.jp.GetBuryCount()
}
func (j *wrapJob) IncBuryCount() {
	j.jp.BuryCount++
}
func (j *wrapJob) KickCount() uint32 {
	return j.jp.GetKickCount()
}
func (j *wrapJob) IncKickCount() {
	j.jp.KickCount++
}
