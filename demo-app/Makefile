REGISTRY ?= ghcr.io/maxanderson95/k8s-autoscaling
IMAGE_NAME ?= demo-app
TAG ?= latest

.PHONY: build docker-build docker-push helm-install helm-upgrade lint helm-lint helm-template test chart-release

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

chart-release: docker-push
	mkdir -p /tmp/chart-release && \
	cp -r chart /tmp/chart-release/chart && \
	yq -i ".version = \"$(TAG)\"" /tmp/chart-release/chart/Chart.yaml && \
	yq -i ".appVersion = \"$(TAG)\"" /tmp/chart-release/chart/Chart.yaml && \
	yq -i ".image.tag = \"$(TAG)\"" /tmp/chart-release/chart/values.yaml && \
	helm package /tmp/chart-release/chart --destination /tmp/chart-release/ && \
	rm -rf /tmp/chart-release/chart