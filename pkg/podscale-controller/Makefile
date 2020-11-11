.PHONY: all build test coverage clean

BUILD_SETTINGS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64

all: build test coverage clean

build:
	$(BUILD_SETTINGS) go build  -trimpath -o "podscale-controller" ./main.go

test:
	@go test -race ./... --coverprofile=coverage.out

coverage: test
	@go tool cover -func=coverage.out

clean:
	@rm -rf ./podscale-controller
	@go clean -cache
	@rm -rf *.out
