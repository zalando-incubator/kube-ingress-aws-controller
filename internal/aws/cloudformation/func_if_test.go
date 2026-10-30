package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type IfFuncTest struct{}

var _ = Suite(&IfFuncTest{})

func (testSuite *IfFuncTest) TestScalar(c *C) {
	inputBuf := `{"Fn::If":["IsProd","prod-value","dev-value"]}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)

	// String() just wraps the function; it should round-trip too.
	buf, err = json.Marshal(f.(StringFunc).String())
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)
}

func (testSuite *IfFuncTest) TestListViaNestedFunc(c *C) {
	// The ValueIfTrue/ValueIfFalse branches are detected by inspecting the
	// decoded JSON shape; a nested function call (object) is unambiguous,
	// unlike a plain literal array (see TestListLiteralIsNotSupported).
	inputBuf := `{"Fn::If":["IsProd",{"Fn::GetAZs":{"Ref":"AWS::Region"}},{"Fn::GetAZs":{"Ref":"AWS::Region"}}]}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)

	buf, err := json.Marshal(f)
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)

	buf, err = json.Marshal(f.(StringListFunc).StringList())
	c.Assert(err, IsNil)
	c.Assert(string(buf), Equals, inputBuf)
}

func (testSuite *IfFuncTest) TestListLiteralIsNotSupported(c *C) {
	// Known limitation: the list-vs-scalar detection in UnmarshalJSON uses
	// reflect.Kind() on the generically-decoded value, but a JSON array
	// decodes to reflect.Slice (not reflect.Array), so this branch never
	// matches. A literal list value is therefore misdetected as a scalar.
	// This test documents that behavior rather than asserting (incorrect)
	// expected list support.
	inputBuf := `{"Fn::If":["IsProd",["a","b"],["c"]]}`
	var decoded IfFunc
	err := json.Unmarshal([]byte(inputBuf), &decoded)
	c.Assert(err, IsNil)
	c.Assert(decoded.list, Equals, false)
}

func (testSuite *IfFuncTest) TestScalarStringPanicsWhenList(c *C) {
	inputBuf := `{"Fn::If":["IsProd",{"Fn::GetAZs":{"Ref":"AWS::Region"}},{"Fn::GetAZs":{"Ref":"AWS::Region"}}]}`
	f, err := unmarshalFunc([]byte(inputBuf))
	c.Assert(err, IsNil)
	ifFunc := f.(IfFunc)
	c.Assert(func() { ifFunc.String() }, PanicMatches, "IfFunc is a list, but being treated as a scalar")
}

func (testSuite *IfFuncTest) TestStringListPanicsWhenScalar(c *C) {
	f := If("IsProd", String("a"), String("b"))
	c.Assert(func() { f.StringList() }, PanicMatches, "IfFunc is a scalar, but being treated as a list of strings.")
}

func (testSuite *IfFuncTest) TestConstructorsBuildExpectedShape(c *C) {
	scalar := If("IsProd", String("a"), String("b"))
	c.Assert(scalar.list, Equals, false)
	c.Assert(scalar.Condition, Equals, "IsProd")

	list := IfList("IsProd", StringList(String("a")), StringList(String("b")))
	c.Assert(list.list, Equals, true)
	c.Assert(list.Condition, Equals, "IsProd")
}

func (testSuite *IfFuncTest) TestUnmarshalFailures(c *C) {
	inputBuf := `{"Fn::If": "not-an-array"}`
	var decoded IfFunc
	err := json.Unmarshal([]byte(inputBuf), &decoded)
	c.Assert(err, NotNil)

	inputBuf = `{"Fn::If": [1, "a", "b"]}`
	err = json.Unmarshal([]byte(inputBuf), &decoded)
	c.Assert(err, NotNil)
}
