VERSION = 3.0.0
OPERATOR_NAME = servicemeshoperator3
HUB = quay.io/maistra-dev
CHANNELS = "stable,stable-3.0"
DEFAULT_CHANNEL = stable
HELM_VALUES_FILE = ossm/values.yaml
GENERATE_RELATED_IMAGES = false

.PHONY: build-fips
build-fips: ## Build sail-operator binary for FIPS mode.
	GOARCH="$(TARGET_ARCH)" CGO_ENABLED=1 LDFLAGS="$(LD_FLAGS) -tags strictfipsruntime" common/scripts/gobuild.sh $(REPO_ROOT)/out/$(TARGET_OS)_$(TARGET_ARCH)/sail-operator cmd/main.go
