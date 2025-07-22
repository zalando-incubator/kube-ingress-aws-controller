package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type BoolTest struct{}

var _ = Suite(&BoolTest{})

func (testSuite *BoolTest) TestBool(c *C) {
	inputBuf := `{"B": true, "C": false, "D": {"Ref": "foo"}, "E": "true"}`

	v := struct {
		A *BoolExpr `json:",omitempty"`
		B *BoolExpr `json:",omitempty"`
		C *BoolExpr `json:",omitempty"`
		D *BoolExpr `json:",omitempty"`
		E *BoolExpr `json:",omitempty"`
	}{}

	err := json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, IsNil)

	c.Assert(v.A, IsNil)
	c.Assert(v.B, DeepEquals, Bool(true))
	c.Assert(v.C, DeepEquals, Bool(false))
	c.Assert(v.D, DeepEquals, Ref("foo").Bool())
	c.Assert(v.E, DeepEquals, Bool(true))

	buf, err := json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals,
		`{"B":true,"C":false,"D":{"Ref":"foo"},"E":true}`)

	inputBuf = `{"A": "invalid"}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "json: cannot unmarshal string.*")

	inputBuf = `{"A": {"Fn::Missing": "invalid"}}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "unknown function Fn::Missing")
}
