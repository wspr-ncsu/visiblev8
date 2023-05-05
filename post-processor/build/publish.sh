#!/bin/bash
set -ex
./build/docker.sh
if [[ ! -z "${DOCKERHUB_PASSWORD}" ]]; then
    docker login --username visiblev8 --password $DOCKERHUB_PASSWORD
else
    echo "\$DOCKER_PASSWORD not set, assuming user is already logged in, skipping docker login"
fi
docker push visiblev8/vv8-postprocessors:$(git rev-parse --short HEAD)
docker tag visiblev8/vv8-postprocessors:$(git rev-parse --short HEAD) visiblev8/vv8-postprocessors:latest
docker push visiblev8/vv8-postprocessors:latest
