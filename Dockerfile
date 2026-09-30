FROM registry.access.redhat.com/ubi10/ubi@sha256:9c6a62df091dd9fcef9c52ca4e10c252bdeaa0cb48b363f7e61b4c975dc75d75 AS packager
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
