# All these commands must run from repository root.
PKG_PREFIX := github.com/VictoriaMetrics/slack2logs

DOCKER_NAMESPACE?= victoriametrics/slack2logs

ROOT_IMAGE ?= alpine:3.18.4
CERTS_IMAGE := alpine:3.18.4
GO_BUILDER_IMAGE := golang:1.22.3-alpine
BUILDER_IMAGE := local/builder:2.0.0-$(shell echo $(GO_BUILDER_IMAGE) | tr :/ __)-1
BASE_IMAGE := local/base:1.1.4-$(shell echo $(ROOT_IMAGE) | tr :/ __)-$(shell echo $(CERTS_IMAGE) | tr :/ __)

DATEINFO_TAG ?= $(shell date -u +'%Y%m%d-%H%M%S')
BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | openssl sha1 | cut -d' ' -f2 | cut -c 1-8)))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

package-base:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(BASE_IMAGE)$$') \
		|| docker build \
			--build-arg root_image=$(ROOT_IMAGE) \
			--build-arg certs_image=$(CERTS_IMAGE) \
			--tag $(BASE_IMAGE) \
			deployment/docker/base

package-builder:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(BUILDER_IMAGE)$$') \
		|| docker build \
			--build-arg go_builder_image=$(GO_BUILDER_IMAGE) \
			--tag $(BUILDER_IMAGE) \
			deployment/docker/builder

app-via-docker: package-builder
	mkdir -p gocache-for-docker
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)",dst=/slack2logs \
		-w /slack2logs \
		--mount type=bind,src="$(shell pwd)/gocache-for-docker",dst=/gocache \
		--env GOCACHE=/gocache \
		--env GO111MODULE=on \
		$(DOCKER_OPTS) \
		$(BUILDER_IMAGE) \
		go build $(RACE) -mod=vendor -trimpath -buildvcs=false \
			-ldflags "-extldflags '-static' $(GO_BUILDINFO)" \
			-tags 'netgo osusergo nethttpomithttp2 musl' \
			-o bin/$(APP_NAME)$(APP_SUFFIX)-prod ./

package-via-docker: package-base
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE)$$') || (\
		$(MAKE) app-via-docker && \
		docker build \
			--build-arg src_binary=$(APP_NAME)$(APP_SUFFIX)-prod \
			--build-arg base_image=$(BASE_IMAGE) \
			--build-arg root_image=$(ROOT_IMAGE) \
			--build-arg certs_image=$(CERTS_IMAGE) \
			--tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) \
			-f deployment/Dockerfile bin )

run-via-docker: package-via-docker
	docker run -it --rm \
		--user $(shell id -u):$(shell id -g) \
		--net host \
		$(DOCKER_OPTS) \
		$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) $(ARGS)

app-via-docker-goarch:
	APP_SUFFIX='-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=linux --env GOARCH=$(GOARCH)' \
	$(MAKE) app-via-docker

publish-via-docker: \
	app-via-docker-linux-amd64
	docker buildx build \
		--platform=linux/amd64 \
		--build-arg certs_image=$(CERTS_IMAGE) \
		--build-arg root_image=$(ROOT_IMAGE) \
		--build-arg APP_NAME=$(APP_NAME) \
		--build-arg base_image=$(BASE_IMAGE) \
		--tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE) \
		--tag $(DOCKER_NAMESPACE)/$(APP_NAME):latest$(RACE) \
		-o type=image \
		-f deployment/Dockerfile \
		--push \
		bin

app-via-docker-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-goos-goarch:
	APP_SUFFIX='-$(GOOS)-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=$(GOOS) --env GOARCH=$(GOARCH)' \
	$(MAKE) app-via-docker

app-via-docker-goarch-arm64:
	APP_SUFFIX='-arm64' \
	DOCKER_OPTS='--env CGO_ENABLED=1 --env GOOS=linux --env GOARCH=arm64 --env CC=/opt/cross-builder/aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc' \
	$(MAKE) app-via-docker

app-via-docker-windows-goarch:
	APP_SUFFIX='-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=0 --env GOOS=windows --env GOARCH=$(GOARCH)' \
	$(MAKE) app-via-docker-windows

app-via-docker-goarch-cgo:
	CGO_ENABLED=1 $(MAKE) app-via-docker-goarch

app-via-docker-goarch-nocgo:
	CGO_ENABLED=0 $(MAKE) app-via-docker-goarch

app-via-docker-pure:
	APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker

app-via-docker-amd64:
	GOARCH=amd64 $(MAKE) app-via-docker-goarch-cgo

app-via-docker-arm64:
	GOARCH=arm64 $(MAKE) app-via-docker-goarch-nocgo

app-via-docker-arm:
	GOARCH=arm $(MAKE) app-via-docker-goarch-nocgo

app-via-docker-ppc64le:
	GOARCH=ppc64le $(MAKE) app-via-docker-goarch-nocgo

app-via-docker-386:
	GOARCH=386 $(MAKE) app-via-docker-goarch-nocgo

app-via-docker-windows-amd64:
	GOARCH=amd64 $(MAKE) app-via-docker-windows-goarch

package-via-docker-goarch:
	APP_SUFFIX='-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=linux --env GOARCH=$(GOARCH)' \
	$(MAKE) package-via-docker

package-via-docker-goarch-arm64:
	APP_SUFFIX='-arm64' \
	DOCKER_OPTS='--env CGO_ENABLED=1 --env GOOS=linux --env GOARCH=arm64 --env CC=/opt/cross-builder/aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc' \
	$(MAKE) package-via-docker

package-via-docker-goarch-cgo:
	CGO_ENABLED=1 $(MAKE) package-via-docker-goarch

package-via-docker-goarch-nocgo:
	CGO_ENABLED=0 $(MAKE) package-via-docker-goarch

package-via-docker-pure:
	APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) package-via-docker

package-via-docker-amd64:
	GOARCH=amd64 $(MAKE) package-via-docker-goarch-cgo

package-via-docker-arm:
	GOARCH=arm $(MAKE) package-via-docker-goarch-nocgo

package-via-docker-arm64:
	GOARCH=arm64 $(MAKE) package-via-docker-goarch-nocgo

package-via-docker-ppc64le:
	GOARCH=ppc64le $(MAKE) package-via-docker-goarch-nocgo

package-via-docker-386:
	GOARCH=386 $(MAKE) package-via-docker-goarch-nocgo
