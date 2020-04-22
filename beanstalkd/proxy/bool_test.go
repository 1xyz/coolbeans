package proxy

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewAtomicBool(t *testing.T) {
	ab := NewAtomicBool()
	assert.Falsef(t, ab.Value(), "expect ab.Value to be initialized to false")

	assert.Falsef(t, ab.ResetIfTrue(), "expect ab.ResetIfTrue to return false")
	assert.Truef(t, ab.SetIfFalse(), "expect ab.SetIfFalse to return true")

	assert.Truef(t, ab.Value(), "expect ab.Value to be true")

	assert.Falsef(t, ab.SetIfFalse(), "expect ab.SetIfFalse to return false")
	assert.Truef(t, ab.ResetIfTrue(), "expect ab.ResetIfTrue to return true")

	assert.Falsef(t, ab.Value(), "expect ab.Value to be false")
}
