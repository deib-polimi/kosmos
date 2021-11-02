BUILD_SETTINGS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
IMAGE = metrics-exposer
IMAGE_VERSION = $(shell git tag --points-at HEAD | sed '/$(IMAGE)\/.*/!s/.*//' | sed 's/\//:/')
REPO = systemautoscaler
TEMP_DIR:=$(shell mktemp -d)
ARCH?=amd64
OUT_DIR?=./_output

OPENAPI_PATH=../../vendor/k8s.io/kube-openapi

VERSION?=latest

.PHONY: all build coverage clean e2e fmt release test vet

all: build test coverage clean

build: generated/openapi/zz_generated.openapi.go fmt vet test
	@sed -i.bak 's|REGISTRY|'${REPO}'|g' configure.yaml
	@rm configure.yaml.bak
	$(BUILD_SETTINGS) go build -trimpath -o "$(IMAGE)" ./main.go

fmt:
	@go fmt ./...

test:
	@go test -race $(shell go list ./... | grep -v e2e) --coverprofile=coverage.out

e2e:
	@go test -race $(shell go list ./... | grep e2e)

coverage: test
	@go tool cover -func=coverage.out

release:
	@if [ -n "$(IMAGE_VERSION)" ]; then \
		echo "Building $(IMAGE_VERSION)" ;\
		docker build -t $(REPO)/$(IMAGE_VERSION) . ;\
		docker push $(REPO)/$(IMAGE_VERSION) ;\
	else \
		echo "$(IMAGE) unchanged: no version tag on HEAD commit" ;\
	fi

dev: build
	@echo "Building dev release"
	@docker build -t $(REPO)/$(IMAGE):dev .
	@docker push $(REPO)/$(IMAGE):dev 
	@echo "Dev release built and pushed"

vet:
	@go vet ./...

clean:
	@rm -rf ./$(IMAGE)
	@go clean -cache
	@rm -rf *.out

generated/openapi/zz_generated.openapi.go:
	@go run $(OPENAPI_PATH)/cmd/openapi-gen/openapi-gen.go --logtostderr \
	    -i k8s.io/metrics/pkg/apis/custom_metrics,k8s.io/metrics/pkg/apis/custom_metrics/v1beta1,k8s.io/metrics/pkg/apis/custom_metrics/v1beta2,k8s.io/metrics/pkg/apis/external_metrics,k8s.io/metrics/pkg/apis/external_metrics/v1beta1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version,k8s.io/api/core/v1 \
	    -h ../../hack/boilerplate.go.txt \
	    -p ./pkg/generated/openapi \
	    -O zz_generated.openapi \
	    -o ./ \
	    -r /dev/null
