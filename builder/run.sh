#!/bin/bash

docker build -t build-direct -f build-direct.dockerfile .
docker run -v $(pwd)/artifacts:/artifacts  build-direct

./vv82dockerhub.sh
LATEST_IMAGE=`docker images --format='{{.ID}}' | head -1`
./run.sh $LATEST_IMAGE trace-apis