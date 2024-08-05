# Contributing guide

Contributions are what makes the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

- fork and clone `https://github.com/alustan/alustan`

- create an issue

- develop 

- make a pull request

## Requirement

> **if using [codespace setup](./quick-setup.md) , these tools are already installed**

To get started with the project , you need to install the following tools:
1. Go 1.22+. 
2. Make. 
3. Docker. 
4. Kubernets cluster (local/remote)
5. kubectl

- Setup relevant github action workflow secrets `DOCKERHUB_USERNAME` `DOCKERHUB_TOKEN` `RELEASE_MAIN`

> `RELEASE_MAIN` should have **administrative** `read` and `write` permission

> This will be required to build your own `controller image` and `helm chart`

- Run `make help`: for relevant make commands

## Develop

- [quick setup on github codespace](./quick-setup.md) 

- `make build-infra` to ensure you can succesfully build terraform binary locally

- `make build-app` to ensure you can successfully build app binary locally

- Build and push image to your own registry using the provided **github workflow**

- Ensure `DEVELOP="true"` in **.env** file before running `./run-controller.sh`

> `kubectl logs <terraform-controller-pod> -n alustan`

> `kubectl logs <app-controller-pod> -n alustan`

- Ensure controller components are up and running


- **For preview environment setup refer to `README` documentation**

*Retrieve the previewURls*

> kubectl get app web-service -n default -o json | jq '.status.previewURLs'

*Add `host` to your `etc file` to be able to access the preview application locally*

> `sudo nano /etc/hosts`

> Add entry `127.0.0.1    <branch-pr>-preview.localhost`

> `ctrl x` and `Enter` to save and exit

*Open deployed application in the browser*

## Pull Request

This repository requires a [Developer Certificate of Origin (DCO)](https://developercertificate.org/) signature. 
When preparing to send in a pull request, please make sure your commit is signed. You can achieve this by doing a `git commit -s -m 'This is my commit message'` .

