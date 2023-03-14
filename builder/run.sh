#!/bin/bash
set -ex

# cleanup any old builds (like build/103.0.5060.134)
# sudo find build/ -type d -name '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*' | sudo xargs rm -rf


docker build --platform linux/amd64 -t build-direct -f build-direct.dockerfile .
docker run --platform linux/amd64 -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build build-direct $1

./vv82dockerhub.sh
LATEST_IMAGE=`docker images --format='{{.ID}}' | head -1`
../tests/run.sh $LATEST_IMAGE trace-apis
# docker run -it --privileged --entrypoint /bin/bash -v $(pwd):/tests --user 0 visiblev8/vv8-base:104.0.5112.79
# test with a quick screenshot: /opt/chromium.org/chromium/chrome --no-sandbox --headless --disable-gpu --screenshot  --virtual-time-budget=30000 --user-data-dir=/tmp --disable-dev-shm-usage https://www.cnn.com
# run v8 tests: python3 ./v8/tools/run-tests.py --out=../out/Release/ cctest unittests