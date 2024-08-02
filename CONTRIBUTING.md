# Contributing guide

Contributions are what makes the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

- fork and clone repo

- create an issue

- develop in a `feat-` branch

- make a pull request

## Requirement

To get started with the project , you need to install the following tools:
1. Go 1.22+. 
2. Make. 
3. Docker. 
4. Kubernets cluster (local/remote)
5. kubectl

## Relevant repositories to fork and clone

- `https://github.com/alustan/alustan`

- `https://github.com/alustan/basic-example`

- `https://github.com/alustan/web-app-demo`


> Setup relevant github action workflow secrets `Docker username` `Docker token`

> Run `make help`: for relevant make commands

## Development

- `make build-infra` to ensure you can succesfully build terraform binary locally

- `make build-app` to ensure you can successfully build app binary locally

- Build and push image to your own registry using the provided **github workflow**

- fetch and untar the helm chart 

```sh
helm fetch oci://registry-1.docker.io/<registry name>/alustan-helm --version <version> --untar=true
```

- generate your `containerRegistrySecret` and add to helm **values** file

**To obtain `containerRegistrySecret` to be supplied to the helm chart: RUN the script below and copy the encoded secret** 

```sh
rm ~/.docker/config.json
docker login -u <YOUR_DOCKERHUB_USERNAME> -p <YOUR_DOCKERHUB_PAT>
cat ~/.docker/config.json > secret.json
base64 -w 0 secret.json 

```

- `helm install controller alustan-helm --timeout 20m0s --debug --atomic`

- `kubectl logs <terraform-controller-pod> -n alustan`

- `kubectl logs <app-controller-pod> -n alustan`

> Ensure controller components are up and running


**Terraform controller**

- To quickly test functionality locally

- You can apply the manifest below and observe progress 

> replace **imageName** with your built image and relevant **semanticVersion**

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: dummy
spec:
  environment: staging
  variables:
    TF_VAR_workspace: "staging"
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

- should have installed `alustan-helm` into your cluster

- Apply `terraform manifest`

- If you prefer to run terraform locally `git clone https://github.com/alustan/basic-example.git`, check the `readme` for instructions 

- You can apply the manifest below and observe progress 

> replace **imageName** with your built image and relevant **semanticVersion**

```yaml
apiVersion: alustan.io/v1alpha1
kind: App
metadata:
  name: web-service
spec:
  environment: staging
  source:
    repoURL: https://github.com/alustan/cluster-manifests
    path: basic-demo
    releaseName: basic-demo
    targetRevision: main
    values:
      service: frontend
      image: 
        repository: alustan/web-app-demo
        tag: 1.0.0
      config:
        DUMMY_1: "{{.dummy_output_1}}"
        DUMMY_2: "{{.dummy_output_2}}"

  containerRegistry:
    provider: docker
    imageName: alustan/web-app-demo #build your own image from this repo alustan/web-app-demo since the 
                               # controller will require access to your registry to get tags that match   semantic constraint. Add registry secret to helm values files as specified in Readme before installing the helm chart in a k8s cluster
    semanticVersion: ">=1.0.0"

```

- For preview environment setup refer to `README` documentation

## Pull Request

This repository requires a [Developer Certificate of Origin (DCO)](https://developercertificate.org/) signature. 
When preparing to send in a pull request, please make sure your commit is signed. You can achieve this by doing a `git commit -s -m 'This is my commit message'` .

