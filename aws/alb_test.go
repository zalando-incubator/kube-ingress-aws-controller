package aws

import (
	"github.com/davecgh/go-spew/spew"
	"testing"
)

func TestFoo(t *testing.T) {
	var x []*int

	for i := 0; i < 10; i++ {
		x = append(x, &i)
	}

	spew.Dump(x)
}
