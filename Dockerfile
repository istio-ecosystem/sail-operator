FROM registry.access.redhat.com/ubi10/ubi-micro:latest 

ARG TARGETOS TARGETARCH

ADD out/${TARGETOS:-linux}_${TARGETARCH:-amd64}/sail-operator /sail-operator
ADD resources /var/lib/sail-operator/resources

USER 65532:65532
WORKDIR /
ENTRYPOINT ["/sail-operator"]
