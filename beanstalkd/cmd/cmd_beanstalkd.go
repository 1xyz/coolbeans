package cmd

import (
	"github.com/1xyz/coolbeans/beanstalkd/core"
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
)

func CmdBeanstalkd(argv []string, version string) {
	usage := `usage: beanstalkd [options]
Options:
  -h --help                         Show this screen.
  --listen-addr=<addr>              Listen on address [default: 0.0.0.0].
  --listen-port=<port>              Listen on port [default:11300].
  --upstream-addrs=<addr_csv>       A CSV of upstream cluster-node servers. Defaults to any empty value where 
                                    a complete cluster node & beanstalkd server runs in a single process [default: ].
  --connect-timeout=<secs>          Upstream connection timeout in seconds [default: 10].
  --max-job-size=<bytes>            Maximum job size in bytes [default: 16384].
  --max-reservation-timeout=<secs>  Maximum reservation timeout in seconds [default: 3600].
`
	opts, err := docopt.ParseArgs(usage, argv[1:], version)
	if err != nil {
		log.Fatalf("error parsing arguments. err=%v", err)
	}

	var bsConfig core.Config
	if err := opts.Bind(&bsConfig); err != nil {
		log.Fatalf("error in opts.bind. err=%v", err)
	}

	if err := RunBeanstalkd(&bsConfig); err != nil {
		log.Fatalf("error RunBeanstalkd. err=%v", err)
	}
}
