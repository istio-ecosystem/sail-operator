OPERATOR_NAME = alaudasailoperator
HUB = build-harbor.alauda.cn/asm
CHANNELS = "stable"
DEFAULT_CHANNEL = stable
HELM_VALUES_FILE = alauda/values.yaml
VERSIONS_YAML_DIR ?= .
VERSIONS_YAML_FILE ?= alauda/versions.yaml
GENERATE_RELATED_IMAGES = false
