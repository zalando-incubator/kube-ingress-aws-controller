package fake

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

type CFOutputs struct {
	DescribeStacks              *APIResponse
	ListStackResources          *APIResponse
	CreateStack                 *APIResponse
	UpdateStack                 *APIResponse
	DeleteStack                 *APIResponse
	RollbackStack               *APIResponse
	UpdateTerminationProtection *APIResponse
}

type CFClient struct {
	templateCreationHistory []string
	paramCreationHistory    [][]types.Parameter
	tagCreationHistory      [][]types.Tag
	Outputs                 CFOutputs
}

func (m *CFClient) GetTemplateCreationHistory() []string {
	return m.templateCreationHistory
}

func (m *CFClient) GetParamCreationHistory() [][]types.Parameter {
	return m.paramCreationHistory
}

func (m *CFClient) GetTagCreationHistory() [][]types.Tag {
	return m.tagCreationHistory
}

func (m *CFClient) CleanCreationHistory() {
	m.paramCreationHistory = [][]types.Parameter{}
	m.tagCreationHistory = [][]types.Tag{}
	m.templateCreationHistory = []string{}
}

func (m *CFClient) DescribeStacks(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	out, ok := m.Outputs.DescribeStacks.response.(*cloudformation.DescribeStacksOutput)
	if !ok {
		return nil, m.Outputs.DescribeStacks.err
	}
	return out, m.Outputs.DescribeStacks.err
}

func (m *CFClient) ListStackResources(ctx context.Context, params *cloudformation.ListStackResourcesInput, fn ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	out, ok := m.Outputs.ListStackResources.response.(*cloudformation.ListStackResourcesOutput)
	if !ok {
		return nil, m.Outputs.ListStackResources.err
	}
	return out, m.Outputs.ListStackResources.err
}

func MockDescribeStacksOutput(stackId *string) *cloudformation.DescribeStacksOutput {
	if stackId == nil {
		return &cloudformation.DescribeStacksOutput{
			Stacks: []types.Stack{},
		}
	}
	return &cloudformation.DescribeStacksOutput{
		Stacks: []types.Stack{
			{
				StackId: stackId,
			},
		},
	}
}

func (m *CFClient) CreateStack(ctx context.Context, params *cloudformation.CreateStackInput, fn ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
	m.tagCreationHistory = append(m.tagCreationHistory, params.Tags)
	m.paramCreationHistory = append(m.paramCreationHistory, params.Parameters)

	m.templateCreationHistory = append(m.templateCreationHistory, *params.TemplateBody)

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

func (m *CFClient) UpdateStack(context.Context, *cloudformation.UpdateStackInput, ...func(*cloudformation.Options)) (*cloudformation.UpdateStackOutput, error) {
	// TODO: https://github.com/zalando-incubator/kube-ingress-aws-controller/issues/653
	// Update stack needs to use different variable to register change history,
	// so createStack and updateStack mocks don't mess with each other states.

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

func (m *CFClient) DeleteStack(context.Context, *cloudformation.DeleteStackInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackOutput, error) {
	out, ok := m.Outputs.DeleteStack.response.(*cloudformation.DeleteStackOutput)
	if !ok {
		return nil, m.Outputs.DeleteStack.err
	}
	return out, m.Outputs.DeleteStack.err
}

func MockDeleteStackOutput(stackId string) *cloudformation.DeleteStackOutput {
	return &cloudformation.DeleteStackOutput{}
}

func (m *CFClient) UpdateTerminationProtection(context.Context, *cloudformation.UpdateTerminationProtectionInput, ...func(*cloudformation.Options)) (*cloudformation.UpdateTerminationProtectionOutput, error) {
	out, ok := m.Outputs.UpdateTerminationProtection.response.(*cloudformation.UpdateTerminationProtectionOutput)
	if !ok {
		return nil, m.Outputs.UpdateTerminationProtection.err
	}
	return out, m.Outputs.UpdateTerminationProtection.err
}

func (m *CFClient) RollbackStack(params *cloudformation.RollbackStackInput) (*cloudformation.RollbackStackOutput, error) {
	out, ok := m.Outputs.RollbackStack.response.(*cloudformation.RollbackStackOutput)
	if !ok {
		return nil, m.Outputs.RollbackStack.err
	}
	return out, m.Outputs.RollbackStack.err
}

func MockRollbackStackOutput(stackId string) *cloudformation.RollbackStackOutput {
	return &cloudformation.RollbackStackOutput{
		StackId: aws.String(stackId),
	}
}
