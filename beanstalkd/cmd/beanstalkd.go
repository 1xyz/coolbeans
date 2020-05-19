package cmd

import (
	"github.com/1xyz/coolbeans/beanstalkd/core"
	"github.com/1xyz/coolbeans/beanstalkd/proto"
	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

// runTCPServer - creates and run a beanstalkd TCP server to
// listen on the configured addr/port. The server listens on
// a separate go-routine and return back to caller.
// Refer method: waitForShutdown,
func runTCPServer(c *core.Config) *proto.TcpServer {
	tcpServer := proto.NewTcpServer(c)
	go func(tcpSrv *proto.TcpServer) {
		if err := tcpSrv.Listen(); err != nil {
			log.Errorf("runTCPServer: tcpServer.listen err=%v", err)
		}
	}(tcpServer)
	return tcpServer
}

// waitForShutdown waits for a terminate or interrupt signal
// terminates the server once a signal is received.
func waitForShutdown(tcpSrv *proto.TcpServer) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-done
	log.Infof("waitForShutdown: Shutdown signal received")
	tcpSrv.Shutdown()
}

func RunBeanstalkd(c *core.Config) error {
	spew.Dump(c)
	tcpSrv := runTCPServer(c)
	waitForShutdown(tcpSrv)
	return nil
}
