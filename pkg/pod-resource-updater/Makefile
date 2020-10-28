.PHONY: all build test clean

all: clean coverage build

build: $(BUILD_FILES)
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "pod-resource-updater" ./main.go

test:
	go test -race ./... --coverprofile=coverage.out

coverage: test
	go tool cover -func=coverage.out

clean:
	rm -rf ./pod-resource-updater
	go clean -cache
	rm -rf *.out
