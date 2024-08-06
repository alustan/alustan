## *Quick setup*

> **Setup and test functionality of this project in less than a minute on `codespace`**

- fork and clone `https://github.com/alustan/alustan.git`

- Enable and load `github codespace` either in the browser or locally in `vscode`

> This will use the repository `.devcontainer` configuration

- Copy `.env.example` to `.env` and provide necessary `env` variables

- RUN `./run-controller.sh` 

> **Feel free to inspect the script. It basically automates the setup process**

- kubectl apply -f examples/infra/basic.yaml

> kubectl get terraform dummy -n default -o json | jq '.status'

- kubectl apply -f examples/app/basic.yaml

> kubectl get app web-service -n default -o json | jq '.status'

**For preview applications**

- kubectl apply -f examples/app/preview.yaml

> This repository `https://github.com/alustan/web-app-demo` already has an **open pullrequest** for testing purpose

*Retrieve the previewURls*

> kubectl get app preview-service -n default -o json | jq '.status.previewURLs'

*To access codespace urls of forwarded ports*

> Open the Command Palette by pressing Ctrl+Shift+P (Windows/Linux) or Cmd+Shift+P (Mac).

> Type "Ports" and select `Ports: Focus on Ports View.`

- Access `argocd` `web-service` and `preview-service` UI in the browser

- To get argocd admin secret

> `kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 --decode`

