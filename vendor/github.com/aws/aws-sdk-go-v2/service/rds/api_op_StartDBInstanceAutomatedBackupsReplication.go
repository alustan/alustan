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

// Enables replication of automated backups to a different Amazon Web Services
// Region.
//
// This command doesn't apply to RDS Custom.
//
// For more information, see [Replicating Automated Backups to Another Amazon Web Services Region] in the Amazon RDS User Guide.
//
// [Replicating Automated Backups to Another Amazon Web Services Region]: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ReplicateBackups.html
func (c *Client) StartDBInstanceAutomatedBackupsReplication(ctx context.Context, params *StartDBInstanceAutomatedBackupsReplicationInput, optFns ...func(*Options)) (*StartDBInstanceAutomatedBackupsReplicationOutput, error) {
	if params == nil {
		params = &StartDBInstanceAutomatedBackupsReplicationInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "StartDBInstanceAutomatedBackupsReplication", params, optFns, c.addOperationStartDBInstanceAutomatedBackupsReplicationMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*StartDBInstanceAutomatedBackupsReplicationOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type StartDBInstanceAutomatedBackupsReplicationInput struct {

	// The Amazon Resource Name (ARN) of the source DB instance for the replicated
	// automated backups, for example, arn:aws:rds:us-west-2:123456789012:db:mydatabase
	// .
	//
	// This member is required.
	SourceDBInstanceArn *string

	// The retention period for the replicated automated backups.
	BackupRetentionPeriod *int32

	// The Amazon Web Services KMS key identifier for encryption of the replicated
	// automated backups. The KMS key ID is the Amazon Resource Name (ARN) for the KMS
	// encryption key in the destination Amazon Web Services Region, for example,
	// arn:aws:kms:us-east-1:123456789012:key/AKIAIOSFODNN7EXAMPLE .
	KmsKeyId *string

	// In an Amazon Web Services GovCloud (US) Region, an URL that contains a
	// Signature Version 4 signed request for the
	// StartDBInstanceAutomatedBackupsReplication operation to call in the Amazon Web
	// Services Region of the source DB instance. The presigned URL must be a valid
	// request for the StartDBInstanceAutomatedBackupsReplication API operation that
	// can run in the Amazon Web Services Region that contains the source DB instance.
	//
	// This setting applies only to Amazon Web Services GovCloud (US) Regions. It's
	// ignored in other Amazon Web Services Regions.
	//
	// To learn how to generate a Signature Version 4 signed request, see [Authenticating Requests: Using Query Parameters (Amazon Web Services Signature Version 4)] and [Signature Version 4 Signing Process].
	//
	// If you are using an Amazon Web Services SDK tool or the CLI, you can specify
	// SourceRegion (or --source-region for the CLI) instead of specifying PreSignedUrl
	// manually. Specifying SourceRegion autogenerates a presigned URL that is a valid
	// request for the operation that can run in the source Amazon Web Services Region.
	//
	// [Authenticating Requests: Using Query Parameters (Amazon Web Services Signature Version 4)]: https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-query-string-auth.html
	// [Signature Version 4 Signing Process]: https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html
	PreSignedUrl *string

	noSmithyDocumentSerde
}

type StartDBInstanceAutomatedBackupsReplicationOutput struct {

	// An automated backup of a DB instance. It consists of system backups,
	// transaction logs, and the database instance properties that existed at the time
	// you deleted the source instance.
	DBInstanceAutomatedBackup *types.DBInstanceAutomatedBackup

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationStartDBInstanceAutomatedBackupsReplicationMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsquery_serializeOpStartDBInstanceAutomatedBackupsReplication{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsquery_deserializeOpStartDBInstanceAutomatedBackupsReplication{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "StartDBInstanceAutomatedBackupsReplication"); err != nil {
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
	if err = addOpStartDBInstanceAutomatedBackupsReplicationValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opStartDBInstanceAutomatedBackupsReplication(options.Region), middleware.Before); err != nil {
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

func newServiceMetadataMiddleware_opStartDBInstanceAutomatedBackupsReplication(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "StartDBInstanceAutomatedBackupsReplication",
	}
}
