package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type GetAZsFuncTest struct{}

var _ = Suite(&GetAZsFuncTest{})

func (testSuite *GetAZsFuncTest) TestBasics(c *C) {
	inputBuf := `{"Fn::GetAZs" : {"Ref": "AWS::Region"}}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringListFunc).StringList(), DeepEquals,
		GetAZs(Ref("AWS::Region")))

	// old way
	c.Assert(f.(StringListFunc).StringList(), DeepEquals,
		GetAZs(*Ref("AWS::Region").String()).StringList())

	// tidy the JSON input
	inputStruct := map[string]interface{}{}
	_ = json.Unmarshal([]byte(inputBuf), &inputStruct)
	expectedBuf, _ := json.Marshal(inputStruct)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, string(expectedBuf))
}

func (testSuite *GetAZsFuncTest) TestFailures(c *C) {
	inputBuf := `{"Fn::GetAZs": 1}`
	_, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::GetAZs": ["1"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::GetAZs": ["1", "2", "3"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
