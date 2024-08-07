## *Quick setup*

> **Setup and test functionality of this project in less than a minute on `codespace`**

**1.** fork and clone `https://github.com/alustan/alustan.git`

**2.** Enable and load `github codespace` either in the browser or locally in `vscode`

> This will use the repository `.devcontainer` configuration

**3.** Copy `.env.example` to `.env` and provide necessary `env` variables

**4.** RUN `./run-controller.sh` 

> **Feel free to inspect the script. It basically automates the setup process**

**5.** `kubectl apply -f examples/infra/basic.yaml`

> `kubectl get terraform dummy -n default -o json | jq '.status'`

**6.** `kubectl apply -f examples/app/basic.yaml`

> `kubectl get app web-service -n default -o json | jq '.status'`

**For preview applications**

**7.** `kubectl apply -f examples/app/preview.yaml`

> `kubectl get app preview-service -n default -o json | jq '.status'`

> This repository `https://github.com/alustan/web-app-demo` already has an **open pullrequest** for testing purpose

*Retrieve the previewURls*

> `kubectl get app preview-service -n default -o json | jq '.status.previewURLs'`

**To check dependent service functionality**

**8.** `kubectl apply -f examples/app/basic-dependent.yaml`

> **When application is up and running try deleting web-service that it depends on**

> `kubectl delete -f examples/app/basic.yaml`

> `kubectl get app web-service -n default -o json | jq '.status'`

**Visibility/UI**

**9.** Access `argocd` `web-service` and `preview-service` UI in the browser

> `kubectl port-forward svc/argo-cd-argocd-server -n argocd 8080:443`

> `kubectl port-forward svc/web-service -n default 3000:80`

> `kubectl port-forward svc/preview-service -n preview-feat-8  8000:80`


**10.** To get argocd admin secret

> `kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 --decode`

