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
	RollbackStack               *APIResponse
	UpdateTerminationProtection *APIResponse
}

type CFClient struct {
	cloudformationiface.CloudFormationAPI
	templateCreationHistory []string
	paramCreationHistory    [][]*cloudformation.Parameter
	tagCreationHistory      [][]*cloudformation.Tag
	Outputs                 CFOutputs
}

func (m *CFClient) GetTemplateCreationHistory() []string {
	return m.templateCreationHistory
}

func (m *CFClient) GetParamCreationHistory() [][]*cloudformation.Parameter {
	return m.paramCreationHistory
}

func (m *CFClient) GetTagCreationHistory() [][]*cloudformation.Tag {
	return m.tagCreationHistory
}

func (m *CFClient) CleanCreationHistory() {
	m.paramCreationHistory = [][]*cloudformation.Parameter{}
	m.tagCreationHistory = [][]*cloudformation.Tag{}
	m.templateCreationHistory = []string{}
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
	print("\n ======== CreateStack ======== \n")
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

func (m *CFClient) UpdateStack(params *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error) {
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
