SUDO := $(shell docker version >/dev/null 2>&1 || echo "sudo")

ifndef VERSION_STRING
        $(error VERSION_STRING environment variable must be set. For example "2.26.0-d2d01d39759f2f315f4af59e4b95700a4def44eb")
endif

DOCKER_PLATFORM := $(shell docker system info --format '{{.OSType}}/{{.Architecture}}')

binary-build:
	$(SUDO) docker run --platform $(DOCKER_PLATFORM) --rm $(DOCKER_OPTS) -v $(shell pwd)/../../:/go/src/github.com/HashCash-Consultants/go \
		--pull always \
		--env CGO_ENABLED=0 \
		--env GOFLAGS="-ldflags=-X=github.com/HashCash-Consultants/go/support/app.version=$(VERSION_STRING)" \
		golang:1.22-bullseye \
		/bin/bash -c '\
			git config --global --add safe.directory /go/src/github.com/HashCash-Consultants/go && \
			cd /go/src/github.com/HashCash-Consultants/go && \
			go build -o hcnet-aurora -trimpath -v ./services/aurora'
