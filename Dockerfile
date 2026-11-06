FROM registry.access.redhat.com/ubi10/ubi@sha256:0e8ccad7d35f5fd756f4f2202f018b8d3c3e8bb9af8e627d874239381a00ab41 AS packager
ARG TARGETOS TARGETARCH

RUN dnf -y --setopt=install_weak_deps=0 --nodocs \
    --installroot /output install \
    setup \
 && dnf clean all --installroot /output
RUN [ -d /usr/share/buildinfo ] && cp -a /usr/share/buildinfo /output/usr/share/buildinfo ||:
RUN [ -d /root/buildinfo ] && cp -a /root/buildinfo /output/root/buildinfo ||:

FROM scratch
ARG TARGETOS TARGETARCH

COPY --from=packager /output /

ADD out/${TARGETOS:-linux}_${TARGETARCH:-amd64}/sail-operator /sail-operator

USER 65532:65532
WORKDIR /
ENTRYPOINT ["/sail-operator"]
