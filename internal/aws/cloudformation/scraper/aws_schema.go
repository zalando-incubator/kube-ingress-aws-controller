package scraper

import (
	"encoding/json"
	"fmt"
)

// See:
// * http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-resource-specification-format.html and
// * http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-resource-specification.html
// for more information

// CloudFormationSchema represents the root of the
// schema
type CloudFormationSchema struct {
	PropertyTypes                map[string]PropertyTypes
	ResourceTypes                map[string]ResourceTypes
	ResourceSpecificationVersion string
}

// PropertyTypes is a definition of a property
type PropertyTypes struct {
	Documentation string
	Properties    map[string]PropertyTypeDefinition
}

// ResourceTypes is a definition of a resource
type ResourceTypes struct {
	Documentation string
	Attributes    map[string]ResourceAttribute
	Properties    map[string]PropertyTypeDefinition
}

// ResourceAttribute are outputs of CloudFormation
// reosurce
type ResourceAttribute struct {
	PrimitiveType string
}

// PropertyItemType represents the type of a property
type PropertyItemType struct {
	Scalar      string
	MultiValues []string
}

// MarshalJSON to handle whichever field is set
func (piType *PropertyItemType) MarshalJSON() ([]byte, error) {
	var value interface{}
	if len(piType.Scalar) != 0 {
		value = piType.Scalar
	} else {
		value = piType.MultiValues
	}
	return json.Marshal(value)
}

// UnmarshalJSON does the custom unmarshalling
func (piType *PropertyItemType) UnmarshalJSON(data []byte) error {
	singleString := ""
	singleErr := json.Unmarshal(data, &singleString)
	if singleErr == nil {
		piType.Scalar = singleString
		return nil
	}
	var stringArray []string
	sliceErr := json.Unmarshal(data, &stringArray)
	if sliceErr == nil {
		piType.MultiValues = stringArray
		return nil
	}
	return fmt.Errorf("Failed to unmarshal type: " + sliceErr.Error())
}

// PropertyTypeDefinition is the definition of a property
type PropertyTypeDefinition struct {
	Required          bool
	Documentation     string
	PrimitiveType     string
	UpdateType        string
	Type              PropertyItemType
	DuplicatesAllowed bool
	ItemType          string
	PrimitiveItemType string
}
