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
	Outputs CfMockOutputs
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
	if out, ok := m.Outputs.DescribeStacks.response.(*cloudformation.DescribeStacksOutput); ok {
		return out, m.Outputs.DescribeStacks.err
	}
	return nil, m.Outputs.DescribeStacks.err
}

func (m *MockCloudFormationClient) CreateStack(params *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	if out, ok := m.Outputs.CreateStack.response.(*cloudformation.CreateStackOutput); ok {
		return out, m.Outputs.CreateStack.err
	}
	return nil, m.Outputs.CreateStack.err
}

func MockCSOutput(stackId string) *cloudformation.CreateStackOutput {
	return &cloudformation.CreateStackOutput{
		StackId: aws.String(stackId),
	}
}

func (m *MockCloudFormationClient) UpdateStack(params *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
	if out, ok := m.Outputs.UpdateStack.response.(*cloudformation.UpdateStackOutput); ok {
		return out, m.Outputs.UpdateStack.err
	}
	return nil, m.Outputs.UpdateStack.err
}

func MockUSOutput(stackId string) *cloudformation.UpdateStackOutput {
	return &cloudformation.UpdateStackOutput{
		StackId: aws.String(stackId),
	}
}

func (m *MockCloudFormationClient) DeleteStack(params *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	if out, ok := m.Outputs.DeleteStack.response.(*cloudformation.DeleteStackOutput); ok {
		return out, m.Outputs.DeleteStack.err
	}
	return nil, m.Outputs.DeleteStack.err
}

func MockDeleteStackOutput(stackId string) *cloudformation.DeleteStackOutput {
	return &cloudformation.DeleteStackOutput{}
}

func (m *MockCloudFormationClient) UpdateTerminationProtection(params *cloudformation.UpdateTerminationProtectionInput) (*cloudformation.UpdateTerminationProtectionOutput, error) {
	if out, ok := m.Outputs.UpdateTerminationProtection.response.(*cloudformation.UpdateTerminationProtectionOutput); ok {
		return out, m.Outputs.UpdateTerminationProtection.err
	}
	return nil, m.Outputs.UpdateTerminationProtection.err
}
