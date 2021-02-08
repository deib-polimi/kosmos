BUILD_SETTINGS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
IMAGE = app
IMAGE_VERSION = $(shell git tag --points-at HEAD | sed '/$(IMAGE)\/.*/!s/.*//' | sed 's/\//:/')
REPO = systemautoscaler

.PHONY: all build test coverage clean release

all: build test coverage clean

build:
	$(BUILD_SETTINGS) go build -trimpath -o http-metrics ./main.go

test:
	@go test -race ./... --coverprofile=coverage.out

coverage: test
	@go tool cover -func=coverage.out

clean:
	@rm -rf ./$(IMAGE)
	@go clean -cache
	@rm -rf *.out
