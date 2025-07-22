package cloudformation

import (
	"encoding/json"
	"testing"

	. "gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type StringTest struct{}

var _ = Suite(&StringTest{})

func (testSuite *StringTest) TestString(c *C) {
	inputBuf := `{"B": "one", "C": "two", "D": {"Ref": "foo"}}`

	v := struct {
		A *StringExpr `json:",omitempty"`
		B *StringExpr `json:",omitempty"`
		C *StringExpr `json:",omitempty"`
		D *StringExpr `json:",omitempty"`
	}{}

	err := json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, IsNil)

	c.Assert(v.A, IsNil)
	c.Assert(v.B, DeepEquals, String("one"))
	c.Assert(v.C, DeepEquals, String("two"))
	c.Assert(v.D, DeepEquals, Ref("foo").String())

	buf, err := json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals,
		`{"B":"one","C":"two","D":{"Ref":"foo"}}`)

	inputBuf = `{"A": false}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "json: cannot unmarshal bool.*")

	inputBuf = `{"A": 1}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "json: cannot unmarshal number.*")

	inputBuf = `{"A": {"Fn::Missing": "hello"}}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "unknown function Fn::Missing")
}
