package aws

import (
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

type elbv2MockOutputs struct {
	registerTargets *apiResponse
}

type mockElbv2Client struct {
	elbv2iface.ELBV2API
	outputs  elbv2MockOutputs
	rtinputs []*elbv2.RegisterTargetsInput
}

func (m *mockElbv2Client) RegisterTargets(in *elbv2.RegisterTargetsInput) (*elbv2.RegisterTargetsOutput, error) {
	m.rtinputs = append(m.rtinputs, in)
	if out, ok := m.outputs.registerTargets.response.(*elbv2.RegisterTargetsOutput); ok {
		return out, m.outputs.registerTargets.err
	}
	return nil, m.outputs.registerTargets.err
}

func mockRTOutput() *elbv2.RegisterTargetsOutput {
	return &elbv2.RegisterTargetsOutput{}
}
