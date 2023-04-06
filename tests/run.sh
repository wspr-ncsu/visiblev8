#!/bin/sh
# Run the VisibleV8 tests inside an installed container (must have both v8_shell and unittests inside /artifacts)
#################################################################################################################

if [ "$1" = "-x" ]; then
  PRIV="--privileged"
  shift
else
  PRIV=""
fi
IMAGE="$1"
SUITE="$2"

if [ -z "$IMAGE" -o -z "$SUITE" ]; then
    echo "usage: $0 [-x] DOCKER_IMAGE OUTPUT_SUITE"
    echo
    echo "Available output suites:"
    find "$(dirname $0)/logs" -mindepth 1 -maxdepth 1 -type d -name "[a-z]*" -exec basename '{}' \;
    exit 1
fi

TEST_ROOT=$(realpath $(dirname $0))
ARTIFACTS_DIR="$TEST_ROOT/../builder/artifacts"
SRC_DIR="$TEST_ROOT/src"
TOOLS_DIR="$TEST_ROOT/logs"
SUITE_DIR="$TEST_ROOT/logs/$SUITE"
if [ ! -d "$SUITE_DIR" ]; then
    echo "error: invalid output-suite $SUITE (no such directory '$SUITE_DIR')"
    exit 1
fi

docker run $PRIV --rm \
    -v "$ARTIFACTS_DIR:/artifacts:rw" \
    -v "$SRC_DIR:/testsrc:ro" \
    -v "$TOOLS_DIR:/tools:ro" \
    -v "$SUITE_DIR:/expected:ro" \
    --entrypoint /bin/sh \
    --workdir /tmp \
    "$IMAGE" \
    "/tools/entry.sh"

