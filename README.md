## Introduction

> **Kubernetes native platform orchestrator**

## Design Goal

- Simple workload specifications

- Extensible via plugins

- Current Implementations works with `AWS`: can be extended via `plugins` to work with other Hyperscalers

- Sync interval; for `terraform-controller: 6hrs`  while `app-controller: 10mins`

> `terraform-controller` syncs directly with your terraform codebase every 6hrs  

> `app-controller` scans your container registry every 10mins and uses the latest image that satifies the specified semantic tag constraint. 

> Argocd continously updates the crd manifests as always

## setup

- To obtain `containerRegistrySecret` to be supplied to the helm chart: RUN the script below and copy the encoded secret `For docker registry`

> Needed by `terraform-controller` to be able to push and pull image from registry

```sh
docker login -u alustan -p dckr_pat_**********************
cat ~/.docker/config.json > secret.json

base64 -w 0 secret.json
```

## Usage

- install the helm chart into a kubernetes cluster

```sh
helm install my-alustan-helm oci://registry-1.docker.io/alustan/alustan-helm --version <version>
```

> **Requires argocd pre-installed in the cluster: needed for `app-controller`**

- **Define your manifest**

##### infrastructure workload specification

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: staging-cluster
  namespace: staging
  label:
   workspace: staging-cluster
   region: us-west-2
spec:
  provider: aws
  variables:
    TF_VAR_provision_cluster: "true"
    TF_VAR_provision_db: "false"
    TF_VAR_vpc_cidr: "10.1.0.0/16"
  scripts:
    deploy: deploy
    destroy: destroy -c
  gitRepo:
    url: https://github.com/alustan/infrastructure
    branch: main
  containerRegistry:
    imageName: docker.io/alustan/terraform-control # imagename to be built by the controller
 ###################################################################################   
status:
  state: "Progressing"
  message: "Starting processing"
  output: {
    "aws_certificate_arn":   "aws_certificate_arn",
    "service_account_role_arn":  "service_account_role_arn",
    "db_instance_address": "db_instance_address"
  }
  ingressURLs: {
    "production": [
      "https://example-production.com"
    ],
    "development": [
      "https://example-development.com",
      "https://another-example-development.com"
    ]
}
  credentials : {
    "argocdUsername": "admin",
    "argocdPassword": "exampleArgoCDPassword",
    "grafanaUsername": "admin",
    "grafanaPassword": "exampleGrafanaPassword"
  }

  cloudResources: {
    "resources": [
       {
            "Service": "EKS",
            "Resource": {
                "ClusterName": "example-cluster",
                "Status": "ACTIVE",
                "Tags": {
                    "Blueprint": "staging"
                }
            }
        },
      {
          "Service": "RDS",
          "Resource": {
              "DBInstanceIdentifier": "example-db",
              "DBInstanceClass": "db.t3.micro",
              "DBInstanceStatus": "available",
              "Tags": [
                  {"Key": "Blueprint", "Value": "staging"}
              ]
          }
      },
        {
          "Service": "ALB/NLB",
          "Resource": {
              "LoadBalancerName": "example-alb",
              "DNSName": "example-alb-123456789.us-west-2.elb.amazonaws.com",
              "Type": "application",
              "Scheme": "internet-facing",
              "State": "active",
              "Tags": [
                  {"Key": "Blueprint", "Value": "staging"}
              ]
          }
      }
  ]
}
```

##### application workload specification

```yaml
apiVersion: alustan.io/v1alpha1
kind: Application
metadata:
  name: web
spec:
  provider: aws
  cluster: staging-cluster
  environment: staging 
  port: 80 
  host: staging.example.com
  strategy: default # canary, preview
  git:
    owner: alustan
    repo: backstage-portal
    branch: main
  containerRegistry:
    provider: docker
    imageName: alustan/app-control # image name to be pulled by app-controller
    semanticVersion: ">=1.0.0" # semantic constraint
  config:
    url: postgresql://$DB_USER:$DB_PASSWORD@postgres:5432/$DB_NAME
    dburl: mysql://$DB_USER:$DB_PASSWORD@mysql:3306/$DB_NAME
 ###################################################################################  
status:
  healthStatus: Synced
  message: Healthy
```

**This is one of multiple projects that aims to setup a functional platform for seamless application delivery and deployment with less technical overhead**

**Check Out:**

1. [infrastructure](https://github.com/alustan/infrastructure) `Modular and extensible infrastructure setup`

2. [manifests](https://github.com/alustan/manifests) `Cluster manifests`

4. [backstage-portal](https://github.com/alustan/backstage-portal) `Backstage portal`
