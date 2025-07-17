package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type RefFuncTest struct{}

var _ = Suite(&RefFuncTest{})

func (testSuite *RefFuncTest) TestRef(c *C) {
	inputBuf := `{"Ref" : "AWS::Region"}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringFunc).String(), DeepEquals, Ref("AWS::Region").String())

	// tidy the JSON input
	inputStruct := map[string]interface{}{}
	_ = json.Unmarshal([]byte(inputBuf), &inputStruct)
	expectedBuf, _ := json.Marshal(inputStruct)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, string(expectedBuf))
}

func (testSuite *RefFuncTest) TestFailures(c *C) {
	inputBuf := `{"Ref": {"Ref": "foo"}}`
	_, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Ref": ["1"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Ref": true}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
