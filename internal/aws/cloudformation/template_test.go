package cloudformation

import (
	"encoding/json"

	. "gopkg.in/check.v1"
)

type TemplateTest struct{}

var _ = Suite(&TemplateTest{})

func (testSuite *TemplateTest) TestNewTemplate(c *C) {
	tmpl := NewTemplate()
	c.Assert(tmpl.AWSTemplateFormatVersion, Equals, "2010-09-09")
	c.Assert(tmpl.Resources, NotNil)
	c.Assert(tmpl.Resources, HasLen, 0)
}

func (testSuite *TemplateTest) TestAddResource(c *C) {
	tmpl := NewTemplate()
	lb := &ElasticLoadBalancingV2LoadBalancer{Name: String("my-lb")}

	res := tmpl.AddResource("LoadBalancer", lb)

	c.Assert(res.Properties, Equals, ResourceProperties(lb))
	c.Assert(tmpl.Resources["LoadBalancer"], Equals, res)
}

func (testSuite *TemplateTest) TestResourceMarshalAndUnmarshalRoundTrip(c *C) {
	tmpl := NewTemplate()
	tmpl.AddResource("LoadBalancer", &ElasticLoadBalancingV2LoadBalancer{
		Name:   String("my-lb"),
		Scheme: String("internal"),
	})

	buf, err := json.Marshal(tmpl)
	c.Assert(err, IsNil)

	var decoded Template
	err = json.Unmarshal(buf, &decoded)
	c.Assert(err, IsNil)

	res := decoded.Resources["LoadBalancer"]
	c.Assert(res, NotNil)
	lb, ok := res.Properties.(*ElasticLoadBalancingV2LoadBalancer)
	c.Assert(ok, Equals, true)
	c.Assert(lb.Name, DeepEquals, String("my-lb"))
	c.Assert(lb.Scheme, DeepEquals, String("internal"))
}

func (testSuite *TemplateTest) TestResourceUnmarshalUnknownType(c *C) {
	buf := []byte(`{"Type":"AWS::Some::UnknownType","Properties":{}}`)

	var res Resource
	err := json.Unmarshal(buf, &res)
	c.Assert(err, ErrorMatches, "unknown resource type: AWS::Some::UnknownType")
}

func (testSuite *TemplateTest) TestResourceUnmarshalInvalidJSON(c *C) {
	var res Resource
	err := json.Unmarshal([]byte(`not-json`), &res)
	c.Assert(err, NotNil)
}

func (testSuite *TemplateTest) TestResourceUnmarshalMetadataAndDependsOn(c *C) {
	buf := []byte(`{
		"Type":"AWS::ElasticLoadBalancingV2::LoadBalancer",
		"DependsOn": ["OtherResource"],
		"DeletionPolicy": "Retain",
		"Metadata": {"foo": "bar"},
		"Properties": {"Name": "my-lb"}
	}`)

	var res Resource
	err := json.Unmarshal(buf, &res)
	c.Assert(err, IsNil)
	c.Assert(res.DeletionPolicy, Equals, "Retain")
	c.Assert(res.Metadata, DeepEquals, map[string]interface{}{"foo": "bar"})

	lb, ok := res.Properties.(*ElasticLoadBalancingV2LoadBalancer)
	c.Assert(ok, Equals, true)
	c.Assert(lb.Name, DeepEquals, String("my-lb"))
}

func (testSuite *TemplateTest) TestNewResourceByTypeKnown(c *C) {
	r := NewResourceByType("AWS::ElasticLoadBalancingV2::LoadBalancer")
	_, ok := r.(*ElasticLoadBalancingV2LoadBalancer)
	c.Assert(ok, Equals, true)
}

func (testSuite *TemplateTest) TestNewResourceByTypeUnknown(c *C) {
	r := NewResourceByType("AWS::Some::UnknownType")
	c.Assert(r, IsNil)
}

func (testSuite *TemplateTest) TestNewResourceByTypeCustomProvider(c *C) {
	type customResource struct {
		ElasticLoadBalancingV2LoadBalancer
	}

	RegisterCustomResourceProvider(func(customResourceType string) ResourceProperties {
		if customResourceType == "Custom::MyType" {
			return &customResource{}
		}
		return nil
	})

	r := NewResourceByType("Custom::MyType")
	_, ok := r.(*customResource)
	c.Assert(ok, Equals, true)

	c.Assert(NewResourceByType("Custom::NotRegistered"), IsNil)
}
