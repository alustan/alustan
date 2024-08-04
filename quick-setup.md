## *Quick setup*

- fork and clone `https://github.com/alustan/alustan.git`

- Enable `github codespace` either in the browser or locally in `vscode`

- Copy `.env.example` to `.env` and provide necessary `env` variables

- RUN `./run-controller.sh` 

> **Feel free to inspect the script. It basically automates the setup process**

- kubectl apply -f examples/infra/basic.yaml

> kubectl logs < terraform-controller-pod > -n alustan

- kubectl apply -f examples/app/basic.yaml

> kubectl logs < app-controller-pod > -n alustan