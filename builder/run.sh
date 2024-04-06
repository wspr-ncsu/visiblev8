#!/bin/bash
set -ex

# the visiblev8 repository that will be mounted in the container
VV8_DIR="$(dirname `pwd`)"
VERSION=${1:-""}
DEBUG=${2:-""}
PUBLISH_ASSETS=${3:-""}
TESTS=${4:-""}
ANDROID=${5:-""}
ARM=${6:-""}

docker build --platform linux/amd64 -t build-direct -f build-direct.dockerfile .
# docker run --platform linux/amd64 -i -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build -v $VV8_DIR:/build/visiblev8 build-direct $VERSION $DEBUG $ANDROID $ARM
docker run --platform linux/amd64 -i -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build -v $VV8_DIR:/build/visiblev8 -v /home/npantel/generalRepo/123.0.6312.105:/tmp/123.0.6312.105 build-direct $VERSION $DEBUG $ANDROID $ARM
# docker run --platform linux/amd64 -i -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build -v $VV8_DIR:/build/visiblev8 -v /home/npantel/generalRepo/122.0.6261.111:/tmp/122.0.6261.111 build-direct $VERSION $DEBUG $ANDROID $ARM #doesn't succeed now with custom debug builds.. 
# docker run --platform linux/amd64 -i -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build -v $VV8_DIR:/build/visiblev8 -v /home/npantel/generalRepo/112.0.5615.49:/tmp/112.0.5615.49 build-direct $VERSION $DEBUG $ANDROID $ARM # didn't succeed


[ ! -d $ARTIFACT_DIR ] && echo "No artifacts. Please build visiblev8 first and place all artifacts in $ARTIFACT_DIR" && exit 1;
PACKAGE_NAME_AMD64=`find ./artifacts -name '*amd64.deb' -printf "%f\n" | sort -V | tail -n 1`
PACKAGE_NAME_ARM64=`find ./artifacts -name '*arm64.deb' -printf "%f\n" | sort -V | tail -n 1`
VERSION=`echo $PACKAGE_NAME_AMD64 | grep -o -E '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*'`

## Run tests before publishing, if we fail we don't upload
if [[ "$TESTS" -eq 1 ]]; then
    LATEST_IMAGE=`docker ps -l --format={{.Image}}`
    ../tests/run.sh -x $LATEST_IMAGE trace-apis-obj
fi


if [[ "$PUBLISH_ASSETS" -eq 1 ]]; then
    ./vv82dockerhub.sh $VERSION $PACKAGE_NAME_AMD64 $PACKAGE_NAME_ARM64
    ./github_release.sh $VERSION
    # Publish a docker container containing the postprocessors
    make -C ../post-processor publish
fi

# docker run -it --privileged --entrypoint /bin/bash -v $(pwd):/tests --user 0 visiblev8/vv8-base:104.0.5112.79
# test with a quick screenshot: /opt/chromium.org/chromium/chrome --no-sandbox --headless --disable-gpu --screenshot  --virtual-time-budget=30000 --user-data-dir=/tmp --disable-dev-shm-usage https://www.cnn.com
# run v8 tests: python3 ./v8/tools/run-tests.py --out=../out/Release/ cctest unittests