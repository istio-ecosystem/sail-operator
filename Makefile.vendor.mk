VERSION = 2.1.0

OPERATOR_NAME = servicemesh-operator2
HUB = build-harbor.alauda.cn/asm
IMAGE_BASE = servicemesh-operator2
CHANNELS = "stable,stable-2.1"
DEFAULT_CHANNEL = stable
HELM_VALUES_FILE = alauda/values.yaml
VERSIONS_YAML_FILE ?= alauda-versions.yaml
# NB(timonwong) Should only generated in release pipeline
GENERATE_RELATED_IMAGES = false

PLATFORMS ?= linux/arm64,linux/amd64

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
