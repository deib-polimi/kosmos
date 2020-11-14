BUILD_SETTINGS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
IMAGE = podscale-controller
IMAGE_VERSION = $(shell git tag --points-at HEAD | sed '/$(IMAGE)\/.*/!s/.*//' | sed 's/\//:/')
REPO = lterrac

.PHONY: all build test coverage clean release

all: build test coverage image clean

build:
	$(BUILD_SETTINGS) go build  -trimpath -o "$(IMAGE)" ./main.go

test:
	@go test -race ./... --coverprofile=coverage.out

coverage: test
	@go tool cover -func=coverage.out


release:
	@if [ -n "$$IMAGE_VERSION" ]; then \
		echo $(IMAGE_VERSION) ;\
		echo $(REPO) ;\
		docker build -t $(REPO)/$(IMAGE_VERSION) . ;\
		docker push $(REPO)/$(IMAGE_VERSION) . ;\
	else \
		echo "$(IMAGE) unchanged: no version tag on HEAD commit" ;\
	fi

clean:
	@rm -rf ./$(IMAGE)
	@go clean -cache
	@rm -rf *.out
