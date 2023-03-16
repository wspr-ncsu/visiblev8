#!/bin/bash
source .env

ARTIFACT_DIR="./artifacts"
VERSION=${1:-""}
PACKAGE_NAME=${2:-""}

TARGET_IMAGE="visiblev8/vv8-base:$VERSION"

# login to docker hub
echo $DOCKERHUB_PASSWORD | docker login --username visiblev8 --password-stdin

# build the docker image
docker build -t $TARGET_IMAGE -f vv82dockerhub.dockerfile --build-arg ARTIFACT_DIR=$ARTIFACT_DIR --build-arg PACKAGE_NAME=$PACKAGE_NAME --build-arg VERSION=$VERSION . || { echo -e '\033[0;31m***Building the docker image failed***\033[0m' ; exit 1; }
docker push $TARGET_IMAGE
docker tag $TARGET_IMAGE visiblev8/vv8-base:latest
docker push visiblev8/vv8-base:latest

# if you want to run it and check things out, it would be something like this:
# docker run -it --privileged --entrypoint /bin/bash visiblev8/vv8-base:103.0.5060.134