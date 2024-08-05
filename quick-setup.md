## *Quick setup*

- fork and clone `https://github.com/alustan/alustan.git`

- Enable and load `github codespace` either in the browser or locally in `vscode`

> This will use `.devcontainer` configuration

- Copy `.env.example` to `.env` and provide necessary `env` variables

- RUN `./run-controller.sh` 

> **Feel free to inspect the script. It basically automates the setup process**

- kubectl apply -f examples/infra/basic.yaml

> kubectl logs < terraform-controller-pod > -n alustan

- kubectl apply -f examples/app/basic.yaml

> kubectl logs < app-controller-pod > -n alustan

- View running application in the browser

> http://localhost:3000

**For preview applications**

- kubectl apply -f examples/infra/preview.yaml

> This repository `https://github.com/alustan/web-app-demo` already has an **open pullrequest** for testing purpose

*Retrieve the previewURls*

> kubectl get app < web-service > -n default -o json | jq '.status.previewURLs'

*Add `host` to your `etc file` to be able to access the preview application locally*

> `sudo nano /etc/hosts`

> Add entry `127.0.0.1    <branch-pr>-preview.localhost`

> `ctrl x` and `Enter` to save and exit

*Open deployed application in the browser*

- View deployed application in argocd ui

> `kubectl port-forward svc/argo-cd-argocd-server -n argocd 8080:443`

> `kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 --decode`

