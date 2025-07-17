package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type IntegerTest struct{}

var _ = Suite(&IntegerTest{})

func (testSuite *IntegerTest) TestInteger(c *C) {
	inputBuf := `{"B": 1, "C": 2, "D": {"Ref": "foo"}, "E": "1"}`

	v := struct {
		A *IntegerExpr `json:",omitempty"`
		B *IntegerExpr `json:",omitempty"`
		C *IntegerExpr `json:",omitempty"`
		D *IntegerExpr `json:",omitempty"`
		E *IntegerExpr `json:",omitempty"`
	}{}

	err := json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, IsNil)

	c.Assert(v.A, IsNil)
	c.Assert(v.B, DeepEquals, Integer(1))
	c.Assert(v.C, DeepEquals, Integer(2))
	c.Assert(v.D, DeepEquals, Ref("foo").Integer())
	c.Assert(v.E, DeepEquals, Integer(1))

	buf, err := json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals,
		`{"B":1,"C":2,"D":{"Ref":"foo"},"E":1}`)

	inputBuf = `{"A": "invalid"}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "json: cannot unmarshal string.*")

	inputBuf = `{"A": {"Fn::Missing": "invalid"}}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "unknown function Fn::Missing")
}
