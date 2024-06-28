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

// Displays backups for both current and deleted DB clusters. For example, use
// this operation to find details about automated backups for previously deleted
// clusters. Current clusters are returned for both the
// DescribeDBClusterAutomatedBackups and DescribeDBClusters operations.
//
// All parameters are optional.
func (c *Client) DescribeDBClusterAutomatedBackups(ctx context.Context, params *DescribeDBClusterAutomatedBackupsInput, optFns ...func(*Options)) (*DescribeDBClusterAutomatedBackupsOutput, error) {
	if params == nil {
		params = &DescribeDBClusterAutomatedBackupsInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "DescribeDBClusterAutomatedBackups", params, optFns, c.addOperationDescribeDBClusterAutomatedBackupsMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*DescribeDBClusterAutomatedBackupsOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type DescribeDBClusterAutomatedBackupsInput struct {

	// (Optional) The user-supplied DB cluster identifier. If this parameter is
	// specified, it must match the identifier of an existing DB cluster. It returns
	// information from the specific DB cluster's automated backup. This parameter
	// isn't case-sensitive.
	DBClusterIdentifier *string

	// The resource ID of the DB cluster that is the source of the automated backup.
	// This parameter isn't case-sensitive.
	DbClusterResourceId *string

	// A filter that specifies which resources to return based on status.
	//
	// Supported filters are the following:
	//
	//   - status
	//
	//   - retained - Automated backups for deleted clusters and after backup
	//   replication is stopped.
	//
	//   - db-cluster-id - Accepts DB cluster identifiers and Amazon Resource Names
	//   (ARNs). The results list includes only information about the DB cluster
	//   automated backups identified by these ARNs.
	//
	//   - db-cluster-resource-id - Accepts DB resource identifiers and Amazon Resource
	//   Names (ARNs). The results list includes only information about the DB cluster
	//   resources identified by these ARNs.
	//
	// Returns all resources by default. The status for each resource is specified in
	// the response.
	Filters []types.Filter

	// The pagination token provided in the previous request. If this parameter is
	// specified the response includes only records beyond the marker, up to MaxRecords
	// .
	Marker *string

	// The maximum number of records to include in the response. If more records exist
	// than the specified MaxRecords value, a pagination token called a marker is
	// included in the response so that you can retrieve the remaining results.
	MaxRecords *int32

	noSmithyDocumentSerde
}

type DescribeDBClusterAutomatedBackupsOutput struct {

	// A list of DBClusterAutomatedBackup backups.
	DBClusterAutomatedBackups []types.DBClusterAutomatedBackup

	// The pagination token provided in the previous request. If this parameter is
	// specified the response includes only records beyond the marker, up to MaxRecords
	// .
	Marker *string

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationDescribeDBClusterAutomatedBackupsMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsquery_serializeOpDescribeDBClusterAutomatedBackups{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsquery_deserializeOpDescribeDBClusterAutomatedBackups{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "DescribeDBClusterAutomatedBackups"); err != nil {
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
	if err = addOpDescribeDBClusterAutomatedBackupsValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opDescribeDBClusterAutomatedBackups(options.Region), middleware.Before); err != nil {
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

// DescribeDBClusterAutomatedBackupsPaginatorOptions is the paginator options for
// DescribeDBClusterAutomatedBackups
type DescribeDBClusterAutomatedBackupsPaginatorOptions struct {
	// The maximum number of records to include in the response. If more records exist
	// than the specified MaxRecords value, a pagination token called a marker is
	// included in the response so that you can retrieve the remaining results.
	Limit int32

	// Set to true if pagination should stop if the service returns a pagination token
	// that matches the most recent token provided to the service.
	StopOnDuplicateToken bool
}

// DescribeDBClusterAutomatedBackupsPaginator is a paginator for
// DescribeDBClusterAutomatedBackups
type DescribeDBClusterAutomatedBackupsPaginator struct {
	options   DescribeDBClusterAutomatedBackupsPaginatorOptions
	client    DescribeDBClusterAutomatedBackupsAPIClient
	params    *DescribeDBClusterAutomatedBackupsInput
	nextToken *string
	firstPage bool
}

// NewDescribeDBClusterAutomatedBackupsPaginator returns a new
// DescribeDBClusterAutomatedBackupsPaginator
func NewDescribeDBClusterAutomatedBackupsPaginator(client DescribeDBClusterAutomatedBackupsAPIClient, params *DescribeDBClusterAutomatedBackupsInput, optFns ...func(*DescribeDBClusterAutomatedBackupsPaginatorOptions)) *DescribeDBClusterAutomatedBackupsPaginator {
	if params == nil {
		params = &DescribeDBClusterAutomatedBackupsInput{}
	}

	options := DescribeDBClusterAutomatedBackupsPaginatorOptions{}
	if params.MaxRecords != nil {
		options.Limit = *params.MaxRecords
	}

	for _, fn := range optFns {
		fn(&options)
	}

	return &DescribeDBClusterAutomatedBackupsPaginator{
		options:   options,
		client:    client,
		params:    params,
		firstPage: true,
		nextToken: params.Marker,
	}
}

// HasMorePages returns a boolean indicating whether more pages are available
func (p *DescribeDBClusterAutomatedBackupsPaginator) HasMorePages() bool {
	return p.firstPage || (p.nextToken != nil && len(*p.nextToken) != 0)
}

// NextPage retrieves the next DescribeDBClusterAutomatedBackups page.
func (p *DescribeDBClusterAutomatedBackupsPaginator) NextPage(ctx context.Context, optFns ...func(*Options)) (*DescribeDBClusterAutomatedBackupsOutput, error) {
	if !p.HasMorePages() {
		return nil, fmt.Errorf("no more pages available")
	}

	params := *p.params
	params.Marker = p.nextToken

	var limit *int32
	if p.options.Limit > 0 {
		limit = &p.options.Limit
	}
	params.MaxRecords = limit

	optFns = append([]func(*Options){
		addIsPaginatorUserAgent,
	}, optFns...)
	result, err := p.client.DescribeDBClusterAutomatedBackups(ctx, &params, optFns...)
	if err != nil {
		return nil, err
	}
	p.firstPage = false

	prevToken := p.nextToken
	p.nextToken = result.Marker

	if p.options.StopOnDuplicateToken &&
		prevToken != nil &&
		p.nextToken != nil &&
		*prevToken == *p.nextToken {
		p.nextToken = nil
	}

	return result, nil
}

// DescribeDBClusterAutomatedBackupsAPIClient is a client that implements the
// DescribeDBClusterAutomatedBackups operation.
type DescribeDBClusterAutomatedBackupsAPIClient interface {
	DescribeDBClusterAutomatedBackups(context.Context, *DescribeDBClusterAutomatedBackupsInput, ...func(*Options)) (*DescribeDBClusterAutomatedBackupsOutput, error)
}

var _ DescribeDBClusterAutomatedBackupsAPIClient = (*Client)(nil)

func newServiceMetadataMiddleware_opDescribeDBClusterAutomatedBackups(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "DescribeDBClusterAutomatedBackups",
	}
}
