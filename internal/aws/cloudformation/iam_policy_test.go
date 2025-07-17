package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type IAMPolicyTest struct{}

var _ = Suite(&IAMPolicyTest{})

func (testSuite *IAMPolicyTest) TestIAMPolicyStatement(c *C) {
	// We are primarily testing the parsing of Principal
	p := `{
  "Version":"2012-10-17",
  "Statement":[
    {
      "Sid":"AddPerm",
      "Effect":"Allow",
      "Principal": "*",
      "Action":["s3:GetObject"],
      "Resource":["arn:aws:s3:::examplebucket/*"]
    }
  ]
}`

	v := IAMPolicyDocument{}
	err := json.Unmarshal([]byte(p), &v)
	c.Assert(err, IsNil)
	c.Assert(v.Version, Equals, "2012-10-17")
	c.Assert(v.Statement, HasLen, 1)
	s := v.Statement[0]
	c.Assert(s.Sid, Equals, "AddPerm")
	c.Assert(s.Effect, Equals, "Allow")
	c.Assert(s.Principal.AWS, DeepEquals, StringList(String("*")))
	c.Assert(s.Principal.CanonicalUser, IsNil)
	c.Assert(s.Principal.Federated, IsNil)
	c.Assert(s.Principal.Service, IsNil)
	c.Assert(s.Action, DeepEquals, StringList(String("s3:GetObject")))
	c.Assert(s.Resource, DeepEquals, StringList(String("arn:aws:s3:::examplebucket/*")))

	// Make sure we marshall the "*" back properly
	b, err := json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(b), Equals, `{"Version":"2012-10-17","Statement":[{"Sid":"AddPerm","Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::examplebucket/*"]}]}`)

	// Try other variations of Principal, as well as a single statement rather than an array
	p = `{
   "Version":"2012-10-17",
   "Id":"PolicyForCloudFrontPrivateContent",
   "Statement": {
       "Sid":" Grant a CloudFront Origin Identity access to support private content",
       "Effect":"Allow",
       "Principal":{"CanonicalUser":"79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be"},
       "Action":"s3:GetObject",
       "Resource":"arn:aws:s3:::example-bucket/*"
    }
}`

	err = json.Unmarshal([]byte(p), &v)
	c.Assert(err, IsNil)
	s = v.Statement[0]
	c.Assert(s.Principal.AWS, IsNil)
	c.Assert(s.Principal.CanonicalUser, DeepEquals, StringList(String("79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be")))
	c.Assert(s.Principal.Federated, IsNil)
	c.Assert(s.Principal.Service, IsNil)

	// Try marshalling this too
	b, err = json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(b), Equals, `{"Version":"2012-10-17","Statement":[{"Sid":" Grant a CloudFront Origin Identity access to support private content","Effect":"Allow","Principal":{"CanonicalUser":["79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::example-bucket/*"]}]}`)

	p = `{
   "Version":"2012-10-17",
   "Id":"PolicyForCloudFrontPrivateContent",
   "Statement":[
     {
       "Sid":" Grant a CloudFront Origin Identity access to support private content",
       "Effect":"Allow",
       "Principal": {
         "Service": [
           "ec2.amazonaws.com",
           "datapipeline.amazonaws.com"
         ]
       },
       "Action":"s3:GetObject",
       "Resource":"arn:aws:s3:::example-bucket/*"
     }
   ]
}`

	err = json.Unmarshal([]byte(p), &v)
	c.Assert(err, IsNil)
	s = v.Statement[0]
	c.Assert(s.Principal.AWS, IsNil)
	c.Assert(s.Principal.CanonicalUser, IsNil)
	c.Assert(s.Principal.Federated, IsNil)
	c.Assert(s.Principal.Service, DeepEquals, StringList(String("ec2.amazonaws.com"), String("datapipeline.amazonaws.com")))

	b, err = json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(b), Equals, `{"Version":"2012-10-17","Statement":[{"Sid":" Grant a CloudFront Origin Identity access to support private content","Effect":"Allow","Principal":{"Service":["ec2.amazonaws.com","datapipeline.amazonaws.com"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::example-bucket/*"]}]}`)
}
