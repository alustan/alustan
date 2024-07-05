## Introduction

> **Infrastructure continuous delivery platform**

## Design Goal

- Simple workload specifications

- Intentionally unopionated, left the level of infra backend abstraction to infra team

- Free to design what constitutes your deploy script instead of simply terraform plan and apply

- Ability to write postDeploy script that can perform any requested action and store the output in custom resource status field

- Intentionally outsourced the packaging of the IAC OCI image to accomodate for different cloud services. [base image sample](./examples/Dockerfile) 

-  Scans your container registry every 6hrs and uses the latest image that satisfies the specified semantic tag constraint.

> The default `sync interval` can be changed in the helm values file

> If you are using a gitops delivery tool such as Argocd or fluxcd. It will continue to reconcile the custom resource manifests as always. However to avoid conflict with the controller `sync interval` the reconciliation is processed when drift in custom resources are noted or if argocd/fluxcd is trying to sync a new CR manifest

> **Note that the infrastructure drift detection and reconciliation is handled directly by the controller**

## setup

- install the helm chart into a kubernetes cluster

```sh
helm install my-alustan-helm oci://registry-1.docker.io/alustan/alustan-helm --version <version>
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

- **Workload specification**

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: staging-cluster
spec:
  variables:
    TF_VAR_provision_cluster: "true"
    TF_VAR_provision_db: "false"
    TF_VAR_vpc_cidr: "10.1.0.0/16"
  scripts:
    deploy: deploy
    destroy: destroy -c # omit if you dont wish to destroy infrastructure when resource is being finalized
  postDeploy:
    script: aws-resource
    args:
      workspace: TF_VAR_workspace
      region: TF_VAR_region
  containerRegistry:
    provider: docker
    imageName: alustan/infrastructure # image name to be pulled by the controller
    semanticVersion: "~1.0.0" # semantic constraint
 ###################################################################################   
status:
  state: "Progressing"
  message: "Starting processing"
  output: 
    aws_certificate_arn:   "aws_certificate_arn"
    service_account_role_arn:  "service_account_role_arn"
    db_instance_address: "db_instance_address"
  
  ingressURLs: 
    production: 
     - "https://example-production.com"
    
    development: 
     - "https://example-development.com"
     - "https://another-example-development.com"
    

  credentials: 
    argocdUsername: "admin"
    argocdPassword: "exampleArgoCDPassword"
    grafanaUsername: "admin"
    grafanaPassword: "exampleGrafanaPassword"
  

  postDeployOutput:
    outputs:
      EC2:
        - InstanceID: "i-1234567890abcdef0"
          InstanceType: "t2.micro"
          State: "running"
          Tags:
            - Key: "Name"
              Value: "MyInstance"
        - InstanceID: "i-0987654321abcdef0"
          InstanceType: "t3.medium"
          State: "stopped"
          Tags:
            - Key: "Name"
              Value: "YourInstance"
      RDS:
        - DBInstanceIdentifier: "my-db-instance"
          DBInstanceClass: "db.t3.medium"
          DBInstanceStatus: "available"
          Tags:
            - Key: "Environment"
              Value: "Production"
      LoadBalancer:
        - LoadBalancerName: "example-alb"
          DNSName: "example-alb-123456789.us-west-2.elb.amazonaws.com"
          Type: "application"
          Scheme: "internet-facing"
          State: "active"
          Tags:
            - Key: "Environment"
              Value: "Development"
```

- The variables should be prefixed with `TF_VAR_` since any `env` variable prefixed with `TF_VAR_` automatically overrides terraform defined variables

```yaml
variables:
  TF_VAR_provision_cluster: "true"
  TF_VAR_provision_db: "false"
  TF_VAR_vpc_cidr: "10.1.0.0/16"

```

- This should be the path to your `deploy` and `destroy` script; specifying just `deploy` or `destroy` assumes the script to be in the root level of your repository

> The `destroy` script should be `omitted` if when custom resource is being finalized (deleted from git repository) you don't wish to destroy your infrastructure

**Sample [deploy](https://github.com/alustan/infrastructure) and [destroy](https://github.com/alustan/infrastructure) script in GO**


```yaml
scripts:
  deploy: deploy
  destroy: destroy -c

```
- `postDeploy` is an additional flexibility tool given to Infra Engineers to write a custom script that will be run by the controller and `output` stored in status field.

> An example implementation was a custom GO script [aws-resource](https://github.com/alustan/infrastructure) (could be any scripting language) that reaches out to aws api and retrieves metadata and status of cloud resources with a specific tag and subsequently stores the output in the custom resource `postDeployOutput` status field.

> The script expects two argument `workspace` and `region` and the values are supposed to be retrieved from env variables specified by users in this case `TF_VAR_workspace` and `TF_VAR_region`

```yaml
postDeploy:
  script: aws-resource
  args:
    workspace: TF_VAR_workspace
    region: TF_VAR_region

``` 

> **The output of your `postDeploy` script should match `map[string]interface{}` with `outputs` key at top level**

```yaml
{
    "outputs": {
      "LoadBalancer": [
        {
            "LoadBalancerName": "example-alb",
            "DNSName": "example-alb-123456789.us-west-2.elb.amazonaws.com",
            "Type": "application",
            "Scheme": "internet-facing",
            "State": "active",
            "Tags": [
                {"Key": "Blueprint", "Value": "staging"}
            ]
        }
        
      ]
    
    }
}
 

```

> **Output in status field looks like this:** 

```yaml
postDeployOutput:
  outputs:
    EC2:
      - InstanceID: "i-1234567890abcdef0"
        InstanceType: "t2.micro"
        State: "running"
        Tags:
          - Key: "Name"
            Value: "MyInstance"
      - InstanceID: "i-0987654321abcdef0"
        InstanceType: "t3.medium"
        State: "stopped"
        Tags:
          - Key: "Name"
            Value: "YourInstance"
    RDS:
      - DBInstanceIdentifier: "my-db-instance"
        DBInstanceClass: "db.t3.medium"
        DBInstanceStatus: "available"
        Tags:
          - Key: "Environment"
            Value: "Production"
    LoadBalancer:
      - LoadBalancerName: "example-alb"
        DNSName: "example-alb-123456789.us-west-2.elb.amazonaws.com"
        Type: "application"
        Scheme: "internet-facing"
        State: "active"
        Tags:
          - Key: "Environment"
            Value: "Development"

```

- `status field` The Status field consists of the followings:

> **`state`: Current state - `Progressing` `error` `Success` `Completed`**

> **`message`: Detailed message regarding current state**

> **`output`: Terraform Output**

> **`ingressURLs`: Lists all urls associated with all of the ingress resource in the cluster**

> **`credentials`: Retrieves `username` and `passwords` associated with some cluster addons; In this case it just attempts to retrieve `argocd` and `grafana` login creds if found in the cluster**

> **`postDeployOutput`: Custom field to store output of your `postdeploy` script if specified**

**Check Out:**

 https://github.com/alustan/infrastructure for infrastructure backend reference implementation

**Alustan:** focuses on building tools and platforms that ensures right implementation of devops principles


