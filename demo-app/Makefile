REGISTRY ?= ghcr.io/maxanderson95
IMAGE_NAME ?= demo-app
TAG ?= latest

.PHONY: build docker-build docker-push helm-install helm-upgrade lint

build:
	go build -o bin/demo-app ./cmd/server

docker-build:
	docker build -t $(REGISTRY)/$(IMAGE_NAME):$(TAG) .

docker-push:
	docker push $(REGISTRY)/$(IMAGE_NAME):$(TAG)

helm-install:
	helm install demo-app ./chart

helm-upgrade:
	helm upgrade demo-app ./chart

lint:
	golangci-lint run ./...

helm-lint:
	helm lint ./chart

helm-template:
	helm template demo-app ./chart

test:
	go test ./...