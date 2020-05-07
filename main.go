package main

import (
	"fmt"
	cmd_beanstalkd "github.com/1xyz/coolbeans/beanstalkd/cmd"
	cmd_cluster "github.com/1xyz/coolbeans/cluster/cmd"
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
	"os"
)

const version = "0.1.alpha"

func init() {
	log.SetFormatter(&log.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	usage := `usage: coolbeans [--version] [--verbose] [--help]
           <command> [<args>...]
options:
   -h, --help
   --verbose      Change the logging level verbosity
The commands are:
   cluster-node   Run a cluster node server
   beanstalkd     Run a beanstalkd server
See 'coolbeans <command> --help' for more information on a specific command.
`
	parser := &docopt.Parser{OptionsFirst: true}
	args, err := parser.ParseArgs(usage, nil, version)
	if err != nil {
		log.Errorf("error = %v", err)
		os.Exit(1)
	}

	cmd := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	fmt.Println("global arguments:", args)
	fmt.Println("command arguments:", cmd, cmdArgs)

	if verbose, err := args.Bool("--verbose"); err != nil {
		log.Fatalf("error parsing verbosity %v", err)
	} else if verbose == true {
		log.SetLevel(log.DebugLevel)
	}

	RunCommand(cmd, cmdArgs, version)
	log.Infof("done")
}

func RunCommand(c string, args []string, version string) {
	argv := append([]string{c}, args...)
	switch c {
	case "cluster-node":
		cmd_cluster.CmdClusterNode(argv, version)
	case "beanstalkd":
		cmd_beanstalkd.CmdBeanstalkd(argv, version)
	default:
		log.Fatalf("RunCommand: %s is not a supported command. See 'coolbeans help'", c)
	}
}
