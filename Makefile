# ansible-ui — build + run helpers (no docker-compose).
# On hosts where docker needs root, run e.g.:  sudo make up

DOCKER ?= docker
HELM   ?= helm
CHART   = deploy/helm/ansible-ui
RELEASE ?= ansible-ui
NAMESPACE ?= ansible-ui

.DEFAULT_GOAL := help
.PHONY: help build up down restart logs smoke run-all verify ws-check helm-lint helm-template helm-install helm-uninstall web-build vet

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Build the api (UI embedded) + runner images
	bash deploy/build.sh

up: build ## Build and run the stack (3 containers: postgres + runner + api)
	bash deploy/docker-run.sh up

down: ## Stop and remove the containers
	bash deploy/docker-run.sh down

restart: ## Recreate the containers
	bash deploy/docker-run.sh restart

logs: ## Tail api + runner logs
	$(DOCKER) logs -f ansible-ui-api &
	$(DOCKER) logs -f ansible-ui-runner

smoke: ## Run the demo-playbook smoke test
	bash deploy/smoke-test.sh

run-all: ## Run all 10 demo playbooks
	bash deploy/run-all.sh

verify: ## Verify auth + git repository + run-from-repo
	bash deploy/verify-repo-auth.sh

ws-check: ## Verify WebSocket upgrade
	bash deploy/ws-check.sh

helm-lint: ## Lint the Helm chart
	$(HELM) lint $(CHART)

helm-template: ## Render the Helm chart
	$(HELM) template $(RELEASE) $(CHART)

helm-install: ## Install/upgrade into Kubernetes
	$(HELM) upgrade --install $(RELEASE) $(CHART) -n $(NAMESPACE) --create-namespace

helm-uninstall: ## Uninstall the release
	$(HELM) uninstall $(RELEASE) -n $(NAMESPACE)

vet: ## go vet (linux target)
	GOOS=linux go vet ./...

web-build: ## Type-check and build the frontend
	cd web && npm run build
