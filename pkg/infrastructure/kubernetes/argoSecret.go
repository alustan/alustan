package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
)

func CreateOrUpdateArgoCluster(
	logger *zap.SugaredLogger,
	clusterClient cluster.ClusterServiceClient,
	clusterName, environment string,
) error {
	if clusterClient == nil {
		return fmt.Errorf("clusterClient is nil")
	}

	if environment == "" {
		return fmt.Errorf("environment is empty")
	}

	// Create a background context with timeout to avoid indefinite blocking
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Retry logic using client-go wait package
	backoffConfig := wait.Backoff{
		Steps:    3,
		Duration: 5 * time.Second,
		Factor:   1.5,
		Jitter:   0.1,
	}

	err := wait.ExponentialBackoff(backoffConfig, func() (bool, error) {
		// Set gRPC call options
		callOptions := []grpc.CallOption{
			grpc.WaitForReady(true), // Wait for connection to be ready
		}

		// List existing clusters
		clusters, err := clusterClient.List(ctx, &cluster.ClusterQuery{}, callOptions...)
		if err != nil {
			logger.Errorf("Failed to list clusters: %v", err)
			if status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Unavailable {
				// Retry on transient errors
				return false, nil
			}
			return false, fmt.Errorf("failed to list clusters: %w", err)
		}

		var defaultCluster *appv1alpha1.Cluster
		for _, cl := range clusters.Items {
			if cl.Server == "https://kubernetes.default.svc" {
				defaultCluster = &cl
			} else if env, exists := cl.Labels["environment"]; exists && env == environment {
				logger.Info("Found existing cluster with the specified environment, returning without creating or updating a cluster.")
				return true, nil
			}
		}

		if defaultCluster != nil {
			// Update the default cluster with the new environment label
			if _, exists := defaultCluster.Labels["environment"]; !exists || defaultCluster.Labels["environment"] != environment {
				defaultCluster.Labels["environment"] = environment
				updateRequest := &cluster.ClusterUpdateRequest{
					Cluster: defaultCluster,
				}
				_, err := clusterClient.Update(ctx, updateRequest, callOptions...)
				if err != nil {
					logger.Errorf("Failed to update default cluster: %v", err)
					if status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Unavailable {
						// Retry on transient errors
						return false, nil
					}
					return false, fmt.Errorf("failed to update default cluster: %w", err)
				}
				logger.Infof("Default cluster updated successfully with environment label: %s", environment)
			}
			return true, nil
		}

		// Define the new cluster configuration
		newCluster := &cluster.ClusterCreateRequest{
			Cluster: &appv1alpha1.Cluster{
				Name:   clusterName,
				Server: "https://kubernetes.default.svc",
				Labels: map[string]string{
					"environment": environment,
				},
			},
		}

		// Create the new cluster
		_, err = clusterClient.Create(ctx, newCluster, callOptions...)
		if err != nil {
			logger.Errorf("Failed to create cluster: %v", err)
			if status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Unavailable {
				// Retry on transient errors
				return false, nil
			}
			return false, fmt.Errorf("failed to create cluster: %w", err)
		}

		logger.Infof("Cluster %s created successfully.", newCluster.Cluster.Name)
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("exceeded retry limit for creating or listing clusters: %w", err)
	}

	return nil
}
