MAKEFLAGS += --no-print-directory
COMPONENTS = pod-replicas-updater pod-autoscaler podscale-controller

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all build coverage clean fmt install install-crds install-rbac manifests release test vet

all: build test coverage manifests release clean

# Build binary
build: fmt manifests test vet
	$(call action, build)

coverage:
	$(call action, coverage)

clean:
	$(call action, clean)

fmt:
	$(call action, fmt)

install: install-crds install-rbac

install-crds:
	@kubectl apply -f config/crd/bases

install-rbac:
	@kubectl apply -f config/permissions

release:
	$(call action, release)

test:
	$(call action, test)

# Run go vet against code
vet:
	$(call action, vet)

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
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
    done
endef
