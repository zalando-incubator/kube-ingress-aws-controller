package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type FuncTest struct{}

var _ = Suite(&FuncTest{})

func (testSuite *FuncTest) TestRef(c *C) {
	inputBuf := `{"Ref": "foo"}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f, DeepEquals, RefFunc{Name: "foo"})

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, `{"Ref":"foo"}`)

	inputBuf = `true`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "json: cannot unmarshal .*")

	inputBuf = `{"Fn::Missin": "Invalid"}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, DeepEquals, UnknownFunctionError{Name: "Fn::Missin"})
	c.Assert(err.Error(), Equals, "unknown function Fn::Missin")

	inputBuf = `{}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
