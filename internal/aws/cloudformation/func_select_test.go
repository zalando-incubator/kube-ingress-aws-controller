package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type SelectFuncTest struct{}

var _ = Suite(&SelectFuncTest{})

func (testSuite *SelectFuncTest) TestRef(c *C) {
	inputBuf := `{"Fn::Select":["2",{"Fn::GetAZs":{"Ref":"AWS::Region"}}]}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringFunc).String(), DeepEquals,
		Select("2", GetAZs(Ref("AWS::Region"))))

	// old way
	c.Assert(f.(StringFunc).String(), DeepEquals,
		SelectFunc{Selector: "2",
			Items: *GetAZs(*Ref("AWS::Region").String()).StringList()}.String())

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)
}

func (testSuite *SelectFuncTest) TestRef2(c *C) {
	inputBuf := `{"Fn::Select":["2",["1","2","3"]]}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringFunc).String(), DeepEquals,
		Select("2", *String("1"), *String("2"), *String("3")).String())

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)
}

func (testSuite *SelectFuncTest) TestFailures(c *C) {
	inputBuf := `{"Fn::Select": 1}`
	_, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Select": ["1"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Select": ["1", [1]]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Select": ["1", ["2"], "3"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::Select": [false, ["2"]]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
