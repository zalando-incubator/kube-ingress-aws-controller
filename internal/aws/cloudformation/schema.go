package cloudformation

// RESOURCE SPECIFICATION VERSION: 206.0.0
import "time"
import "encoding/json"
import _ "gopkg.in/go-playground/validator.v9" // Used for struct level validation tags

const ResourceSpecificationVersion = "206.0.0"

var _ = time.Now

// CustomResourceProvider allows extend the NewResourceByType factory method
// with their own resource types.
type CustomResourceProvider func(customResourceType string) ResourceProperties

var customResourceProviders []CustomResourceProvider

// RegisterCustomResourceProvider registers a custom resource provider with
// go-cloudformation. Multiple
// providers may be registered. The first provider that returns a non-nil
// interface will be used and there is no check for a uniquely registered
// resource type.
func RegisterCustomResourceProvider(provider CustomResourceProvider) {
	customResourceProviders = append(customResourceProviders, provider)
}

//
//  ____                            _   _
// |  _ \ _ __ ___  _ __   ___ _ __| |_(_) ___  ___
// | |_) | '__/ _ \| '_ \ / _ \ '__| __| |/ _ \/ __|
// |  __/| | | (_) | |_) |  __/ |  | |_| |  __/\__ \
// |_|   |_|  \___/| .__/ \___|_|   \__|_|\___||___/
//                 |_|
//

// CloudWatchAlarmDimension represents the AWS::CloudWatch::Alarm.Dimension CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-dimension.html
type CloudWatchAlarmDimension struct {
	// Name docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-dimension.html#cfn-cloudwatch-alarm-dimension-name
	Name *StringExpr `json:"Name,omitempty" validate:"dive,required"`
	// Value docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-dimension.html#cfn-cloudwatch-alarm-dimension-value
	Value *StringExpr `json:"Value,omitempty" validate:"dive,required"`
}

// CloudWatchAlarmDimensionList represents a list of CloudWatchAlarmDimension
type CloudWatchAlarmDimensionList []CloudWatchAlarmDimension

// UnmarshalJSON sets the object from the provided JSON representation
func (l *CloudWatchAlarmDimensionList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := CloudWatchAlarmDimension{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = CloudWatchAlarmDimensionList{item}
		return nil
	}
	list := []CloudWatchAlarmDimension{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = CloudWatchAlarmDimensionList(list)
		return nil
	}
	return err
}

// CloudWatchAlarmMetric represents the AWS::CloudWatch::Alarm.Metric CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metric.html
type CloudWatchAlarmMetric struct {
	// Dimensions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metric.html#cfn-cloudwatch-alarm-metric-dimensions
	Dimensions *CloudWatchAlarmDimensionList `json:"Dimensions,omitempty"`
	// MetricName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metric.html#cfn-cloudwatch-alarm-metric-metricname
	MetricName *StringExpr `json:"MetricName,omitempty"`
	// Namespace docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metric.html#cfn-cloudwatch-alarm-metric-namespace
	Namespace *StringExpr `json:"Namespace,omitempty"`
}

// CloudWatchAlarmMetricList represents a list of CloudWatchAlarmMetric
type CloudWatchAlarmMetricList []CloudWatchAlarmMetric

// UnmarshalJSON sets the object from the provided JSON representation
func (l *CloudWatchAlarmMetricList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := CloudWatchAlarmMetric{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = CloudWatchAlarmMetricList{item}
		return nil
	}
	list := []CloudWatchAlarmMetric{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = CloudWatchAlarmMetricList(list)
		return nil
	}
	return err
}

// CloudWatchAlarmMetricDataQuery represents the AWS::CloudWatch::Alarm.MetricDataQuery CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html
type CloudWatchAlarmMetricDataQuery struct {
	// AccountID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html#cfn-cloudwatch-alarm-metricdataquery-accountid
	AccountID *StringExpr `json:"AccountId,omitempty"`
	// Expression docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html#cfn-cloudwatch-alarm-metricdataquery-expression
	Expression *StringExpr `json:"Expression,omitempty"`
	// ID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html#cfn-cloudwatch-alarm-metricdataquery-id
	ID *StringExpr `json:"Id,omitempty" validate:"dive,required"`
	// Label docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html#cfn-cloudwatch-alarm-metricdataquery-label
	Label *StringExpr `json:"Label,omitempty"`
	// MetricStat docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html#cfn-cloudwatch-alarm-metricdataquery-metricstat
	MetricStat *CloudWatchAlarmMetricStat `json:"MetricStat,omitempty"`
	// Period docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html#cfn-cloudwatch-alarm-metricdataquery-period
	Period *IntegerExpr `json:"Period,omitempty"`
	// ReturnData docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricdataquery.html#cfn-cloudwatch-alarm-metricdataquery-returndata
	ReturnData *BoolExpr `json:"ReturnData,omitempty"`
}

// CloudWatchAlarmMetricDataQueryList represents a list of CloudWatchAlarmMetricDataQuery
type CloudWatchAlarmMetricDataQueryList []CloudWatchAlarmMetricDataQuery

// UnmarshalJSON sets the object from the provided JSON representation
func (l *CloudWatchAlarmMetricDataQueryList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := CloudWatchAlarmMetricDataQuery{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = CloudWatchAlarmMetricDataQueryList{item}
		return nil
	}
	list := []CloudWatchAlarmMetricDataQuery{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = CloudWatchAlarmMetricDataQueryList(list)
		return nil
	}
	return err
}

// CloudWatchAlarmMetricStat represents the AWS::CloudWatch::Alarm.MetricStat CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricstat.html
type CloudWatchAlarmMetricStat struct {
	// Metric docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricstat.html#cfn-cloudwatch-alarm-metricstat-metric
	Metric *CloudWatchAlarmMetric `json:"Metric,omitempty" validate:"dive,required"`
	// Period docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricstat.html#cfn-cloudwatch-alarm-metricstat-period
	Period *IntegerExpr `json:"Period,omitempty" validate:"dive,required"`
	// Stat docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricstat.html#cfn-cloudwatch-alarm-metricstat-stat
	Stat *StringExpr `json:"Stat,omitempty" validate:"dive,required"`
	// Unit docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudwatch-alarm-metricstat.html#cfn-cloudwatch-alarm-metricstat-unit
	Unit *StringExpr `json:"Unit,omitempty"`
}

// CloudWatchAlarmMetricStatList represents a list of CloudWatchAlarmMetricStat
type CloudWatchAlarmMetricStatList []CloudWatchAlarmMetricStat

// UnmarshalJSON sets the object from the provided JSON representation
func (l *CloudWatchAlarmMetricStatList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := CloudWatchAlarmMetricStat{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = CloudWatchAlarmMetricStatList{item}
		return nil
	}
	list := []CloudWatchAlarmMetricStat{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = CloudWatchAlarmMetricStatList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerAction represents the AWS::ElasticLoadBalancingV2::Listener.Action CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html
type ElasticLoadBalancingV2ListenerAction struct {
	// AuthenticateCognitoConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-authenticatecognitoconfig
	AuthenticateCognitoConfig *ElasticLoadBalancingV2ListenerAuthenticateCognitoConfig `json:"AuthenticateCognitoConfig,omitempty"`
	// AuthenticateOidcConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-authenticateoidcconfig
	AuthenticateOidcConfig *ElasticLoadBalancingV2ListenerAuthenticateOidcConfig `json:"AuthenticateOidcConfig,omitempty"`
	// FixedResponseConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-fixedresponseconfig
	FixedResponseConfig *ElasticLoadBalancingV2ListenerFixedResponseConfig `json:"FixedResponseConfig,omitempty"`
	// ForwardConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-forwardconfig
	ForwardConfig *ElasticLoadBalancingV2ListenerForwardConfig `json:"ForwardConfig,omitempty"`
	// Order docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-order
	Order *IntegerExpr `json:"Order,omitempty"`
	// RedirectConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-redirectconfig
	RedirectConfig *ElasticLoadBalancingV2ListenerRedirectConfig `json:"RedirectConfig,omitempty"`
	// TargetGroupArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-targetgrouparn
	TargetGroupArn *StringExpr `json:"TargetGroupArn,omitempty"`
	// Type docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-action.html#cfn-elasticloadbalancingv2-listener-action-type
	Type *StringExpr `json:"Type,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerActionList represents a list of ElasticLoadBalancingV2ListenerAction
type ElasticLoadBalancingV2ListenerActionList []ElasticLoadBalancingV2ListenerAction

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerActionList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerAction{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerActionList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerAction{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerActionList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerAuthenticateCognitoConfig represents the AWS::ElasticLoadBalancingV2::Listener.AuthenticateCognitoConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html
type ElasticLoadBalancingV2ListenerAuthenticateCognitoConfig struct {
	// AuthenticationRequestExtraParams docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-authenticationrequestextraparams
	AuthenticationRequestExtraParams interface{} `json:"AuthenticationRequestExtraParams,omitempty"`
	// OnUnauthenticatedRequest docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-onunauthenticatedrequest
	OnUnauthenticatedRequest *StringExpr `json:"OnUnauthenticatedRequest,omitempty"`
	// Scope docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-scope
	Scope *StringExpr `json:"Scope,omitempty"`
	// SessionCookieName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-sessioncookiename
	SessionCookieName *StringExpr `json:"SessionCookieName,omitempty"`
	// SessionTimeout docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-sessiontimeout
	SessionTimeout *StringExpr `json:"SessionTimeout,omitempty"`
	// UserPoolArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-userpoolarn
	UserPoolArn *StringExpr `json:"UserPoolArn,omitempty" validate:"dive,required"`
	// UserPoolClientID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-userpoolclientid
	UserPoolClientID *StringExpr `json:"UserPoolClientId,omitempty" validate:"dive,required"`
	// UserPoolDomain docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listener-authenticatecognitoconfig-userpooldomain
	UserPoolDomain *StringExpr `json:"UserPoolDomain,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerAuthenticateCognitoConfigList represents a list of ElasticLoadBalancingV2ListenerAuthenticateCognitoConfig
type ElasticLoadBalancingV2ListenerAuthenticateCognitoConfigList []ElasticLoadBalancingV2ListenerAuthenticateCognitoConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerAuthenticateCognitoConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerAuthenticateCognitoConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerAuthenticateCognitoConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerAuthenticateCognitoConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerAuthenticateCognitoConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerAuthenticateOidcConfig represents the AWS::ElasticLoadBalancingV2::Listener.AuthenticateOidcConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html
type ElasticLoadBalancingV2ListenerAuthenticateOidcConfig struct {
	// AuthenticationRequestExtraParams docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-authenticationrequestextraparams
	AuthenticationRequestExtraParams interface{} `json:"AuthenticationRequestExtraParams,omitempty"`
	// AuthorizationEndpoint docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-authorizationendpoint
	AuthorizationEndpoint *StringExpr `json:"AuthorizationEndpoint,omitempty" validate:"dive,required"`
	// ClientID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-clientid
	ClientID *StringExpr `json:"ClientId,omitempty" validate:"dive,required"`
	// ClientSecret docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-clientsecret
	ClientSecret *StringExpr `json:"ClientSecret,omitempty"`
	// Issuer docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-issuer
	Issuer *StringExpr `json:"Issuer,omitempty" validate:"dive,required"`
	// OnUnauthenticatedRequest docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-onunauthenticatedrequest
	OnUnauthenticatedRequest *StringExpr `json:"OnUnauthenticatedRequest,omitempty"`
	// Scope docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-scope
	Scope *StringExpr `json:"Scope,omitempty"`
	// SessionCookieName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-sessioncookiename
	SessionCookieName *StringExpr `json:"SessionCookieName,omitempty"`
	// SessionTimeout docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-sessiontimeout
	SessionTimeout *StringExpr `json:"SessionTimeout,omitempty"`
	// TokenEndpoint docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-tokenendpoint
	TokenEndpoint *StringExpr `json:"TokenEndpoint,omitempty" validate:"dive,required"`
	// UseExistingClientSecret docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-useexistingclientsecret
	UseExistingClientSecret *BoolExpr `json:"UseExistingClientSecret,omitempty"`
	// UserInfoEndpoint docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listener-authenticateoidcconfig-userinfoendpoint
	UserInfoEndpoint *StringExpr `json:"UserInfoEndpoint,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerAuthenticateOidcConfigList represents a list of ElasticLoadBalancingV2ListenerAuthenticateOidcConfig
type ElasticLoadBalancingV2ListenerAuthenticateOidcConfigList []ElasticLoadBalancingV2ListenerAuthenticateOidcConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerAuthenticateOidcConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerAuthenticateOidcConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerAuthenticateOidcConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerAuthenticateOidcConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerAuthenticateOidcConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerCertificateProperty represents the AWS::ElasticLoadBalancingV2::Listener.Certificate CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-certificate.html
type ElasticLoadBalancingV2ListenerCertificateProperty struct {
	// CertificateArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-certificate.html#cfn-elasticloadbalancingv2-listener-certificate-certificatearn
	CertificateArn *StringExpr `json:"CertificateArn,omitempty"`
}

// ElasticLoadBalancingV2ListenerCertificatePropertyList represents a list of ElasticLoadBalancingV2ListenerCertificateProperty
type ElasticLoadBalancingV2ListenerCertificatePropertyList []ElasticLoadBalancingV2ListenerCertificateProperty

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerCertificatePropertyList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerCertificateProperty{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerCertificatePropertyList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerCertificateProperty{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerCertificatePropertyList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerFixedResponseConfig represents the AWS::ElasticLoadBalancingV2::Listener.FixedResponseConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-fixedresponseconfig.html
type ElasticLoadBalancingV2ListenerFixedResponseConfig struct {
	// ContentType docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-fixedresponseconfig.html#cfn-elasticloadbalancingv2-listener-fixedresponseconfig-contenttype
	ContentType *StringExpr `json:"ContentType,omitempty"`
	// MessageBody docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-fixedresponseconfig.html#cfn-elasticloadbalancingv2-listener-fixedresponseconfig-messagebody
	MessageBody *StringExpr `json:"MessageBody,omitempty"`
	// StatusCode docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-fixedresponseconfig.html#cfn-elasticloadbalancingv2-listener-fixedresponseconfig-statuscode
	StatusCode *StringExpr `json:"StatusCode,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerFixedResponseConfigList represents a list of ElasticLoadBalancingV2ListenerFixedResponseConfig
type ElasticLoadBalancingV2ListenerFixedResponseConfigList []ElasticLoadBalancingV2ListenerFixedResponseConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerFixedResponseConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerFixedResponseConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerFixedResponseConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerFixedResponseConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerFixedResponseConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerForwardConfig represents the AWS::ElasticLoadBalancingV2::Listener.ForwardConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-forwardconfig.html
type ElasticLoadBalancingV2ListenerForwardConfig struct {
	// TargetGroupStickinessConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-forwardconfig.html#cfn-elasticloadbalancingv2-listener-forwardconfig-targetgroupstickinessconfig
	TargetGroupStickinessConfig *ElasticLoadBalancingV2ListenerTargetGroupStickinessConfig `json:"TargetGroupStickinessConfig,omitempty"`
	// TargetGroups docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-forwardconfig.html#cfn-elasticloadbalancingv2-listener-forwardconfig-targetgroups
	TargetGroups *ElasticLoadBalancingV2ListenerTargetGroupTupleList `json:"TargetGroups,omitempty"`
}

// ElasticLoadBalancingV2ListenerForwardConfigList represents a list of ElasticLoadBalancingV2ListenerForwardConfig
type ElasticLoadBalancingV2ListenerForwardConfigList []ElasticLoadBalancingV2ListenerForwardConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerForwardConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerForwardConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerForwardConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerForwardConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerForwardConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerListenerAttribute represents the AWS::ElasticLoadBalancingV2::Listener.ListenerAttribute CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-listenerattribute.html
type ElasticLoadBalancingV2ListenerListenerAttribute struct {
	// Key docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-listenerattribute.html#cfn-elasticloadbalancingv2-listener-listenerattribute-key
	Key *StringExpr `json:"Key,omitempty"`
	// Value docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-listenerattribute.html#cfn-elasticloadbalancingv2-listener-listenerattribute-value
	Value *StringExpr `json:"Value,omitempty"`
}

// ElasticLoadBalancingV2ListenerListenerAttributeList represents a list of ElasticLoadBalancingV2ListenerListenerAttribute
type ElasticLoadBalancingV2ListenerListenerAttributeList []ElasticLoadBalancingV2ListenerListenerAttribute

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerListenerAttributeList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerListenerAttribute{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerListenerAttributeList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerListenerAttribute{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerListenerAttributeList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerMutualAuthentication represents the AWS::ElasticLoadBalancingV2::Listener.MutualAuthentication CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-mutualauthentication.html
type ElasticLoadBalancingV2ListenerMutualAuthentication struct {
	// AdvertiseTrustStoreCaNames docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-mutualauthentication.html#cfn-elasticloadbalancingv2-listener-mutualauthentication-advertisetruststorecanames
	AdvertiseTrustStoreCaNames *StringExpr `json:"AdvertiseTrustStoreCaNames,omitempty"`
	// IgnoreClientCertificateExpiry docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-mutualauthentication.html#cfn-elasticloadbalancingv2-listener-mutualauthentication-ignoreclientcertificateexpiry
	IgnoreClientCertificateExpiry *BoolExpr `json:"IgnoreClientCertificateExpiry,omitempty"`
	// Mode docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-mutualauthentication.html#cfn-elasticloadbalancingv2-listener-mutualauthentication-mode
	Mode *StringExpr `json:"Mode,omitempty"`
	// TrustStoreArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-mutualauthentication.html#cfn-elasticloadbalancingv2-listener-mutualauthentication-truststorearn
	TrustStoreArn *StringExpr `json:"TrustStoreArn,omitempty"`
}

// ElasticLoadBalancingV2ListenerMutualAuthenticationList represents a list of ElasticLoadBalancingV2ListenerMutualAuthentication
type ElasticLoadBalancingV2ListenerMutualAuthenticationList []ElasticLoadBalancingV2ListenerMutualAuthentication

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerMutualAuthenticationList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerMutualAuthentication{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerMutualAuthenticationList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerMutualAuthentication{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerMutualAuthenticationList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRedirectConfig represents the AWS::ElasticLoadBalancingV2::Listener.RedirectConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html
type ElasticLoadBalancingV2ListenerRedirectConfig struct {
	// Host docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html#cfn-elasticloadbalancingv2-listener-redirectconfig-host
	Host *StringExpr `json:"Host,omitempty"`
	// Path docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html#cfn-elasticloadbalancingv2-listener-redirectconfig-path
	Path *StringExpr `json:"Path,omitempty"`
	// Port docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html#cfn-elasticloadbalancingv2-listener-redirectconfig-port
	Port *StringExpr `json:"Port,omitempty"`
	// Protocol docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html#cfn-elasticloadbalancingv2-listener-redirectconfig-protocol
	Protocol *StringExpr `json:"Protocol,omitempty"`
	// Query docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html#cfn-elasticloadbalancingv2-listener-redirectconfig-query
	Query *StringExpr `json:"Query,omitempty"`
	// StatusCode docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html#cfn-elasticloadbalancingv2-listener-redirectconfig-statuscode
	StatusCode *StringExpr `json:"StatusCode,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerRedirectConfigList represents a list of ElasticLoadBalancingV2ListenerRedirectConfig
type ElasticLoadBalancingV2ListenerRedirectConfigList []ElasticLoadBalancingV2ListenerRedirectConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRedirectConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRedirectConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRedirectConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRedirectConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRedirectConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerTargetGroupStickinessConfig represents the AWS::ElasticLoadBalancingV2::Listener.TargetGroupStickinessConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-targetgroupstickinessconfig.html
type ElasticLoadBalancingV2ListenerTargetGroupStickinessConfig struct {
	// DurationSeconds docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-targetgroupstickinessconfig.html#cfn-elasticloadbalancingv2-listener-targetgroupstickinessconfig-durationseconds
	DurationSeconds *IntegerExpr `json:"DurationSeconds,omitempty"`
	// Enabled docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-targetgroupstickinessconfig.html#cfn-elasticloadbalancingv2-listener-targetgroupstickinessconfig-enabled
	Enabled *BoolExpr `json:"Enabled,omitempty"`
}

// ElasticLoadBalancingV2ListenerTargetGroupStickinessConfigList represents a list of ElasticLoadBalancingV2ListenerTargetGroupStickinessConfig
type ElasticLoadBalancingV2ListenerTargetGroupStickinessConfigList []ElasticLoadBalancingV2ListenerTargetGroupStickinessConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerTargetGroupStickinessConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerTargetGroupStickinessConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerTargetGroupStickinessConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerTargetGroupStickinessConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerTargetGroupStickinessConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerTargetGroupTuple represents the AWS::ElasticLoadBalancingV2::Listener.TargetGroupTuple CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-targetgrouptuple.html
type ElasticLoadBalancingV2ListenerTargetGroupTuple struct {
	// TargetGroupArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-targetgrouptuple.html#cfn-elasticloadbalancingv2-listener-targetgrouptuple-targetgrouparn
	TargetGroupArn *StringExpr `json:"TargetGroupArn,omitempty"`
	// Weight docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-targetgrouptuple.html#cfn-elasticloadbalancingv2-listener-targetgrouptuple-weight
	Weight *IntegerExpr `json:"Weight,omitempty"`
}

// ElasticLoadBalancingV2ListenerTargetGroupTupleList represents a list of ElasticLoadBalancingV2ListenerTargetGroupTuple
type ElasticLoadBalancingV2ListenerTargetGroupTupleList []ElasticLoadBalancingV2ListenerTargetGroupTuple

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerTargetGroupTupleList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerTargetGroupTuple{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerTargetGroupTupleList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerTargetGroupTuple{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerTargetGroupTupleList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerCertificateCertificate represents the AWS::ElasticLoadBalancingV2::ListenerCertificate.Certificate CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-certificates.html
type ElasticLoadBalancingV2ListenerCertificateCertificate struct {
	// CertificateArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-certificates.html#cfn-elasticloadbalancingv2-listener-certificates-certificatearn
	CertificateArn *StringExpr `json:"CertificateArn,omitempty"`
}

// ElasticLoadBalancingV2ListenerCertificateCertificateList represents a list of ElasticLoadBalancingV2ListenerCertificateCertificate
type ElasticLoadBalancingV2ListenerCertificateCertificateList []ElasticLoadBalancingV2ListenerCertificateCertificate

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerCertificateCertificateList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerCertificateCertificate{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerCertificateCertificateList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerCertificateCertificate{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerCertificateCertificateList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleAction represents the AWS::ElasticLoadBalancingV2::ListenerRule.Action CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html
type ElasticLoadBalancingV2ListenerRuleAction struct {
	// AuthenticateCognitoConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-authenticatecognitoconfig
	AuthenticateCognitoConfig *ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfig `json:"AuthenticateCognitoConfig,omitempty"`
	// AuthenticateOidcConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-authenticateoidcconfig
	AuthenticateOidcConfig *ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfig `json:"AuthenticateOidcConfig,omitempty"`
	// FixedResponseConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-fixedresponseconfig
	FixedResponseConfig *ElasticLoadBalancingV2ListenerRuleFixedResponseConfig `json:"FixedResponseConfig,omitempty"`
	// ForwardConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-forwardconfig
	ForwardConfig *ElasticLoadBalancingV2ListenerRuleForwardConfig `json:"ForwardConfig,omitempty"`
	// Order docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-order
	Order *IntegerExpr `json:"Order,omitempty"`
	// RedirectConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-redirectconfig
	RedirectConfig *ElasticLoadBalancingV2ListenerRuleRedirectConfig `json:"RedirectConfig,omitempty"`
	// TargetGroupArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-targetgrouparn
	TargetGroupArn *StringExpr `json:"TargetGroupArn,omitempty"`
	// Type docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-type
	Type *StringExpr `json:"Type,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerRuleActionList represents a list of ElasticLoadBalancingV2ListenerRuleAction
type ElasticLoadBalancingV2ListenerRuleActionList []ElasticLoadBalancingV2ListenerRuleAction

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleActionList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleAction{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleActionList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleAction{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleActionList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.AuthenticateCognitoConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html
type ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfig struct {
	// AuthenticationRequestExtraParams docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-authenticationrequestextraparams
	AuthenticationRequestExtraParams interface{} `json:"AuthenticationRequestExtraParams,omitempty"`
	// OnUnauthenticatedRequest docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-onunauthenticatedrequest
	OnUnauthenticatedRequest *StringExpr `json:"OnUnauthenticatedRequest,omitempty"`
	// Scope docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-scope
	Scope *StringExpr `json:"Scope,omitempty"`
	// SessionCookieName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-sessioncookiename
	SessionCookieName *StringExpr `json:"SessionCookieName,omitempty"`
	// SessionTimeout docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-sessiontimeout
	SessionTimeout *IntegerExpr `json:"SessionTimeout,omitempty"`
	// UserPoolArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-userpoolarn
	UserPoolArn *StringExpr `json:"UserPoolArn,omitempty" validate:"dive,required"`
	// UserPoolClientID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-userpoolclientid
	UserPoolClientID *StringExpr `json:"UserPoolClientId,omitempty" validate:"dive,required"`
	// UserPoolDomain docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticatecognitoconfig-userpooldomain
	UserPoolDomain *StringExpr `json:"UserPoolDomain,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfigList represents a list of ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfig
type ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfigList []ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleAuthenticateCognitoConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.AuthenticateOidcConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html
type ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfig struct {
	// AuthenticationRequestExtraParams docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-authenticationrequestextraparams
	AuthenticationRequestExtraParams interface{} `json:"AuthenticationRequestExtraParams,omitempty"`
	// AuthorizationEndpoint docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-authorizationendpoint
	AuthorizationEndpoint *StringExpr `json:"AuthorizationEndpoint,omitempty" validate:"dive,required"`
	// ClientID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-clientid
	ClientID *StringExpr `json:"ClientId,omitempty" validate:"dive,required"`
	// ClientSecret docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-clientsecret
	ClientSecret *StringExpr `json:"ClientSecret,omitempty"`
	// Issuer docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-issuer
	Issuer *StringExpr `json:"Issuer,omitempty" validate:"dive,required"`
	// OnUnauthenticatedRequest docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-onunauthenticatedrequest
	OnUnauthenticatedRequest *StringExpr `json:"OnUnauthenticatedRequest,omitempty"`
	// Scope docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-scope
	Scope *StringExpr `json:"Scope,omitempty"`
	// SessionCookieName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-sessioncookiename
	SessionCookieName *StringExpr `json:"SessionCookieName,omitempty"`
	// SessionTimeout docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-sessiontimeout
	SessionTimeout *IntegerExpr `json:"SessionTimeout,omitempty"`
	// TokenEndpoint docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-tokenendpoint
	TokenEndpoint *StringExpr `json:"TokenEndpoint,omitempty" validate:"dive,required"`
	// UseExistingClientSecret docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-useexistingclientsecret
	UseExistingClientSecret *BoolExpr `json:"UseExistingClientSecret,omitempty"`
	// UserInfoEndpoint docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-authenticateoidcconfig.html#cfn-elasticloadbalancingv2-listenerrule-authenticateoidcconfig-userinfoendpoint
	UserInfoEndpoint *StringExpr `json:"UserInfoEndpoint,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfigList represents a list of ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfig
type ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfigList []ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleAuthenticateOidcConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleFixedResponseConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.FixedResponseConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-fixedresponseconfig.html
type ElasticLoadBalancingV2ListenerRuleFixedResponseConfig struct {
	// ContentType docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-fixedresponseconfig.html#cfn-elasticloadbalancingv2-listenerrule-fixedresponseconfig-contenttype
	ContentType *StringExpr `json:"ContentType,omitempty"`
	// MessageBody docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-fixedresponseconfig.html#cfn-elasticloadbalancingv2-listenerrule-fixedresponseconfig-messagebody
	MessageBody *StringExpr `json:"MessageBody,omitempty"`
	// StatusCode docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-fixedresponseconfig.html#cfn-elasticloadbalancingv2-listenerrule-fixedresponseconfig-statuscode
	StatusCode *StringExpr `json:"StatusCode,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerRuleFixedResponseConfigList represents a list of ElasticLoadBalancingV2ListenerRuleFixedResponseConfig
type ElasticLoadBalancingV2ListenerRuleFixedResponseConfigList []ElasticLoadBalancingV2ListenerRuleFixedResponseConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleFixedResponseConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleFixedResponseConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleFixedResponseConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleFixedResponseConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleFixedResponseConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleForwardConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.ForwardConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-forwardconfig.html
type ElasticLoadBalancingV2ListenerRuleForwardConfig struct {
	// TargetGroupStickinessConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-forwardconfig.html#cfn-elasticloadbalancingv2-listenerrule-forwardconfig-targetgroupstickinessconfig
	TargetGroupStickinessConfig *ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfig `json:"TargetGroupStickinessConfig,omitempty"`
	// TargetGroups docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-forwardconfig.html#cfn-elasticloadbalancingv2-listenerrule-forwardconfig-targetgroups
	TargetGroups *ElasticLoadBalancingV2ListenerRuleTargetGroupTupleList `json:"TargetGroups,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleForwardConfigList represents a list of ElasticLoadBalancingV2ListenerRuleForwardConfig
type ElasticLoadBalancingV2ListenerRuleForwardConfigList []ElasticLoadBalancingV2ListenerRuleForwardConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleForwardConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleForwardConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleForwardConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleForwardConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleForwardConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleHostHeaderConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.HostHeaderConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-hostheaderconfig.html
type ElasticLoadBalancingV2ListenerRuleHostHeaderConfig struct {
	// Values docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-hostheaderconfig.html#cfn-elasticloadbalancingv2-listenerrule-hostheaderconfig-values
	Values *StringListExpr `json:"Values,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleHostHeaderConfigList represents a list of ElasticLoadBalancingV2ListenerRuleHostHeaderConfig
type ElasticLoadBalancingV2ListenerRuleHostHeaderConfigList []ElasticLoadBalancingV2ListenerRuleHostHeaderConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleHostHeaderConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleHostHeaderConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleHostHeaderConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleHostHeaderConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleHostHeaderConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.HttpHeaderConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-httpheaderconfig.html
type ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfig struct {
	// HTTPHeaderName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-httpheaderconfig.html#cfn-elasticloadbalancingv2-listenerrule-httpheaderconfig-httpheadername
	HTTPHeaderName *StringExpr `json:"HttpHeaderName,omitempty"`
	// Values docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-httpheaderconfig.html#cfn-elasticloadbalancingv2-listenerrule-httpheaderconfig-values
	Values *StringListExpr `json:"Values,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfigList represents a list of ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfig
type ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfigList []ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.HttpRequestMethodConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-httprequestmethodconfig.html
type ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfig struct {
	// Values docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-httprequestmethodconfig.html#cfn-elasticloadbalancingv2-listenerrule-httprequestmethodconfig-values
	Values *StringListExpr `json:"Values,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfigList represents a list of ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfig
type ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfigList []ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRulePathPatternConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.PathPatternConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-pathpatternconfig.html
type ElasticLoadBalancingV2ListenerRulePathPatternConfig struct {
	// Values docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-pathpatternconfig.html#cfn-elasticloadbalancingv2-listenerrule-pathpatternconfig-values
	Values *StringListExpr `json:"Values,omitempty"`
}

// ElasticLoadBalancingV2ListenerRulePathPatternConfigList represents a list of ElasticLoadBalancingV2ListenerRulePathPatternConfig
type ElasticLoadBalancingV2ListenerRulePathPatternConfigList []ElasticLoadBalancingV2ListenerRulePathPatternConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRulePathPatternConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRulePathPatternConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRulePathPatternConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRulePathPatternConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRulePathPatternConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleQueryStringConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.QueryStringConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-querystringconfig.html
type ElasticLoadBalancingV2ListenerRuleQueryStringConfig struct {
	// Values docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-querystringconfig.html#cfn-elasticloadbalancingv2-listenerrule-querystringconfig-values
	Values *ElasticLoadBalancingV2ListenerRuleQueryStringKeyValueList `json:"Values,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleQueryStringConfigList represents a list of ElasticLoadBalancingV2ListenerRuleQueryStringConfig
type ElasticLoadBalancingV2ListenerRuleQueryStringConfigList []ElasticLoadBalancingV2ListenerRuleQueryStringConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleQueryStringConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleQueryStringConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleQueryStringConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleQueryStringConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleQueryStringConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleQueryStringKeyValue represents the AWS::ElasticLoadBalancingV2::ListenerRule.QueryStringKeyValue CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-querystringkeyvalue.html
type ElasticLoadBalancingV2ListenerRuleQueryStringKeyValue struct {
	// Key docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-querystringkeyvalue.html#cfn-elasticloadbalancingv2-listenerrule-querystringkeyvalue-key
	Key *StringExpr `json:"Key,omitempty"`
	// Value docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-querystringkeyvalue.html#cfn-elasticloadbalancingv2-listenerrule-querystringkeyvalue-value
	Value *StringExpr `json:"Value,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleQueryStringKeyValueList represents a list of ElasticLoadBalancingV2ListenerRuleQueryStringKeyValue
type ElasticLoadBalancingV2ListenerRuleQueryStringKeyValueList []ElasticLoadBalancingV2ListenerRuleQueryStringKeyValue

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleQueryStringKeyValueList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleQueryStringKeyValue{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleQueryStringKeyValueList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleQueryStringKeyValue{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleQueryStringKeyValueList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleRedirectConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.RedirectConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-redirectconfig.html
type ElasticLoadBalancingV2ListenerRuleRedirectConfig struct {
	// Host docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-redirectconfig.html#cfn-elasticloadbalancingv2-listenerrule-redirectconfig-host
	Host *StringExpr `json:"Host,omitempty"`
	// Path docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-redirectconfig.html#cfn-elasticloadbalancingv2-listenerrule-redirectconfig-path
	Path *StringExpr `json:"Path,omitempty"`
	// Port docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-redirectconfig.html#cfn-elasticloadbalancingv2-listenerrule-redirectconfig-port
	Port *StringExpr `json:"Port,omitempty"`
	// Protocol docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-redirectconfig.html#cfn-elasticloadbalancingv2-listenerrule-redirectconfig-protocol
	Protocol *StringExpr `json:"Protocol,omitempty"`
	// Query docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-redirectconfig.html#cfn-elasticloadbalancingv2-listenerrule-redirectconfig-query
	Query *StringExpr `json:"Query,omitempty"`
	// StatusCode docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-redirectconfig.html#cfn-elasticloadbalancingv2-listenerrule-redirectconfig-statuscode
	StatusCode *StringExpr `json:"StatusCode,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2ListenerRuleRedirectConfigList represents a list of ElasticLoadBalancingV2ListenerRuleRedirectConfig
type ElasticLoadBalancingV2ListenerRuleRedirectConfigList []ElasticLoadBalancingV2ListenerRuleRedirectConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleRedirectConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleRedirectConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleRedirectConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleRedirectConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleRedirectConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleRuleCondition represents the AWS::ElasticLoadBalancingV2::ListenerRule.RuleCondition CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html
type ElasticLoadBalancingV2ListenerRuleRuleCondition struct {
	// Field docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-field
	Field *StringExpr `json:"Field,omitempty"`
	// HostHeaderConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-hostheaderconfig
	HostHeaderConfig *ElasticLoadBalancingV2ListenerRuleHostHeaderConfig `json:"HostHeaderConfig,omitempty"`
	// HTTPHeaderConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-httpheaderconfig
	HTTPHeaderConfig *ElasticLoadBalancingV2ListenerRuleHTTPHeaderConfig `json:"HttpHeaderConfig,omitempty"`
	// HTTPRequestMethodConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-httprequestmethodconfig
	HTTPRequestMethodConfig *ElasticLoadBalancingV2ListenerRuleHTTPRequestMethodConfig `json:"HttpRequestMethodConfig,omitempty"`
	// PathPatternConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-pathpatternconfig
	PathPatternConfig *ElasticLoadBalancingV2ListenerRulePathPatternConfig `json:"PathPatternConfig,omitempty"`
	// QueryStringConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-querystringconfig
	QueryStringConfig *ElasticLoadBalancingV2ListenerRuleQueryStringConfig `json:"QueryStringConfig,omitempty"`
	// SourceIPConfig docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-sourceipconfig
	SourceIPConfig *ElasticLoadBalancingV2ListenerRuleSourceIPConfig `json:"SourceIpConfig,omitempty"`
	// Values docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-values
	Values *StringListExpr `json:"Values,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleRuleConditionList represents a list of ElasticLoadBalancingV2ListenerRuleRuleCondition
type ElasticLoadBalancingV2ListenerRuleRuleConditionList []ElasticLoadBalancingV2ListenerRuleRuleCondition

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleRuleConditionList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleRuleCondition{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleRuleConditionList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleRuleCondition{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleRuleConditionList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleSourceIPConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.SourceIpConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-sourceipconfig.html
type ElasticLoadBalancingV2ListenerRuleSourceIPConfig struct {
	// Values docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-sourceipconfig.html#cfn-elasticloadbalancingv2-listenerrule-sourceipconfig-values
	Values *StringListExpr `json:"Values,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleSourceIPConfigList represents a list of ElasticLoadBalancingV2ListenerRuleSourceIPConfig
type ElasticLoadBalancingV2ListenerRuleSourceIPConfigList []ElasticLoadBalancingV2ListenerRuleSourceIPConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleSourceIPConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleSourceIPConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleSourceIPConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleSourceIPConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleSourceIPConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfig represents the AWS::ElasticLoadBalancingV2::ListenerRule.TargetGroupStickinessConfig CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-targetgroupstickinessconfig.html
type ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfig struct {
	// DurationSeconds docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-targetgroupstickinessconfig.html#cfn-elasticloadbalancingv2-listenerrule-targetgroupstickinessconfig-durationseconds
	DurationSeconds *IntegerExpr `json:"DurationSeconds,omitempty"`
	// Enabled docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-targetgroupstickinessconfig.html#cfn-elasticloadbalancingv2-listenerrule-targetgroupstickinessconfig-enabled
	Enabled *BoolExpr `json:"Enabled,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfigList represents a list of ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfig
type ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfigList []ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfig

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfigList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfig{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfigList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfig{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleTargetGroupStickinessConfigList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2ListenerRuleTargetGroupTuple represents the AWS::ElasticLoadBalancingV2::ListenerRule.TargetGroupTuple CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-targetgrouptuple.html
type ElasticLoadBalancingV2ListenerRuleTargetGroupTuple struct {
	// TargetGroupArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-targetgrouptuple.html#cfn-elasticloadbalancingv2-listenerrule-targetgrouptuple-targetgrouparn
	TargetGroupArn *StringExpr `json:"TargetGroupArn,omitempty"`
	// Weight docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-targetgrouptuple.html#cfn-elasticloadbalancingv2-listenerrule-targetgrouptuple-weight
	Weight *IntegerExpr `json:"Weight,omitempty"`
}

// ElasticLoadBalancingV2ListenerRuleTargetGroupTupleList represents a list of ElasticLoadBalancingV2ListenerRuleTargetGroupTuple
type ElasticLoadBalancingV2ListenerRuleTargetGroupTupleList []ElasticLoadBalancingV2ListenerRuleTargetGroupTuple

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2ListenerRuleTargetGroupTupleList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2ListenerRuleTargetGroupTuple{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleTargetGroupTupleList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2ListenerRuleTargetGroupTuple{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2ListenerRuleTargetGroupTupleList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute represents the AWS::ElasticLoadBalancingV2::LoadBalancer.LoadBalancerAttribute CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-loadbalancerattribute.html
type ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute struct {
	// Key docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-loadbalancerattribute.html#cfn-elasticloadbalancingv2-loadbalancer-loadbalancerattribute-key
	Key *StringExpr `json:"Key,omitempty"`
	// Value docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-loadbalancerattribute.html#cfn-elasticloadbalancingv2-loadbalancer-loadbalancerattribute-value
	Value *StringExpr `json:"Value,omitempty"`
}

// ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList represents a list of ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute
type ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList []ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacity represents the AWS::ElasticLoadBalancingV2::LoadBalancer.MinimumLoadBalancerCapacity CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-minimumloadbalancercapacity.html
type ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacity struct {
	// CapacityUnits docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-minimumloadbalancercapacity.html#cfn-elasticloadbalancingv2-loadbalancer-minimumloadbalancercapacity-capacityunits
	CapacityUnits *IntegerExpr `json:"CapacityUnits,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacityList represents a list of ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacity
type ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacityList []ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacity

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacityList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacity{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacityList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacity{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacityList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2LoadBalancerSubnetMapping represents the AWS::ElasticLoadBalancingV2::LoadBalancer.SubnetMapping CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-subnetmapping.html
type ElasticLoadBalancingV2LoadBalancerSubnetMapping struct {
	// AllocationID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-subnetmapping.html#cfn-elasticloadbalancingv2-loadbalancer-subnetmapping-allocationid
	AllocationID *StringExpr `json:"AllocationId,omitempty"`
	// IPv6Address docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-subnetmapping.html#cfn-elasticloadbalancingv2-loadbalancer-subnetmapping-ipv6address
	IPv6Address *StringExpr `json:"IPv6Address,omitempty"`
	// PrivateIPv4Address docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-subnetmapping.html#cfn-elasticloadbalancingv2-loadbalancer-subnetmapping-privateipv4address
	PrivateIPv4Address *StringExpr `json:"PrivateIPv4Address,omitempty"`
	// SourceNatIPv6Prefix docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-subnetmapping.html#cfn-elasticloadbalancingv2-loadbalancer-subnetmapping-sourcenatipv6prefix
	SourceNatIPv6Prefix *StringExpr `json:"SourceNatIpv6Prefix,omitempty"`
	// SubnetID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-subnetmapping.html#cfn-elasticloadbalancingv2-loadbalancer-subnetmapping-subnetid
	SubnetID *StringExpr `json:"SubnetId,omitempty" validate:"dive,required"`
}

// ElasticLoadBalancingV2LoadBalancerSubnetMappingList represents a list of ElasticLoadBalancingV2LoadBalancerSubnetMapping
type ElasticLoadBalancingV2LoadBalancerSubnetMappingList []ElasticLoadBalancingV2LoadBalancerSubnetMapping

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2LoadBalancerSubnetMappingList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2LoadBalancerSubnetMapping{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2LoadBalancerSubnetMappingList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2LoadBalancerSubnetMapping{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2LoadBalancerSubnetMappingList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2TargetGroupMatcher represents the AWS::ElasticLoadBalancingV2::TargetGroup.Matcher CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-matcher.html
type ElasticLoadBalancingV2TargetGroupMatcher struct {
	// GrpcCode docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-matcher.html#cfn-elasticloadbalancingv2-targetgroup-matcher-grpccode
	GrpcCode *StringExpr `json:"GrpcCode,omitempty"`
	// HTTPCode docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-matcher.html#cfn-elasticloadbalancingv2-targetgroup-matcher-httpcode
	HTTPCode *StringExpr `json:"HttpCode,omitempty"`
}

// ElasticLoadBalancingV2TargetGroupMatcherList represents a list of ElasticLoadBalancingV2TargetGroupMatcher
type ElasticLoadBalancingV2TargetGroupMatcherList []ElasticLoadBalancingV2TargetGroupMatcher

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2TargetGroupMatcherList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2TargetGroupMatcher{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2TargetGroupMatcherList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2TargetGroupMatcher{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2TargetGroupMatcherList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2TargetGroupTargetDescription represents the AWS::ElasticLoadBalancingV2::TargetGroup.TargetDescription CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-targetdescription.html
type ElasticLoadBalancingV2TargetGroupTargetDescription struct {
	// AvailabilityZone docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-targetdescription.html#cfn-elasticloadbalancingv2-targetgroup-targetdescription-availabilityzone
	AvailabilityZone *StringExpr `json:"AvailabilityZone,omitempty"`
	// ID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-targetdescription.html#cfn-elasticloadbalancingv2-targetgroup-targetdescription-id
	ID *StringExpr `json:"Id,omitempty" validate:"dive,required"`
	// Port docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-targetdescription.html#cfn-elasticloadbalancingv2-targetgroup-targetdescription-port
	Port *IntegerExpr `json:"Port,omitempty"`
}

// ElasticLoadBalancingV2TargetGroupTargetDescriptionList represents a list of ElasticLoadBalancingV2TargetGroupTargetDescription
type ElasticLoadBalancingV2TargetGroupTargetDescriptionList []ElasticLoadBalancingV2TargetGroupTargetDescription

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2TargetGroupTargetDescriptionList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2TargetGroupTargetDescription{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2TargetGroupTargetDescriptionList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2TargetGroupTargetDescription{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2TargetGroupTargetDescriptionList(list)
		return nil
	}
	return err
}

// ElasticLoadBalancingV2TargetGroupTargetGroupAttribute represents the AWS::ElasticLoadBalancingV2::TargetGroup.TargetGroupAttribute CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-targetgroupattribute.html
type ElasticLoadBalancingV2TargetGroupTargetGroupAttribute struct {
	// Key docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-targetgroupattribute.html#cfn-elasticloadbalancingv2-targetgroup-targetgroupattribute-key
	Key *StringExpr `json:"Key,omitempty"`
	// Value docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-targetgroup-targetgroupattribute.html#cfn-elasticloadbalancingv2-targetgroup-targetgroupattribute-value
	Value *StringExpr `json:"Value,omitempty"`
}

// ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList represents a list of ElasticLoadBalancingV2TargetGroupTargetGroupAttribute
type ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList []ElasticLoadBalancingV2TargetGroupTargetGroupAttribute

// UnmarshalJSON sets the object from the provided JSON representation
func (l *ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := ElasticLoadBalancingV2TargetGroupTargetGroupAttribute{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList{item}
		return nil
	}
	list := []ElasticLoadBalancingV2TargetGroupTargetGroupAttribute{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList(list)
		return nil
	}
	return err
}

// Tag represents the Tag CloudFormation property type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-resource-tags.html
type Tag struct {
	// Key docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-resource-tags.html#cfn-resource-tags-key
	Key *StringExpr `json:"Key,omitempty" validate:"dive,required"`
	// Value docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-resource-tags.html#cfn-resource-tags-value
	Value *StringExpr `json:"Value,omitempty" validate:"dive,required"`
}

// TagList represents a list of Tag
type TagList []Tag

// UnmarshalJSON sets the object from the provided JSON representation
func (l *TagList) UnmarshalJSON(buf []byte) error {
	// Cloudformation allows a single object when a list of objects is expected
	item := Tag{}
	if err := json.Unmarshal(buf, &item); err == nil {
		*l = TagList{item}
		return nil
	}
	list := []Tag{}
	err := json.Unmarshal(buf, &list)
	if err == nil {
		*l = TagList(list)
		return nil
	}
	return err
}

//
//  ____
// |  _ \ ___  ___  ___  _   _ _ __ ___ ___  ___
// | |_) / _ \/ __|/ _ \| | | | '__/ __/ _ \/ __|
// |  _ <  __/\__ \ (_) | |_| | | | (_|  __/\__ \
// |_| \_\___||___/\___/ \__,_|_|  \___\___||___/
//

// CloudWatchAlarm represents the AWS::CloudWatch::Alarm CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html
type CloudWatchAlarm struct {
	// ActionsEnabled docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-actionsenabled
	ActionsEnabled *BoolExpr `json:"ActionsEnabled,omitempty"`
	// AlarmActions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-alarmactions
	AlarmActions *StringListExpr `json:"AlarmActions,omitempty"`
	// AlarmDescription docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-alarmdescription
	AlarmDescription *StringExpr `json:"AlarmDescription,omitempty"`
	// AlarmName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-alarmname
	AlarmName *StringExpr `json:"AlarmName,omitempty"`
	// ComparisonOperator docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-comparisonoperator
	ComparisonOperator *StringExpr `json:"ComparisonOperator,omitempty" validate:"dive,required"`
	// DatapointsToAlarm docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-datapointstoalarm
	DatapointsToAlarm *IntegerExpr `json:"DatapointsToAlarm,omitempty"`
	// Dimensions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-dimensions
	Dimensions *CloudWatchAlarmDimensionList `json:"Dimensions,omitempty"`
	// EvaluateLowSampleCountPercentile docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-evaluatelowsamplecountpercentile
	EvaluateLowSampleCountPercentile *StringExpr `json:"EvaluateLowSampleCountPercentile,omitempty"`
	// EvaluationPeriods docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-evaluationperiods
	EvaluationPeriods *IntegerExpr `json:"EvaluationPeriods,omitempty" validate:"dive,required"`
	// ExtendedStatistic docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-extendedstatistic
	ExtendedStatistic *StringExpr `json:"ExtendedStatistic,omitempty"`
	// InsufficientDataActions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-insufficientdataactions
	InsufficientDataActions *StringListExpr `json:"InsufficientDataActions,omitempty"`
	// MetricName docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-metricname
	MetricName *StringExpr `json:"MetricName,omitempty"`
	// Metrics docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-metrics
	Metrics *CloudWatchAlarmMetricDataQueryList `json:"Metrics,omitempty"`
	// Namespace docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-namespace
	Namespace *StringExpr `json:"Namespace,omitempty"`
	// OKActions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-okactions
	OKActions *StringListExpr `json:"OKActions,omitempty"`
	// Period docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-period
	Period *IntegerExpr `json:"Period,omitempty"`
	// Statistic docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-statistic
	Statistic *StringExpr `json:"Statistic,omitempty"`
	// Tags docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-tags
	Tags *TagList `json:"Tags,omitempty"`
	// Threshold docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-threshold
	Threshold *IntegerExpr `json:"Threshold,omitempty"`
	// ThresholdMetricID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-thresholdmetricid
	ThresholdMetricID *StringExpr `json:"ThresholdMetricId,omitempty"`
	// TreatMissingData docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-treatmissingdata
	TreatMissingData *StringExpr `json:"TreatMissingData,omitempty"`
	// Unit docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-alarm.html#cfn-cloudwatch-alarm-unit
	Unit *StringExpr `json:"Unit,omitempty"`
}

// CfnResourceType returns AWS::CloudWatch::Alarm to implement the ResourceProperties interface
func (s CloudWatchAlarm) CfnResourceType() string {

	return "AWS::CloudWatch::Alarm"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s CloudWatchAlarm) CfnResourceAttributes() []string {
	return []string{"Arn"}
}

// ElasticLoadBalancingV2Listener represents the AWS::ElasticLoadBalancingV2::Listener CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html
type ElasticLoadBalancingV2Listener struct {
	// AlpnPolicy docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-alpnpolicy
	AlpnPolicy *StringListExpr `json:"AlpnPolicy,omitempty"`
	// Certificates docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-certificates
	Certificates *ElasticLoadBalancingV2ListenerCertificatePropertyList `json:"Certificates,omitempty"`
	// DefaultActions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-defaultactions
	DefaultActions *ElasticLoadBalancingV2ListenerActionList `json:"DefaultActions,omitempty" validate:"dive,required"`
	// ListenerAttributes docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-listenerattributes
	ListenerAttributes *ElasticLoadBalancingV2ListenerListenerAttributeList `json:"ListenerAttributes,omitempty"`
	// LoadBalancerArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-loadbalancerarn
	LoadBalancerArn *StringExpr `json:"LoadBalancerArn,omitempty" validate:"dive,required"`
	// MutualAuthentication docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-mutualauthentication
	MutualAuthentication *ElasticLoadBalancingV2ListenerMutualAuthentication `json:"MutualAuthentication,omitempty"`
	// Port docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-port
	Port *IntegerExpr `json:"Port,omitempty"`
	// Protocol docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-protocol
	Protocol *StringExpr `json:"Protocol,omitempty"`
	// SslPolicy docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listener.html#cfn-elasticloadbalancingv2-listener-sslpolicy
	SslPolicy *StringExpr `json:"SslPolicy,omitempty"`
}

// CfnResourceType returns AWS::ElasticLoadBalancingV2::Listener to implement the ResourceProperties interface
func (s ElasticLoadBalancingV2Listener) CfnResourceType() string {

	return "AWS::ElasticLoadBalancingV2::Listener"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s ElasticLoadBalancingV2Listener) CfnResourceAttributes() []string {
	return []string{"ListenerArn"}
}

// ElasticLoadBalancingV2ListenerCertificate represents the AWS::ElasticLoadBalancingV2::ListenerCertificate CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenercertificate.html
type ElasticLoadBalancingV2ListenerCertificate struct {
	// Certificates docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenercertificate.html#cfn-elasticloadbalancingv2-listenercertificate-certificates
	Certificates *ElasticLoadBalancingV2ListenerCertificateCertificateList `json:"Certificates,omitempty" validate:"dive,required"`
	// ListenerArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenercertificate.html#cfn-elasticloadbalancingv2-listenercertificate-listenerarn
	ListenerArn *StringExpr `json:"ListenerArn,omitempty" validate:"dive,required"`
}

// CfnResourceType returns AWS::ElasticLoadBalancingV2::ListenerCertificate to implement the ResourceProperties interface
func (s ElasticLoadBalancingV2ListenerCertificate) CfnResourceType() string {

	return "AWS::ElasticLoadBalancingV2::ListenerCertificate"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s ElasticLoadBalancingV2ListenerCertificate) CfnResourceAttributes() []string {
	return []string{}
}

// ElasticLoadBalancingV2ListenerRule represents the AWS::ElasticLoadBalancingV2::ListenerRule CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenerrule.html
type ElasticLoadBalancingV2ListenerRule struct {
	// Actions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenerrule.html#cfn-elasticloadbalancingv2-listenerrule-actions
	Actions *ElasticLoadBalancingV2ListenerRuleActionList `json:"Actions,omitempty" validate:"dive,required"`
	// Conditions docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenerrule.html#cfn-elasticloadbalancingv2-listenerrule-conditions
	Conditions *ElasticLoadBalancingV2ListenerRuleRuleConditionList `json:"Conditions,omitempty" validate:"dive,required"`
	// ListenerArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenerrule.html#cfn-elasticloadbalancingv2-listenerrule-listenerarn
	ListenerArn *StringExpr `json:"ListenerArn,omitempty"`
	// Priority docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenerrule.html#cfn-elasticloadbalancingv2-listenerrule-priority
	Priority *IntegerExpr `json:"Priority,omitempty" validate:"dive,required"`
}

// CfnResourceType returns AWS::ElasticLoadBalancingV2::ListenerRule to implement the ResourceProperties interface
func (s ElasticLoadBalancingV2ListenerRule) CfnResourceType() string {

	return "AWS::ElasticLoadBalancingV2::ListenerRule"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s ElasticLoadBalancingV2ListenerRule) CfnResourceAttributes() []string {
	return []string{"IsDefault", "RuleArn"}
}

// ElasticLoadBalancingV2LoadBalancer represents the AWS::ElasticLoadBalancingV2::LoadBalancer CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html
type ElasticLoadBalancingV2LoadBalancer struct {
	// EnablePrefixForIPv6SourceNat docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-enableprefixforipv6sourcenat
	EnablePrefixForIPv6SourceNat *StringExpr `json:"EnablePrefixForIpv6SourceNat,omitempty"`
	// EnforceSecurityGroupInboundRulesOnPrivateLinkTraffic docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-enforcesecuritygroupinboundrulesonprivatelinktraffic
	EnforceSecurityGroupInboundRulesOnPrivateLinkTraffic *StringExpr `json:"EnforceSecurityGroupInboundRulesOnPrivateLinkTraffic,omitempty"`
	// IPAddressType docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-ipaddresstype
	IPAddressType *StringExpr `json:"IpAddressType,omitempty"`
	// IPv4IPamPoolID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-ipv4ipampoolid
	IPv4IPamPoolID *StringExpr `json:"Ipv4IpamPoolId,omitempty"`
	// LoadBalancerAttributes docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-loadbalancerattributes
	LoadBalancerAttributes *ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList `json:"LoadBalancerAttributes,omitempty"`
	// MinimumLoadBalancerCapacity docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-minimumloadbalancercapacity
	MinimumLoadBalancerCapacity *ElasticLoadBalancingV2LoadBalancerMinimumLoadBalancerCapacity `json:"MinimumLoadBalancerCapacity,omitempty"`
	// Name docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-name
	Name *StringExpr `json:"Name,omitempty"`
	// Scheme docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-scheme
	Scheme *StringExpr `json:"Scheme,omitempty"`
	// SecurityGroups docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-securitygroups
	SecurityGroups *StringListExpr `json:"SecurityGroups,omitempty"`
	// SubnetMappings docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-subnetmappings
	SubnetMappings *ElasticLoadBalancingV2LoadBalancerSubnetMappingList `json:"SubnetMappings,omitempty"`
	// Subnets docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-subnets
	Subnets *StringListExpr `json:"Subnets,omitempty"`
	// Tags docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-tags
	Tags *TagList `json:"Tags,omitempty"`
	// Type docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-loadbalancer.html#cfn-elasticloadbalancingv2-loadbalancer-type
	Type *StringExpr `json:"Type,omitempty"`
}

// CfnResourceType returns AWS::ElasticLoadBalancingV2::LoadBalancer to implement the ResourceProperties interface
func (s ElasticLoadBalancingV2LoadBalancer) CfnResourceType() string {

	return "AWS::ElasticLoadBalancingV2::LoadBalancer"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s ElasticLoadBalancingV2LoadBalancer) CfnResourceAttributes() []string {
	return []string{"CanonicalHostedZoneID", "DNSName", "LoadBalancerArn", "LoadBalancerFullName", "LoadBalancerName", "SecurityGroups"}
}

// ElasticLoadBalancingV2TargetGroup represents the AWS::ElasticLoadBalancingV2::TargetGroup CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html
type ElasticLoadBalancingV2TargetGroup struct {
	// HealthCheckEnabled docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthcheckenabled
	HealthCheckEnabled *BoolExpr `json:"HealthCheckEnabled,omitempty"`
	// HealthCheckIntervalSeconds docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthcheckintervalseconds
	HealthCheckIntervalSeconds *IntegerExpr `json:"HealthCheckIntervalSeconds,omitempty"`
	// HealthCheckPath docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthcheckpath
	HealthCheckPath *StringExpr `json:"HealthCheckPath,omitempty"`
	// HealthCheckPort docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthcheckport
	HealthCheckPort *StringExpr `json:"HealthCheckPort,omitempty"`
	// HealthCheckProtocol docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthcheckprotocol
	HealthCheckProtocol *StringExpr `json:"HealthCheckProtocol,omitempty"`
	// HealthCheckTimeoutSeconds docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthchecktimeoutseconds
	HealthCheckTimeoutSeconds *IntegerExpr `json:"HealthCheckTimeoutSeconds,omitempty"`
	// HealthyThresholdCount docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthythresholdcount
	HealthyThresholdCount *IntegerExpr `json:"HealthyThresholdCount,omitempty"`
	// IPAddressType docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-ipaddresstype
	IPAddressType *StringExpr `json:"IpAddressType,omitempty"`
	// Matcher docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-matcher
	Matcher *ElasticLoadBalancingV2TargetGroupMatcher `json:"Matcher,omitempty"`
	// Name docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-name
	Name *StringExpr `json:"Name,omitempty"`
	// Port docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-port
	Port *IntegerExpr `json:"Port,omitempty"`
	// Protocol docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-protocol
	Protocol *StringExpr `json:"Protocol,omitempty"`
	// ProtocolVersion docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-protocolversion
	ProtocolVersion *StringExpr `json:"ProtocolVersion,omitempty"`
	// Tags docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-tags
	Tags *TagList `json:"Tags,omitempty"`
	// TargetGroupAttributes docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-targetgroupattributes
	TargetGroupAttributes *ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList `json:"TargetGroupAttributes,omitempty"`
	// TargetType docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-targettype
	TargetType *StringExpr `json:"TargetType,omitempty"`
	// Targets docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-targets
	Targets *ElasticLoadBalancingV2TargetGroupTargetDescriptionList `json:"Targets,omitempty"`
	// UnhealthyThresholdCount docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-unhealthythresholdcount
	UnhealthyThresholdCount *IntegerExpr `json:"UnhealthyThresholdCount,omitempty"`
	// VPCID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-vpcid
	VPCID *StringExpr `json:"VpcId,omitempty"`
}

// CfnResourceType returns AWS::ElasticLoadBalancingV2::TargetGroup to implement the ResourceProperties interface
func (s ElasticLoadBalancingV2TargetGroup) CfnResourceType() string {

	return "AWS::ElasticLoadBalancingV2::TargetGroup"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s ElasticLoadBalancingV2TargetGroup) CfnResourceAttributes() []string {
	return []string{"LoadBalancerArns", "TargetGroupArn", "TargetGroupFullName", "TargetGroupName"}
}

// WAFRegionalWebACLAssociation represents the AWS::WAFRegional::WebACLAssociation CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-wafregional-webaclassociation.html
type WAFRegionalWebACLAssociation struct {
	// ResourceArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-wafregional-webaclassociation.html#cfn-wafregional-webaclassociation-resourcearn
	ResourceArn *StringExpr `json:"ResourceArn,omitempty" validate:"dive,required"`
	// WebACLID docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-wafregional-webaclassociation.html#cfn-wafregional-webaclassociation-webaclid
	WebACLID *StringExpr `json:"WebACLId,omitempty" validate:"dive,required"`
}

// CfnResourceType returns AWS::WAFRegional::WebACLAssociation to implement the ResourceProperties interface
func (s WAFRegionalWebACLAssociation) CfnResourceType() string {

	return "AWS::WAFRegional::WebACLAssociation"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s WAFRegionalWebACLAssociation) CfnResourceAttributes() []string {
	return []string{}
}

// WAFv2WebACLAssociation represents the AWS::WAFv2::WebACLAssociation CloudFormation resource type
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-wafv2-webaclassociation.html
type WAFv2WebACLAssociation struct {
	// ResourceArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-wafv2-webaclassociation.html#cfn-wafv2-webaclassociation-resourcearn
	ResourceArn *StringExpr `json:"ResourceArn,omitempty" validate:"dive,required"`
	// WebACLArn docs: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-wafv2-webaclassociation.html#cfn-wafv2-webaclassociation-webaclarn
	WebACLArn *StringExpr `json:"WebACLArn,omitempty" validate:"dive,required"`
}

// CfnResourceType returns AWS::WAFv2::WebACLAssociation to implement the ResourceProperties interface
func (s WAFv2WebACLAssociation) CfnResourceType() string {

	return "AWS::WAFv2::WebACLAssociation"
}

// CfnResourceAttributes returns the attributes produced by this resource
func (s WAFv2WebACLAssociation) CfnResourceAttributes() []string {
	return []string{}
}

// NewResourceByType returns a new resource object correspoding with the provided type
func NewResourceByType(typeName string) ResourceProperties {
	switch typeName {
	case "AWS::CloudWatch::Alarm":
		return &CloudWatchAlarm{}
	case "AWS::ElasticLoadBalancingV2::Listener":
		return &ElasticLoadBalancingV2Listener{}
	case "AWS::ElasticLoadBalancingV2::ListenerCertificate":
		return &ElasticLoadBalancingV2ListenerCertificate{}
	case "AWS::ElasticLoadBalancingV2::ListenerRule":
		return &ElasticLoadBalancingV2ListenerRule{}
	case "AWS::ElasticLoadBalancingV2::LoadBalancer":
		return &ElasticLoadBalancingV2LoadBalancer{}
	case "AWS::ElasticLoadBalancingV2::TargetGroup":
		return &ElasticLoadBalancingV2TargetGroup{}
	case "AWS::WAFRegional::WebACLAssociation":
		return &WAFRegionalWebACLAssociation{}
	case "AWS::WAFv2::WebACLAssociation":
		return &WAFv2WebACLAssociation{}

	default:
		for _, eachProvider := range customResourceProviders {
			customType := eachProvider(typeName)
			if nil != customType {
				return customType
			}
		}
	}
	return nil
}
