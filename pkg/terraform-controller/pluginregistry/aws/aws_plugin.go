package aws

import (
	"context"
	"encoding/json"
	"strings"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	
)

type AWSPlugin struct {
	workspace string
	region    string
}

type ResourceInfo struct {
	Service  string
	Resource map[string]interface{}
}

func NewAWSPlugin(workspace, region string) *AWSPlugin {
	return &AWSPlugin{
		workspace: workspace,
		region:    region,
	}
}

func init() {
	// We don't register the plugin here, because we need to pass the workspace, and region dynamically
}

func (p *AWSPlugin) GetDockerfileAdditions() string {
	return `RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
            apt install unzip && \
            unzip awscliv2.zip && \
            ./aws/install && \
            rm -rf awscliv2.zip aws`
}

func (p *AWSPlugin) Execute() (map[string]interface{}, error) {
	resources, err := FetchAWSResourcesWithTag(p.workspace, p.region)
	if (err != nil) {
		return nil, err
	}
	result, err := json.Marshal(resources)
	if (err != nil) {
		return nil, fmt.Errorf("error marshalling creds: %v", err)
	}

	var resultMap map[string]interface{}
	err = json.Unmarshal(result, &resultMap)
	if (err != nil) {
		return nil, fmt.Errorf("error unmarshalling creds: %v", err)
	}
	return resultMap, nil
}

func FetchAWSResourcesWithTag(workspace, region string) ([]ResourceInfo, error) {
	tagKey := "Blueprint"
	tagValue := workspace
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if (err != nil) {
		return nil, fmt.Errorf("unable to load AWS SDK config: %v", err)
	}

	var resources []ResourceInfo

	// Fetch EC2 instances
	ec2Resources, err := fetchEC2Resources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, ec2Resources...)

	// Fetch RDS instances
	rdsResources, err := fetchRDSResources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, rdsResources...)

	// Fetch S3 buckets
	s3Resources, err := fetchS3Resources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, s3Resources...)

	// Fetch EKS clusters
	eksResources, err := fetchEKSResources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, eksResources...)

	// Fetch EBS volumes
	ebsResources, err := fetchEBSResources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, ebsResources...)

	// Fetch Load Balancers
	elbResources, err := fetchALBAndNLBResources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, elbResources...)

	// Fetch DynamoDB tables
	dynamodbResources, err := fetchDynamoDBResources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, dynamodbResources...)

	// Fetch CloudWatch Alarms
	cloudWatchResources, err := fetchCloudWatchResources(cfg, tagKey, tagValue)
	if (err != nil) {
		return nil, err
	}
	resources = append(resources, cloudWatchResources...)

	return resources, nil
}

func fetchEC2Resources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", tagKey)),
				Values: []string{tagValue},
			},
		},
	}

	result, err := svc.DescribeInstances(context.TODO(), input)
	if (err != nil) {
		return nil, fmt.Errorf("failed to describe EC2 instances: %v", err)
	}

	var resources []ResourceInfo
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instanceInfo := map[string]interface{}{
				"InstanceID":   *instance.InstanceId,
				"InstanceType": instance.InstanceType,
				"State":        instance.State.Name,
				"Tags":         instance.Tags,
			}
			resources = append(resources, ResourceInfo{Service: "EC2", Resource: instanceInfo})
		}
	}

	return resources, nil
}

func fetchRDSResources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := rds.NewFromConfig(cfg)

	input := &rds.DescribeDBInstancesInput{}

	result, err := svc.DescribeDBInstances(context.TODO(), input)
	if (err != nil) {
		return nil, fmt.Errorf("failed to describe RDS instances: %v", err)
	}

	var resources []ResourceInfo
	for _, dbInstance := range result.DBInstances {
		for _, tag := range dbInstance.TagList {
			if *tag.Key == tagKey && *tag.Value == tagValue {
				instanceInfo := map[string]interface{}{
					"DBInstanceIdentifier": *dbInstance.DBInstanceIdentifier,
					"DBInstanceClass":      *dbInstance.DBInstanceClass,
					"DBInstanceStatus":     *dbInstance.DBInstanceStatus,
					"Tags":                 dbInstance.TagList,
				}
				resources = append(resources, ResourceInfo{Service: "RDS", Resource: instanceInfo})
			}
		}
	}

	return resources, nil
}

func fetchS3Resources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := s3.NewFromConfig(cfg)

	result, err := svc.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if (err != nil) {
		return nil, fmt.Errorf("failed to list S3 buckets: %v", err)
	}

	var resources []ResourceInfo
	for _, bucket := range result.Buckets {
		tagging, err := svc.GetBucketTagging(context.TODO(), &s3.GetBucketTaggingInput{
			Bucket: bucket.Name,
		})

		if (err != nil) {
			// If the bucket has no tags, skip it
			if strings.Contains(err.Error(), "NoSuchTagSet") {
				continue
			}
			return nil, fmt.Errorf("failed to get tags for bucket %s: %v", *bucket.Name, err)
		}

		for _, tag := range tagging.TagSet {
			if *tag.Key == tagKey && *tag.Value == tagValue {
				bucketInfo := map[string]interface{}{
					"BucketName": *bucket.Name,
					"Tags":       tagging.TagSet,
				}
				resources = append(resources, ResourceInfo{Service: "S3", Resource: bucketInfo})
			}
		}
	}

	return resources, nil
}

func fetchEKSResources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := eks.NewFromConfig(cfg)

	result, err := svc.ListClusters(context.TODO(), &eks.ListClustersInput{})
	if (err != nil) {
		return nil, fmt.Errorf("failed to list EKS clusters: %v", err)
	}

	var resources []ResourceInfo
	for _, clusterName := range result.Clusters {
		describeClusterOutput, err := svc.DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if (err != nil) {
			return nil, fmt.Errorf("failed to describe EKS cluster %s: %v", clusterName, err)
		}

		cluster := describeClusterOutput.Cluster
		for key, value := range cluster.Tags {
			if key == tagKey && value == tagValue {
				clusterInfo := map[string]interface{}{
					"ClusterName": *cluster.Name,
					"Status":      cluster.Status,
					"Tags":        cluster.Tags,
				}
				resources = append(resources, ResourceInfo{Service: "EKS", Resource: clusterInfo})
			}
		}
	}

	return resources, nil
}

func fetchEBSResources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeVolumesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", tagKey)),
				Values: []string{tagValue},
			},
		},
	}

	result, err := svc.DescribeVolumes(context.TODO(), input)
	if (err != nil) {
		return nil, fmt.Errorf("failed to describe EBS volumes: %v", err)
	}

	var resources []ResourceInfo
	for _, volume := range result.Volumes {
		volumeInfo := map[string]interface{}{
			"VolumeID":   *volume.VolumeId,
			"VolumeType": volume.VolumeType,
			"State":      volume.State,
			"Tags":       volume.Tags,
		}
		resources = append(resources, ResourceInfo{Service: "EBS", Resource: volumeInfo})
	}

	return resources, nil
}

func fetchALBAndNLBResources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := elasticloadbalancingv2.NewFromConfig(cfg)

	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{}
	result, err := svc.DescribeLoadBalancers(context.TODO(), input)
	if (err != nil) {
		return nil, fmt.Errorf("failed to describe load balancers: %v", err)
	}

	var resources []ResourceInfo
	for _, loadBalancer := range result.LoadBalancers {
		tagDescription, err := svc.DescribeTags(context.TODO(), &elasticloadbalancingv2.DescribeTagsInput{
			ResourceArns: []string{*loadBalancer.LoadBalancerArn},
		})
		if (err != nil) {
			return nil, fmt.Errorf("failed to get tags for load balancer %s: %v", *loadBalancer.LoadBalancerArn, err)
		}

		for _, tagDesc := range tagDescription.TagDescriptions {
			for _, tag := range tagDesc.Tags {
				if *tag.Key == tagKey && *tag.Value == tagValue {
					loadBalancerInfo := map[string]interface{}{
						"LoadBalancerName": *loadBalancer.LoadBalancerName,
						"Type":             loadBalancer.Type,
						"Scheme":           loadBalancer.Scheme,
						"State":            loadBalancer.State.Code,
						"Tags":             tagDesc.Tags,
					}
					resources = append(resources, ResourceInfo{Service: "ELB", Resource: loadBalancerInfo})
				}
			}
		}
	}

	return resources, nil
}

func fetchDynamoDBResources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := dynamodb.NewFromConfig(cfg)

	input := &dynamodb.ListTablesInput{}
	result, err := svc.ListTables(context.TODO(), input)
	if (err != nil) {
		return nil, fmt.Errorf("failed to list DynamoDB tables: %v", err)
	}

	var resources []ResourceInfo
	for _, tableName := range result.TableNames {
		tagsOutput, err := svc.ListTagsOfResource(context.TODO(), &dynamodb.ListTagsOfResourceInput{
			ResourceArn: aws.String(fmt.Sprintf("arn:aws:dynamodb:%s:%s:table/%s", cfg.Region, "", tableName)),
		})
		if (err != nil) {
			return nil, fmt.Errorf("failed to get tags for DynamoDB table %s: %v", tableName, err)
		}

		for _, tag := range tagsOutput.Tags {
			if *tag.Key == tagKey && *tag.Value == tagValue {
				tableInfo := map[string]interface{}{
					"TableName": tableName,
					"Tags":      tagsOutput.Tags,
				}
				resources = append(resources, ResourceInfo{Service: "DynamoDB", Resource: tableInfo})
			}
		}
	}

	return resources, nil
}

func fetchCloudWatchResources(cfg aws.Config, tagKey, tagValue string) ([]ResourceInfo, error) {
	svc := cloudwatch.NewFromConfig(cfg)

	input := &cloudwatch.DescribeAlarmsInput{}
	result, err := svc.DescribeAlarms(context.TODO(), input)
	if (err != nil) {
		return nil, fmt.Errorf("failed to describe CloudWatch alarms: %v", err)
	}

	var resources []ResourceInfo
	for _, alarm := range result.MetricAlarms {
		alarmTags, err := svc.ListTagsForResource(context.TODO(), &cloudwatch.ListTagsForResourceInput{
			ResourceARN: alarm.AlarmArn,
		})
		if (err != nil) {
			return nil, fmt.Errorf("failed to get tags for CloudWatch alarm %s: %v", *alarm.AlarmName, err)
		}

		for _, tag := range alarmTags.Tags {
			if *tag.Key == tagKey && *tag.Value == tagValue {
				alarmInfo := map[string]interface{}{
					"AlarmName":  *alarm.AlarmName,
					"StateValue": alarm.StateValue,
					"Tags":       alarmTags.Tags,
				}
				resources = append(resources, ResourceInfo{Service: "CloudWatch", Resource: alarmInfo})
			}
		}
	}

	return resources, nil
}
