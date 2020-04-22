package proxy

import "sync/atomic"

// Encapsulates a boolean value that is safe to perform
// conditional set/resets across go-routines.
type AtomicBool int32

// Construct a new AtomicBool with a value of false
func NewAtomicBool() *AtomicBool {
	a := new(AtomicBool)
	atomic.StoreInt32((*int32)(a), 0)
	return a
}

// Value returns the current boolean value
func (a *AtomicBool) Value() bool {
	return atomic.LoadInt32((*int32)(a)) == 1
}

// SetIfFalse updates the boolean value to true from false.
// Returns the boolean result of this transition operation, true if successful, false otherwise
func (a *AtomicBool) SetIfFalse() bool {
	return atomic.CompareAndSwapInt32((*int32)(a), 0, 1)
}

// ResetIfTrue updates the boolean value to false from true.
// Returns the boolean result of this transition operation, true if successful, false otherwise
func (a *AtomicBool) ResetIfTrue() bool {
	return atomic.CompareAndSwapInt32((*int32)(a), 1, 0)
}
