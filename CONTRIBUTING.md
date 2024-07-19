# Contributing guide

Contributions are what makes the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

## Setting up a development environment

To get started with the project on your machine, you need to install the following tools:
1. Go 1.22+. 
2. Make. 
3. Docker. 
4. Kubernets cluster (local/remote)

Once required tools are installed, clone or fork this repository. `git clone https://github.com/alustan/alustan.git`.

`Setup your github workflow secrets`: will be required to push your controller image and helm chart to your OCI registry

`make help`: for relevant make commands


### Testing basic functionalities

To quickly test functionality locally

Use this manifest that references a basic setup

**Terraform controller**

```yaml

apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: dummy-example
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
    imageName: alustan/example #build your own image from this repo alustan/basic-example since the 
                               # controller will require access to your registry to get tags that match   semantic constraint. Add registry secret to helm values files as specified in Readme before installing the helm chart in a k8s cluster
    semanticVersion: ">=0.2.0"

```

**Service controller**

```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
metadata:
  name: api-service
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
      image: alustan/basic-web:0.2.0
      config:
        DUMMY_1: ${workspace.dummy_output_1}
        DUMMY_2: ${workspace.dummy_output_2}

  containerRegistry:
    provider: docker
    imageName: alustan/basic-web
    semanticVersion: ">=0.1.0"

```

## Pull Request

This repository requires a [Developer Certificate of Origin (DCO)](https://developercertificate.org/) signature. 
When preparing to send in a pull request, please make sure your commit is signed. You can achieve this by doing a `git commit -s -m 'This is my commit message'` .

