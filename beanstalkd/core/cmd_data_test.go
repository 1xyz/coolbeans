package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseCommandLine(t *testing.T) {
	// put <pri> <delay> <ttr> <bytes>
	var entries = []struct {
		inCmdLine string
		outCmData *CmdData
		err       error
		msg       string
	}{
		{"put 0 100 30 32898",
			&CmdData{Put, "0 100 30 32898", []byte{}, true},
			nil,
			"expect valid put parsing"},
		{"split 0 100 30 32898",
			nil,
			ErrCmdNotFound,
			"expect ErrCmdNotFound to be returned"},
		{"put",
			&CmdData{Put, "", []byte{}, true},
			nil,
			"expect cmdline to be parse even if there are no args"},
		{"  ",
			nil,
			ErrCmdTokensMissing,
			"expect ErrCmdTokensMissing to be returned"},
	}

	for _, e := range entries {
		cmdData, err := ParseCommandLine(e.inCmdLine)
		assert.Equalf(t, e.err, err, e.msg)
		assert.Equalf(t, e.outCmData, cmdData, e.msg)
	}
}

func TestPutArg(t *testing.T) {
	// put <pri> <delay> <ttr> <bytes>
	var entries = []struct {
		inArg  string
		outArg *putArg
		err    error
		msg    string
	}{
		{"0 100 30 32898", &putArg{pri: 0, delay: 100, ttr: 30, size: 32898}, nil,
			"expect valid put arg"},
		{"0 100 30 32898 892", nil, ErrBadFormat,
			"put args must have exact arg count"},
		{"0 100 30 ", nil, ErrBadFormat,
			"put args must have exact arg count"},
		{"0 100 30 32898abc", nil, ErrBadFormat,
			"put args must have numeric args"},
	}

	for _, e := range entries {
		d := &CmdData{
			CmdType:  Unknown,
			Args:     e.inArg,
			Data:     nil,
			NeedData: false,
		}
		pa, err := NewPutArg(d)
		assert.Equalf(t, e.err, err, e.msg)
		assert.Equalf(t, e.outArg, pa, e.msg)
	}
}

func TestTubeArg(t *testing.T) {
	var entries = []struct {
		inArg   string
		tubeArg *tubeArg
		err     error
		msg     string
	}{
		{"foo", &tubeArg{tubeName: "foo"}, nil,
			"expect valid tubename"},
		{"pizza day", nil, ErrBadFormat,
			"tube cannot have spaces"},
		{strN('a', 201), nil, ErrBadFormat,
			"tube name cannot exceed 200 bytes"},
	}

	for _, e := range entries {
		d := &CmdData{
			CmdType:  Unknown,
			Args:     e.inArg,
			Data:     nil,
			NeedData: false,
		}
		tc, err := NewTubeArg(d)
		assert.Equalf(t, e.err, err, e.msg)
		assert.Equalf(t, e.tubeArg, tc, e.msg)
	}
}

func TestIDArg(t *testing.T) {
	var entries = []struct {
		inArg string
		idArg *idArg
		err   error
		msg   string
	}{
		{"123456", &idArg{id: 123456}, nil,
			"expect valid job id"},
		{"pizza", nil, ErrBadFormat,
			"id has to be numeric"},
		{"12345 ", nil, ErrBadFormat,
			"id can't have spaces"},
		{"12345 678", nil, ErrBadFormat,
			"id field has only one word"},
		{strN('1', 200), nil, ErrBadFormat,
			"id field has only one word"},
		{"-12234", nil, ErrBadFormat,
			"id has to be unsigned"},
	}

	for _, e := range entries {
		d := &CmdData{
			CmdType:  Unknown,
			Args:     e.inArg,
			Data:     nil,
			NeedData: false,
		}
		tc, err := NewIDArg(d)
		assert.Equalf(t, e.err, err, e.msg)
		assert.Equalf(t, e.idArg, tc, e.msg)
	}
}

func strN(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func TestReserveWithTimeoutArg(t *testing.T) {
	var entries = []struct {
		inArg  string
		outArg *reserveWithTimeoutArg
		err    error
		msg    string
	}{
		{"123456", &reserveWithTimeoutArg{timeoutSeconds: 123456}, nil,
			"expect valid argument"},
		{"0", &reserveWithTimeoutArg{timeoutSeconds: 0}, nil,
			"expect valid argument"},
		{"123456 3287", nil, ErrBadFormat,
			"timeoutSeconds has to be a single numeric value"},
		{"-123456", nil, ErrBadFormat,
			"timeoutSeconds cannot be negative"},
		{strN('2', 200), nil, ErrBadFormat,
			"timeoutSeconds cannot exceed max int"},
		{"pizza", nil, ErrBadFormat,
			"timeoutSeconds has to be numeric"},
	}

	for _, e := range entries {
		d := &CmdData{
			CmdType:  Unknown,
			Args:     e.inArg,
			Data:     nil,
			NeedData: false,
		}
		tc, err := NewReserveWithTimeoutArg(d)
		assert.Equalf(t, e.err, err, e.msg)
		assert.Equalf(t, e.outArg, tc, e.msg)
	}
}