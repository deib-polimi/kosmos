MAKEFLAGS += --no-print-directory
COMPONENTS = contention-manager pod-replicas-updater pod-resource-updater recommender podscale-controller

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all build coverage clean manifests test

all: build coverage clean manifests test

build:
	$(call action, build)

coverage:
	$(call action, coverage)

clean:
	$(call action, clean)

release:
	$(call action, release)

test:
	$(call action, test)

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
