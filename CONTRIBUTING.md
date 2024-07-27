# Contributing guide

Contributions are what makes the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

## Setting up a development environment

To get started with the project on your machine, you need to install the following tools:
1. Go 1.22+. 
2. Make. 
3. Docker. 
4. Kubernets cluster (local/remote)
5. kubectl

- Once required tools are installed, fork and clone this repository. `https://github.com/alustan/alustan`.

- Checkout a feat branch

> `Setup your github workflow secrets`: will be required to push your controller image and helm chart to your OCI registry

> `make help`: for relevant make commands


## Develop

**Terraform controller**

`make build-infra` to ensure you can succesfully build binary locally

- To quickly test functionality locally

`git clone https://github.com/alustan/basic-example.git`

> You can build and push your own image using the **workflow_dispatch** of github action


- fetch and untar the helm chart

```sh
helm fetch oci://registry-1.docker.io/alustan/alustan-helm --version <version> --untar=true
```

**To obtain `containerRegistrySecret` to be supplied to the helm chart: RUN the script below and copy the encoded secret** 

 - **If using `dockerhub` as OCI registry**

```sh
rm ~/.docker/config.json
docker login -u <YOUR_DOCKERHUB_USERNAME> -p <YOUR_DOCKERHUB_PAT>
cat ~/.docker/config.json > secret.json
base64 -w 0 secret.json 

```

- **If using `GHCR` as OCI registry**

```sh
rm ~/.docker/config.json
docker login ghcr.io -u <YOUR_GITHUB_USERNAME> -p <YOUR_GITHUB_PAT>
cat ~/.docker/config.json > secret.json
base64 -w 0 secret.json 

```
- To get the terraform controller logs

`kubectl logs <terraform-controller-pod> -n alustan`

- You can apply the manifest below and observe progress 

> replace imageName with your built image

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: dummy
spec:
  variables:
    TF_VAR_workspace: "default"
    TF_VAR_region: "us-east-1"
    TF_VAR_provision_cluster: "true"
    TF_VAR_provision_db: "false"
    TF_VAR_vpc_cidr: "10.0.0.0/16"
  scripts:
    deploy: deploy.sh
    destroy: destroy.sh
  postDeploy:
    script: postdeploy.sh
    args:
      workspace: TF_VAR_workspace
      region: TF_VAR_region
  containerRegistry:
    provider: docker
    imageName: alustan/example  #build your own image from this repo alustan/basic-example since the 
                               # controller will require access to your registry to get tags that match   semantic constraint. Add registry secret to helm values files as specified in Readme before installing the helm chart in a k8s cluster
    semanticVersion: ">=0.1.0"

```

**App controller**

- To get the app controller logs

`kubectl logs <app-controller-pod> -n alustan`

> Ensure argocd server is succesfully ruuning

- You can apply the manifest below and observe progess 

```yaml
apiVersion: alustan.io/v1alpha1
kind: App
metadata:
  name: web-service
spec:
  workspace: default
  source:
    repoURL: https://github.com/alustan/cluster-manifests
    path: basic-demo
    releaseName: basic-demo
    targetRevision: main
    values:
      cluster: ${workspace.CLUSTER_NAME}
      service: frontend
      image: alustan/web-app-demo:1.0.0
      config:
        DUMMY_1: ${workspace.dummy_output_1}
        DUMMY_2: ${workspace.dummy_output_2}

  containerRegistry:
    provider: docker
    imageName: alustan/web-app-demo
    semanticVersion: ">=1.0.0"

```

## Pull Request

This repository requires a [Developer Certificate of Origin (DCO)](https://developercertificate.org/) signature. 
When preparing to send in a pull request, please make sure your commit is signed. You can achieve this by doing a `git commit -s -m 'This is my commit message'` .

