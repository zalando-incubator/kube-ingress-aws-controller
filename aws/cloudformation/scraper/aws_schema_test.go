package scraper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// Top level resources must comply with the ResourceProperties interface
const topLevelTemplate = `// CfnResourceType returns {{.AWSTypeName}} to implement the ResourceProperties interface
func (s {{.GoTypeName}}) CfnResourceType() string {
	{{if .IsCustomResource -}}
	if "" != s.ResourceTypeName {
 		return s.ResourceTypeName
	}
	{{- end}}
	return "{{.AWSTypeName}}"
}
`

// Non-top level properties must have a custom Unmarshaller to handle
// heterogeneous types
const nonTopLevelTemplate = `// {{.GoTypeName}}List represents a list of {{.GoTypeName}}
type {{.GoTypeName}}List []{{.GoTypeName}}

// UnmarshalJSON sets the object from the provided JSON representation
func (l *{{.GoTypeName}}List) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := {{.GoTypeName}}{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = {{.GoTypeName}}List{item}
		return nil
	}
	list := []{{.GoTypeName}}{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = {{.GoTypeName}}List(list)
		return nil
	}
	return err
}
`

// Typical transformations that Golint is going to complain about
// See https://github.com/golang/lint/blob/master/lint.go#L739
var golintTransformations = map[string]string{
	"Id":      "ID",
	"Ssh":     "SSH",
	"Api":     "API",
	"Url":     "URL",
	"Acl":     "ACL",
	"Ip":      "IP",
	"Tls":     "TLS",
	"Uri":     "URI",
	"Http":    "HTTP",
	"Dns":     "DNS",
	"Sql":     "SQL",
	"Ttl":     "TTL",
	"RamDisk": "RAMDisk",
	"Xss":     "XSS",
	"Cpu":     "CPU",
	"Json":    "JSON",
	"Vpc":     "VPC",
}

// download the latest CloudFormation JSON schema for the given AWS_REGION,
// defaulting to us-east-1 if that isn't set
func getLatestSchema(t *testing.T) string {

	tmpFile, tmpFileErr := ioutil.TempFile("", "cloudformation")
	if nil != tmpFileErr {
		t.Fatalf("Failed to create temp file")
	}
	defer tmpFile.Close()

	// URLs posted to: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-resource-specification.html
	schemaURL := ""
	switch os.Getenv("AWS_REGION") {
	case "us-east-2":
		schemaURL = "https://dnwj8swjjbsbt.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "us-west-1":
		schemaURL = "https://d68hl49wbnanq.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "us-west-2":
		schemaURL = "https://d201a2mn26r7lk.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "ap-south-1":
		schemaURL = "https://d2senuesg1djtx.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "ap-northeast-2":
		schemaURL = "https://d1ane3fvebulky.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "ap-southeast-1":
		schemaURL = "https://doigdx0kgq9el.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "ap-southeast-2":
		schemaURL = "https://d2stg8d246z9di.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "ap-northeast-1":
		schemaURL = "https://d33vqc0rt9ld30.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "eu-central-1":
		schemaURL = "https://d1mta8qj7i28i2.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "eu-west-1":
		schemaURL = "https://d3teyb21fexa9r.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "eu-west-2":
		schemaURL = "https://d1742qcu2c1ncx.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	case "sa-east-1":
		schemaURL = "https://d3c9jyj3w509b0.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	default:
		// Virginia
		schemaURL = "https://d1uauaxba7bl26.cloudfront.net/latest/gzip/CloudFormationResourceSpecification.json"
	}

	// Get the data
	resp, respErr := http.Get(schemaURL)
	if nil != respErr {
		t.Fatalf("Failed to download CloudFormation schema from: %s", schemaURL)
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, copyErr := io.Copy(tmpFile, resp.Body)
	if nil != copyErr {
		t.Fatalf("Failed to download CloudFormation schema from: %s. Error: %s", schemaURL, copyErr)
	}
	t.Logf("Downloaded %s schema to: %s", schemaURL, tmpFile.Name())
	return tmpFile.Name()
}

// Utility function to create an output file in the package
// root with the given name and contents
func writeOutputFile(t *testing.T, filename string, contents []byte) error {
	gopath := os.Getenv("GOPATH")
	if "" == gopath {
		gopath = os.ExpandEnv("${HOME}/go")
	}
	outputFilepath := filepath.Join(gopath,
		"src",
		"github.com",
		"mweagle",
		"go-cloudformation",
		filename)
	ioWriteErr := ioutil.WriteFile(outputFilepath, contents, 0644)
	if nil != ioWriteErr {
		t.Logf("WARN: Failed to write %s output\n", outputFilepath)
	} else {
		t.Logf("Created output file: %s\n", outputFilepath)
	}
	return ioWriteErr
}

////////////////////////////////////////////////////////////////////////////////
// Property Exporters
////////////////////////////////////////////////////////////////////////////////

func golintTransformedIdentifier(identifier string) string {
	canonicalName := identifier
	for eachMatch, eachReplacement := range golintTransformations {
		// Exception for anything with Identity - leave that the way it is
		// If this is a cognito identity thing, don't update the identity part
		canonicalName = strings.Replace(canonicalName, eachMatch, eachReplacement, -1)
		// If we changed "Identity" to "IDentity", then change it back
		canonicalName = strings.Replace(canonicalName, "IDentity", "Identity", -1)
	}
	return canonicalName
}

func canonicalGoTypename(t *testing.T, awsName string, isTopLevel bool) string {
	// If it's Tag, then it's Tag
	if "Tag" == awsName {
		return "Tag"
	}
	reSplit := regexp.MustCompile(`[:\.]+`)
	nameParts := reSplit.Split(awsName, -1)
	if len(nameParts) <= 1 {
		t.Fatalf("Failed to determine Golang typename for AWS name: %s", awsName)
	}
	// If the first element is "AWS", skip it
	if "AWS" == nameParts[0] {
		nameParts = nameParts[1:]
	}
	// AWS::ApiGatewayV2::Integration.ResponseParameterList
	// Special case "AWS::RDS::DBSecurityGroup.Ingress", which is defined
	// as both property and resource
	canonicalName := strings.Join(nameParts, "")
	if !isTopLevel {
		switch canonicalName {
		case "RDSDBSecurityGroupIngress",
			"EC2SecurityGroupIngress",
			"EC2SecurityGroupEgress",
			"ElasticLoadBalancingV2ListenerCertificate":
			canonicalName = fmt.Sprintf("%sProperty", canonicalName)
		case "KendraDataSourceSalesforceCustomKnowledgeArticleTypeConfigurationList",
			"KendraDataSourceSalesforceStandardObjectConfigurationList",
			"KendraDataSourceDataSourceToIndexFieldMappingList",
			"KendraIndexDocumentMetadataConfigurationList",
			"KendraIndexUserTokenConfigurationList":
			canonicalName = fmt.Sprintf("%sProperty", canonicalName)
		case "NetworkFirewallRuleGroupRulesSourceList":
			canonicalName = fmt.Sprintf("%sProperty", canonicalName)
		case "LicenseManagerGrantFilterList",
			"LicenseManagerLicenseEntitlementList",
			"LicenseManagerLicenseFilterList",
			"LicenseManagerLicenseMetadataList",
			"LicenseManagerLicenseRuleList":
			canonicalName = fmt.Sprintf("%sProperty", canonicalName)
		case "ApiGatewayV2IntegrationResponseParameterList":
			canonicalName = fmt.Sprintf("%sProperty", canonicalName)
		case "MediaConnectFlowSource":
			canonicalName = fmt.Sprintf("%sProperty", canonicalName)
		default:
			// NOP
		}
	}
	// Any transformations to apply?
	return golintTransformedIdentifier(canonicalName)
}

func writePropertyFieldDefinition(t *testing.T,
	cloudFormationPropertyTypeName string,
	propertyTypeName string,
	propertyTypeProperties PropertyTypeDefinition,
	isTopLevel bool,
	w io.Writer) {

	// String, Long, Integer, Double, Boolean, Timestamp or Json
	golangPrimitiveValueType := func(cloudformationType string) string {
		golangPrimitiveType := ""
		switch cloudformationType {
		case "String":
			golangPrimitiveType = "*StringExpr"
			if strings.HasSuffix(propertyTypeName, "Time") {
				golangPrimitiveType = "time.Time"
			}
		case "Timestamp":
			golangPrimitiveType = "time.Time"
		case "Boolean":
			golangPrimitiveType = "*BoolExpr"
		case "Integer", "Double", "Long":
			golangPrimitiveType = "*IntegerExpr"
		case "Json":
			golangPrimitiveType = "interface{}"
		case "Map":
			golangPrimitiveType = "interface{}"
		default:
			// Any chance it's another property reference?
			t.Fatalf("Can't determine Go primitive type for: %s\nName: %s\nProperties: %+v",
				cloudformationType,
				propertyTypeName,
				propertyTypeProperties)
		}
		return golangPrimitiveType
	}

	golangComplexValueType := func() string {
		internalTypeName := cloudFormationPropertyTypeName
		if strings.Contains(internalTypeName, ".") {
			nameParts := strings.Split(internalTypeName, ".")
			nameParts = nameParts[0 : len(nameParts)-1]
			internalTypeName = strings.Join(nameParts, "")
		}
		// Great, we have the prefix, one of these values should be non-empty
		// so that we can put it at the end and figure out
		// the name
		internalSubType := ""
		if "" != propertyTypeProperties.ItemType {
			internalSubType = propertyTypeProperties.ItemType
		} else if "" != propertyTypeProperties.Type.Scalar {
			internalSubType = propertyTypeProperties.Type.Scalar
		} else {
			t.Fatalf("Failed to find type for entry %s.%s", cloudFormationPropertyTypeName, propertyTypeName)
		}
		// push it, return the value
		fullInternalType := fmt.Sprintf("%s.%s", internalTypeName, internalSubType)
		return canonicalGoTypename(t, fullInternalType, false)
	}
	// Implementation
	golangType := ""
	if "" != propertyTypeProperties.Type.Scalar {
		// It's either a list, a map, or another property type
		switch propertyTypeProperties.Type.Scalar {
		case "List":
			{
				if "Tag" == propertyTypeProperties.ItemType {
					golangType = "*TagList"
				} else if "String" == propertyTypeProperties.ItemType ||
					"String" == propertyTypeProperties.PrimitiveItemType {
					golangType = "*StringListExpr"
				} else if "Json" == propertyTypeProperties.PrimitiveItemType ||
					"Json" == propertyTypeProperties.ItemType {
					golangType = "[]interface{}"
				} else if "Tag" == propertyTypeProperties.Type.Scalar {
					golangType = "interface{}"
				} else if "" != propertyTypeProperties.PrimitiveItemType {
					golangType = fmt.Sprintf("[]*%s", golangPrimitiveValueType(propertyTypeProperties.PrimitiveItemType))
				} else {
					// Create the internal type.
					golangType = fmt.Sprintf("*%s%s",
						golangComplexValueType(),
						propertyTypeProperties.Type.Scalar)

					// Special case the DBIngressRule, as the Go typename is both a
					// property name and a top level resource name
					if isTopLevel &&
						"AWS::RDS::DBSecurityGroup" == cloudFormationPropertyTypeName &&
						"DBSecurityGroupIngress" == propertyTypeName {
						golangType = canonicalGoTypename(t,
							fmt.Sprintf("%s.%s", cloudFormationPropertyTypeName, propertyTypeProperties.ItemType),
							false)
						// And add the list, since it's a list...
						golangType = fmt.Sprintf("%s%s", golangType, propertyTypeProperties.Type.Scalar)
					}
				}
			}
		case "Map":
			{
				golangType = "interface{}"
			}
		case "Tag":
			{
				golangType = "interface{}"
			}
		default:
			{
				// Subproperty name, which could be defined the context of an
				// existing property or a top level Resource
				golangType = fmt.Sprintf("*%s", golangComplexValueType())
			}
		}
	} else if "" != propertyTypeProperties.PrimitiveType {
		golangType = golangPrimitiveValueType(propertyTypeProperties.PrimitiveType)
	} else if propertyTypeProperties.Documentation == "http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecr-repository.html#cfn-ecr-repository-repositorypolicytext" ||
		propertyTypeProperties.Documentation == "http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-kms-key.html#cfn-kms-key-keypolicy" {
		golangType = golangPrimitiveValueType("Json")
	} else {
		t.Logf("WARNING: Failed to get Go type for %+v", propertyTypeProperties)
		golangType = "interface{}"
	}

	golintPropName := golintTransformedIdentifier(propertyTypeName)
	fmt.Fprintf(w, "\t// %s docs: %s\n", golintPropName, propertyTypeProperties.Documentation)

	// Validation tags
	validationTags := ""
	if propertyTypeProperties.Required {
		validationTags = " validate:\"dive,required\""
	}
	fmt.Fprintf(w,
		"\t%s %s `json:\"%s,omitempty\"%s`\n",
		golintPropName,
		golangType,
		propertyTypeName,
		validationTags)
}

func writePropertyDefinition(t *testing.T,
	cloudFormationPropertyTypeName string,
	propertyTypes map[string]PropertyTypeDefinition,
	documentationURL string,
	isTopLevel bool,
	w io.Writer) {

	// Sort the property names
	sortedPropertyNames := make([]string, 0)
	for eachName := range propertyTypes {
		sortedPropertyNames = append(sortedPropertyNames, eachName)
	}
	sort.Slice(sortedPropertyNames, func(lhs, rhs int) bool {
		return sortedPropertyNames[lhs] < sortedPropertyNames[rhs]
	})

	//Create the type struct entry
	golangTypename := canonicalGoTypename(t, cloudFormationPropertyTypeName, isTopLevel)
	modifierText := "resource type"
	if !isTopLevel {
		modifierText = "property type"
	}
	fmt.Fprintf(w, "// %s represents the %s CloudFormation %s\n",
		golangTypename,
		cloudFormationPropertyTypeName,
		modifierText)
	fmt.Fprintf(w, "// See %s \n", documentationURL)
	fmt.Fprintf(w, "type %s struct {\n", golangTypename)
	for _, eachSortedProp := range sortedPropertyNames {
		// Ensure that the first character in the name is capitalized...
		capName := strings.Title(eachSortedProp)
		writePropertyFieldDefinition(t,
			cloudFormationPropertyTypeName,
			capName,
			propertyTypes[eachSortedProp],
			isTopLevel,
			w)
	}

	// Special case the CustomResource
	// Issue: https://github.com/crewjam/go-cloudformation/issues/9
	if "AWS::CloudFormation::CustomResource" == cloudFormationPropertyTypeName {
		fmt.Fprintf(w, `
	// The user-defined Custom::* name to use for the resource.  If empty,
	// the default "AWS::CloudFormation::CustomResource" value will be used.
	// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-custom-resources.html
	ResourceTypeName string
`)
	}
	fmt.Fprintf(w, "}\n\n")

	// Write out the ResourceProperties function
	templateParams := struct {
		AWSTypeName      string
		GoTypeName       string
		IsCustomResource bool
	}{
		cloudFormationPropertyTypeName,
		golangTypename,
		cloudFormationPropertyTypeName == "AWS::CloudFormation::CustomResource",
	}

	// Property level items should always have Lists created for them
	templateData := topLevelTemplate
	if !isTopLevel {
		templateData = nonTopLevelTemplate
	}

	codeTemplate := template.Must(template.New("golang").Parse(templateData))
	templateErr := codeTemplate.Execute(w, templateParams)
	if nil != templateErr {
		t.Fatalf("Failed to expand JSON template: %s", templateErr)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Write Header
////////////////////////////////////////////////////////////////////////////////
func writeHeader(t *testing.T,
	buildID string,
	resourceSpecVersion string,
	w io.Writer) {

	headerText := fmt.Sprintf(`package cloudformation
// RESOURCE SPECIFICATION VERSION: %s
// SOURCE CODE SHA: %s
// GENERATED: %s
import "time"
import "encoding/json"
import _ "gopkg.in/go-playground/validator.v9" // Used for struct level validation tags

const ResourceSpecificationVersion = "%s"

// CustomResourceProvider allows extend the NewResourceByType factory method
// with their own resource types.
type CustomResourceProvider func(customResourceType string) ResourceProperties

var customResourceProviders []CustomResourceProvider

// RegisterCustomResourceProvider registers a custom resource provider with
// go-cloudformation. Multiple
// providers may be registered. The first provider that returns a non-nil
// interface will be used and there is no check for a uniquely registered
// resource type.
func RegisterCustomResourceProvider(provider CustomResourceProvider) {
	customResourceProviders = append(customResourceProviders, provider)
}
`,
		resourceSpecVersion,
		buildID,
		time.Now().UTC().String(),
		resourceSpecVersion)

	_, writeErr := w.Write([]byte(headerText))
	if nil != writeErr {
		t.Fatalf("Failed to write header: %s", writeErr)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Write referenced properties
////////////////////////////////////////////////////////////////////////////////
func writePropertyTypesDefinition(t *testing.T,
	propertyTypes map[string]PropertyTypes,
	w io.Writer) {

	// Sort the property names
	sortedPropertyNames := make([]string, 0)
	for eachName := range propertyTypes {
		sortedPropertyNames = append(sortedPropertyNames, eachName)
	}
	sort.Slice(sortedPropertyNames, func(lhs, rhs int) bool {
		return sortedPropertyNames[lhs] < sortedPropertyNames[rhs]
	})
	fmt.Fprintf(w, `
//
//  ____                            _   _
// |  _ \ _ __ ___  _ __   ___ _ __| |_(_) ___  ___
// | |_) | '__/ _ \| '_ \ / _ \ '__| __| |/ _ \/ __|
// |  __/| | | (_) | |_) |  __/ |  | |_| |  __/\__ \
// |_|   |_|  \___/| .__/ \___|_|   \__|_|\___||___/
//                 |_|
//

`)

	for _, eachPropertyName := range sortedPropertyNames {
		eachProp := propertyTypes[eachPropertyName]
		writePropertyDefinition(t, eachPropertyName, eachProp.Properties, eachProp.Documentation, false, w)
	}
}

// Utility function to sort the resource names
func sortedResourceNames(resourceTypes map[string]ResourceTypes) []string {
	// Sort the property names
	sortedResourceNames := make([]string, 0)
	for eachName := range resourceTypes {
		sortedResourceNames = append(sortedResourceNames, eachName)
	}
	sort.Slice(sortedResourceNames, func(lhs, rhs int) bool {
		return sortedResourceNames[lhs] < sortedResourceNames[rhs]
	})
	return sortedResourceNames
}

////////////////////////////////////////////////////////////////////////////////
// Write top level resources
////////////////////////////////////////////////////////////////////////////////
func writeResourceTypesDefinition(t *testing.T, resourceTypes map[string]ResourceTypes, w io.Writer) {
	sortedResourceNames := sortedResourceNames(resourceTypes)
	fmt.Fprintf(w, `
//
//  ____
// |  _ \ ___  ___  ___  _   _ _ __ ___ ___  ___
// | |_) / _ \/ __|/ _ \| | | | '__/ __/ _ \/ __|
// |  _ <  __/\__ \ (_) | |_| | | | (_|  __/\__ \
// |_| \_\___||___/\___/ \__,_|_|  \___\___||___/
//

`)
	for _, eachResourceName := range sortedResourceNames {
		eachResourceType := resourceTypes[eachResourceName]
		writePropertyDefinition(t,
			eachResourceName,
			eachResourceType.Properties,
			eachResourceType.Documentation,
			true,
			w)
		attrNames := make([]string, 0)
		for eachAttrName := range eachResourceType.Attributes {
			// Create the set of attributes for this type
			attrNames = append(attrNames, eachAttrName)
		}

		// Sort them...
		sort.Strings(attrNames)
		// Create the entry that produces the set of all attributes...
		golangTypename := canonicalGoTypename(t, eachResourceName, true)
		// Write out the function that returns these as a string
		fmt.Fprintln(w, "// CfnResourceAttributes returns the attributes produced by this resource")
		fmt.Fprintf(w, "func (s %s) CfnResourceAttributes() []string {\n", golangTypename)
		fmt.Fprintf(w, "	return []string{")
		for index, eachAttr := range attrNames {
			fmt.Fprintf(w, `"%s"`, eachAttr)
			if index < len(attrNames)-1 {
				fmt.Fprint(w, ",")
			}
		}
		fmt.Fprintf(w, "}\n")
		fmt.Fprintf(w, "}\n")
	}
}

////////////////////////////////////////////////////////////////////////////////
// Write footer properties
////////////////////////////////////////////////////////////////////////////////
func writeFactoryFooter(t *testing.T, resourceTypes map[string]ResourceTypes, w io.Writer) {
	fmt.Fprintf(w, `// NewResourceByType returns a new resource object correspoding with the provided type
func NewResourceByType(typeName string) ResourceProperties {
	switch typeName {
`)

	// Again, sort the names please
	sortedResourceNames := sortedResourceNames(resourceTypes)
	for _, eachName := range sortedResourceNames {
		fmt.Fprintf(w, `	case "%s":
		return &%s{}
`,
			eachName,
			canonicalGoTypename(t, eachName, true))
	}
	fmt.Fprintf(w, `
	default:
		for _, eachProvider := range customResourceProviders {
			customType := eachProvider(typeName)
			if nil != customType {
				return customType
			}
		}
	}
	return nil
}`)
}

////////////////////////////////////////////////////////////////////////////////
// ███████╗ ██████╗██╗  ██╗███████╗███╗   ███╗ █████╗
// ██╔════╝██╔════╝██║  ██║██╔════╝████╗ ████║██╔══██╗
// ███████╗██║     ███████║█████╗  ██╔████╔██║███████║
// ╚════██║██║     ██╔══██║██╔══╝  ██║╚██╔╝██║██╔══██║
// ███████║╚██████╗██║  ██║███████╗██║ ╚═╝ ██║██║  ██║
// ╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝╚═╝  ╚═╝
////////////////////////////////////////////////////////////////////////////////

func TestSchema(t *testing.T) {
	// Who are we?
	cmd := exec.Command("git",
		"rev-parse",
		"HEAD")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmdErr := cmd.Run()
	if cmdErr != nil {
		t.Error(cmdErr)
	}
	buildID := strings.TrimSpace(string(stdout.Bytes()))

	// Go get the latest JSON file
	schemaFile := getLatestSchema(t)
	schemaInput, schemaInputErr := ioutil.ReadFile(schemaFile)
	if nil != schemaInputErr {
		t.Error(schemaInputErr)
	}
	// Log the schema to output
	t.Logf("Latest CloudFormation Schema:\n%s", string(schemaInput))
	writeOutputFile(t, "schema.json", schemaInput)

	var data CloudFormationSchema
	unmarshalErr := json.Unmarshal(schemaInput, &data)
	if nil != unmarshalErr {
		t.Error(unmarshalErr)
	}
	// For each property, make the necessary property statement
	var output bytes.Buffer
	writeHeader(t, buildID, data.ResourceSpecificationVersion, &output)
	writePropertyTypesDefinition(t, data.PropertyTypes, &output)
	writeResourceTypesDefinition(t, data.ResourceTypes, &output)
	writeFactoryFooter(t, data.ResourceTypes, &output)

	// Write it out
	writeOutputFile(t, "schema.go", output.Bytes())

}
