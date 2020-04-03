package state

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidIndex no job entry found at the index
	ErrInvalidIndex = errors.New("provided index is out of range of the heap")

	// ErrMismatchJobEntry - returned when job entry at index does not match the heap's value
	ErrMismatchJobEntry = errors.New("job entry at index does not match provided entry")

	// ErrEntryExists - returned when an entry exists in the existing map/set to prevents from overriding
	ErrEntryExists = errors.New("entry exists in container")

	// ErrEntryMissing - returned when an entry is not found in the container
	ErrEntryMissing = errors.New("entry not found in container")

	// ErrContainerEmpty - returned when the container such as a list/map/slice etc is empty
	ErrContainerEmpty = errors.New("the container is empty")

	// ErrInvalidJobTransition - the current state of the job prevents this transition
	ErrInvalidJobTransition = errors.New("invalid transition")

	// ErrUnauthorizedOperation - This state requires a matching client to perform this transition
	ErrUnauthorizedOperation = errors.New("client is not authorized to perform this operation")

	// ErrCancel - indicates the request is cancelled
	ErrCancel = errors.New("cancelled")

	// ErrNoReservation - indicates that a request for a reservation could not be completed
	ErrNoReservationFound = errors.New("no reservation could be found")

	// ErrInvalidResvTimeout - indicates the provided reservation timeout is invalid
	ErrInvalidResvTimeout = errors.New("the provided reservation timeout is invalid")

	// ErrClientIsWaitingForReservation - Indicates the client is waiting for a reservation
	ErrClientIsWaitingForReservation = errors.New("the request client cannot request for another reservation")
)

type ResultError struct {
	// ID of the request
	RequestID string

	// Identifier for the error code
	ErrorCode int32

	// Error
	Err error
}

func (r *ResultError) Error() string {
	return fmt.Sprintf("ReqID: %s | ErrorCode: %d | Err: %v",
		r.RequestID, r.ErrorCode, r.Err)
}

func (r *ResultError) Unwrap() error {
	return r.Err
}
