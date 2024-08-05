#!/bin/bash

# Load environment variables from .env file if needed
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

# Sanitize environment variables
DOCKER_USERNAME=$(echo "$DOCKER_USERNAME" | tr -d '\r')
DOCKER_TOKEN=$(echo "$DOCKER_TOKEN" | tr -d '\r')
HELM_VERSION=$(echo "$HELM_VERSION" | tr -d '\r')
GIT_TOKEN=$(echo "$GIT_TOKEN" | tr -d '\r')
GIT_ORG_URL=$(echo "$GIT_ORG_URL" | tr -d '\r')
GIT_SSH_SECRET=$(echo "$GIT_SSH_SECRET" | tr -d '\r')

# Authenticate with Docker Hub
echo "$DOCKER_TOKEN" | docker login --username "$DOCKER_USERNAME" --password-stdin

# Pull the images from Docker Hub
docker pull alustan/example:0.31.0
docker pull alustan/web-app-demo:0.10.0

# Tag the images
docker tag alustan/example:0.31.0 "$DOCKER_USERNAME"/example:0.31.0
docker tag alustan/web-app-demo:0.10.0 "$DOCKER_USERNAME"/web-app-demo:0.10.0

# Push the images to your registry
docker push "$DOCKER_USERNAME"/example:0.31.0
docker push "$DOCKER_USERNAME"/web-app-demo:0.10.0

# Generate Docker config and encode it in base64
DOCKER_CONFIG_JSON=$(cat ~/.docker/config.json | base64 -w 0)

kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# Create the namespace
kubectl create ns alustan

# Update the imageName field in the YAML configuration using yq
yq --inplace ".spec.containerRegistry.imageName = \"${DOCKER_USERNAME}/example\"" examples/infra/basic.yaml
yq --inplace ".spec.source.values.image.repository = \"${DOCKER_USERNAME}/web-app-demo\"" examples/app/basic.yaml
yq --inplace ".spec.containerRegistry.imageName = \"${DOCKER_USERNAME}/web-app-demo\"" examples/app/basic.yaml
yq --inplace ".spec.source.values.image.repository = \"${DOCKER_USERNAME}/web-app-demo\"" examples/app/preview.yaml

# Check if DEVELOP is true
if [ "$DEVELOP" = "true" ]; then
  REPO_URL="$DOCKER_USERNAME"
else
  REPO_URL="alustan"
fi

# Install the Helm chart with the version from the .env file, injecting the Docker config directly
helm install "alustan-controller" oci://registry-1.docker.io/${REPO_URL}/alustan-helm \
  --version "$HELM_VERSION" \
  --timeout 20m0s \
  --debug \
  --atomic \
  --set containerRegistry.containerRegistrySecret="$DOCKER_CONFIG_JSON" \
  --set gitToken="$GIT_TOKEN" \
  --set gitOrgUrl="$GIT_ORG_URL" \
  --set gitSSHSecret="$GIT_SSH_SECRET"
