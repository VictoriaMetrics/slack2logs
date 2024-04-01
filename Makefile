include deployment/Makefile

app-local:
	CGO_ENABLED=1 GO111MODULE=on go build $(RACE) -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)$(RACE) ./

app-local-with-goarch:
	GO111MODULE=on go build $(RACE) -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)-$(GOARCH)$(RACE) ./

slack2logs-amd64-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker-amd64

slack2logs-arm-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker-arm

slack2logs-arm64-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker-arm64

slack2logs-ppc64le-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker-ppc64le

slack2logs-386-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker-386

slack2logs-darwin-amd64-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker-darwin-amd64

slack2logs-darwin-arm64-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker-darwin-arm64

package-slack2logs:
	APP_NAME=slack2logs $(MAKE) package-via-docker

package-slack2logs-amd64:
	APP_NAME=slack2logs $(MAKE) package-via-docker-amd64

package-slack2logs-arm:
	APP_NAME=slack2logs $(MAKE) package-via-docker-arm

package-slack2logs-arm64:
	APP_NAME=slack2logs $(MAKE) package-via-docker-arm64

package-slack2logs-ppc64le:
	APP_NAME=slack2logs $(MAKE) package-via-docker-ppc64le

package-slack2logs-386:
	APP_NAME=slack2logs $(MAKE) package-via-docker-386

publish-slack2logs:
	APP_NAME=slack2logs $(MAKE) publish-via-docker

slack2logs:
	APP_NAME=slack2logs $(MAKE) app-local

slack2logs-race:
	APP_NAME=slack2logs RACE=-race $(MAKE) app-local

slack2logs-prod:
	APP_NAME=slack2logs $(MAKE) app-via-docker
