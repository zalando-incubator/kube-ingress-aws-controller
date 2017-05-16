package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

type cfMockOutputs struct {
	describeStackPages *apiResponse
	describeStacks     *apiResponse
	createStack        *apiResponse
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
