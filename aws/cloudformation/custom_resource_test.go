package cloudformation

import (
	"encoding/json"
	"testing"
)

type MyResource struct {
	CloudFormationCustomResource
	Foo *StringExpr
}

func customTypeProvider(resourceType string) ResourceProperties {
	switch resourceType {
	case "Custom::MyResource":
		return &MyResource{}
	}
	return nil
}

func init() {
	RegisterCustomResourceProvider(customTypeProvider)
}

func TestCustomResource(t *testing.T) {
	myResourceInstance, ok := NewResourceByType("Custom::MyResource").(*MyResource)
	if !ok {
		t.Fatalf("Failed to call user provided CustomResourceProvider")
	}
	myResourceInstance.ServiceToken = String("arn:aws:sns:us-east-1:84969EXAMPLE:CRTest")
	myResourceInstance.Foo = String("Hello World")

	templ := NewTemplate()
	templ.Description = "Test Custom Resource"
	templ.AddResource("MyCustomResource", myResourceInstance)

	// Verify marshaled results
	output, err := json.Marshal(templ)
	if err != nil {
		t.Fatalf("marshal: %s", err)
	}
	parsedOutput := map[string]interface{}{}
	json.Unmarshal(output, &parsedOutput)

	resources := parsedOutput["Resources"].(map[string]interface{})
	customResource := resources["MyCustomResource"].(map[string]interface{})
	properties := customResource["Properties"].(map[string]interface{})
	if "" == properties["ServiceToken"].(string) {
		t.Fatalf("Properties.ServiceToken is empty")
	}
	if "" == properties["Foo"].(string) {
		t.Fatalf("Properties.Foo is empty")
	}
}
