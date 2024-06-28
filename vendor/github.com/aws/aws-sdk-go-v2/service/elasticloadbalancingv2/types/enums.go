// Code generated by smithy-go-codegen DO NOT EDIT.

package types

type ActionTypeEnum string

// Enum values for ActionTypeEnum
const (
	ActionTypeEnumForward             ActionTypeEnum = "forward"
	ActionTypeEnumAuthenticateOidc    ActionTypeEnum = "authenticate-oidc"
	ActionTypeEnumAuthenticateCognito ActionTypeEnum = "authenticate-cognito"
	ActionTypeEnumRedirect            ActionTypeEnum = "redirect"
	ActionTypeEnumFixedResponse       ActionTypeEnum = "fixed-response"
)

// Values returns all known values for ActionTypeEnum. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (ActionTypeEnum) Values() []ActionTypeEnum {
	return []ActionTypeEnum{
		"forward",
		"authenticate-oidc",
		"authenticate-cognito",
		"redirect",
		"fixed-response",
	}
}

type AnomalyResultEnum string

// Enum values for AnomalyResultEnum
const (
	AnomalyResultEnumAnomalous AnomalyResultEnum = "anomalous"
	AnomalyResultEnumNormal    AnomalyResultEnum = "normal"
)

// Values returns all known values for AnomalyResultEnum. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (AnomalyResultEnum) Values() []AnomalyResultEnum {
	return []AnomalyResultEnum{
		"anomalous",
		"normal",
	}
}

type AuthenticateCognitoActionConditionalBehaviorEnum string

// Enum values for AuthenticateCognitoActionConditionalBehaviorEnum
const (
	AuthenticateCognitoActionConditionalBehaviorEnumDeny         AuthenticateCognitoActionConditionalBehaviorEnum = "deny"
	AuthenticateCognitoActionConditionalBehaviorEnumAllow        AuthenticateCognitoActionConditionalBehaviorEnum = "allow"
	AuthenticateCognitoActionConditionalBehaviorEnumAuthenticate AuthenticateCognitoActionConditionalBehaviorEnum = "authenticate"
)

// Values returns all known values for
// AuthenticateCognitoActionConditionalBehaviorEnum. Note that this can be expanded
// in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (AuthenticateCognitoActionConditionalBehaviorEnum) Values() []AuthenticateCognitoActionConditionalBehaviorEnum {
	return []AuthenticateCognitoActionConditionalBehaviorEnum{
		"deny",
		"allow",
		"authenticate",
	}
}

type AuthenticateOidcActionConditionalBehaviorEnum string

// Enum values for AuthenticateOidcActionConditionalBehaviorEnum
const (
	AuthenticateOidcActionConditionalBehaviorEnumDeny         AuthenticateOidcActionConditionalBehaviorEnum = "deny"
	AuthenticateOidcActionConditionalBehaviorEnumAllow        AuthenticateOidcActionConditionalBehaviorEnum = "allow"
	AuthenticateOidcActionConditionalBehaviorEnumAuthenticate AuthenticateOidcActionConditionalBehaviorEnum = "authenticate"
)

// Values returns all known values for
// AuthenticateOidcActionConditionalBehaviorEnum. Note that this can be expanded in
// the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (AuthenticateOidcActionConditionalBehaviorEnum) Values() []AuthenticateOidcActionConditionalBehaviorEnum {
	return []AuthenticateOidcActionConditionalBehaviorEnum{
		"deny",
		"allow",
		"authenticate",
	}
}

type DescribeTargetHealthInputIncludeEnum string

// Enum values for DescribeTargetHealthInputIncludeEnum
const (
	DescribeTargetHealthInputIncludeEnumAnomaly DescribeTargetHealthInputIncludeEnum = "AnomalyDetection"
	DescribeTargetHealthInputIncludeEnumAll     DescribeTargetHealthInputIncludeEnum = "All"
)

// Values returns all known values for DescribeTargetHealthInputIncludeEnum. Note
// that this can be expanded in the future, and so it is only as up to date as the
// client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (DescribeTargetHealthInputIncludeEnum) Values() []DescribeTargetHealthInputIncludeEnum {
	return []DescribeTargetHealthInputIncludeEnum{
		"AnomalyDetection",
		"All",
	}
}

type EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum string

// Enum values for EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum
const (
	EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnumOn  EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum = "on"
	EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnumOff EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum = "off"
)

// Values returns all known values for
// EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum) Values() []EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum {
	return []EnforceSecurityGroupInboundRulesOnPrivateLinkTrafficEnum{
		"on",
		"off",
	}
}

type IpAddressType string

// Enum values for IpAddressType
const (
	IpAddressTypeIpv4                       IpAddressType = "ipv4"
	IpAddressTypeDualstack                  IpAddressType = "dualstack"
	IpAddressTypeDualstackWithoutPublicIpv4 IpAddressType = "dualstack-without-public-ipv4"
)

// Values returns all known values for IpAddressType. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (IpAddressType) Values() []IpAddressType {
	return []IpAddressType{
		"ipv4",
		"dualstack",
		"dualstack-without-public-ipv4",
	}
}

type LoadBalancerSchemeEnum string

// Enum values for LoadBalancerSchemeEnum
const (
	LoadBalancerSchemeEnumInternetFacing LoadBalancerSchemeEnum = "internet-facing"
	LoadBalancerSchemeEnumInternal       LoadBalancerSchemeEnum = "internal"
)

// Values returns all known values for LoadBalancerSchemeEnum. Note that this can
// be expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (LoadBalancerSchemeEnum) Values() []LoadBalancerSchemeEnum {
	return []LoadBalancerSchemeEnum{
		"internet-facing",
		"internal",
	}
}

type LoadBalancerStateEnum string

// Enum values for LoadBalancerStateEnum
const (
	LoadBalancerStateEnumActive         LoadBalancerStateEnum = "active"
	LoadBalancerStateEnumProvisioning   LoadBalancerStateEnum = "provisioning"
	LoadBalancerStateEnumActiveImpaired LoadBalancerStateEnum = "active_impaired"
	LoadBalancerStateEnumFailed         LoadBalancerStateEnum = "failed"
)

// Values returns all known values for LoadBalancerStateEnum. Note that this can
// be expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (LoadBalancerStateEnum) Values() []LoadBalancerStateEnum {
	return []LoadBalancerStateEnum{
		"active",
		"provisioning",
		"active_impaired",
		"failed",
	}
}

type LoadBalancerTypeEnum string

// Enum values for LoadBalancerTypeEnum
const (
	LoadBalancerTypeEnumApplication LoadBalancerTypeEnum = "application"
	LoadBalancerTypeEnumNetwork     LoadBalancerTypeEnum = "network"
	LoadBalancerTypeEnumGateway     LoadBalancerTypeEnum = "gateway"
)

// Values returns all known values for LoadBalancerTypeEnum. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (LoadBalancerTypeEnum) Values() []LoadBalancerTypeEnum {
	return []LoadBalancerTypeEnum{
		"application",
		"network",
		"gateway",
	}
}

type MitigationInEffectEnum string

// Enum values for MitigationInEffectEnum
const (
	MitigationInEffectEnumYes MitigationInEffectEnum = "yes"
	MitigationInEffectEnumNo  MitigationInEffectEnum = "no"
)

// Values returns all known values for MitigationInEffectEnum. Note that this can
// be expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (MitigationInEffectEnum) Values() []MitigationInEffectEnum {
	return []MitigationInEffectEnum{
		"yes",
		"no",
	}
}

type ProtocolEnum string

// Enum values for ProtocolEnum
const (
	ProtocolEnumHttp   ProtocolEnum = "HTTP"
	ProtocolEnumHttps  ProtocolEnum = "HTTPS"
	ProtocolEnumTcp    ProtocolEnum = "TCP"
	ProtocolEnumTls    ProtocolEnum = "TLS"
	ProtocolEnumUdp    ProtocolEnum = "UDP"
	ProtocolEnumTcpUdp ProtocolEnum = "TCP_UDP"
	ProtocolEnumGeneve ProtocolEnum = "GENEVE"
)

// Values returns all known values for ProtocolEnum. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (ProtocolEnum) Values() []ProtocolEnum {
	return []ProtocolEnum{
		"HTTP",
		"HTTPS",
		"TCP",
		"TLS",
		"UDP",
		"TCP_UDP",
		"GENEVE",
	}
}

type RedirectActionStatusCodeEnum string

// Enum values for RedirectActionStatusCodeEnum
const (
	RedirectActionStatusCodeEnumHttp301 RedirectActionStatusCodeEnum = "HTTP_301"
	RedirectActionStatusCodeEnumHttp302 RedirectActionStatusCodeEnum = "HTTP_302"
)

// Values returns all known values for RedirectActionStatusCodeEnum. Note that
// this can be expanded in the future, and so it is only as up to date as the
// client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (RedirectActionStatusCodeEnum) Values() []RedirectActionStatusCodeEnum {
	return []RedirectActionStatusCodeEnum{
		"HTTP_301",
		"HTTP_302",
	}
}

type RevocationType string

// Enum values for RevocationType
const (
	RevocationTypeCrl RevocationType = "CRL"
)

// Values returns all known values for RevocationType. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (RevocationType) Values() []RevocationType {
	return []RevocationType{
		"CRL",
	}
}

type TargetGroupIpAddressTypeEnum string

// Enum values for TargetGroupIpAddressTypeEnum
const (
	TargetGroupIpAddressTypeEnumIpv4 TargetGroupIpAddressTypeEnum = "ipv4"
	TargetGroupIpAddressTypeEnumIpv6 TargetGroupIpAddressTypeEnum = "ipv6"
)

// Values returns all known values for TargetGroupIpAddressTypeEnum. Note that
// this can be expanded in the future, and so it is only as up to date as the
// client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (TargetGroupIpAddressTypeEnum) Values() []TargetGroupIpAddressTypeEnum {
	return []TargetGroupIpAddressTypeEnum{
		"ipv4",
		"ipv6",
	}
}

type TargetHealthReasonEnum string

// Enum values for TargetHealthReasonEnum
const (
	TargetHealthReasonEnumRegistrationInProgress   TargetHealthReasonEnum = "Elb.RegistrationInProgress"
	TargetHealthReasonEnumInitialHealthChecking    TargetHealthReasonEnum = "Elb.InitialHealthChecking"
	TargetHealthReasonEnumResponseCodeMismatch     TargetHealthReasonEnum = "Target.ResponseCodeMismatch"
	TargetHealthReasonEnumTimeout                  TargetHealthReasonEnum = "Target.Timeout"
	TargetHealthReasonEnumFailedHealthChecks       TargetHealthReasonEnum = "Target.FailedHealthChecks"
	TargetHealthReasonEnumNotRegistered            TargetHealthReasonEnum = "Target.NotRegistered"
	TargetHealthReasonEnumNotInUse                 TargetHealthReasonEnum = "Target.NotInUse"
	TargetHealthReasonEnumDeregistrationInProgress TargetHealthReasonEnum = "Target.DeregistrationInProgress"
	TargetHealthReasonEnumInvalidState             TargetHealthReasonEnum = "Target.InvalidState"
	TargetHealthReasonEnumIpUnusable               TargetHealthReasonEnum = "Target.IpUnusable"
	TargetHealthReasonEnumHealthCheckDisabled      TargetHealthReasonEnum = "Target.HealthCheckDisabled"
	TargetHealthReasonEnumInternalError            TargetHealthReasonEnum = "Elb.InternalError"
)

// Values returns all known values for TargetHealthReasonEnum. Note that this can
// be expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (TargetHealthReasonEnum) Values() []TargetHealthReasonEnum {
	return []TargetHealthReasonEnum{
		"Elb.RegistrationInProgress",
		"Elb.InitialHealthChecking",
		"Target.ResponseCodeMismatch",
		"Target.Timeout",
		"Target.FailedHealthChecks",
		"Target.NotRegistered",
		"Target.NotInUse",
		"Target.DeregistrationInProgress",
		"Target.InvalidState",
		"Target.IpUnusable",
		"Target.HealthCheckDisabled",
		"Elb.InternalError",
	}
}

type TargetHealthStateEnum string

// Enum values for TargetHealthStateEnum
const (
	TargetHealthStateEnumInitial           TargetHealthStateEnum = "initial"
	TargetHealthStateEnumHealthy           TargetHealthStateEnum = "healthy"
	TargetHealthStateEnumUnhealthy         TargetHealthStateEnum = "unhealthy"
	TargetHealthStateEnumUnhealthyDraining TargetHealthStateEnum = "unhealthy.draining"
	TargetHealthStateEnumUnused            TargetHealthStateEnum = "unused"
	TargetHealthStateEnumDraining          TargetHealthStateEnum = "draining"
	TargetHealthStateEnumUnavailable       TargetHealthStateEnum = "unavailable"
)

// Values returns all known values for TargetHealthStateEnum. Note that this can
// be expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (TargetHealthStateEnum) Values() []TargetHealthStateEnum {
	return []TargetHealthStateEnum{
		"initial",
		"healthy",
		"unhealthy",
		"unhealthy.draining",
		"unused",
		"draining",
		"unavailable",
	}
}

type TargetTypeEnum string

// Enum values for TargetTypeEnum
const (
	TargetTypeEnumInstance TargetTypeEnum = "instance"
	TargetTypeEnumIp       TargetTypeEnum = "ip"
	TargetTypeEnumLambda   TargetTypeEnum = "lambda"
	TargetTypeEnumAlb      TargetTypeEnum = "alb"
)

// Values returns all known values for TargetTypeEnum. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (TargetTypeEnum) Values() []TargetTypeEnum {
	return []TargetTypeEnum{
		"instance",
		"ip",
		"lambda",
		"alb",
	}
}

type TrustStoreStatus string

// Enum values for TrustStoreStatus
const (
	TrustStoreStatusActive   TrustStoreStatus = "ACTIVE"
	TrustStoreStatusCreating TrustStoreStatus = "CREATING"
)

// Values returns all known values for TrustStoreStatus. Note that this can be
// expanded in the future, and so it is only as up to date as the client.
//
// The ordering of this slice is not guaranteed to be stable across updates.
func (TrustStoreStatus) Values() []TrustStoreStatus {
	return []TrustStoreStatus{
		"ACTIVE",
		"CREATING",
	}
}
