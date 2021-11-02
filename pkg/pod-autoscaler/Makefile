BUILD_SETTINGS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
IMAGE = pod-autoscaler
IMAGE_VERSION = $(shell git tag --points-at HEAD | sed '/$(IMAGE)\/.*/!s/.*//' | sed 's/\//:/')
REPO = systemautoscaler

.PHONY: all build coverage clean e2e fmt release test vet

all: build test coverage clean

build: fmt vet test
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
