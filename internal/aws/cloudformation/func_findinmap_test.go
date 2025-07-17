package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type FindInMapFuncTest struct{}

var _ = Suite(&FindInMapFuncTest{})

func (testSuite *FindInMapFuncTest) TestBasics(c *C) {
	inputBuf := `{ "Fn::FindInMap" : [ "AWSRegionArch2AMI", { "Ref" : "AWS::Region" },
                          { "Fn::FindInMap" : [ "AWSInstanceType2Arch", { "Ref" : "InstanceType" }, "Arch" ] } ] }`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(StringFunc).String(), DeepEquals, FindInMap(
		"AWSRegionArch2AMI", Ref("AWS::Region"),
		FindInMap("AWSInstanceType2Arch", Ref("InstanceType"),
			String("Arch"))))

	// old way
	c.Assert(f.(StringFunc).String(), DeepEquals, FindInMap(
		"AWSRegionArch2AMI", *Ref("AWS::Region").String(),
		*FindInMap("AWSInstanceType2Arch", *Ref("InstanceType").String(),
			*String("Arch")).String()).String())

	// tidy the JSON input
	inputStruct := map[string]interface{}{}
	_ = json.Unmarshal([]byte(inputBuf), &inputStruct)
	expectedBuf, _ := json.Marshal(inputStruct)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, string(expectedBuf))
}

func (testSuite *FindInMapFuncTest) TestFailures(c *C) {
	inputBuf := `{"Fn::FindInMap": 1}`
	_, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::FindInMap": [1, "2", "3"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::FindInMap": ["1", 2, "3"]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")

	inputBuf = `{"Fn::FindInMap": ["1", "2", 3]}`
	_, err = unmarshalFunc([]byte(inputBuf))
	c.Assert(err, ErrorMatches, "cannot decode function")
}
