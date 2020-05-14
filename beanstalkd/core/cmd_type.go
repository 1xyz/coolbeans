package core

import (
	"unicode"
	"unicode/utf8"
)

// CmdType refers to the type of command in beanstalkd context

//go:generate stringer -type=CmdType --output cmd_type_string.go
type CmdType int

const (
	Unknown CmdType = iota
	Bury
	Delete
	Ignore
	Kick
	KickJob
	ListTubes
	ListTubeUser
	PauseTube
	Peek
	PeekBuried
	PeekDelayed
	PeekReady
	Put
	Quit
	Release
	Reserve
	ReserveJob
	ReserveWithTimeout
	Stats
	StatsJob
	StatsTube
	Touch
	Use
	Watch
	Max
)

var commandTypeStrings map[string]CmdType

func init() {
	commandTypeStrings = make(map[string]CmdType)
	for c := Unknown + 1; c < Max; c++ {
		commandTypeStrings[kebabCase(c.String())] = c
	}
}

func kebabCase(s string) string {
	result := make([]byte, 0, len(s))
	for i, ch := range s {
		if unicode.IsUpper(ch) && i > 0 {
			result = append(result, '-')
		}

		ch = unicode.ToLower(ch)
		eLen := utf8.RuneLen(ch)
		b := make([]byte, eLen, eLen)
		n := utf8.EncodeRune(b, ch)
		for j := 0; j < n; j++ {
			result = append(result, b[j])
		}
	}

	return string(result)
}
