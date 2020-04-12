package main

import (
	"fmt"
	"github.com/beanstalkd/go-beanstalk"
	"github.com/docopt/docopt-go"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"time"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	usage := `usage: bs-client [--version] [--addr=<addr>] <command> [<args>...]
options:
   --addr=<addr>  Beanstalkd Address [default: :11300].
   -h, --help
The commands are:
   put        Put a job into a beanstalkd tube.
   reserve    Reserve a job from one or more tubes.
`
	parser := &docopt.Parser{OptionsFirst: true}
	args, err := parser.ParseArgs(usage, nil, "bs-demo-client version 0.1")
	if err != nil {
		log.Errorf("err = %v", err)
		os.Exit(1)
	}

	cmd := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	addr, err := args.String("--addr")
	if err != nil {
		log.Errorf("args.String(--addr). err=%v", err)
		os.Exit(1)
	}

	c, err := beanstalk.Dial("tcp", addr)
	if err != nil {
		log.Errorf("error dial beanstalkd %v\n", err)
		return
	}

	if err := runCommand(c, cmd, cmdArgs); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runCommand(c *beanstalk.Conn, cmd string, args []string) (err error) {
	argv := append([]string{cmd}, args...)
	switch cmd {
	case "put":
		return cmdPut(c, argv)
	case "reserve":
		return cmdReserve(c, argv)
	default:
		return fmt.Errorf("%s is not a valid command", cmd)
	}
}

func cmdReserve(c *beanstalk.Conn, argv []string) error {
	usage := `usage: reserve [--timeout=<timeout>] [--tubes=<tubes>] [--no-delete]
options:
	-h, --help
	--timeout=<timeout>   reservation timeout in seconds [default: 0]
	--tubes=<tubes>       csv of tubes [default: default]
    --no-delete           do not delete (aka. ACK) the job once reserved [default: false]

example:
	watch for reservations on default tube (topic)
	reserve
	
    watch for reservations on tubes foo & bar with timeout of 10 seconds
	reserve --timeout 10 --tubes=foo,bar`

	opts, err := docopt.ParseArgs(usage, argv[1:], "version")
	if err != nil {
		log.Fatalf("error parsing arguments. err=%v", err)
	}

	log.Debugf("args:...%v", opts)
	timeout, err := opts.Int("--timeout")
	if err != nil {
		return err
	}

	tubes, err := opts.String("--tubes")
	if err != nil {
		return err
	}

	noDel, err := opts.Bool("--no-delete")
	if err != nil {
		return err
	}

	tubeNames := strings.Split(tubes, ",")
	log.Infof("c.reserve() timeout=%v sec tubes=%v no-delete=%v", timeout, tubeNames, noDel)
	ts := beanstalk.NewTubeSet(c, tubeNames...)
	id, body, err := ts.Reserve(time.Duration(timeout) * time.Second)
	if err != nil {
		log.Errorf("reserve. err=%v", err)
		return err
	}

	log.Infof("reserved job id=%v body=%v", id, body)

	if !noDel {
		if err := c.Delete(id); err != nil {
			log.Errorf("delete. err=%v", err)
			return err
		}

		log.Infof("deleted job %v", id)
	}

	return nil
}

func cmdPut(c *beanstalk.Conn, argv []string) error {
	usage := `usage: put [--body=<body>] [--pri=<pri>] [--ttr=<ttr>] [--delay=<delay>] [--tube=<tube>]
options:
	-h, --help
	--body=<body>     body [default: hello]
	--pri=<pri>       job priority [default: 1]
	--ttr=<ttr>       ttr in seconds [default: 10]
	--delay=<delay    job delay in seconds [default: 0]
    --tube=<tube>     tube (topic) to put the job [default: default]

example:
	put --body "hello world"
	put --body "hello world" --tube foo`

	opts, err := docopt.ParseArgs(usage, argv[1:], "version")
	if err != nil {
		log.Fatalf("error parsing arguments. err=%v", err)
	}

	tube, err := opts.String("--tube")
	if err != nil {
		return err
	}

	var t *beanstalk.Tube = nil
	if tube != "default" {
		t = &beanstalk.Tube{Conn: c, Name: tube}
	}

	log.Debugf("args:...%v", opts)
	body, err := opts.String("--body")
	if err != nil {
		return err
	}

	pri, err := opts.Int("--pri")
	if err != nil {
		return err
	}

	ttr, err := opts.Int("--ttr")
	if err != nil {
		return err
	}

	delay, err := opts.Int("--delay")
	if err != nil {
		return err
	}

	log.Infof("c.Put() body=%v, pri=%v, delay=%v sec, ttr=%v sec tube=%v",
		body, pri, ttr, delay, tube)

	var id uint64
	if t == nil {
		id, err = c.Put([]byte(body), uint32(pri), time.Duration(delay)*time.Second, time.Duration(ttr)*time.Second)
	} else {
		id, err = t.Put([]byte(body), uint32(pri), time.Duration(delay)*time.Second, time.Duration(ttr)*time.Second)
	}

	if err != nil {
		log.Errorf("c.Put(...), error %v\n", err)
		return err
	}

	log.Infof("c.Put() returned id = %v", id)
	return nil
}
