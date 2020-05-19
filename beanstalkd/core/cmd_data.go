package core

import (
	"fmt"
	"github.com/1xyz/coolbeans/state"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strconv"
)

type CmdData struct {
	CmdType  CmdType
	Args     string
	Data     []byte
	NeedData bool
}

func (c CmdData) String() string {
	return fmt.Sprintf("CmdType: %v Args:[%v] NeedData:[%v]",
		c.CmdType, c.Args, c.NeedData)
}

var (
	spaceRe = regexp.MustCompile(`(\s{2,}|\s+(^|$))`)
	splitRe = regexp.MustCompile(`\s`)
)

// ParseCommandLine parses the command line string provided a connected client
// into a valid CmdData struct
func ParseCommandLine(cmdLine string) (*CmdData, error) {
	s := spaceRe.ReplaceAllLiteralString(cmdLine, "")
	tokens := splitRe.Split(s, 2)
	if len(tokens) == 0 || (len(tokens) == 1 && tokens[0] == "") {
		return nil, ErrCmdTokensMissing
	}

	if c, ok := commandTypeStrings[tokens[0]]; !ok {
		return nil, ErrCmdNotFound
	} else {
		var args string
		if len(tokens) == 2 {
			args = tokens[1]
		}

		var data []byte = nil
		cd := &CmdData{
			CmdType:  c,
			Args:     args,
			Data:     data,
			NeedData: false,
		}

		if c == Put {
			cd.Data = make([]byte, 0)
			cd.NeedData = true
			arg, err := NewPutArg(cd)
			if err != nil {
				log.Errorf("ParseCommandLine: NewPutArg err = %v", err)
				return nil, err
			}
			if arg.size > MaxJobDataSizeBytes {
				return nil, ErrJobSizeTooBig
			}
		}

		return cd, nil
	}
}

type tokenMap map[string]string

func matchNamedGroups(args string, re *regexp.Regexp) (tokenMap, bool) {
	if !re.MatchString(args) {
		return nil, false
	}

	names := re.SubexpNames()
	matches := re.FindAllStringSubmatch(args, -1)
	res := make(map[string]string)
	for _, e := range matches {
		for i, f := range e {
			if names[i] == "" {
				continue
			}

			res[names[i]] = f
		}
	}

	return res, true
}

var (
	// put command regex -- put <pri> <delay> <ttr> <bytes>
	putRe = regexp.MustCompile(`^(?P<pri>\d+) (?P<delay>\d+) (?P<ttr>\d+) (?P<bytes>\d+)$`)

	// tube arg regex -- watch <tube> | ignore <tube> | use <tube>
	tubeArgRe = regexp.MustCompile(`(?P<tube>^\w{1,200}$)`)

	// id arg regex --
	// delete <id>
	// kick-job <id>
	idArgRe = regexp.MustCompile(`(?P<id>^\d+$)`)

	// reserve-with-timeo
	// ut regex -- reserve-with-timeout <seconds>
	reserveWithTimeoutRe = regexp.MustCompile(`(?P<seconds>^\d+$)`)

	// bury <id> <pri>
	buryArgRe = regexp.MustCompile(`^(?P<id>^\d+) (?P<pri>\d+)$`)

	// kick <bound>
	kickNArgRe = regexp.MustCompile(`(?P<bound>^\d+$)`)

	// release <id> <pri> <delay>
	releaseArgRe = regexp.MustCompile(`^(?P<id>^\d+) (?P<pri>\d+) (?P<delay>\d+)$`)
)

type putArg struct {
	pri   uint32
	delay int64
	ttr   int
	size  int
	data  []byte
}

func NewPutArg(data *CmdData) (*putArg, error) {
	ctxLog := log.WithFields(log.Fields{"method": "NewPutArg"})
	tm, ok := matchNamedGroups(data.Args, putRe)
	if !ok {
		ctxLog.Errorf("matchNamedGroups ok=false")
		return nil, ErrBadFormat
	}

	ctxLog.Debugf("matchResponse %v", tm)
	pri, err := strconv.ParseUint(tm["pri"], 10, 32)
	if err != nil {
		ctxLog.Errorf("ParseUint(pri) err=%v", err)
		return nil, ErrBadFormat
	}

	delay, err := strconv.ParseInt(tm["delay"], 10, 64)
	if err != nil {
		ctxLog.Errorf("strconv.ParseInt(delay) err=%v", err)
		return nil, ErrBadFormat
	}

	ttr, err := strconv.Atoi(tm["ttr"])
	if err != nil {
		ctxLog.Errorf("atoi(ttr) %v", err)
		return nil, ErrBadFormat
	}

	bytes, err := strconv.Atoi(tm["bytes"])
	if err != nil {
		ctxLog.Errorf("atoi(bytes) %v", err)
		return nil, ErrBadFormat
	}

	return &putArg{
		pri:   uint32(pri),
		delay: delay,
		ttr:   ttr,
		size:  bytes,
		data:  data.Data,
	}, nil
}

type tubeArg struct {
	tubeName state.TubeName
}

func NewTubeArg(data *CmdData) (*tubeArg, error) {
	ctxLog := log.WithFields(log.Fields{"method": "NewTubeArg"})
	tm, ok := matchNamedGroups(data.Args, tubeArgRe)
	if !ok {
		ctxLog.Errorf("matchNamedGroups ok=false")
		return nil, ErrBadFormat
	}

	ctxLog.Debugf("matchResponse %v", tm)
	return &tubeArg{
		tubeName: state.TubeName(tm["tube"]),
	}, nil
}

type idArg struct {
	id state.JobID
}

func NewIDArg(data *CmdData) (*idArg, error) {
	ctxLog := log.WithFields(log.Fields{"method": "NewIDArg"})
	tm, ok := matchNamedGroups(data.Args, idArgRe)
	if !ok {
		ctxLog.Errorf("matchNamedGroups ok=false")
		return nil, ErrBadFormat
	}

	ctxLog.Debugf("matchResponse %v", tm)
	id, err := strconv.ParseUint(tm["id"], 10, 64)
	if err != nil {
		ctxLog.Errorf("ParseUint(id) err=%v", err)
		return nil, ErrBadFormat
	}
	return &idArg{
		id: state.JobID(id),
	}, nil
}

type reserveWithTimeoutArg struct {
	timeoutSeconds int
}

func NewReserveWithTimeoutArg(data *CmdData) (*reserveWithTimeoutArg, error) {
	ctxLog := log.WithFields(log.Fields{"method": "NewReserveWithTimeoutArg"})
	tm, ok := matchNamedGroups(data.Args, reserveWithTimeoutRe)
	if !ok {
		ctxLog.Errorf("matchNamedGroups ok=false")
		return nil, ErrBadFormat
	}

	ctxLog.Debugf("matchResponse %v", tm)
	timeoutSeconds, err := strconv.Atoi(tm["seconds"])
	if err != nil {
		ctxLog.Errorf("atoi(seconds) %v", err)
		return nil, ErrBadFormat
	}

	return &reserveWithTimeoutArg{
		timeoutSeconds: timeoutSeconds,
	}, nil
}

type buryArg struct {
	id  state.JobID
	pri uint32
}

func NewBuryArg(data *CmdData) (*buryArg, error) {
	tm, ok := matchNamedGroups(data.Args, buryArgRe)
	if !ok {
		log.Errorf("NewBuryArg: matchNamedGroups ok=false")
		return nil, ErrBadFormat
	}

	id, err := strconv.ParseUint(tm["id"], 10, 64)
	if err != nil {
		log.Errorf("NewBuryArg: ParseUint(id) err=%v", err)
		return nil, ErrBadFormat
	}

	pri, err := strconv.ParseUint(tm["pri"], 10, 32)
	if err != nil {
		log.Errorf("NewBuryArg: ParseUint(pri) err=%v", err)
		return nil, ErrBadFormat
	}

	return &buryArg{
		id:  state.JobID(id),
		pri: uint32(pri),
	}, nil
}

type kickNArg struct {
	bound int
}

func NewKickNArg(data *CmdData) (*kickNArg, error) {
	tm, ok := matchNamedGroups(data.Args, kickNArgRe)
	if !ok {
		log.Errorf("NewKickNArg: matchNamedGroups ok=false")
		return nil, ErrBadFormat
	}

	bound, err := strconv.Atoi(tm["bound"])
	if err != nil {
		log.Errorf("NewKickNArg: atoi(bound) %v", err)
		return nil, ErrBadFormat
	}
	return &kickNArg{
		bound: bound,
	}, nil
}

type releaseArg struct {
	id    state.JobID
	pri   uint32
	delay int64
}

func NewReleaseArg(data *CmdData) (*releaseArg, error) {
	tm, ok := matchNamedGroups(data.Args, releaseArgRe)
	if !ok {
		log.Errorf("NewReleaseArg: matchNamedGroups ok=false")
		return nil, ErrBadFormat
	}

	id, err := strconv.ParseUint(tm["id"], 10, 64)
	if err != nil {
		log.Errorf("NewReleaseArg: ParseUint(id) err=%v", err)
		return nil, ErrBadFormat
	}

	pri, err := strconv.ParseUint(tm["pri"], 10, 32)
	if err != nil {
		log.Errorf("NewReleaseArg: ParseUint(pri) err=%v", err)
		return nil, ErrBadFormat
	}

	delay, err := strconv.ParseInt(tm["delay"], 10, 64)
	if err != nil {
		log.Errorf("NewReleaseArg: ParseInt(delay) err=%v", err)
		return nil, ErrBadFormat
	}

	return &releaseArg{
		id:    state.JobID(id),
		pri:   uint32(pri),
		delay: delay,
	}, nil
}
