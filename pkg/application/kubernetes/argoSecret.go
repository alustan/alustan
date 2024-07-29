package kubernetes

import (
	"context"
	"fmt"
	"time"

	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"go.uber.org/zap"
	
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CreateOrUpdateArgoCluster(
	logger *zap.SugaredLogger,
	clusterClient clusterpkg.ClusterServiceClient,
	clusterName, environment string,
) error {
	// Create a background context with timeout to avoid indefinite blocking
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retry logic for transient errors
	backoffConfig := backoff.DefaultConfig
	backoffConfig.MaxDelay = 5 * time.Second

	retryCount := 3
	for i := 0; i < retryCount; i++ {
		// List existing clusters
		clusters, err := clusterClient.List(ctx, &clusterpkg.ClusterQuery{})
		if err != nil {
			if status.Code(err) == codes.Canceled || status.Code(err) == codes.Unavailable {
				logger.Warnf("Attempt %d: failed to list clusters: %v", i+1, err)
				time.Sleep(backoffConfig.MaxDelay)
				continue
			}
			return fmt.Errorf("failed to list clusters: %w", err)
		}

		// Check if any cluster exists
		if len(clusters.Items) > 0 {
			logger.Info("Found existing clusters, returning without creating a new cluster.")
			return nil
		}

		// Define the new cluster configuration
		newCluster := &clusterpkg.ClusterCreateRequest{
			Cluster: &appv1alpha1.Cluster{
				Name:   clusterName,
				Server: "https://kubernetes.default.svc",
				Labels: map[string]string{
					"environment": environment,
				},
			},
		}

		// Create the new cluster
		_, err = clusterClient.Create(ctx, newCluster)
		if err != nil {
			if status.Code(err) == codes.Canceled || status.Code(err) == codes.Unavailable {
				logger.Warnf("Attempt %d: failed to create cluster: %v", i+1, err)
				time.Sleep(backoffConfig.MaxDelay)
				continue
			}
			return fmt.Errorf("failed to create cluster: %w", err)
		}

		logger.Infof("Cluster %s created successfully.", newCluster.Cluster.Name)
		return nil
	}

	return fmt.Errorf("exceeded retry limit for creating or listing clusters")
}
