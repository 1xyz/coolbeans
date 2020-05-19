package proto

import (
	"fmt"
	"github.com/1xyz/coolbeans/beanstalkd/core"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
)

// Encapsulates the connection (stream) with a client
type Conn struct {
	// clientRegistration refers to client's unique registration
	clientReg *core.ClientReg

	// Reference to the command processor to execute client commands
	cmdProcessor core.CommandProcessor

	// represents the underlying client network stream
	conn net.Conn

	// reader for the incoming connection
	reader io.Reader

	// Current state of this connection
	state ConnState

	// Buffer of un-processed bytes read from the connection/reader
	buffer []byte

	// Represents the last parsed commandData. Used primarily
	// in the case, when the commandData need additional data
	// Currently, only the PUT command works makes use of this.
	lastCmd *core.CmdData
}

func NewConn(conn net.Conn, clientReg *core.ClientReg, cmdProcessor core.CommandProcessor) *Conn {
	return &Conn{
		clientReg:    clientReg,
		cmdProcessor: cmdProcessor,
		conn:         conn,
		reader:       conn,
		state:        WantCommand,
		buffer:       make([]byte, 0, 1024),
		lastCmd:      nil,
	}
}

func (c *Conn) reply(s string) {
	c.replyBytes([]byte(s))
}

func (c *Conn) replyBytes(b []byte) {
	if n, err := c.conn.Write(b); err != nil {
		log.Warnf("conn.replyBytes: c.conn.Write. err = %v", err)
	} else if n != len(b) {
		log.Warnf("conn.replyBytes: n=%d != len(b) = %d", n, len(b))
	}
	if _, err := c.conn.Write([]byte("\r\n")); err != nil {
		log.Warnf("conn.replyBytes: c.conn.Write (newline). err = %v", err)
	}
}

func (c *Conn) dispatchCommand() {
	ctxLog := log.WithFields(log.Fields{
		"method":   "conn.dispatchCommand",
		"clientID": c.clientReg.ID})

	if c.lastCmd == nil {
		ctxLog.Panicf("inconsistent state: expected lastCmd to not be nil")
	}

	defer func() { c.lastCmd = nil }()

	if c.lastCmd.NeedData {
		ctxLog.Debugf("Command Data %v", string(c.lastCmd.Data))
	}

	req, err := core.NewCmdRequest(c.lastCmd, c.clientReg.ID)
	log.Infof("conn.dispatchCommand: CmdRequest %v", req)
	if err != nil {
		ctxLog.Errorf("core.NewCmdRequest err=%v", err)
		if err == core.ErrCmdNotFound {
			c.reply(core.MsgUnknownCommand)
		} else if err == core.ErrBadFormat {
			ctxLog.Errorf("Badformat")
			c.reply(core.MsgBadFormat)
		} else {
			c.reply(core.MsgInternalError)
		}
		return
	}

	ctxLog = ctxLog.WithField("requestID", req.ID)
	ctxLog.Debugf("Dispatching cmdRequest req %v", req)
	c.cmdProcessor.DispatchRequest(req)
	for {
		reply := <-c.clientReg.ResponseCh
		if reply.Response != nil {
			c.replyBytes(reply.Response)
		}

		if !reply.HasMore {
			break
		}
	}

	// Note we have to drain all messages and stop this SM
	if req.CmdType == core.Quit {
		for e := range c.clientReg.ResponseCh {
			ctxLog.Debugf("Discarding response %v", e)
		}
		c.state = Stopped
	}
}

func (c *Conn) close() {
	if c.state != Close {
		return
	}

	c.lastCmd = &core.CmdData{
		CmdType: core.Quit,
	}
	c.dispatchCommand()
}

func (c *Conn) stop() {
	if c.state != Stopped {
		return
	}

	log.WithField("method", "conn.stop").Debugf("closing connection")
	if err := c.conn.Close(); err != nil {
		log.Warnf("conn.stop: c.conn.close(): err = %v", err)
	}
}

func (c *Conn) wantEndLine() {
	ctxLog := log.WithField("method", "conn.wantEndLine")
	if c.state != WantEndLine {
		return
	}

	extraBytes, err := core.Discard(c.reader, c.buffer)
	c.buffer = extraBytes
	if err != nil {
		if err == io.EOF {
			c.state = Close
		} else {
			ctxLog.Errorf("internal-error %v", err)
			c.reply(core.MsgInternalError)
		}

		return
	}

	c.state = WantCommand
}

func (c *Conn) wantData() {
	if c.state != WantData {
		return
	}

	if c.lastCmd == nil {
		panic("inconsistent state: expected lastCmd to be nil")
	}

	ctxLog := log.WithField("method", "conn.wantData")
	dataBytes, extraBytes, err := core.ScanJobData(c.reader, c.buffer, c.cmdProcessor.MaxJobDataSize())
	c.buffer = extraBytes
	if err != nil {
		if err == io.EOF {
			ctxLog.Errorf("EOF detected")
			c.state = Close
		} else if err == core.ErrDelimiterMissing {
			ctxLog.Errorf("Delimiter not detected")
			c.reply(core.MsgExpectCRLF)
			c.state = WantEndLine
		} else {
			ctxLog.Errorf("internal-error %v", err)
			c.reply(core.MsgInternalError)
		}

		return
	}

	c.lastCmd.Data = dataBytes
	c.dispatchCommand()
	c.state = WantCommand
}

func (c *Conn) wantCommand() {
	if c.state != WantCommand {
		return
	}

	ctxLog := log.WithField("method", "conn.wantCommand")
	cmdBytes, extraBytes, err := core.ScanCmdLine(c.reader, c.buffer)
	c.buffer = extraBytes
	if err != nil {
		if err == io.EOF {
			ctxLog.Infof("EOF detected")
			c.state = Close
		} else if err == core.ErrDelimiterMissing {
			ctxLog.Errorf("Badformat")
			c.reply(core.MsgBadFormat)
			c.state = WantEndLine
		} else {
			ctxLog.Errorf("internal-error %v", err)
			c.reply(core.MsgInternalError)
		}

		return
	}

	cmdData, err := core.ParseCommandLine(string(cmdBytes),
		c.cmdProcessor.MaxJobDataSize())
	if err != nil {
		if err == core.ErrCmdNotFound || err == core.ErrCmdTokensMissing {
			ctxLog.Errorf("Badformat")
			c.reply(core.MsgBadFormat)
		} else if err == core.ErrJobSizeTooBig {
			ctxLog.Errorf("ErrJobSizeTooBig")
			c.reply(core.MsgJobTooBig)
			c.state = WantEndLine
		} else {
			ctxLog.Errorf("internal-error %v", err)
			c.reply(core.MsgInternalError)
		}

		return
	}

	c.lastCmd = cmdData
	if cmdData.NeedData {
		c.state = WantData
		return
	}

	ctxLog.Debugf("Cmddata = %v", cmdData)
	c.dispatchCommand()
}

func (c *Conn) Tick() {
	ctxLog := log.WithField("method", "conn.Tick")
	for c.state != Stopped {
		ctxLog.Debugf("current state = %v", c.state)
		c.wantCommand()
		c.wantData()
		c.wantEndLine()
		c.close()
		c.stop()
	}
}

func (c *Conn) String() string {
	return fmt.Sprintf("Client: %v State: %v conn.localAddr: %v conn.remoteAddr: %v",
		c.clientReg, c.state, c.conn.LocalAddr(), c.conn.RemoteAddr())
}
