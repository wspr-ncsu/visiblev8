ARG BASE_IMAGE=node:lts
FROM --platform=$TARGETPLATFORM $BASE_IMAGE

ARG ARTIFACT_DIR
ARG PACKAGE_NAME_AMD64
ARG PACKAGE_NAME_ARM64
ARG VERSION
ARG TARGETPLATFORM
ARG RUN_USER=node
RUN mkdir -p /artifacts
COPY $ARTIFACT_DIR/${VERSION}/*.deb /artifacts
COPY $ARTIFACT_DIR/${VERSION}/*.pickle /artifacts
COPY $ARTIFACT_DIR/${VERSION}/*.json /artifacts

COPY ./install.sh /artifacts/install.sh
RUN /artifacts/install.sh

USER $RUN_USER
WORKDIR /home/$RUN_USER

ENV CHROME_EXE "/opt/chromium.org/chromium/chrome"
ENTRYPOINT ["/opt/chromium.org/chromium/chrome"]

