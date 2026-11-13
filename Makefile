SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

APP_DIR     := app
TF_DIR      := terraform
ENVS        := staging prod
SERVICE     := items-api
VERSION     ?= dev
AWS_REGION  ?= us-east-1
CHECKOV     ?= checkov
GOLANGCI    ?= golangci-lint

.PHONY: help check app-test app-lint app-image render-test \
        tf-fmt tf-validate tf-lint tf-scan bootstrap-image smoke

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*## "} {printf "  %-18s %s\n", $$1, $$2}'

check: app-test app-lint render-test tf-fmt tf-validate tf-lint tf-scan ## Run every local quality gate

app-test: ## Go tests with -race and the 80% coverage gate
	cd $(APP_DIR) && ./scripts/coverage-gate.sh

app-lint: ## go vet and golangci-lint
	cd $(APP_DIR) && go vet ./... && $(GOLANGCI) run ./...

app-image: ## Build the container image locally
	docker build -t $(SERVICE):$(VERSION) --build-arg SERVICE_VERSION=$(VERSION) $(APP_DIR)

render-test: ## Offline tests for the release renderer
	deploy/test/render_test.sh

tf-fmt: ## Check Terraform formatting
	terraform -chdir=$(TF_DIR) fmt -check -recursive

tf-validate: ## Init without a backend and validate each environment
	@for env in $(ENVS); do \
	  echo "== validate $$env"; \
	  terraform -chdir=$(TF_DIR)/envs/$$env init -backend=false -input=false >/dev/null && \
	  terraform -chdir=$(TF_DIR)/envs/$$env validate; \
	done

tf-lint: ## tflint with the AWS ruleset, per environment (recurses into modules)
	@for env in $(ENVS); do \
	  echo "== tflint $$env"; \
	  (cd $(TF_DIR)/envs/$$env && tflint --init >/dev/null && tflint --config="$(CURDIR)/.tflint.hcl" --recursive); \
	done

tf-scan: ## Checkov hard-fail scan of all Terraform
	$(CHECKOV) -d $(TF_DIR) --framework terraform --config-file .checkov.yaml

bootstrap-image: ## Push the one-time bootstrap image (needs ECR_REPOSITORY_URL and AWS credentials)
	@test -n "$(ECR_REPOSITORY_URL)" || { echo "set ECR_REPOSITORY_URL=<account>.dkr.ecr.$(AWS_REGION).amazonaws.com/$(SERVICE)"; exit 1; }
	aws ecr get-login-password --region $(AWS_REGION) | \
	  docker login --username AWS --password-stdin $(word 1,$(subst /, ,$(ECR_REPOSITORY_URL)))
	docker build -t $(ECR_REPOSITORY_URL):bootstrap --build-arg SERVICE_VERSION=bootstrap $(APP_DIR)
	docker push $(ECR_REPOSITORY_URL):bootstrap

smoke: ## Smoke test a deployed environment: make smoke BASE_URL=https://...
	@test -n "$(BASE_URL)" || { echo "set BASE_URL"; exit 1; }
	deploy/smoke-test.sh "$(BASE_URL)"
