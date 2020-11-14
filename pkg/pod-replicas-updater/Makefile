.PHONY: all build test coverage clean release
BUILD_SETTINGS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64

all: build test coverage clean

build:
	$(BUILD_SETTINGS) go build -trimpath -o "pod-replicas-updater" ./main.go

test:
	@go test -race ./... --coverprofile=coverage.out

coverage: test
	@go tool cover -func=coverage.out

release:

clean:
	@rm -rf ./pod-replicas-updater
	@go clean -cache
	@rm -rf *.out
