package proto

import (
	"errors"
	"fmt"
	"github.com/1xyz/coolbeans/beanstalkd/core"
	"github.com/1xyz/coolbeans/beanstalkd/proxy"
	"github.com/1xyz/coolbeans/state"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

type TcpServer struct {
	// Address:Port to open the connection
	address string

	// Command processor reference
	cmdProc core.CommandProcessor

	// bool to signal stopping the server
	doneCh chan bool

	// waitgroup to wait for shutdown
	shutdownWG *sync.WaitGroup
}

func NewJSM(jsmAddrs string) state.JSM {
	if len(jsmAddrs) == 0 {
		jsm, err := state.NewJSM()
		if err != nil {
			log.Panicf("CommandProcessor NewJSM() err=%v", err)
		}

		return jsm
	}

	nc := proxy.NewClient(uuid.New().URN(), []string{jsmAddrs}, 10*time.Second)
	if err := nc.Open(); err != nil {
		log.Panicf("proxyClient.Open(..). err=%v", err)
	}

	return nc
}

func NewTcpServer(address string, jsmAddrs string) *TcpServer {
	jsm := NewJSM(jsmAddrs)

	return &TcpServer{
		address:    address,
		cmdProc:    core.NewCommandProcess(jsm),
		doneCh:     make(chan bool),
		shutdownWG: &sync.WaitGroup{},
	}
}

const deadline = time.Second

func (srv *TcpServer) Listen() error {
	srv.shutdownWG.Add(1)
	ctxLog := log.WithFields(log.Fields{"method": "TcpServer.Listen", "addr": srv.address})
	tcpAddr, err := net.ResolveTCPAddr("tcp", srv.address)
	if err != nil {
		return fmt.Errorf("resolveTcpAddr failed %v", err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("creating listener failed %v", err)
	}
	ctxLog.Debugf("Listener started")
	go srv.cmdProc.Run()
	for {
		select {
		// check to see if the doneCh has been signalled
		case <-srv.doneCh:
			if err := listener.Close(); err != nil {
				log.Errorf("listener.close err=%v", err)
			}
			srv.cmdProc.Shutdown()
			log.Infof("waiting for server shutdown")
			srv.shutdownWG.Done()
			return nil
		default:
			// Nothing to do here
		}

		// set the deadline for the TCP listener; forces accept to timeout
		if err := listener.SetDeadline(time.Now().Add(deadline)); err != nil {
			return fmt.Errorf("setDeadline err=%v", err)
		}

		conn, err := listener.Accept()
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Timeout() {
			continue
		} else if err != nil {
			ctxLog.Errorf("listener.Accept err %v", err)
			continue
		}

		clientReg := srv.cmdProc.RegisterClient()
		if clientReg.Error != nil {
			ctxLog.Errorf("Unable to register client %v", clientReg.Error)
			if err := conn.Close(); err != nil {
				ctxLog.Errorf("conn.close err=%v", err)
			}
			continue
		}

		c := NewConn(conn, clientReg, srv.cmdProc)
		ctxLog.Debugf("Connected a new client connection %v", c)
		go c.Tick()
	}
}

func (srv *TcpServer) Shutdown() {
	srv.doneCh <- true
	close(srv.doneCh)
	srv.shutdownWG.Wait()
}
