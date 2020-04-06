package main

import (
	"fmt"
	command "github.com/1xyz/coolbeans/cmd"
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

	command.RunCommand(cmd, cmdArgs, version)
	log.Infof("done")
}
