OPERATOR_NAME = servicemeshoperator2
HUB = build-harbor.alauda.cn/asm
CHANNELS = "stable,stable-2.0"
DEFAULT_CHANNEL = stable
HELM_VALUES_FILE = alauda/values.yaml
VERSIONS_YAML_FILE ?= alauda-versions.yaml
GENERATE_RELATED_IMAGES = false

PLATFORMS ?= linux/arm64,linux/amd64
