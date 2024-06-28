// Code generated by smithy-go-codegen DO NOT EDIT.

package rds

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Modifies the properties of a DBProxyTargetGroup .
func (c *Client) ModifyDBProxyTargetGroup(ctx context.Context, params *ModifyDBProxyTargetGroupInput, optFns ...func(*Options)) (*ModifyDBProxyTargetGroupOutput, error) {
	if params == nil {
		params = &ModifyDBProxyTargetGroupInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "ModifyDBProxyTargetGroup", params, optFns, c.addOperationModifyDBProxyTargetGroupMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*ModifyDBProxyTargetGroupOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type ModifyDBProxyTargetGroupInput struct {

	// The name of the proxy.
	//
	// This member is required.
	DBProxyName *string

	// The name of the target group to modify.
	//
	// This member is required.
	TargetGroupName *string

	// The settings that determine the size and behavior of the connection pool for
	// the target group.
	ConnectionPoolConfig *types.ConnectionPoolConfiguration

	// The new name for the modified DBProxyTarget . An identifier must begin with a
	// letter and must contain only ASCII letters, digits, and hyphens; it can't end
	// with a hyphen or contain two consecutive hyphens.
	NewName *string

	noSmithyDocumentSerde
}

type ModifyDBProxyTargetGroupOutput struct {

	// The settings of the modified DBProxyTarget .
	DBProxyTargetGroup *types.DBProxyTargetGroup

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationModifyDBProxyTargetGroupMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsquery_serializeOpModifyDBProxyTargetGroup{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsquery_deserializeOpModifyDBProxyTargetGroup{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "ModifyDBProxyTargetGroup"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = addClientRequestID(stack); err != nil {
		return err
	}
	if err = addComputeContentLength(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = addComputePayloadSHA256(stack); err != nil {
		return err
	}
	if err = addRetry(stack, options); err != nil {
		return err
	}
	if err = addRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = addRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addTimeOffsetBuild(stack, c); err != nil {
		return err
	}
	if err = addUserAgentRetryMode(stack, options); err != nil {
		return err
	}
	if err = addOpModifyDBProxyTargetGroupValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opModifyDBProxyTargetGroup(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addRecursionDetection(stack); err != nil {
		return err
	}
	if err = addRequestIDRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	return nil
}

func newServiceMetadataMiddleware_opModifyDBProxyTargetGroup(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "ModifyDBProxyTargetGroup",
	}
}
