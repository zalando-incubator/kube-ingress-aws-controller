package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type JoinFuncTest struct{}

var _ = Suite(&JoinFuncTest{})

func (testSuite *JoinFuncTest) TestRef(c *C) {
	inputBuf := `{"Fn::Join":["x",["y",{"Ref":"foo"},"1"]]}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringFunc).String(), DeepEquals,
		Join("x", String("y"), Ref("foo"), String("1")))

	// old way
	c.Assert(f.(StringFunc).String(), DeepEquals,
		Join("x", *String("y"), *Ref("foo").String(), *String("1")).String())

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)
}

func (testSuite *JoinFuncTest) TestFailures(c *C) {
	inputBuf := `{"Fn::Join": 1}`
	_, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Join": ["1"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Join": ["1", [1]]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Join": ["1", ["2"], "3"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Join": [false, ["2"]]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
