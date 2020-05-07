package cmd

import (
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
)

func CmdBeanstalkd(argv []string, version string) {
	usage := `usage: beanstalkd [-l=<addr> | --listen-addr=<addr>] [-p=<port> | --listen-port=<port>] [--jsm-addrs=<addrs>] [-t=<timeout> | --connectTimeout=<timeout>]
Options:
  -h --help                                  Show this screen.
  -l=<address> --listen-addr=<addr>          Listen on address [default: 0.0.0.0].
  -p=<port> --listen-port=<port>             Listen on port [default:11300]
  -j=<addrs> --jsm-addrs=<addrs>             Connect to JSM servers, local if ignored [default: ].
  -t=<timeout> --connect-timeout=<timeout>   Connection timeout in seconds [default: 10].
`
	opts, err := docopt.ParseArgs(usage, argv[1:], version)
	if err != nil {
		log.Fatalf("error parsing arguments. err=%v", err)
	}

	var bsConfig Config
	if err := opts.Bind(&bsConfig); err != nil {
		log.Fatalf("error in opts.bind. err=%v", err)
	}

	if err := RunBeanstalkd(&bsConfig); err != nil {
		log.Fatalf("error RunBeanstalkd. err=%v", err)
	}
}
