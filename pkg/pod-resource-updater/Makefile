.PHONY: all build test coverage clean

all: build test coverage clean

build:
	go build -trimpath -o "pod-resource-updater" ./main.go

test:
	@go test -race ./... --coverprofile=coverage.out

coverage: test
	@go tool cover -func=coverage.out

clean:
	@rm -rf ./pod-resource-updater
	@go clean -cache
	@rm -rf *.out
