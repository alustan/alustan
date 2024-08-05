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

- View deployed application in argocd ui

> `kubectl port-forward svc/argo-cd-argocd-server -n argocd 8080:443`

> `kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 --decode`

- View running application in the browser

> http://localhost:3000