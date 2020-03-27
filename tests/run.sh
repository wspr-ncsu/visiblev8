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
VV8_LOG_DIR="$3"

if [ -z "$IMAGE" -o -z "$SUITE" ]; then
    echo "usage: $0 [-x] DOCKER_IMAGE OUTPUT_SUITE [VV8_LOG_DIR]"
    echo
    echo "Available output suites:"
    find "$(dirname $0)/logs" -mindepth 1 -maxdepth 1 -type d -name "[a-z]*" -exec basename '{}' \;
    exit 1
fi

TEST_ROOT=$(realpath $(dirname $0))
SRC_DIR="$TEST_ROOT/src"
TOOLS_DIR="$TEST_ROOT/logs"
SUITE_DIR="$TEST_ROOT/logs/$SUITE"
if [ ! -d "$SUITE_DIR" ]; then
    echo "error: invalid output-suite $SUITE (no such directory '$SUITE_DIR')"
    exit 1
fi

if [ -d "$VV8_LOG_DIR" ]; then
	WORK_DIR_HOST="$VV8_LOG_DIR"
else
	WORK_DIR_HOST=$(mktemp -d tmp.vv8test.XXXX)
	if [ ! -z "$VV8_LOG_DIR" ]; then
		echo "specified log dir, '$VV8_LOG_DIR', is not a directory; creating '$WORK_DIR_HOST' instead..."
	fi
	
fi
echo "Using '$WORK_DIR_HOST' as the working directory/logfile directory..."

docker run $PRIV --rm \
    -v "$SRC_DIR:/testsrc:ro" \
    -v "$TOOLS_DIR:/tools:ro" \
    -v "$SUITE_DIR:/expected:ro" \
    -v "$WORK_DIR_HOST:/work" \
    --entrypoint /bin/sh \
    --workdir /work \
    --net host \
    "$IMAGE" \
    "/tools/entry.sh"

if [ "$VV8_LOG_DIR" != "$WORK_DIR_HOST" ]; then
	echo "Purging temp working dir '$WORK_DIR_HOST'...";
	rm -rf "$WORK_DIR_HOST"
fi
