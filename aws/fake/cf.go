package fake

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

type CfMockOutputs struct {
	DescribeStackPages          *ApiResponse
	DescribeStacks              *ApiResponse
	CreateStack                 *ApiResponse
	UpdateStack                 *ApiResponse
	DeleteStack                 *ApiResponse
	UpdateTerminationProtection *ApiResponse
}

type MockCloudFormationClient struct {
	cloudformationiface.CloudFormationAPI
	lastGeneratedTemplate string
	Outputs               CfMockOutputs
}

func (m *MockCloudFormationClient) GetLastGeneratedTemplate() string {
	return m.lastGeneratedTemplate
}

func (m *MockCloudFormationClient) DescribeStacksPages(in *cloudformation.DescribeStacksInput, fn func(*cloudformation.DescribeStacksOutput, bool) bool) (err error) {
	if m.Outputs.DescribeStackPages != nil {
		err = m.Outputs.DescribeStackPages.err
	}
	if err != nil {
		return
	}

	if out, ok := m.Outputs.DescribeStacks.response.(*cloudformation.DescribeStacksOutput); ok {
		fn(out, true)
	}
	if m.Outputs.DescribeStacks != nil {
		err = m.Outputs.DescribeStacks.err
	}

	return
}

func (m *MockCloudFormationClient) DescribeStacks(in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	out, ok := m.Outputs.DescribeStacks.response.(*cloudformation.DescribeStacksOutput)
	if !ok {
		return nil, m.Outputs.DescribeStacks.err
	}
	return out, m.Outputs.DescribeStacks.err
}

func (m *MockCloudFormationClient) CreateStack(params *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	m.lastGeneratedTemplate = *params.TemplateBody
	out, ok := m.Outputs.CreateStack.response.(*cloudformation.CreateStackOutput)
	if !ok {
		return nil, m.Outputs.CreateStack.err
	}
	return out, m.Outputs.CreateStack.err
}

func MockCSOutput(stackId string) *cloudformation.CreateStackOutput {
	return &cloudformation.CreateStackOutput{
		StackId: aws.String(stackId),
	}
}

func (m *MockCloudFormationClient) UpdateStack(params *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
	m.lastGeneratedTemplate = *params.TemplateBody
	out, ok := m.Outputs.UpdateStack.response.(*cloudformation.UpdateStackOutput)
	if !ok {
		return nil, m.Outputs.UpdateStack.err
	}
	return out, m.Outputs.UpdateStack.err
}

func MockUSOutput(stackId string) *cloudformation.UpdateStackOutput {
	return &cloudformation.UpdateStackOutput{
		StackId: aws.String(stackId),
	}
}

func (m *MockCloudFormationClient) DeleteStack(params *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	out, ok := m.Outputs.DeleteStack.response.(*cloudformation.DeleteStackOutput)
	if !ok {
		return nil, m.Outputs.DeleteStack.err
	}
	return out, m.Outputs.DeleteStack.err
}

func MockDeleteStackOutput(stackId string) *cloudformation.DeleteStackOutput {
	return &cloudformation.DeleteStackOutput{}
}

func (m *MockCloudFormationClient) UpdateTerminationProtection(params *cloudformation.UpdateTerminationProtectionInput) (*cloudformation.UpdateTerminationProtectionOutput, error) {
	out, ok := m.Outputs.UpdateTerminationProtection.response.(*cloudformation.UpdateTerminationProtectionOutput)
	if !ok {
		return nil, m.Outputs.UpdateTerminationProtection.err
	}
	return out, m.Outputs.UpdateTerminationProtection.err
}
