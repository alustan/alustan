# Contributing guide

Contributions are what makes the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

## Setting up a development environment

To get started with the project on your machine, you need to install the following tools:
1. Go 1.22+. 
2. Make. 
3. Docker. 
4. Kubernets cluster (local/remote)

Once required tools are installed, clone this repository. `git clone https://github.com/alustan/alustan.git`.

Setup your github workflow secrets

`make help`: for relevant make commands


### Testing basic functionalities

To quickly test functionality locally

Use this manifest that references a basic setup

```yaml

apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: dummy-example
spec:
  variables:
    TF_VAR_workspace: "dev"
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
    imageName: alustan/example #build your own image, from this repo alustan/basic-example since the 
                               # controller will require access to your registry to get tags that match semantic constraint 
    semanticVersion: ">=0.2.0"

```

## Pull Request

This repository requires a [Developer Certificate of Origin (DCO)](https://developercertificate.org/) signature. 
When preparing to send in a pull request, please make sure your commit is signed. You can achieve this by doing a `git commit -s -m 'This is my commit message'` .

