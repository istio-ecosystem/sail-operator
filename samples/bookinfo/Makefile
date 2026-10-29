# Builds the pure-Go Bookinfo images (FROM scratch).
#
# Every version below is both a make target and a --target stage of the
# Dockerfile, so a single template rule builds (and optionally pushes) them:
#
#   make                                                # builds all images using podman
#   make reviews-v3                                     # build a single image
#   make CONTAINER_CLI=docker HUB=ghcr.io/me TAGS=1.0   # override defaults
#   make PUSH=1 HUB=ghcr.io/me                          # build and push
#
# Images are built as multi-arch manifest lists (podman's native --platform
# + --manifest support, no buildx required) so they run on every architecture
# the operator supports.

CONTAINER_CLI  ?= podman
HUB            ?= quay.io/sail-dev
TAGS           ?= latest
PUSH           ?= 0
PLATFORMS      ?= linux/amd64,linux/arm64,linux/s390x,linux/ppc64le

images := productpage-v1 details-v1 details-v2 reviews-v1 reviews-v2 \
          reviews-v3 ratings-v1 ratings-v2

image = $(HUB)/examples-bookinfo-go-$@:$(TAGS)

.PHONY: all $(images)

all: $(images)

ifneq ($(PUSH),0)
ifeq ($(CONTAINER_CLI),podman)
push_cmd = && $(CONTAINER_CLI) manifest push --all $(image) docker://$(image)
else
push_cmd = --push
endif
endif

$(images):
ifeq ($(CONTAINER_CLI),podman)
	$(CONTAINER_CLI) build --platform $(PLATFORMS) --manifest $(image) --target $@ . $(push_cmd)
else
	$(CONTAINER_CLI) buildx build --platform $(PLATFORMS) -t $(image) --target $@ $(push_cmd) .
endif
