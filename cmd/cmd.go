package cmd

import (
	log "github.com/sirupsen/logrus"
)

func RunCommand(c string, args []string, version string) {
	argv := append([]string{c}, args...)
	switch c {
	case "cluster-node":
		CmdClusterNode(argv, version)
	case "beanstalkd":
		cmdBeanstalkd(argv, version)
	default:
		log.Fatalf("%s is not a supported command. See 'coolbeans help'", c)
	}
}
