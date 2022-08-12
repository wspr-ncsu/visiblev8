#!/bin/bash
ARTIFACT_DIR="./artifacts"

[ ! -d $ARTIFACT_DIR ] && echo "No artifacts. Please build visiblev8 first and place all artifacts in $ARTIFACT_DIR" && exit 1;
PACKAGE_NAME=`find ./artifacts -name '*.deb' -printf "%f\n" | sort | tail -n 1`
VERSION=`echo $PACKAGE_NAME | grep -o -E '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*'`
TARGET_IMAGE="visiblev8/vv8-base:$VERSION"

# build the docker image
docker build -t $TARGET_IMAGE -f vv82dockerhub.dockerfile --build-arg ARTIFACT_DIR=$ARTIFACT_DIR --build-arg PACKAGE_NAME=$PACKAGE_NAME --build-arg VERSION=$VERSION .

# if you want to run it and check things out, it would be something like this:
# docker run -it --privileged --entrypoint /bin/bash visiblev8/vv8-base:103.0.5060.134