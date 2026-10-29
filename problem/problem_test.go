package problem

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAdd(t *testing.T) {
	p := &List{}

	result := p.Add("something went wrong: %s", "boom")

	require.Same(t, p, result, "Add should return the same *List for chaining")
	require.Len(t, p.Errors(), 1)
	assert.EqualError(t, p.Errors()[0], "something went wrong: boom")
}

func TestListAddChaining(t *testing.T) {
	p := &List{}

	p.Add("first error").Add("second error: %d", 42)

	errs := p.Errors()
	require.Len(t, errs, 2)
	assert.EqualError(t, errs[0], "first error")
	assert.EqualError(t, errs[1], "second error: 42")
}

func TestListErrorsEmpty(t *testing.T) {
	p := &List{}

	assert.Empty(t, p.Errors())
}
