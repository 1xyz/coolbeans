package core

import (
	"fmt"
	"github.com/1xyz/coolbeans/state"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"time"
)

// CommandProcessor manages command processing for a the beanstalkd service
type CommandProcessor interface {
	// Register a new client with this system
	// Returns an ID and error
	RegisterClient() *ClientReg

	// Dispatch this request to the command process
	DispatchRequest(request CmdRequest)

	// Run this processor
	Run()

	// Shutdown this processor
	Shutdown()

	// Return the maximum job data size in Bytes
	MaxJobDataSize() int
}

// NewCommandProcessor returns an instance of CommandProcessor for the
// specified job state machine & config.
func NewCommandProcessor(jsm state.JSM, cfg *Config) CommandProcessor {
	return &cmdProcessor{
		newClientReqCh:        make(chan bool),
		newClientRespCh:       make(chan ClientReg),
		clients:               make(ClientSet),
		cmdRequestCh:          make(chan CmdRequest),
		ticker:                time.NewTicker(TickDuration),
		jsm:                   jsm,
		shutdownCh:            make(chan bool),
		maxJobSizeBytes:       cfg.MaxJobSize,
		maxReserveTimeoutSecs: cfg.MaxReservationTimeout,
	}
}

// Routine that processes commands
type cmdProcessor struct {
	// channel to send a request for a new client
	newClientReqCh chan bool

	// channel to send response for a new client request
	newClientRespCh chan ClientReg

	// channel to send a new Command Req
	cmdRequestCh chan CmdRequest

	// Clients set is a set of clients currently registered
	clients ClientSet

	// a ticker raised to invoke tickXXX methods to be called
	ticker *time.Ticker

	// Reference to the job state machine
	jsm state.JSM

	// channel to signal a shutdown
	shutdownCh chan bool

	// Maximum job size in bytes
	maxJobSizeBytes int

	// Maximum reservation timeout in seconds.
	maxReserveTimeoutSecs int
}

// Run this command processor
func (c *cmdProcessor) Run() {
	for {
		select {
		case <-c.newClientReqCh:
			nc := NewClient(defaultTubeName)
			err := c.clients.Set(nc)
			if err != nil {
				log.Debugf("cmdProcessor.Run: error c.client.Set(%v) err=%v", nc, err)
			}

			c.newClientRespCh <- ClientReg{
				ID:         nc.id,
				ResponseCh: nc.responseCh,
				Error:      err,
			}

		case cmdRequest := <-c.cmdRequestCh:
			c.processRequest(&cmdRequest)

		case <-c.ticker.C:
			now := nowSeconds()
			resv, err := c.jsm.Tick(now)
			if err != nil {
				log.Errorf("cmdProcessor.Run: c.jsm.Tick. error = %v", err)
			} else {
				for _, r := range resv {
					c.replyReservation(r, c.clients[r.ClientId], r.RequestId)
				}
			}

		case <-c.shutdownCh:
			log.Infof("cmdProcessor.Run: shutdown signal for command processor")
			c.ticker.Stop()
			close(c.newClientRespCh)
			return
		}
	}
}

// Shutdown this cmdProcessor
func (c *cmdProcessor) Shutdown() {
	log.Infof("cmdProcessor.Shutdown: shutdown signalled")
	c.shutdownCh <- true
	close(c.shutdownCh)
	close(c.newClientReqCh)
	if err := c.jsm.Stop(); err != nil {
		log.Errorf("cmdProcessor.Shutdown:  c.jsm.Stop() err = %v", err)
	}
}

func (c *cmdProcessor) RegisterClient() *ClientReg {
	c.newClientReqCh <- true
	resp, ok := <-c.newClientRespCh
	if !ok {
		panic("client response channel is closed")
	}

	return &resp
}

func (c *cmdProcessor) MaxJobDataSize() int {
	return c.maxJobSizeBytes
}

func (c *cmdProcessor) DispatchRequest(request CmdRequest) {
	log.WithFields(log.Fields{
		"method":     "cmdProcessor.DispatchRequest",
		"cmdRequest": request}).Debugf("Send cmdRequest to cmdRequestCh")
	c.cmdRequestCh <- request
}

func (c *cmdProcessor) processRequest(req *CmdRequest) {
	ctxLog := logCtx(req, "cmdProcessor.processRequest")
	// Lookup the client associated with this connection (clientID)
	cli, err := c.clients.Find(req.ClientID)
	if err != nil {
		// Note: since a client is associated with a tcp connection
		// it should be active as long as the TCP connection exists
		ctxLog.Panicf("c.clients.Find(%v) error %v", req.ClientID, err)
	}

	// Execute the command based on the commandType
	var resp *CmdResponse = nil
	closeResp := false
	switch req.CmdType {
	case Bury:
		resp = c.bury(cli, req)
	case Delete:
		resp = c.delete(cli, req)
	case Ignore:
		resp = c.ignore(cli, req)
	case Kick:
		resp = c.kickN(cli, req)
	case KickJob:
		resp = c.kick(cli, req)
	case ListTubes:
		resp = c.listTubes(cli, req)
	case ListTubesWatched:
		resp = c.listTubesWatched(cli, req)
	case ListTubeUsed:
		resp = c.listTubeUsed(cli, req)
	case Peek:
		resp = c.peek(cli, req)
	case PeekBuried:
		resp = c.callPeekFunc(c.jsm.PeekBuriedJob, cli, req)
	case PeekDelayed:
		resp = c.callPeekFunc(c.jsm.PeekDelayedJob, cli, req)
	case PeekReady:
		resp = c.callPeekFunc(c.jsm.PeekReadyJob, cli, req)
	case Put:
		resp = c.put(cli, req)
	case Release:
		resp = c.releaseWith(cli, req)
	case Reserve:
		c.reserve(cli, req)
	case ReserveWithTimeout:
		resp = c.reserveWithTimeout(cli, req)
	case StatsJob:
		resp = c.statsJob(cli, req)
	case StatsTube:
		resp = c.statsTube(cli, req)
	case Stats:
		resp = c.stats(cli, req)
	case Touch:
		resp = c.touch(cli, req)
	case Use:
		resp = c.use(cli, req)
	case Watch:
		resp = c.watch(cli, req)
	case Quit:
		resp = c.quit(cli, req)
		closeResp = true
	default:
		resp = NewCmdResponseFromReq(req)
		resp.setResponse(MsgUnknownCommand)
	}

	if resp != nil {
		cli.responseCh <- *resp
	}

	if closeResp {
		ctxLog.Debugf("closing responseChannel")
		close(cli.responseCh)
	}
}

func (c *cmdProcessor) put(cli *client, req *CmdRequest) *CmdResponse {
	logc := logCtx(req, "cmdProcessor.put")
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*putArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		logc.Panicf("cast-error, cannot cast to *putCmd")
	}

	newJobID, err := c.jsm.Put(nowSeconds(), cmd.pri, cmd.delay, cmd.ttr, cmd.size, cmd.data, cli.useTube)
	if err != nil {
		logc.Errorf("c.jsm.Put(...) err=%v", err)
		resp.setResponse(MsgInternalError)
	} else {
		logc.Debugf("created job with id=%v", newJobID)
		resp.setResponse(fmt.Sprintf("INSERTED %d", newJobID))
	}

	return resp
}

func (c *cmdProcessor) bury(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*buryArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.bury: cast-error, cannot cast to *buryArg")
	}

	err := c.jsm.Bury(nowSeconds(), cmd.id, cmd.pri, cli.id)
	if err != nil {
		resp.setResponse(MsgNotFound)
	} else {
		resp.setResponse("BURIED")
	}

	return resp
}

func (c *cmdProcessor) delete(cli *client, req *CmdRequest) *CmdResponse {
	ctxLog := logCtx(req, "cmdProcessor.delete")
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*idArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		ctxLog.Panicf("cast-error, cannot cast to *idArg")
	}

	err := c.jsm.Delete(cmd.id, cli.id)
	if err != nil {
		resp.setResponse(MsgNotFound)
	} else {
		resp.setResponse("DELETED")
	}

	return resp
}

func (c *cmdProcessor) ignore(cli *client, req *CmdRequest) *CmdResponse {
	ctxLog := logCtx(req, "cmdProcessor.ignore")
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*tubeArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		ctxLog.Panicf("cast-error, cannot cast to *tubeArg")
	}

	ctxLog.Infof("ignore req %v", cmd.tubeName)

	if cli.watchingTubes.Len() > 1 {
		if err := cli.watchingTubes.Remove(cmd.tubeName); err != nil && err != state.ErrEntryMissing {
			ctxLog.Errorf("cli.watchingTubes.remove(%v) err %v", cmd.tubeName, err)
			resp.setResponse(MsgInternalError)
		} else {
			resp.setResponse(fmt.Sprintf("WATCHING %d", cli.watchingTubes.Len()))
		}
	} else {
		resp.setResponse(MsgCannotIgnoreTube)
	}

	return resp
}

func (c *cmdProcessor) kick(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*idArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.kick: cast-error, cannot cast to *idArg")
	}

	err := c.jsm.Kick(cmd.id)
	if err != nil {
		resp.setResponse(MsgNotFound)
	} else {
		resp.setResponse("KICKED")
	}

	return resp
}

func (c *cmdProcessor) kickN(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*kickNArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.kickN: cast-error, cannot cast to *kickNArg")
	}

	n, err := c.jsm.KickN(cli.useTube, cmd.bound)
	if err != nil {
		resp.setResponse(MsgInternalError)
	} else {
		resp.setResponse(fmt.Sprintf("KICKED %d", n))
	}

	return resp
}

func (c *cmdProcessor) listTubes(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	tubes, err := c.jsm.GetTubes()
	if err != nil {
		resp.setResponse(MsgInternalError)
		return resp
	}

	b, err := yaml.Marshal(tubes)
	if err != nil {
		log.Errorf("cmdProcessor.listTubes: yaml.Marshal err = %v", err)
		resp.setResponse(MsgInternalError)
		return resp
	}

	sendYamlResponse(b, req, cli)
	return nil
}

func (c *cmdProcessor) listTubesWatched(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)

	watchedTubes := make([]state.TubeName, 0)
	for t := range cli.watchingTubes {
		watchedTubes = append(watchedTubes, t)
	}

	b, err := yaml.Marshal(watchedTubes)
	if err != nil {
		log.Errorf("cmdProcessor.listTubesWatched: yaml.Marshal err = %v", err)
		resp.setResponse(MsgInternalError)
		return resp
	}

	sendYamlResponse(b, req, cli)
	return nil
}

func (c *cmdProcessor) listTubeUsed(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)

	resp.setResponse(fmt.Sprintf("USING %s", cli.useTube))
	return resp
}

func (c *cmdProcessor) peek(cli *client, req *CmdRequest) *CmdResponse {
	cmd, ok := req.cmd.(*idArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.peek: cast-error, cannot cast to *idArg")
	}

	j, err := c.jsm.GetJob(cmd.id)
	return replyPeek(j, err, cli, req)
}

type peekTube func(name state.TubeName) (state.Job, error)

func (c *cmdProcessor) callPeekFunc(pf peekTube, cli *client, req *CmdRequest) *CmdResponse {
	j, err := pf(cli.useTube)
	return replyPeek(j, err, cli, req)
}

type processFunc func(*client, *CmdRequest) *CmdResponse

func peekTubeFunc(pf peekTube) processFunc {
	return func(cli *client, req *CmdRequest) *CmdResponse {
		j, err := pf(cli.useTube)
		return replyPeek(j, err, cli, req)
	}
}

func replyPeek(j state.Job, err error, cli *client, req *CmdRequest) *CmdResponse {
	if err != nil {
		log.Errorf("cmdProcessor.replyPeek: err=%v", err)
		resp := NewCmdResponseFromReq(req)
		resp.setResponse(MsgNotFound)
		return resp
	}

	s := fmt.Sprintf("FOUND %d %d", j.ID(), j.BodySize())
	sendCmdResponse(req.ID, cli, []byte(s), true /*hasMore*/)
	sendCmdResponse(req.ID, cli, j.Body(), false /*hasMore*/)
	return nil
}

func (c *cmdProcessor) releaseWith(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*releaseArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.kickN: cast-error, cannot cast to *releaseArg")
	}

	err := c.jsm.ReleaseWith(nowSeconds(), cmd.id, cli.id, cmd.pri, cmd.delay)
	if err != nil {
		resp.setResponse(MsgNotFound)
	} else {
		resp.setResponse("RELEASED")
	}

	return resp
}

func (c *cmdProcessor) statsJob(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*idArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.statsJob: cast-error, cannot cast to *idArg")
	}

	b, err := c.jsm.GetStatsJobAsYaml(nowSeconds(), cmd.id)
	if err != nil {
		resp.setResponse(MsgNotFound)
		return resp
	}

	sendYamlResponse(b, req, cli)
	return nil
}

func (c *cmdProcessor) statsTube(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*tubeArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.statsTube: cast-error, cannot cast to *tubeArg")
	}

	b, err := c.jsm.GetStatsTubeAsYaml(nowSeconds(), cmd.tubeName)
	if err != nil {
		resp.setResponse(MsgNotFound)
		return resp
	}

	sendYamlResponse(b, req, cli)
	return nil
}

func (c *cmdProcessor) stats(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	b, err := c.jsm.GetStatsAsYaml(nowSeconds())
	if err != nil {
		resp.setResponse(MsgNotFound)
		return resp
	}

	sendYamlResponse(b, req, cli)
	return nil
}

var yamlHdr = []byte{'-', '-', '-', '\n'}

func sendYamlResponse(b []byte, req *CmdRequest, cli *client) {
	respBytes := make([]byte, 0)
	respBytes = append(respBytes, yamlHdr...)
	respBytes = append(respBytes, b...)
	s := fmt.Sprintf("OK %d", len(respBytes))
	sendCmdResponse(req.ID, cli, []byte(s), true /*hasMore*/)
	sendCmdResponse(req.ID, cli, respBytes, false)
}

func (c *cmdProcessor) touch(cli *client, req *CmdRequest) *CmdResponse {
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*idArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		log.Panicf("cmdProcessor.touch: cast-error, cannot cast to *idArg")
	}

	err := c.jsm.Touch(nowSeconds(), cmd.id, req.ClientID)
	if err != nil {
		resp.setResponse(MsgNotFound)
	} else {
		resp.setResponse("TOUCHED")
	}

	return resp
}

func (c *cmdProcessor) use(cli *client, req *CmdRequest) *CmdResponse {
	ctxLog := logCtx(req, "cmdProcessor.use")
	cmd, ok := req.cmd.(*tubeArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		ctxLog.Panicf("cast-error, cannot cast to *tubeCmd")
	}

	resp := NewCmdResponseFromReq(req)
	cli.useTube = cmd.tubeName
	resp.setResponse(fmt.Sprintf("USING %s", cli.useTube))
	return resp
}

func (c *cmdProcessor) watch(cli *client, req *CmdRequest) *CmdResponse {
	ctxLog := logCtx(req, "cmdProcessor.watch")
	resp := NewCmdResponseFromReq(req)
	cmd, ok := req.cmd.(*tubeArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		ctxLog.Panicf("cast-error, cannot cast to *tubeCmd")
	}

	// Add this tube to the client's watch list
	err := cli.watchingTubes.Set(cmd.tubeName)
	if err != nil && err != state.ErrEntryExists {
		ctxLog.Errorf("cli.watchingTubes.Set(%s) err=%v", cmd.tubeName, err)
		resp.setResponse(MsgInternalError)
	}

	ctxLog.Debugf("watching tube=%v count=%d", cmd.tubeName, cli.watchingTubes.Len())
	resp.setResponse(fmt.Sprintf("WATCHING %d", cli.watchingTubes.Len()))
	return resp
}

func (c *cmdProcessor) quit(cli *client, req *CmdRequest) *CmdResponse {
	ctxLog := logCtx(req, "cmdProcessor.quit")
	ctxLog.Debugf("Remove client")
	err := c.clients.Remove(cli)
	if err != nil {
		// we expect that the client to be removed exactly once
		ctxLog.Panicf("c.cliente.Remove error=%v", err)
	}
	for name := range cli.watchingTubes {
		if err := cli.watchingTubes.Remove(name); err != nil {
			ctxLog.Errorf("cli.watchingTubes.Remove(%s) err=%v", name, err)
		}
		cli.watchingTubes = nil
	}

	return NewCmdResponseFromReq(req)
}

func (c *cmdProcessor) reserveWithTimeout(cli *client, req *CmdRequest) *CmdResponse {
	ctxLog := logCtx(req, "cmdProcessor.reserveWithTimeout")
	cmd, ok := req.cmd.(*reserveWithTimeoutArg)
	if !ok {
		// Note: this is indicative of code-bug where the CmdType and cmd don't match up
		ctxLog.Panicf("cast-error, cannot cast to *reserveWithTimeoutCmd")
	}

	if cmd.timeoutSeconds > c.maxReserveTimeoutSecs {
		log.Errorf("reserveWithTimeout: cmd.timeoutSecs=%d exceeds limit of %d",
			cmd.timeoutSeconds, c.maxReserveTimeoutSecs)
		resp := NewCmdResponseFromReq(req)
		resp.setResponse(MsgBadFormat)
		return resp
	}

	c.appendReservation(cli, req.ID, nowSeconds(), addToNow(cmd.timeoutSeconds))
	return nil
}

func (c *cmdProcessor) reserve(cli *client, req *CmdRequest) {
	c.appendReservation(cli, req.ID, nowSeconds(), addToNow(c.maxReserveTimeoutSecs))
}

func (c *cmdProcessor) appendReservation(cli *client, reqID string, nowSecs, deadlineAt int64) {
	watchedTubes := make([]state.TubeName, 0)
	for t := range cli.watchingTubes {
		watchedTubes = append(watchedTubes, t)
	}

	r, err := c.jsm.AppendReservation(cli.id, reqID, watchedTubes, nowSecs, deadlineAt)
	if err != nil {
		log.Errorf("cmdProcessor.appendReservation: c.jsm.AppendReservation cli.Id=%v err=%v",
			cli.id, err)
		c.replyReservation(nil, cli, reqID)
		return
	}

	c.replyReservation(r, cli, reqID)
}

func (c *cmdProcessor) replyReservation(r *state.Reservation, cli *client, reqID string) {
	if r == nil {
		log.Errorf("cmdProcessor.replyReservation: clientId=%v MsgInternalError", cli.id)
		sendCmdResponse(reqID, cli, []byte(MsgInternalError), false)
		return
	} else if r.Status == state.Matched {
		sendCmdResponse(r.RequestId, cli,
			[]byte(fmt.Sprintf("RESERVED %d %d", r.JobId, r.BodySize)), true)
		sendCmdResponse(r.RequestId, cli, r.Body, false)
	} else if r.Status == state.Timeout {
		sendCmdResponse(r.RequestId, cli, []byte(MsgTimedOut), false)
	} else if r.Status == state.DeadlineSoon {
		sendCmdResponse(r.RequestId, cli, []byte(MsgDeadlineSoon), false)
	} else if r.Status == state.Queued {
		log.Debugf("cmdProcessor.replyReservation: clientId=%v status=Queued. skip responding", cli.id)
	} else {
		log.Errorf("cmdProcessor.replyReservation: clientId=%v r.Status=%v MsgInternalError", cli.id, r.Status)
		sendCmdResponse(r.RequestId, c.clients[r.ClientId], []byte(MsgInternalError), false)
	}
}

func logCtx(req *CmdRequest, method string) *log.Entry {
	ctxLog := log.WithFields(log.Fields{
		"method":    method,
		"clientID":  req.ClientID,
		"CmdType":   req.CmdType,
		"requestID": req.ID})
	return ctxLog
}

// ClientReg encapsulates a Client Registration information
type ClientReg struct {
	// A unique identifier for this client
	ID state.ClientID

	// Responce channel on which all responses are sent to this client
	ResponseCh <-chan CmdResponse

	// Represents an error, encountered during registration
	Error error
}

func (cr ClientReg) String() string {
	return fmt.Sprintf("ClienReg ID:%v error:%v", cr.ID, cr.Error)
}

// CmdRequest represents a command requested by a client.
type CmdRequest struct {
	ID       string
	ClientID state.ClientID
	CmdType  CmdType
	cmd      interface{}
}

func (req CmdRequest) String() string {
	arg := ""
	if req.cmd != nil {
		if req.CmdType == Put {
			// Don't want to display the data bytes in put
			if parg, ok := req.cmd.(*putArg); ok {
				arg = fmt.Sprintf("[pri=%v delay=%v size=%v ttr=%v]", parg.pri, parg.delay, parg.size, parg.ttr)
			}
		} else {
			arg = fmt.Sprintf("[%+v]", req.cmd)
		}
	}
	return fmt.Sprintf("CmdRequest (ReqID: %v, CmdType: %s, Arg: %s ClientID: %s)",
		req.ID, req.CmdType, arg, req.ClientID)
}

// NewCmdRequest constructs a new CmdRequest from the provided information.
func NewCmdRequest(cmdData *CmdData, clientID state.ClientID) (CmdRequest, error) {
	cmdRequest := CmdRequest{
		ID:       uuid.New().URN(),
		ClientID: clientID,
		CmdType:  cmdData.CmdType,
		cmd:      nil,
	}

	var err error = nil
	switch cmdData.CmdType {
	case Bury:
		cmdRequest.cmd, err = NewBuryArg(cmdData)
	case Delete, KickJob:
		cmdRequest.cmd, err = NewIDArg(cmdData)
	case Kick:
		cmdRequest.cmd, err = NewKickNArg(cmdData)
	case Put:
		cmdRequest.cmd, err = NewPutArg(cmdData)
	case Peek:
		cmdRequest.cmd, err = NewIDArg(cmdData)
	case Ignore, Use, Watch:
		cmdRequest.cmd, err = NewTubeArg(cmdData)
	case Release:
		cmdRequest.cmd, err = NewReleaseArg(cmdData)
	case ReserveWithTimeout:
		cmdRequest.cmd, err = NewReserveWithTimeoutArg(cmdData)
	case Touch:
		cmdRequest.cmd, err = NewIDArg(cmdData)
	case StatsJob:
		cmdRequest.cmd, err = NewIDArg(cmdData)
	case StatsTube:
		cmdRequest.cmd, err = NewTubeArg(cmdData)
	case ListTubes, ListTubesWatched, ListTubeUsed, PeekReady, PeekDelayed, PeekBuried, Quit, Reserve, Stats:
	default:
		err = ErrCmdNotFound
	}

	return cmdRequest, err
}

// CmdResponse is the command response for the specific client
type CmdResponse struct {
	RequestID string
	ClientID  state.ClientID
	Response  []byte
	HasMore   bool
}

// NewCmdResponseFromReq creates a new response.
func NewCmdResponseFromReq(req *CmdRequest) *CmdResponse {
	return &CmdResponse{
		RequestID: req.ID,
		ClientID:  req.ClientID,
		Response:  nil,
		HasMore:   false,
	}
}

func (c *CmdResponse) setResponse(s string) {
	c.Response = []byte(s)
}

func sendCmdResponse(reqID string, cli *client, body []byte, hasMore bool) {
	cli.responseCh <- CmdResponse{
		RequestID: reqID,
		ClientID:  cli.id,
		Response:  body,
		HasMore:   hasMore,
	}
}
