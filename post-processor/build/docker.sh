#!/bin/sh
set -ex
docker build -t vv8-postprocessors-local -t visiblev8/vv8-postprocessors:$(git rev-parse --short HEAD) -f ./build/Dockerfile .
docker cp $(docker create vv8-postprocessors-local):/artifacts ./artifacts