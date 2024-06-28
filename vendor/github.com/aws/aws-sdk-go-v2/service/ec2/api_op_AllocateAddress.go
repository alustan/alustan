// Code generated by smithy-go-codegen DO NOT EDIT.

package ec2

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Allocates an Elastic IP address to your Amazon Web Services account. After you
// allocate the Elastic IP address you can associate it with an instance or network
// interface. After you release an Elastic IP address, it is released to the IP
// address pool and can be allocated to a different Amazon Web Services account.
//
// You can allocate an Elastic IP address from an address pool owned by Amazon Web
// Services or from an address pool created from a public IPv4 address range that
// you have brought to Amazon Web Services for use with your Amazon Web Services
// resources using bring your own IP addresses (BYOIP). For more information, see [Bring Your Own IP Addresses (BYOIP)]
// in the Amazon EC2 User Guide.
//
// If you release an Elastic IP address, you might be able to recover it. You
// cannot recover an Elastic IP address that you released after it is allocated to
// another Amazon Web Services account. To attempt to recover an Elastic IP address
// that you released, specify it in this operation.
//
// For more information, see [Elastic IP Addresses] in the Amazon EC2 User Guide.
//
// You can allocate a carrier IP address which is a public IP address from a
// telecommunication carrier, to a network interface which resides in a subnet in a
// Wavelength Zone (for example an EC2 instance).
//
// [Bring Your Own IP Addresses (BYOIP)]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-byoip.html
// [Elastic IP Addresses]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/elastic-ip-addresses-eip.html
func (c *Client) AllocateAddress(ctx context.Context, params *AllocateAddressInput, optFns ...func(*Options)) (*AllocateAddressOutput, error) {
	if params == nil {
		params = &AllocateAddressInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "AllocateAddress", params, optFns, c.addOperationAllocateAddressMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*AllocateAddressOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type AllocateAddressInput struct {

	// The Elastic IP address to recover or an IPv4 address from an address pool.
	Address *string

	// The ID of a customer-owned address pool. Use this parameter to let Amazon EC2
	// select an address from the address pool. Alternatively, specify a specific
	// address from the address pool.
	CustomerOwnedIpv4Pool *string

	// The network ( vpc ).
	Domain types.DomainType

	// Checks whether you have the required permissions for the action, without
	// actually making the request, and provides an error response. If you have the
	// required permissions, the error response is DryRunOperation . Otherwise, it is
	// UnauthorizedOperation .
	DryRun *bool

	//  A unique set of Availability Zones, Local Zones, or Wavelength Zones from
	// which Amazon Web Services advertises IP addresses. Use this parameter to limit
	// the IP address to this location. IP addresses cannot move between network border
	// groups.
	NetworkBorderGroup *string

	// The ID of an address pool that you own. Use this parameter to let Amazon EC2
	// select an address from the address pool. To specify a specific address from the
	// address pool, use the Address parameter instead.
	PublicIpv4Pool *string

	// The tags to assign to the Elastic IP address.
	TagSpecifications []types.TagSpecification

	noSmithyDocumentSerde
}

type AllocateAddressOutput struct {

	// The ID that represents the allocation of the Elastic IP address.
	AllocationId *string

	// The carrier IP address. This option is only available for network interfaces
	// that reside in a subnet in a Wavelength Zone.
	CarrierIp *string

	// The customer-owned IP address.
	CustomerOwnedIp *string

	// The ID of the customer-owned address pool.
	CustomerOwnedIpv4Pool *string

	// The network ( vpc ).
	Domain types.DomainType

	// The set of Availability Zones, Local Zones, or Wavelength Zones from which
	// Amazon Web Services advertises IP addresses.
	NetworkBorderGroup *string

	// The Elastic IP address.
	PublicIp *string

	// The ID of an address pool.
	PublicIpv4Pool *string

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationAllocateAddressMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsEc2query_serializeOpAllocateAddress{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsEc2query_deserializeOpAllocateAddress{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "AllocateAddress"); err != nil {
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
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opAllocateAddress(options.Region), middleware.Before); err != nil {
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

func newServiceMetadataMiddleware_opAllocateAddress(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "AllocateAddress",
	}
}
