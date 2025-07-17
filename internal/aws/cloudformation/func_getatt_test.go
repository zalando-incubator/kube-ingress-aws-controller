package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type GetAttFuncTest struct{}

var _ = Suite(&GetAttFuncTest{})

func (testSuite *GetAttFuncTest) TestBasics(c *C) {
	inputBuf := `{"Fn::GetAtt" : ["MySQLDatabase", "Endpoint.Address"]}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringFunc).String(), DeepEquals,
		GetAtt("MySQLDatabase", "Endpoint.Address"))
	// old way
	c.Assert(f.(StringFunc).String(), DeepEquals,
		GetAtt("MySQLDatabase", "Endpoint.Address").String())

	// tidy the JSON input
	inputStruct := map[string]interface{}{}
	_ = json.Unmarshal([]byte(inputBuf), &inputStruct)
	expectedBuf, _ := json.Marshal(inputStruct)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, string(expectedBuf))
}

func (testSuite *GetAttFuncTest) TestFailures(c *C) {
	inputBuf := `{"Fn::GetAtt": 1}`
	_, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::GetAtt": ["1"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::GetAtt": ["1", "2", "3"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
