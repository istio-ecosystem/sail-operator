ALAUDA_PLATFORM_ARCHITECTURES := amd64 arm64
ALAUDA_BUILD_TARGETS := $(addprefix alauda-build-linux-,$(ALAUDA_PLATFORM_ARCHITECTURES))

.PHONY: alauda-build-all
alauda-build-all: $(ALAUDA_BUILD_TARGETS)

alauda-build-linux-%:
	@echo "Building for architecture: $*"
	GOOS=linux GOARCH=$* CGO_ENABLED=$(CGO_ENABLED) LDFLAGS="$(LD_FLAGS)" \
		common/scripts/gobuild.sh $(REPO_ROOT)/out/linux_$*/sail-operator cmd/main.go

.PHONY: alauda-docker-buildx
alauda-docker-buildx: alauda-build-all
	docker buildx build $(BUILDX_OUTPUT) --platform=$(PLATFORMS) -f Dockerfile.alauda --tag ${IMAGE} \
	$(BUILDX_ADDITIONAL_TAGS) .
