# *Alustan*

> **End to end continuous delivery orchestrator**

## Design 

- Leverages **argocd apiclient** abstracting its complexities into a unified solution

 **App-controller**

- The `App controller` extracts external resource metadata from **alustan cluster secret** annotations, therefore it is expected that when provisioning your infrastructure the metadata should be stored in annotations field of a secret with `label` **"alustan.io/secret-type": cluster** in namespace **alustan**

> should have a labelkey `environment` and a value which is same as that specified in the **environment field**

```yaml
apiVersion: alustan.io/v1alpha1
kind: App
spec:
  environment: staging
```
- Ability to deploy services and specify dependency pattern which will be respected when provisioning and destroying

- All dependent services should be deployed in same namespace

> 

```yaml
apiVersion: alustan.io/v1alpha1
kind: App
spec:
  dependencies:
    service: 
    - api-service

  ```

- The level of abstraction largely depends on the structure of your helm values file and your terraform variables file

>
```yaml
apiVersion: alustan.io/v1alpha1
kind: App
spec:
  source:
    repoURL: https://github.com/alustan/cluster-manifests
    path: application-helm
    releaseName: web-application
    targetRevision: main
    values:
      image:
        repository: alustan/web:1.1.0
      service: frontend}
      ...
```

- To reference deployed infrastructure variables in your application use `{{.NAME}}` **go template** this will be automatically populated for you. the `NAME` field should be same as that stored in **alustan cluster secret**

> 
```yaml
apiVersion: alustan.io/v1alpha1
kind: App
values:
  config:
    DB_URL: postgresql://{{.DB_USER}}:{{.DB_PASSWORD}}@postgres:5432/{{.DB_NAME}}
   
```

-  Peculiarities when `preview environment` is enabled

**Your Pullrequest label `tag` should be `preview`**

**For private git repo: provide `gitToken` in helm values file**

> If you wish to expose the application running on an ephemeral environment via `Ingress` the controller expects the Ingress field to be structured as specified below in your helm chart so as to dynamically update the host field with appropriate host url. updated url will look something like this `preview-{branch}-{pr-number}-chart-example.local`
> 
```yaml
apiVersion: alustan.io/v1alpha1
kind: App
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
kind: App
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

- Specify the **environment**, this will create or update argocd in-cluster secret label with the specified environment under the hood, wiil be used by `app-controller` to determine the cluster to deploy to`

> will first check if argocd cluster secret exists with specified label *may be manually created by user* before attempting to create one

> the **Terraform workload environment** should match the **App workload environment** 

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: staging
spec:
  environment: staging
```

- The variables should be prefixed with `TF_VAR_` since any `env` variable prefixed with `TF_VAR_` automatically overrides terraform defined variables

```yaml
variables:
  TF_VAR_workspace: staging
  TF_VAR_region: us-east-1
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

> An example implementation was a custom GO script [aws-resource](https://github.com/alustan/infrastructure/blob/main/postdeploy) (could be any scripting language) that reaches out to aws api and retrieves metadata and health status of provisioned cloud resources with a specific tag and subsequently stores the output in the custom resource `postDeployOutput` status field.

> The script expects two argument `workspace` and `region` and the values are supposed to be retrieved from env variables specified by users in this case `TF_VAR_workspace` and `TF_VAR_region`

```yaml
postDeploy:
  script: aws-resource
  args:
    workspace: TF_VAR_workspace
    region: TF_VAR_region

``` 
> **The output of your `postDeploy` script should have key: as `outputs`, body: `any arbitrary data structure`**

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
helm install my-alustan-helm oci://registry-1.docker.io/alustan/alustan-helm --version <version> --set containerRegistry.containerRegistrySecret=""
```
- Alternatively

```sh
helm fetch oci://registry-1.docker.io/alustan/alustan-helm --version <version> --untar=true
```
- Update helm **values** file with relevant `secrets`

- `helm install controller alustan-helm  --debug`


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

**For private git repository; Ensure to supply the `gitSSHSecret` in the controller helm values file**


## Workload specification

- **Terraform**

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: staging
spec:
  environment: staging
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

- **App**

```yaml
apiVersion: alustan.io/v1alpha1
kind: App
metadata:
  name: api-service
spec:
  environment: staging
  source:
    repoURL: https://github.com/alustan/cluster-manifests
    path: application-helm
    releaseName: backend-application
    targetRevision: main
    values:
      service: backend
      image: alustan/backend:0.2.0
      config:
        DB_URL: "postgresql://{{.DB_USER}}:{{.DB_PASSWORD}}@postgres:5432/{{.DB_NAME}}"

  containerRegistry:
    provider: docker
    imageName: alustan/backend
    semanticVersion: ">=0.2.0"

---
apiVersion: alustan.io/v1alpha1
kind: App
metadata:
  name: web-service
spec:
  environment: staging
  previewEnvironment:
    enabled: true
    gitOwner: alustan
    gitRepo: web-app-demo
    intervalSeconds: 600
  source:
    repoURL: https://github.com/alustan/cluster-manifests
    path: application-helm
    releaseName: web-application
    targetRevision: main
    values:
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




