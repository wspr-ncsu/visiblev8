ARG BASE_IMAGE=node:lts
FROM $BASE_IMAGE

ARG ARTIFACT_DIR
ARG PACKAGE_NAME
ARG VERSION
ARG RUN_USER=node
RUN mkdir -p /artifacts
COPY $ARTIFACT_DIR/${VERSION}/*.deb /artifacts
COPY $ARTIFACT_DIR/${VERSION}/*.pickle /artifacts

RUN dpkg -i "/artifacts/$PACKAGE_NAME" || true
RUN apt update && apt install -f --no-install-recommends --yes
RUN dpkg -i "/artifacts/$PACKAGE_NAME"
RUN rm "/artifacts/$PACKAGE_NAME"

USER $RUN_USER

ENV CHROME_EXE "/opt/chromium.org/chromium/chrome"
ENTRYPOINT ["/opt/chromium.org/chromium/chrome"]

