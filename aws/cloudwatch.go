package aws

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/ghodss/yaml"
	cloudformation "github.com/mweagle/go-cloudformation"
)

// CloudWatchAlarmList represents a list of CloudWatch Alarms directly usable
// in Cloudformation Stacks.
type CloudWatchAlarmList []cloudformation.CloudWatchAlarm

// Hash computes a hash of the CloudWatchAlarmList which can be used to detect
// changes between two versions.
func (c CloudWatchAlarmList) Hash() string {
	if len(c) == 0 {
		return ""
	}

	hash := sha256.New()

	buf, _ := json.Marshal(c)

	hash.Write(buf)

	return hex.EncodeToString(hash.Sum(nil))
}

// NewCloudWatchAlarmListFromYAML parses a raw slice of yaml bytes into a new
// CloudWatchAlarmList.
func NewCloudWatchAlarmListFromYAML(b []byte) (CloudWatchAlarmList, error) {
	config := CloudWatchAlarmList{}

	err := yaml.Unmarshal(b, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// normalizeCloudWatchAlarmName prefixes the alarm name (if it is non-nil) with
// the stack name to minimize the probability for collisions as the alarm name
// has to be unique across the AWS account.
func normalizeCloudWatchAlarmName(alarmName *cloudformation.StringExpr) *cloudformation.StringExpr {
	if alarmName != nil {
		return cloudformation.Join("-", cloudformation.Ref("AWS::StackName"), alarmName)
	}

	return alarmName
}

// normalizeCloudWatchAlarmNamespace sets the alarm namespace to sane default
// (AWS/ApplicationELB) if it is not set.
func normalizeCloudWatchAlarmNamespace(alarmNamespace *cloudformation.StringExpr) *cloudformation.StringExpr {
	if alarmNamespace == nil {
		return cloudformation.String("AWS/ApplicationELB")
	}

	return alarmNamespace
}

// normalizeCloudWatchAlarmDimensions replaces the values of the dimensions
// LoadBalancer and TargetGroup (if set) with the generated names of these
// resources. If alarmDimensions is nil or empty, it will return a sane default
// that includes the dimension LoadBalancer with its generated name.
func normalizeCloudWatchAlarmDimensions(alarmDimensions *cloudformation.CloudWatchAlarmDimensionList) *cloudformation.CloudWatchAlarmDimensionList {
	if alarmDimensions == nil || len(*alarmDimensions) == 0 {
		// For convenience, include LoadBalancer in the dimensions if the user
		// provided nothing in the configuration. This is the dimension we want
		// to use most of the time.
		return &cloudformation.CloudWatchAlarmDimensionList{
			{
				Name:  cloudformation.String("LoadBalancer"),
				Value: cloudformation.GetAtt("LB", "LoadBalancerFullName").String(),
			},
		}
	}

	dimensions := make(cloudformation.CloudWatchAlarmDimensionList, len(*alarmDimensions))

	for i, dimension := range *alarmDimensions {
		value := dimension.Value

		switch dimension.Name.Literal {
		case "LoadBalancer":
			value = cloudformation.GetAtt("LB", "LoadBalancerFullName").String()
		case "TargetGroup":
			value = cloudformation.GetAtt("TG", "TargetGroupFullName").String()
		}

		dimensions[i] = cloudformation.CloudWatchAlarmDimension{
			Name:  dimension.Name,
			Value: value,
		}
	}

	return &dimensions
}
