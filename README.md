# *Alustan*

> **End to end continuous delivery orchestrator**

## Design 

 **Service-controller**

- The `Service controller` extracts external resource metadata from **alustan cluster secret** annotations, therefore it is expected that when provisioning your infrastructure the metadata should be stored in annotations field of a secret with `label` **"alustan.io/secret-type": cluster** in namespace **alustan**

> should have a labelkey `environment` and a value which is same as that specified in the **workspace field**

```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
spec:
  workspace: staging
```
- Ability to deploy services and specify dependency pattern which will be respected when provisioning and destroying

- All dependent services should be deployed in same namespace

> 

```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
spec:
  dependencies:
    service: 
    - api-service

  ```

- The level of abstraction depends largely  on the structure of your helm values file and your terraform variables file

- Each service is deployed in the specified `cluster`. therefore `cluster Name` should be provided.

The `service-controller` expects the cluster be specified in your helm chart
**For example this extracts cluster name from a key `CLUSTER_NAME` stored in cluster secret**
>
```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
spec:
  source:
    values:
      cluster: ${workspace.CLUSTER_NAME}


```

- To reference deployed infrastructure variables in your application use `${workspace.NAME}` this will automatically be populated for you. the `NAME` field should be same as that stored in **cluster secret**

> 
```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
values:
  config:
    DB_URL: postgresql://${workspace.DB_USER}:${workspace.DB_PASSWORD}@postgres:5432/${workspace.DB_NAME}
   
```

-  Peculiarities when `preview environment` is enabled

**Your Pullrequest label tag should be `preview`**

> If you wish to expose the application running on an epheramal environment via `Ingress` the controller expects the Ingress field to be structured as specified below in your helm chart so as to dynamically update the host field with appropriate host url. updated url will look something like this `preview-{branch}-{pr-number}-chart-example.local`
> 
```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
spec:
  previewEnvironment:
    enabled: true
    gitOwner: alustan
    gitRepo: web-app-demo
  source:
    values:
      ingress:
        hosts:
          - host: chart-example.local
        tls: 
        - hosts:
            - chart-example.local
  ```

-  Scans your container registry every `5 mins`  and uses the latest image that satisfies the specified `semantic tag constraint`.

> The default `appSyncInterval` can be changed in the controller helm values file

```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
spec:
  containerRegistry:
    provider: docker
    imageName: alustan/backend
    semanticVersion: ">=0.2.0"

```
- `status field` The Status field consists of the followings:

> **`state`: Current state - `Progressing` `Error` `Failed` `Blocked` `Completed`**

> **`message`: Detailed message regarding current state**

> **`previewURLs`: Urls of running applications in the ephemeral environment**

> **`healthStatus`: This basically holds refrence to applicationset status**


**Terraform-controller**

- The variables should be prefixed with `TF_VAR_` since any `env` variable prefixed with `TF_VAR_` automatically overrides terraform defined variables

```yaml
variables:
  TF_VAR_provision_cluster: "true"
  TF_VAR_provision_db: "false"
  TF_VAR_vpc_cidr: "10.1.0.0/16"

```
- This should be the path to your `deploy` and `destroy` script; specifying just `deploy` or `destroy` assumes the script to be in the root level of your repository

> The `destroy` script should be `omitted` if when custom resource is being finalized (deleted from git repository) you don't wish to destroy your infrastructure

**Sample [deploy](https://github.com/alustan/infrastructure/blob/main/setup/cmd/deploy) and [destroy](https://github.com/alustan/infrastructure/blob/main/setup/cmd/destroy) script in GO**


```yaml
scripts:
  deploy: deploy
  destroy: destroy -c

```

- `postDeploy` is an additional flexibility tool that will enable end users write a custom script that will be run by the controller and `output` stored in status field.

> An example implementation was a custom GO script [aws-resource](https://github.com/alustan/infrastructure/blob/main/postdeploy) (could be any scripting language) that reaches out to aws api and retrieves metadata and health status of cloud resources with a specific tag and subsequently stores the output in the custom resource `postDeployOutput` status field.

> The script expects two argument `workspace` and `region` and the values are supposed to be retrieved from env variables specified by users in this case `TF_VAR_workspace` and `TF_VAR_region`

```yaml
postDeploy:
  script: aws-resource
  args:
    workspace: TF_VAR_workspace
    region: TF_VAR_region

``` 
> **The output of your `postDeploy` script should match `(map[string]interface{}` with `outputs` key at top level**

> **key: is a `string`, body: `any arbitrary data structure`**


```yaml
{
  "outputs": {
    "externalresources": [
      {
        "Service": "RDS",
        "Resource": {
          "DBInstanceIdentifier": "mydbinstance",
          "DBInstanceClass": "db.t2.micro",
          "DBInstanceStatus": "available",
          "Tags": [
            {
              "Key": "Blueprint",
              "Value": "staging"
            }
          ]
        }
      }
     ]
  }
}

```
 
- Intentionally outsourced the packaging of the `IAC OCI image `to be runtime and hyperscalers agnostic . [base image sample](./examples/Dockerfile) 

-  Scans your container registry every `6hrs`  and uses the latest image that satisfies the specified `semantic tag constraint`.

> The default `infraSyncInterval` can be changed in the controller helm values file

> 
```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
spec:
  containerRegistry:
    provider: docker
    imageName: alustan/infra
    semanticVersion: "~1.0.0"
```

- `status field` The Status field consists of the followings:

> **`state`: Current state - `Progressing` `Error` `Success` `Failed` `Completed`**

> **`message`: Detailed message regarding current state**

> **`postDeployOutput`: Custom field to store output of your `postdeploy` script if specified**


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

**If previewEnvironment is enabled for private git repository; Ensure to supply the `githubToken` in the controller helm values file**


## Workload specification

- **Terraform**

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: staging-cluster
spec:
  variables:
    TF_VAR_workspace: "staging"
    TF_VAR_region: "us-east-1"
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
 
```

- **Service**

```yaml
apiVersion: alustan.io/v1alpha1
kind: Service
metadata:
  name: api-service
spec:
  workspace: staging
  source:
    repoURL: https://github.com/alustan/cluster-manifests
    path: application-helm
    releaseName: backend-application
    targetRevision: main
    values:
      cluster: ${workspace.CLUSTER_NAME}
      service: backend
      image: alustan/backend:0.2.0
      config:
        DB_URL: postgresql://${workspace.DB_USER}:${workspace.DB_PASSWORD}@postgres:5432/${workspace.DB_NAME}

  containerRegistry:
    provider: docker
    imageName: alustan/backend
    semanticVersion: ">=0.2.0"

---
apiVersion: alustan.io/v1alpha1
kind: Service
metadata:
  name: web-service
spec:
  workspace: staging
  previewEnvironment:
    enabled: true
    gitOwner: alustan
    gitRepo: web-app-demo
  source:
    repoURL: https://github.com/alustan/cluster-manifests
    path: application-helm
    releaseName: web-application
    targetRevision: main
    values:
      cluster: ${workspace.CLUSTER_NAME}
      image:
        repository: alustan/web:1.1.0
      service: frontend
      ingress:
        hosts:
         - host: chart-example.local
        tls: 
        - hosts:
            - chart-example.local
      
  containerRegistry:
    provider: docker
    imageName: alustan/web
    semanticVersion: "~1.1.0"
  dependencies:
    service: 
    - api-service


```

**Check Out:**

- https://github.com/alustan/infrastructure for infrastructure backend reference implementation

- https://github.com/alustan/cluster-manifests/blob/main/application-helm for reference implementation of application helm chart

*Basic reference setup for local testing*

- https://github.com/alustan/basic-example for dummy backend implementation

- https://github.com/alustan/cluster-manifests/blob/main/basic-demo for dummy implementation of application helm chart




