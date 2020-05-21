package cmd

import (
	"fmt"
	"github.com/1xyz/coolbeans/cluster/client"
	"github.com/1xyz/coolbeans/tools"
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

func CmdClusterClient(argv []string, version string) {
	usage := `
Usage:
    cluster-client is_leader [--node-addr=<addr>] [--timeout=<secs>]
   
options:
    -h, --help
    --node-addr=<addr>   Address of a cluster node [default: 127.0.0.1:11000].
    --timeout=<secs>     Connect timeout in seconds [default: 30]
`
	opts, err := docopt.ParseArgs(usage, argv[1:], version)
	if err != nil {
		log.Fatalf("CmdClusterClient: error parsing arguments. err=%v", err)
	}

	ldr := tools.OptsBool(opts, "is_leader")
	if ldr {
		cmdLeader(
			tools.OptsStr(opts, "--node-addr"),
			tools.OptsSeconds(opts, "--timeout"))
		return
	}
	log.Infof("Unknown command")
}

func cmdLeader(nodeAddr string, timeout time.Duration) {
	c, err := client.NewClusterNodeClient(nodeAddr, timeout)
	if err != nil {
		log.Fatalf("cmdLeader: err = %v", err)
	}
	defer c.Close()

	b, err := c.IsNodeLeader()
	if err != nil {
		log.Fatalf("cmdLeader err %v", err)
	}
	fmt.Printf("isNodeLeader: %v\n", b)
	if !b {
		os.Exit(1)
	}
}
