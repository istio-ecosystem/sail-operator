FROM registry.access.redhat.com/ubi10/ubi@sha256:ad0d8e4fcd4c83cf4a2d025de402f010fd068e43f253d0a57b5681d1ad45f044 AS packager
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
