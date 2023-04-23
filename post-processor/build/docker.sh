#!/bin/sh
set -ex
docker build -t vv8-postprocessor-builder -f ./build/Dockerfile.builder .
docker run  --rm -u 0 -v $(pwd):/visiblev8 -v $(pwd)/build/rust_build_cache:/root/.cargo/registry:rw -v $(pwd)/build/go_build_cache:/go/pkg:rw  vv8-postprocessor-builder make -C /visiblev8
docker build -t visiblev8/vv8-postprocessors:$(git rev-parse --short HEAD) -t vv8-postprocessors-local -f ./build/Dockerfile.vv8 .