BUILD_SETTINGS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
IMAGE = pod-autoscaler
IMAGE_VERSION = $(shell git tag --points-at HEAD | sed '/$(IMAGE)\/.*/!s/.*//' | sed 's/\//:/')
REPO = systemautoscaler

.PHONY: all build coverage clean fmt release test vet

all: build test coverage clean

build: fmt vet test
	$(BUILD_SETTINGS) go build -trimpath -o "$(IMAGE)" ./main.go

fmt:
	@go fmt ./...

test:
	@go test -race $(go list ./... | grep -v e2e) --coverprofile=coverage.out

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

vet:
	@go vet ./...

clean:
	@rm -rf ./$(IMAGE)
	@go clean -cache
	@rm -rf *.out