ARG BASE_IMAGE=node:lts
FROM $BASE_IMAGE

ARG ARTIFACT_DIR=/artifacts
ARG PACKAGE_NAME
ARG VERSION
ARG RUN_USER=node
COPY ./artifacts $ARTIFACT_DIR

RUN dpkg -i "$ARTIFACT_DIR/$VERSION/$PACKAGE_NAME" || true
RUN apt update && apt install -f --no-install-recommends --yes
RUN dpkg -i "$ARTIFACT_DIR/$VERSION/$PACKAGE_NAME"
RUN rm "$ARTIFACT_DIR/$VERSION/$PACKAGE_NAME"

USER $RUN_USER

ENV CHROME_EXE "/opt/chromium.org/chromium/chrome"
ENTRYPOINT ["/opt/chromium.org/chromium/chrome"]

