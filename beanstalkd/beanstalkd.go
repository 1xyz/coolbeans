package beanstalkd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/1xyz/coolbeans/beanstalkd/proto"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ListenAddr     string
	ListenPort     int
	JsmAddrs       string
	ConnectTimeout int
}

func (c Config) String() string {
	return fmt.Sprintf("Listen Addr=%v, Port=%v JSMAddresses=%v ConnectTimeout=%v",
		c.ListenAddr, c.ListenPort, c.JsmAddrs, c.ConnectTimeout)
}

// runTCPServer - creates and run a beanstalkd TCP server to
// listen on the configured addr/port. The server listens on
// a separate go-routine and return back to caller.
// Refer method: waitForShutdown,
func runTCPServer(c *Config) *proto.TcpServer {
	addr := fmt.Sprintf("%s:%v", c.ListenAddr, c.ListenPort)
	tcpServer := proto.NewTcpServer(addr, c.JsmAddrs, time.Duration(c.ConnectTimeout)*time.Second)
	go func(tcpSrv *proto.TcpServer) {
		if err := tcpSrv.Listen(); err != nil {
			log.Errorf("tcpServer.listen err=%v", err)
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
	log.Infof("Shutdown signal received")
	tcpSrv.Shutdown()
}

func RunBeanstalkd(c *Config) error {
	log.Debugf("Loaded Config: %v", c)
	// CPU profiling by default
	// defer profile.Start().Stop()

	tcpSrv := runTCPServer(c)
	waitForShutdown(tcpSrv)
	return nil
}
