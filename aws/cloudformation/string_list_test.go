package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type StringListTest struct{}

var _ = Suite(&StringListTest{})

func (testSuite *StringListTest) TestStringList(c *C) {
	inputBuf := `{"B": ["one"], "C": ["two", {"Ref": "foo"}]}`

	v := struct {
		A *StringListExpr `json:",omitempty"`
		B *StringListExpr `json:",omitempty"`
		C *StringListExpr `json:",omitempty"`
	}{}

	err := json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, IsNil)

	c.Assert(v.A, IsNil)
	c.Assert(v.B, DeepEquals, StringList(String("one")))
	c.Assert(v.C, DeepEquals, StringList(String("two"), Ref("foo")))

	// old way still works
	c.Assert(v.B, DeepEquals, StringList(*String("one")))
	c.Assert(v.C, DeepEquals, StringList(*String("two"), *Ref("foo").String()))

	buf, err := json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals,
		`{"B":["one"],"C":["two",{"Ref":"foo"}]}`)

	v.B, v.C = nil, nil
	inputBuf = `{"A":{"Fn::GetAZs":""}}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, IsNil)
	c.Assert(v.A, DeepEquals, GetAZs(String("")))
	c.Assert(v.A, DeepEquals, GetAZs(*String("")).StringList()) // old way still works
	buf, err = json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)

	inputBuf = `{"A": false}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "json: cannot unmarshal .*")

	// A single string where a string list is expected returns
	// a string list.
	inputBuf = `{"A": "asdf"}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, IsNil)
	buf, err = json.Marshal(v)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, `{"A":["asdf"]}`)

	inputBuf = `{"A": [false]}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, "json: cannot unmarshal .*")

	// Base64 is not available in stringlist context
	inputBuf = `{"A": {"Fn::Base64": "hello"}}`
	err = json.Unmarshal([]byte(inputBuf), &v)
	c.Assert(err, ErrorMatches, ".* is not a StringListFunc")
}
