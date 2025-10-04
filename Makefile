BIN_DIR=bin
APP=site-audit
SRC=./cmd/main.go
COMPOSE_SERVICE_NAME ?= site-audit

define print-target
	@echo
	@printf "\033[31m*\033[0m Executing target: \033[31m$@\033[0m\n"
endef

.DEFAULT_GOAL := all

.PHONY: all
all: audit tidy test

.PHONY: help
help: ## List targets
	@printf "\033[31m[ Targets ]\033[0m\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[31m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: clean
clean: ## Clean the artifacts of build and test
	$(call print-target)
	rm -rf testresults
	rm -rf out

.PHONY: audit
audit: ## Audit for vulnerabilities
	$(call print-target)
	go mod tidy -diff
	go mod verify
	go vet ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: tidy
tidy: ## Tidy dependencies and format
	$(call print-target)
	go mod tidy -v
	go fmt ./...

.PHONY: quick-test
quick-test: ## Run the default go test
	$(call print-target)
	go test -race ./...

.PHONY: test
test: ## Run tests
	$(call print-target)
	mkdir -p testresults
	go tool gotestsum --junitfile testresults/unit-tests.xml -- -race -covermode=atomic -coverprofile=testresults/cover.out -v ./...
	go tool cover -html=testresults/cover.out -o testresults/coverage.html

.PHONY: docker-build
docker-build: ## Build the docker image for the application
	$(call print-target)
	docker build --build-arg APP=${APP} -t ${APP} .

.PHONY: docker-run
docker-run: ## Run the docker image for the application
	$(call print-target)
	docker run -t ${APP}

.PHONY: compose-build
compose-build: ## Build the application image using docker-compose
	$(call print-target)
	docker-compose build ${COMPOSE_SERVICE_NAME}

.PHONY: compose-run
compose-run: ## Run the application as a one-off task using docker-compose
	$(call print-target)
	docker-compose run --rm ${COMPOSE_SERVICE_NAME}

.PHONY: compose-down
compose-down: ## Stop and remove containers, networks created by docker-compose
	$(call print-target)
	docker-compose down

.PHONY: build
build: ## Build the application Go binary
	$(call print-target)
	go build -o ${BIN_DIR}/${APP} ${SRC}

.PHONY: run
run: ## Run the application
	$(call print-target)
	go run cmd/main.go -local=true