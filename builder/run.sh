#!/bin/bash
set -ex

# the visiblev8 repository that will be mounted in the container
VV8_DIR="$(dirname `pwd`)"
VERSION=${1:-""}
DEBUG=${2:-""}
DOCKERHUB=${3:-""}
TESTS=${4:-""}

docker build --platform linux/amd64 -t build-direct -f build-direct.dockerfile .
docker run --platform linux/amd64 -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build -v $VV8_DIR:/build/visiblev8 build-direct $VERSION $DEBUG

if [[ "$DOCKERHUB" -eq 1 ]]; then
    ./vv82dockerhub.sh
fi

if [[ "$TESTS" -eq 1 ]]; then
    LATEST_IMAGE=`docker images --format='{{.ID}}' | head -1`
    ../tests/run.sh $LATEST_IMAGE trace-apis
fi

# docker run -it --privileged --entrypoint /bin/bash -v $(pwd):/tests --user 0 visiblev8/vv8-base:104.0.5112.79
# test with a quick screenshot: /opt/chromium.org/chromium/chrome --no-sandbox --headless --disable-gpu --screenshot  --virtual-time-budget=30000 --user-data-dir=/tmp --disable-dev-shm-usage https://www.cnn.com
# run v8 tests: python3 ./v8/tools/run-tests.py --out=../out/Release/ cctest unittests