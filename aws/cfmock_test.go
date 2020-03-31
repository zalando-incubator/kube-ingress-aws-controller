package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

type cfMockOutputs struct {
	describeStackPages          *apiResponse
	describeStacks              *apiResponse
	createStack                 *apiResponse
	updateStack                 *apiResponse
	deleteStack                 *apiResponse
	updateTerminationProtection *apiResponse
}

type mockCloudFormationClient struct {
	cloudformationiface.CloudFormationAPI
	outputs cfMockOutputs
}

func (m *mockCloudFormationClient) DescribeStacksPages(in *cloudformation.DescribeStacksInput, fn func(*cloudformation.DescribeStacksOutput, bool) bool) (err error) {
	if m.outputs.describeStackPages != nil {
		err = m.outputs.describeStackPages.err
	}
	if err != nil {
		return
	}

	if out, ok := m.outputs.describeStacks.response.(*cloudformation.DescribeStacksOutput); ok {
		fn(out, true)
	}
	if m.outputs.describeStacks != nil {
		err = m.outputs.describeStacks.err
	}

	return
}

func (m *mockCloudFormationClient) DescribeStacks(in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	if out, ok := m.outputs.describeStacks.response.(*cloudformation.DescribeStacksOutput); ok {
		return out, m.outputs.describeStacks.err
	}
	return nil, m.outputs.describeStacks.err
}

func (m *mockCloudFormationClient) CreateStack(params *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	if out, ok := m.outputs.createStack.response.(*cloudformation.CreateStackOutput); ok {
		return out, m.outputs.createStack.err
	}
	return nil, m.outputs.createStack.err
}

func mockCSOutput(stackId string) *cloudformation.CreateStackOutput {
	return &cloudformation.CreateStackOutput{
		StackId: aws.String(stackId),
	}
}

func (m *mockCloudFormationClient) UpdateStack(params *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
	if out, ok := m.outputs.updateStack.response.(*cloudformation.UpdateStackOutput); ok {
		return out, m.outputs.updateStack.err
	}
	return nil, m.outputs.updateStack.err
}

func mockUSOutput(stackId string) *cloudformation.UpdateStackOutput {
	return &cloudformation.UpdateStackOutput{
		StackId: aws.String(stackId),
	}
}

func (m *mockCloudFormationClient) DeleteStack(params *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	if out, ok := m.outputs.deleteStack.response.(*cloudformation.DeleteStackOutput); ok {
		return out, m.outputs.deleteStack.err
	}
	return nil, m.outputs.deleteStack.err
}

func mockDeleteStackOutput(stackId string) *cloudformation.DeleteStackOutput {
	return &cloudformation.DeleteStackOutput{}
}

func (m *mockCloudFormationClient) UpdateTerminationProtection(params *cloudformation.UpdateTerminationProtectionInput) (*cloudformation.UpdateTerminationProtectionOutput, error) {
	if out, ok := m.outputs.updateTerminationProtection.response.(*cloudformation.UpdateTerminationProtectionOutput); ok {
		return out, m.outputs.updateTerminationProtection.err
	}
	return nil, m.outputs.updateTerminationProtection.err
}
