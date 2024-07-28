package kubernetes

import (
	"context"
	"fmt"

	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"go.uber.org/zap"
)

func CreateOrUpdateArgoCluster(
	logger *zap.SugaredLogger,
	clusterClient clusterpkg.ClusterServiceClient,
	clusterName, environment string,
) error {
	// List existing clusters
	clusters, err := clusterClient.List(context.Background(), &clusterpkg.ClusterQuery{})
	if err != nil {
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
			Config: appv1alpha1.ClusterConfig{
				TLSClientConfig: appv1alpha1.TLSClientConfig{
					Insecure: false,
				},
			},
			Labels: map[string]string{
				"environment": environment,
			},
		},
	}

	// Create the new cluster
	ctx := context.Background()
	_, err = clusterClient.Create(ctx, newCluster)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	logger.Infof("Cluster %s created successfully.\n", newCluster.Cluster.Name)
	return nil
}
