# CloudWatch Alarm configuration in the Kubernetes Ingress Controller for AWS

The controller provides the option to create CloudWatch Alarms for the ALBs
created by it. These alarms can be configured in a ConfigMap.

## Setup

To make use of CloudWatch Alarms you need to provide the
`--cloudwatch-alarms-config-map` CLI flag to the controller and point it to a
ConfigMap resource consisting of namespace and name seperated by a `/`, e.g.:
`kube-system/kube-ingress-aws-controller-cw-alarms`. If the ConfigMap does not
exist, is malformed or the controller's ServiceAccount permissions prevent
access to it, the controller will just gracefully ignore it.

The controller will re-read the ConfigMap in the interval configured by the
`--polling-interval` flag (default: 30 seconds) and apply potential changes in
the alarm configuration to all load balancer Cloudformation Stacks.

Also make sure the IAM role of the controller includes the following
permissions required to [manage CloudWatch
Alarms](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/permissions-reference-cw.html#cw-permissions-table):

* `cloudwatch:DeleteAlarms`
* `cloudwatch:DescribeAlarms`
* `cloudwatch:DisableAlarmActions`
* `cloudwatch:EnableAlarmActions`
* `cloudwatch:PutMetricAlarm`


## Configuration

The CloudWatch Alarm configuration supports all properties listed in the
[Cloudformation documentation for the CloudWatch Alarm
resource](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cw-alarm.html#aws-properties-cw-alarm-syntax.yaml)
in YAML format with a few exceptions (see [below](#special-configuration-properties)). This is an example for a
ConfigMap that contains CloudWatch Alarm configuration:

```yaml
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: kube-ingress-aws-controller-cw-alarms
  namespace: kube-system
data:
  alarms: |
    - ActionsEnabled: true
      AlarmActions:
      - arn:aws:sns:region:account-id:sns-topic-name
      AlarmDescription: "There are no healthy backends"
      AlarmName: "no-healthy-backends"
      ComparisonOperator: LessThanThreshold
      Dimensions:
      - Name: LoadBalancer
      - Name: TargetGroup
      EvaluationPeriods: 1
      InsufficientDataActions: 
      - arn:aws:sns:region:account-id:sns-topic-name
      MetricName: HealthyHostCount
      OKActions:
      - arn:aws:sns:region:account-id:sns-topic-name
      Period: 60
      Statistic: Average
      Threshold: 1
      TreatMissingData: notBreaching
      Unit: Count
  some-other-config-key: |
    - AlarmDescription: "Increased rate of 5xx errors"
      ComparisonOperator: GreaterThanOrEqualToThreshold
      EvaluationPeriods: 1
      MetricName: HTTPCode_ELB_5XX_Count
      Period: 60
      Statistic: Sum
      Threshold: 100
      TreatMissingData: notBreaching
      Unit: Count
    - ActionsEnabled: true
      AlarmActions:
      - arn:aws:sns:region:account-id:sns-topic-name
      AlarmDescription: "Some description of the alarm"
      AlarmName: "and-other-alarm"
      [...]
```

Refer to the [CloudWatch Metrics documentation for
ALBs](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-cloudwatch-metrics.html#load-balancer-metrics-alb)
to get a list of available metrics that can be used to set up alarms.

The controller will treat all of the ConfigMap's data attributes as YAML arrays
containing CloudWatch Alarm configuration and will try to parse them as such.
The names of the keys can be arbitrary and are ignored. Keys not containing
valid YAML arrays matching the CloudWatch Alarm configuration structure will be
ignored.

### Special configuration properties

The following CloudWatch Alarm properties will receive special treatment:
* If set, `AlarmName` will be prefixed with the name of the Cloudformation
  Stack of each load balancer to avoid name clashes since AWS requires alarm
  names to be unique across the whole account.
* `DatapointsToAlarm` will be ignored if set as it is currently not supported
  by the cloudformation library used.
* If absent or empty, `Dimensions` will be set to
  `[{Name: LoadBalancer, Value: <generated-alb-name>}]`. If `LoadBalancer` or
  `TargetGroup` is used as the name of a dimension, its value will be replaced
  by the generated name of the load balancer/target group.
* If unset, `Namespace` will default to `AWS/ApplicationELB`.

## Alarm configuration versions

The controller keeps track of changes in the alarm configuration by adding the
tag `cloudwatch:alarm-config-hash` to the load balancer's CloudFormation Stack.
The tag's value is a checksum of the currently applied alarm configuration. The
CloudFormation Stack is updated once the hash of a newer alarm configuration
differs from this value.
