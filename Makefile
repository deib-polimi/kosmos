GOFILES := $(wildcard *.go)
.PHONY: all build test clean

all: clean coverage build

build:
	(cd pkg/contention-manager; make build)
	(cd pkg/pod-replicas-updater; make build)
	(cd pkg/pod-resource-updater; make build)
	(cd pkg/recommender; make build)

test:
	(cd pkg/contention-manager; make test)
	(cd pkg/pod-replicas-updater; make test)
	(cd pkg/pod-resource-updater; make test)
	(cd pkg/recommender; make test)

# TODO: fix
# coverage: test
# 	go tool cover -func=coverage.out

clean:
	(cd pkg/contention-manager; rm -rf ./contention-manager; go clean -cache; rm -rf *.out)
	(cd pkg/pod-replicas-updater; rm -rf ./pod-replicas-updater; go clean -cache; rm -rf *.out)
	(cd pkg/pod-resource-updater; rm -rf ./pod-resource-updater; go clean -cache; rm -rf *.out)
	(cd pkg/recommender; rm -rf ./recommender; go clean -cache; rm -rf *.out)
