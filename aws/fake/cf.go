package fake

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

type CFOutputs struct {
	DescribeStackPages          *APIResponse
	DescribeStacks              *APIResponse
	CreateStack                 *APIResponse
	UpdateStack                 *APIResponse
	DeleteStack                 *APIResponse
	UpdateTerminationProtection *APIResponse
}

type CFClient struct {
	cloudformationiface.CloudFormationAPI
	lastStackTemplate string
	lastStackParams   []*cloudformation.Parameter
	lastStackTags     []*cloudformation.Tag
	Outputs           CFOutputs
}

func (m *CFClient) GetLastStackTemplate() string {
	return m.lastStackTemplate
}

func (m *CFClient) GetLastStackParams() []*cloudformation.Parameter {
	return m.lastStackParams
}

func (m *CFClient) GetLastStackTags() []*cloudformation.Tag {
	return m.lastStackTags
}

func (m *CFClient) DescribeStacksPages(in *cloudformation.DescribeStacksInput, fn func(*cloudformation.DescribeStacksOutput, bool) bool) (err error) {
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

func (m *CFClient) DescribeStacks(in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	out, ok := m.Outputs.DescribeStacks.response.(*cloudformation.DescribeStacksOutput)
	if !ok {
		return nil, m.Outputs.DescribeStacks.err
	}
	return out, m.Outputs.DescribeStacks.err
}

func (m *CFClient) CreateStack(params *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	m.lastStackTags = params.Tags
	m.lastStackParams = params.Parameters
	m.lastStackTemplate = *params.TemplateBody

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

func (m *CFClient) UpdateStack(params *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
	m.lastStackTags = params.Tags
	m.lastStackParams = params.Parameters
	m.lastStackTemplate = *params.TemplateBody

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

func (m *CFClient) DeleteStack(params *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	out, ok := m.Outputs.DeleteStack.response.(*cloudformation.DeleteStackOutput)
	if !ok {
		return nil, m.Outputs.DeleteStack.err
	}
	return out, m.Outputs.DeleteStack.err
}

func MockDeleteStackOutput(stackId string) *cloudformation.DeleteStackOutput {
	return &cloudformation.DeleteStackOutput{}
}

func (m *CFClient) UpdateTerminationProtection(params *cloudformation.UpdateTerminationProtectionInput) (*cloudformation.UpdateTerminationProtectionOutput, error) {
	out, ok := m.Outputs.UpdateTerminationProtection.response.(*cloudformation.UpdateTerminationProtectionOutput)
	if !ok {
		return nil, m.Outputs.UpdateTerminationProtection.err
	}
	return out, m.Outputs.UpdateTerminationProtection.err
}
