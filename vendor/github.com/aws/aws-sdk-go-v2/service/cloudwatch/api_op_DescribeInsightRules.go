// Code generated by smithy-go-codegen DO NOT EDIT.

package cloudwatch

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Returns a list of all the Contributor Insights rules in your account.
//
// For more information about Contributor Insights, see [Using Contributor Insights to Analyze High-Cardinality Data].
//
// [Using Contributor Insights to Analyze High-Cardinality Data]: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContributorInsights.html
func (c *Client) DescribeInsightRules(ctx context.Context, params *DescribeInsightRulesInput, optFns ...func(*Options)) (*DescribeInsightRulesOutput, error) {
	if params == nil {
		params = &DescribeInsightRulesInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "DescribeInsightRules", params, optFns, c.addOperationDescribeInsightRulesMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*DescribeInsightRulesOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type DescribeInsightRulesInput struct {

	// The maximum number of results to return in one operation. If you omit this
	// parameter, the default of 500 is used.
	MaxResults *int32

	// Include this value, if it was returned by the previous operation, to get the
	// next set of rules.
	NextToken *string

	noSmithyDocumentSerde
}

type DescribeInsightRulesOutput struct {

	// The rules returned by the operation.
	InsightRules []types.InsightRule

	// If this parameter is present, it is a token that marks the start of the next
	// batch of returned results.
	NextToken *string

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationDescribeInsightRulesMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsquery_serializeOpDescribeInsightRules{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsquery_deserializeOpDescribeInsightRules{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "DescribeInsightRules"); err != nil {
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
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opDescribeInsightRules(options.Region), middleware.Before); err != nil {
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

// DescribeInsightRulesPaginatorOptions is the paginator options for
// DescribeInsightRules
type DescribeInsightRulesPaginatorOptions struct {
	// The maximum number of results to return in one operation. If you omit this
	// parameter, the default of 500 is used.
	Limit int32

	// Set to true if pagination should stop if the service returns a pagination token
	// that matches the most recent token provided to the service.
	StopOnDuplicateToken bool
}

// DescribeInsightRulesPaginator is a paginator for DescribeInsightRules
type DescribeInsightRulesPaginator struct {
	options   DescribeInsightRulesPaginatorOptions
	client    DescribeInsightRulesAPIClient
	params    *DescribeInsightRulesInput
	nextToken *string
	firstPage bool
}

// NewDescribeInsightRulesPaginator returns a new DescribeInsightRulesPaginator
func NewDescribeInsightRulesPaginator(client DescribeInsightRulesAPIClient, params *DescribeInsightRulesInput, optFns ...func(*DescribeInsightRulesPaginatorOptions)) *DescribeInsightRulesPaginator {
	if params == nil {
		params = &DescribeInsightRulesInput{}
	}

	options := DescribeInsightRulesPaginatorOptions{}
	if params.MaxResults != nil {
		options.Limit = *params.MaxResults
	}

	for _, fn := range optFns {
		fn(&options)
	}

	return &DescribeInsightRulesPaginator{
		options:   options,
		client:    client,
		params:    params,
		firstPage: true,
		nextToken: params.NextToken,
	}
}

// HasMorePages returns a boolean indicating whether more pages are available
func (p *DescribeInsightRulesPaginator) HasMorePages() bool {
	return p.firstPage || (p.nextToken != nil && len(*p.nextToken) != 0)
}

// NextPage retrieves the next DescribeInsightRules page.
func (p *DescribeInsightRulesPaginator) NextPage(ctx context.Context, optFns ...func(*Options)) (*DescribeInsightRulesOutput, error) {
	if !p.HasMorePages() {
		return nil, fmt.Errorf("no more pages available")
	}

	params := *p.params
	params.NextToken = p.nextToken

	var limit *int32
	if p.options.Limit > 0 {
		limit = &p.options.Limit
	}
	params.MaxResults = limit

	optFns = append([]func(*Options){
		addIsPaginatorUserAgent,
	}, optFns...)
	result, err := p.client.DescribeInsightRules(ctx, &params, optFns...)
	if err != nil {
		return nil, err
	}
	p.firstPage = false

	prevToken := p.nextToken
	p.nextToken = result.NextToken

	if p.options.StopOnDuplicateToken &&
		prevToken != nil &&
		p.nextToken != nil &&
		*prevToken == *p.nextToken {
		p.nextToken = nil
	}

	return result, nil
}

// DescribeInsightRulesAPIClient is a client that implements the
// DescribeInsightRules operation.
type DescribeInsightRulesAPIClient interface {
	DescribeInsightRules(context.Context, *DescribeInsightRulesInput, ...func(*Options)) (*DescribeInsightRulesOutput, error)
}

var _ DescribeInsightRulesAPIClient = (*Client)(nil)

func newServiceMetadataMiddleware_opDescribeInsightRules(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "DescribeInsightRules",
	}
}
