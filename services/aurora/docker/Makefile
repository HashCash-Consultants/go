SUDO := $(shell docker version >/dev/null 2>&1 || echo "sudo")

# https://github.com/opencontainers/image-spec/blob/master/annotations.md
BUILD_DATE := $(shell date -u +%FT%TZ)

TAG ?= hcnet/hcnet-aurora:$(VERSION)

docker-build:
ifndef VERSION
	$(error VERSION environment variable must be set. For example VERSION=2.4.1-101 )
endif
ifndef HCNET_CORE_VERSION
	$(SUDO) docker build --pull $(DOCKER_OPTS) \
	--label org.opencontainers.image.created="$(BUILD_DATE)" \
	--build-arg VERSION=$(VERSION) --build-arg ALLOW_CORE_UNSTABLE=$(ALLOW_CORE_UNSTABLE) \
	-t $(TAG) .
else
	$(SUDO) docker build --pull $(DOCKER_OPTS) \
	--label org.opencontainers.image.created="$(BUILD_DATE)" \
	--build-arg VERSION=$(VERSION) --build-arg HCNET_CORE_VERSION=$(HCNET_CORE_VERSION) \
        --build-arg ALLOW_CORE_UNSTABLE=$(ALLOW_CORE_UNSTABLE) \
	-t $(TAG) .
endif

docker-push:
ifndef TAG
	$(error Must set VERSION or TAG environment variable. For example VERSION=2.4.1-101 )
endif
	$(SUDO) docker push $(TAG)
