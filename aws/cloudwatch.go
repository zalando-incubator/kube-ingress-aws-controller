package aws

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/ghodss/yaml"
	cloudformation "github.com/mweagle/go-cloudformation"
	log "github.com/sirupsen/logrus"
)

// CloudWatchAlarmList represents a list of CloudWatch Alarms directly usable
// in Cloudformation Stacks.
type CloudWatchAlarmList []cloudformation.CloudWatchAlarm

// Hash computes a hash of the CloudWatchAlarmList which can be used to detect
// changes between two versions. The hash string will be empty if c is empty or
// there was an error while encoding.
func (c CloudWatchAlarmList) Hash() string {
	if len(c) == 0 {
		return ""
	}

	buf, err := json.Marshal(c)
	if err != nil {
		log.Errorf("failed to marshal cloudwatch alarm list: %v", err)
		return ""
	}

	hash := sha256.New()
	hash.Write(buf)

	return hex.EncodeToString(hash.Sum(nil))
}

// NewCloudWatchAlarmListFromYAML parses a raw slice of yaml bytes into a new
// CloudWatchAlarmList.
func NewCloudWatchAlarmListFromYAML(b []byte) (CloudWatchAlarmList, error) {
	alarmList := CloudWatchAlarmList{}

	err := yaml.Unmarshal(b, &alarmList)
	if err != nil {
		return nil, err
	}

	return alarmList, nil
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
		return &cloudformation.CloudWatchAlarmDimensionList{
			{
				Name:  cloudformation.String("LoadBalancer"),
				Value: cloudformation.GetAtt(LoadBalancerResourceLogicalID, "LoadBalancerFullName").String(),
			},
		}
	}

	dimensions := make(cloudformation.CloudWatchAlarmDimensionList, len(*alarmDimensions))

	for i, dimension := range *alarmDimensions {
		value := dimension.Value

		switch dimension.Name.Literal {
		case "LoadBalancer":
			value = cloudformation.GetAtt(LoadBalancerResourceLogicalID, "LoadBalancerFullName").String()
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
