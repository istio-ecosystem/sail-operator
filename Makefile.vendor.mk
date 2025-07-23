VERSION = 3.1.0
OPERATOR_NAME = servicemeshoperator3
CHANNELS = "stable,stable-3.1"
DEFAULT_CHANNEL=stable
HELM_VALUES_FILE = ossm/values.yaml
VERSIONS_YAML_FILE ?= versions.ossm.yaml
USE_IMAGE_DIGESTS = false
GENERATE_RELATED_IMAGES = false
IMAGE ?= $$\{OSSM_OPERATOR_3_1\}

.PHONY: build-fips
build-fips: ## Build sail-operator binary for FIPS mode.
	GOARCH="$(TARGET_ARCH)" CGO_ENABLED=1 LDFLAGS="$(LD_FLAGS) -tags strictfipsruntime" common/scripts/gobuild.sh $(REPO_ROOT)/out/$(TARGET_OS)_$(TARGET_ARCH)/sail-operator cmd/main.go
