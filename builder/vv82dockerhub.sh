#!/bin/bash
source .env

ARTIFACT_DIR="./artifacts"
VERSION=${1:-""}
PACKAGE_NAME_AMD64=${2:-""}
PACKAGE_NAME_ARM64=${3:-""}
GIT_COMMIT=$(git rev-parse --short HEAD)

TARGET_IMAGE="visiblev8/vv8-base:${GIT_COMMIT}_${VERSION}"

# login to docker hub
echo $DOCKERHUB_PASSWORD | docker login --username visiblev8 --password-stdin
docker buildx rm vv8-base-builder
docker run --privileged --rm tonistiigi/binfmt --install all
docker buildx create --name vv8-base-builder --driver docker-container --bootstrap --platform linux/amd64,linux/arm64
docker buildx use vv8-base-builder

# build the docker image
docker buildx build --push --platform=linux/amd64,linux/arm64 -t $TARGET_IMAGE -f vv82dockerhub.dockerfile --build-arg ARTIFACT_DIR=$ARTIFACT_DIR --build-arg PACKAGE_NAME_AMD64=$PACKAGE_NAME_AMD64 --build-arg PACKAGE_NAME_ARM64=$PACKAGE_NAME_ARM64 --build-arg VERSION=$VERSION . || { echo -e '\033[0;31m***Building the docker image failed***\033[0m' ; exit 1; }
docker buildx imagetools create -t visiblev8/vv8-base:latest $TARGET_IMAGE

# if you want to run it and check things out, it would be something like this:
# docker run -it --privileged --entrypoint /bin/bash visiblev8/vv8-base:103.0.5060.134