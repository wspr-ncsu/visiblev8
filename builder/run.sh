#!/bin/bash
set -ex

# the visiblev8 repository that will be mounted in the container
VV8_DIR="$(dirname `pwd`)"
VERSION=${1:-""}
DEBUG=${2:-""}
PUBLISH_ASSETS=${3:-""}
TESTS=${4:-""}
ANDROID=${5:-""}

docker build --platform linux/amd64 -t build-direct -f build-direct.dockerfile .
docker run --platform linux/amd64 -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build -v $VV8_DIR:/build/visiblev8 build-direct $VERSION $DEBUG $ANDROID

[ ! -d $ARTIFACT_DIR ] && echo "No artifacts. Please build visiblev8 first and place all artifacts in $ARTIFACT_DIR" && exit 1;
PACKAGE_NAME=`find ./artifacts -name '*.deb' -printf "%f\n" | sort -V | tail -n 1`
VERSION=`echo $PACKAGE_NAME | grep -o -E '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*'`

## Run tests before publishing, if we fail we don't upload
if [[ "$TESTS" -eq 1 ]]; then
    LATEST_IMAGE=`docker ps -l --format={{.Image}}`
    ../tests/run.sh -x $LATEST_IMAGE trace-apis-obj
fi


if [[ "$PUBLISH_ASSETS" -eq 1 ]]; then
    ./vv82dockerhub.sh $VERSION $PACKAGE_NAME
    ./github_release.sh $VERSION
    # Publish a docker container containing the postprocessors
    make -C ../post-processor publish
fi

# docker run -it --privileged --entrypoint /bin/bash -v $(pwd):/tests --user 0 visiblev8/vv8-base:104.0.5112.79
# test with a quick screenshot: /opt/chromium.org/chromium/chrome --no-sandbox --headless --disable-gpu --screenshot  --virtual-time-budget=30000 --user-data-dir=/tmp --disable-dev-shm-usage https://www.cnn.com
# run v8 tests: python3 ./v8/tools/run-tests.py --out=../out/Release/ cctest unittests