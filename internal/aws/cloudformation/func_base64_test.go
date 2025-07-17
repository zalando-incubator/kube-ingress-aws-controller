package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type Base64FuncTest struct{}

var _ = Suite(&Base64FuncTest{})

func (testSuite *Base64FuncTest) TestRef(c *C) {
	inputBuf := `{ "Fn::Base64" : { "Fn::Join" : ["", [
		"#!/bin/bash -xe\n",
		"yum update -y aws-cfn-bootstrap\n",

		"# Install the files and packages from the metadata\n",
		"/opt/aws/bin/cfn-init -v ",
		"         --stack ", { "Ref" : "AWS::StackName" },
		"         --resource LaunchConfig ",
		"         --region ", { "Ref" : "AWS::Region" }, "\n",

		"# Signal the status from cfn-init\n",
		"/opt/aws/bin/cfn-signal -e $? ",
		"         --stack ", { "Ref" : "AWS::StackName" },
		"         --resource WebServerGroup ",
		"         --region ", { "Ref" : "AWS::Region" }, "\n"
	]]}}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	c.Assert(f.(Stringable).String(), DeepEquals, Base64(Join("",
		String("#!/bin/bash -xe\n"),
		String("yum update -y aws-cfn-bootstrap\n"),
		String("# Install the files and packages from the metadata\n"),
		String("/opt/aws/bin/cfn-init -v "),
		String("         --stack "), Ref("AWS::StackName").String(),
		String("         --resource LaunchConfig "),
		String("         --region "), Ref("AWS::Region").String(), String("\n"),
		String("# Signal the status from cfn-init\n"),
		String("/opt/aws/bin/cfn-signal -e $? "),
		String("         --stack "), Ref("AWS::StackName").String(),
		String("         --resource WebServerGroup "),
		String("         --region "), Ref("AWS::Region").String(), String("\n"),
	)))

	// old way still compiles
	c.Assert(f.(StringFunc).String(), DeepEquals, Base64(*Join("",
		*String("#!/bin/bash -xe\n"),
		*String("yum update -y aws-cfn-bootstrap\n"),
		*String("# Install the files and packages from the metadata\n"),
		*String("/opt/aws/bin/cfn-init -v "),
		*String("         --stack "), *Ref("AWS::StackName").String(),
		*String("         --resource LaunchConfig "),
		*String("         --region "), *Ref("AWS::Region").String(), *String("\n"),
		*String("# Signal the status from cfn-init\n"),
		*String("/opt/aws/bin/cfn-signal -e $? "),
		*String("         --stack "), *Ref("AWS::StackName").String(),
		*String("         --resource WebServerGroup "),
		*String("         --region "), *Ref("AWS::Region").String(), *String("\n"),
	).String()).String())

	// tidy the JSON input
	inputStruct := map[string]interface{}{}
	_ = json.Unmarshal([]byte(inputBuf), &inputStruct)
	expectedBuf, _ := json.Marshal(inputStruct)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, string(expectedBuf))
}
