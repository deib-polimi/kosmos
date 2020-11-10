.PHONY: all build test coverage clean

all: build test coverage clean

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "pod-autoscaler" ./main.go

test:
	@go test -race ./... --coverprofile=coverage.out

coverage: test
	@go tool cover -func=coverage.out

clean:
	@rm -rf ./pod-resource-updater
	@go clean -cache
	@rm -rf *.out
