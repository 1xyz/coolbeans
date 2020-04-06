package cmd

import (
	"github.com/1xyz/coolbeans/beanstalkd"
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
)

func cmdBeanstalkd(argv []string, version string) {
	usage := `usage: beanstalkd [-l=<addr> | --listen-addr=<addr>] [-p=<port> | --listen-port=<port>]
Options:
  -h --help                          Show this screen.
  -l=<address> --listen-addr=<addr>  Listen on address [default: 0.0.0.0].
  -p=<port> --listen-port=<port>     Listen on port [default:11300].
`
	opts, err := docopt.ParseArgs(usage, argv[1:], version)
	if err != nil {
		log.Fatalf("error parsing arguments. err=%v", err)
	}

	var bsConfig beanstalkd.Config
	if err := opts.Bind(&bsConfig); err != nil {
		log.Fatalf("error in opts.bind. err=%v", err)
	}

	if err := beanstalkd.RunBeanstalkd(&bsConfig); err != nil {
		log.Fatalf("error RunBeanstalkd. err=%v", err)
	}
}
