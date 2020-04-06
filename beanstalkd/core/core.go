package core

import (
	"errors"
	"github.com/1xyz/coolbeans/state"
	"regexp"
	"time"
)

var (
	// Delimiter for commands and data
	DelimRe = regexp.MustCompile(`\r\n`)

	// request count
	requestCount int64 = 0
)

const (
	TickDuration = 1000 * time.Millisecond

	// Max. Command size in bytes (inclusive of 2 byte delimiter)
	MaxCmdSizeBytes = 226

	// Max. Job Data size in bytes (inclusive of 2 byte delimiter)
	MaxJobDataSizeBytes = (16 * 1024) + 2

	// The size of a read buffer
	readBufferSizeBytes = 4 * 1024

	// Default tube name
	defaultTubeName = state.TubeName("default")

	// Maximum reservation timeout
	MaxReservationTimeout = 24 * 3600
)

const (
	// Error message response indicating an internal server error. Typically, indicative
	// of a bug in the implementation.
	MsgInternalError = "INTERNAL_ERROR"

	// The client sent a command line that was not well-formed.
	// This can happen if the line's length exceeds 224 bytes including \r\n,
	// if the name of a tube exceeds 200 bytes, if non-numeric
	// characters occur where an integer is expected, if the wrong number of
	// arguments are present, or if the command line is mal-formed in any other way.
	MsgBadFormat = "BAD_FORMAT"

	// The client sent a command that the server does not know.
	MsgUnknownCommand = "UNKNOWN_COMMAND"

	// Error message if the client attempts to ignore the only tube in its watch list.
	MsgCannotIgnoreTube = "NOT_IGNORED"

	// Error message to indicate if a reservation request timed out
	MsgTimedOut = "TIMED_OUT"

	// Error message to indicate if a reservation request is in DeadlineSoon
	MsgDeadlineSoon = "DEADLINE_SOON"

	// Error message to indicate if the entity (job etc) cannot be found
	MsgNotFound = "NOT_FOUND"
)

var (
	// ErrNoData - when the input stream has no data this can be the case
	// if the underlying reader return zero bytes  and is however not at EOF
	ErrNoData = errors.New("no data")

	// ErrDelimiterMissing - when the input stream has no newlines (\r\n)
	ErrDelimiterMissing = errors.New("delimiter (\\r\\n) missing in command")

	// ErrBadFormat The client sent a command line that was not well-formed.
	// This can happen if the line's length exceeds 224 bytes including \r\n,
	// if the name of a tube exceeds 200 bytes, if non-numeric
	// characters occur where an integer is expected, if the wrong number of
	// arguments are present, or if the command line is mal-formed in any other
	ErrBadFormat = errors.New("bad format command")

	// ErrCmdTokensMissing - when the provided command has no tokens
	ErrCmdTokensMissing = errors.New("bad command, cannot find atleast one token")

	// ErrCmdNotFound - the provided command is not found or supported
	ErrCmdNotFound = errors.New("command not found")
)

func nowSeconds() int64 {
	return time.Now().UTC().Unix()
}

func addToNow(delaySeconds int) int64 {
	return nowSeconds() + int64(delaySeconds)
}
