ARG BASE_IMAGE=node:lts-jessie
FROM $BASE_IMAGE

ARG ARTIFACT_DIR=/artifacts
ARG PACKAGE_NAME
ARG RUN_USER=node
COPY . $ARTIFACT_DIR

RUN dpkg -i "$ARTIFACT_DIR/$PACKAGE_NAME.deb" || true
RUN apt update && apt install -f -y
RUN dpkg -i "$ARTIFACT_DIR/$PACKAGE_NAME.deb"
RUN rm "$ARTIFACT_DIR/$PACKAGE_NAME.deb"

USER $RUN_USER

ENV CHROME_EXE "/opt/chromium.org/chromium/chrome"
ENTRYPOINT ["/opt/chromium.org/chromium/chrome"]

