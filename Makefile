MAKEFLAGS += --no-print-directory
COMPONENTS = pod-replicas-updater pod-autoscaler containerscale-controller

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all build cluster clean coverage controller-gen e2e fmt install install-crds install-rbac manifests release test vet

all: build test coverage manifests release clean

# Build binary
build: fmt manifests test vet
	$(call action, build)

coverage:
	$(call action, coverage)

clean:
	$(call action, clean)

cluster: kind
	@kind create cluster --config ./config/cluster-conf/development-cluster.yaml --image systemautoscaler/kindest-node:latest

fmt:
	$(call action, fmt)

install: install-crds install-rbac

install-crds: manifests
	@echo "install CRDs manifests"
	@kubectl apply -f config/crd/bases

install-rbac:
	@echo "install RBAC"
	@kubectl apply -f config/permissions

release:
	$(call action, release)

test:
	@echo "run local tests"
	$(call action, test)

e2e: install
	@echo "run e2e tests"
	@kubectl apply -f ./config/cluster-conf/e2e-namespace.yaml
	$(call action, e2e)

vet:
	$(call action, vet)

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	@echo "generate CRDs manifests"
	$(CONTROLLER_GEN) crd paths="./pkg/apis/systemautoscaler/..." output:crd:artifacts:config=config/crd/bases

controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

define action
	@for c in $(COMPONENTS); \
		do \
		$(MAKE) $(1) -C pkg/$$c; \
		if [ $$? -ne 0 ]; then \
			return 1; \
		fi \
    done
endef
