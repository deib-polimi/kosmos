MAKEFLAGS += --no-print-directory

COMPONENTS = contention-manager pod-replicas-updater pod-resource-updater recommender

.PHONY: all build test coverage clean

all: build test coverage clean

build:
	$(call action, build)

test:
	$(call action, test)

coverage:
	$(call action, coverage)

clean:
	$(call action, clean)

define action
	@for c in $(COMPONENTS); \
		do \
		$(MAKE) $(1) -C pkg/$$c; \
    done
endef
