package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type ImportValueFuncTest struct{}

var _ = Suite(&ImportValueFuncTest{})

func (testSuite *ImportValueFuncTest) TestRef(c *C) {
	inputBuf := `{"Fn::ImportValue" : "sharedValueToImport"}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringFunc).String(), DeepEquals, ImportValue(String("sharedValueToImport")).String())

	// tidy the JSON input
	inputStruct := map[string]interface{}{}
	_ = json.Unmarshal([]byte(inputBuf), &inputStruct)
	expectedBuf, _ := json.Marshal(inputStruct)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, string(expectedBuf))
}

func (testSuite *ImportValueFuncTest) TestFailures(c *C) {
	inputBuf := `{"Fn::ImportValue": ["1"]}`
	_, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::ImportValue": true}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
