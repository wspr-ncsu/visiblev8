#!/bin/bash
ARTIFACT_DIR="./artifacts"
VERSION=${1:-""}

TARGET_IMAGE="visiblev8/vv8-base:$VERSION"

# login to docker hub
cat vv82dockerhub_password | docker login --username visiblev8 --password-stdin

# build the docker image
docker build -t $TARGET_IMAGE -f vv82dockerhub.dockerfile --build-arg ARTIFACT_DIR=$ARTIFACT_DIR --build-arg PACKAGE_NAME=$PACKAGE_NAME --build-arg VERSION=$VERSION .
docker push $TARGET_IMAGE

# if you want to run it and check things out, it would be something like this:
# docker run -it --privileged --entrypoint /bin/bash visiblev8/vv8-base:103.0.5060.134