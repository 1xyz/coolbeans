package core

import (
	"fmt"
	"github.com/1xyz/coolbeans/state"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"time"
)

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
}

func NewCommandProcess(jsm state.JSM) CommandProcessor {
	return &cmdProcessor{
		newClientReqCh:  make(chan bool),
		newClientRespCh: make(chan ClientReg),
		clients:         make(ClientSet),
		cmdRequestCh:    make(chan CmdRequest),
		ticker:          time.NewTicker(TickDuration),
		jsm:             jsm,
		shutdownCh:      make(chan bool),
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
}

func (c *cmdProcessor) Run() {
	logc := log.WithFields(log.Fields{"method": "cmdProcessor.Run"})
	for {
		select {
		case <-c.newClientReqCh:
			logc.Debugf("Recv from newClientReqCh")
			nc := NewClient(defaultTubeName)
			err := c.clients.Set(nc)
			if err != nil {
				logc.Debugf("error c.client.Set(%v) err=%v", nc, err)
			}
			c.newClientRespCh <- ClientReg{
				ID:         nc.id,
				ResponseCh: nc.responseCh,
				Error:      err,
			}

		case cmdRequest := <-c.cmdRequestCh:
			logc.Debugf("Recv cmdRequest %v from cmdRequestCh", cmdRequest)
			c.processRequest(&cmdRequest)

		case <-c.ticker.C:
			now := nowSeconds()
			resv, err := c.jsm.Tick(now)
			if err != nil {
				logc.Errorf("error = %v", err)
			} else {
				for _, r := range resv {
					logc.Infof("reservation = %v", r)
					switch r.Status {
					case state.Matched:
						cli := c.clients[r.ClientId]
						// send the header following by the body
						sendCmdResponse(r.RequestId, cli,
							[]byte(fmt.Sprintf("RESERVED %d %d", r.JobId, r.BodySize)), true)
						sendCmdResponse(r.RequestId, cli, r.Body, false)

					case state.Timeout:
						sendCmdResponse(r.RequestId, c.clients[r.ClientId], []byte(MsgTimedOut), false)
					case state.DeadlineSoon:
						sendCmdResponse(r.RequestId, c.clients[r.ClientId], []byte(MsgDeadlineSoon), false)
					case state.Error:
						sendCmdResponse(r.RequestId, c.clients[r.ClientId], []byte(MsgInternalError), false)
					default:
						logc.Errorf("Unsupported reservation status=%v", r.Status)
					}
				}
			}

		case <-c.shutdownCh:
			logc.Infof("shutdown signal for command processor")
			c.ticker.Stop()
			close(c.newClientRespCh)
			return
		}
	}
}

// shutdown this cmdProcessor
func (c *cmdProcessor) Shutdown() {
	log.WithFields(log.Fields{"method": "cmdProcessor.Shutdown"}).
		Infof("Shutdown cmdProcessor")
	c.shutdownCh <- true
	close(c.shutdownCh)
	close(c.newClientReqCh)
	// ToDo: cannot safely close cmdRequestCh. We have to keep track
	// of individual conn objects and close when conn object count is
	// zero.
	// close(c.cmdRequestCh)
}

func (c *cmdProcessor) RegisterClient() *ClientReg {
	c.newClientReqCh <- true
	resp, ok := <-c.newClientRespCh
	if !ok {
		panic("client response channel is closed")
	}

	return &resp
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
	case Delete:
		resp = c.delete(cli, req)
	case Ignore:
		resp = c.ignore(cli, req)
	case Put:
		resp = c.put(cli, req)
	case Reserve:
		c.reserve(cli, req)
	case ReserveWithTimeout:
		resp = c.reserveWithTimeout(cli, req)
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
	for name, _ := range cli.watchingTubes {
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

	if cmd.timeoutSeconds > MaxReservationTimeout {
		resp := NewCmdResponseFromReq(req)
		resp.setResponse(MsgBadFormat)
		return resp
	}

	c.appendReservation(cli, req.ID, nowSeconds(), addToNow(cmd.timeoutSeconds))
	return nil
}

func (c *cmdProcessor) reserve(cli *client, req *CmdRequest) {
	c.appendReservation(cli, req.ID, nowSeconds(), addToNow(MaxReservationTimeout))
}

func (c *cmdProcessor) appendReservation(cli *client, reqID string, nowSecs, deadlineAt int64) {
	watchedTubes := make([]state.TubeName, 0)
	for t, _ := range cli.watchingTubes {
		watchedTubes = append(watchedTubes, t)
	}
	resv, err := c.jsm.AppendReservation(cli.id, reqID, watchedTubes, nowSecs, deadlineAt)
	if err != nil {
		log.WithField("method", "appendReservation").Panicf("error %v", err)
	} else if resv.Status == state.Matched {
		// reply to the client job metadata
		cli.responseCh <- CmdResponse{
			RequestID: resv.RequestId,
			ClientID:  cli.id,
			Response:  []byte(fmt.Sprintf("RESERVED %d %d", resv.JobId, resv.BodySize)),
			HasMore:   true,
		}
		// reply to the client job body
		cli.responseCh <- CmdResponse{
			RequestID: resv.RequestId,
			ClientID:  cli.id,
			Response:  resv.Body,
			HasMore:   false,
		}
	} else {
		log.WithField("method", "appendReservation").Infof("reservation status = %v", resv.Status)
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

// Encapsulates a Client Registration information
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

type CmdRequest struct {
	ID       string
	ClientID state.ClientID
	CmdType  CmdType
	cmd      interface{}
}

func (req CmdRequest) String() string {
	return fmt.Sprintf("CmdRequest (ID: %v, CmdType: %s, ClientID: %s)",
		req.ID, req.CmdType, req.ClientID)
}

func NewCmdRequest(cmdData *CmdData, clientID state.ClientID) (CmdRequest, error) {
	cmdRequest := CmdRequest{
		ID:       uuid.New().URN(),
		ClientID: clientID,
		CmdType:  cmdData.CmdType,
		cmd:      nil,
	}

	var err error = nil
	switch cmdData.CmdType {
	case Delete:
		cmdRequest.cmd, err = NewIDArg(cmdData)
	case Put:
		cmdRequest.cmd, err = NewPutArg(cmdData)
	case Ignore, Use, Watch:
		cmdRequest.cmd, err = NewTubeArg(cmdData)
	case ReserveWithTimeout:
		cmdRequest.cmd, err = NewReserveWithTimeoutArg(cmdData)
	case Quit, Reserve:
	default:
		err = ErrCmdNotFound
	}

	return cmdRequest, err
}

type CmdResponse struct {
	RequestID string
	ClientID  state.ClientID
	Response  []byte
	HasMore   bool
}

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
