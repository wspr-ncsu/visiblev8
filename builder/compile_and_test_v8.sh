#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VV8_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION="${1:-155.0.8059.30}"

# Normalize line endings on test files (prevent CRLF character offset discrepancies)
sed -i 's/\r$//' "$VV8_DIR/tests/src"/* "$VV8_DIR/tests/logs"/* 2>/dev/null || true

# Look for pre-compiled vv8-shell matching $VERSION in artifacts
ARTIFACT_SHELL=$(find "$VV8_DIR/builder/artifacts/$VERSION" "$HOME/visiblev8/builder/artifacts/$VERSION" /artifacts/$VERSION -name "vv8-shell*" -type f 2>/dev/null | head -n 1)
if [ -z "$ARTIFACT_SHELL" ]; then
    ARTIFACT_SHELL=$(find "$VV8_DIR/builder/artifacts" "$HOME/visiblev8/builder/artifacts" /artifacts -name "*vv8-shell*$VERSION*" -type f 2>/dev/null | head -n 1)
fi

if [ -n "$ARTIFACT_SHELL" ]; then
    echo "=== Found Pre-compiled V8 Shell: $ARTIFACT_SHELL ==="
    echo "=== Running 3-Minute Canary Oracle (Regression Suite) ==="
    chmod +x "$VV8_DIR/tests/run.sh" "$VV8_DIR/tests/logs/entry.sh"
    cd "$VV8_DIR/tests"
    ./run.sh python:3-bookworm trace-apis-obj
    echo "=========================================="
    echo "SUCCESS: ALL TESTS PASSED WITH 0 DIFFS!"
    echo "=========================================="
    exit 0
fi

# If vv8-shell is not yet built, build it via build-direct or local V8 tree
echo "=== v8_shell not found in artifacts, initiating compilation ==="
if [ -d "/work/v8" ] && command -v ninja >/dev/null 2>&1; then
    export PATH="/work/depot_tools:$PATH"
    export DEPOT_TOOLS_UPDATE=0
    cd /work/v8
    mkdir -p out/Release
    cat > out/Release/args.gn << 'EOF'
is_debug = false
dcheck_always_on = false
symbol_level = 0
v8_enable_lazy_source_positions = false
vv8_trace_properties = true
EOF
    /work/v8/buildtools/linux64/gn gen out/Release
    ninja -C out/Release v8_shell
    mkdir -p "$VV8_DIR/builder/artifacts/$VERSION"
    cp out/Release/v8_shell "$VV8_DIR/builder/artifacts/$VERSION/vv8-shell-$VERSION"
    chmod +x "$VV8_DIR/builder/artifacts/$VERSION/vv8-shell-$VERSION"
    cd "$VV8_DIR/tests"
    ./run.sh python:3-bookworm trace-apis-obj
else
    echo "=== Building standalone v8_shell via Docker ==="
    cd "$SCRIPT_DIR"
    docker build --platform linux/amd64 -t build-direct -f build-direct.dockerfile .
    docker run --platform linux/amd64 --rm \
      -v "$SCRIPT_DIR/artifacts:/artifacts" \
      -v "$SCRIPT_DIR/build:/build" \
      -v "$VV8_DIR:/build/visiblev8" \
      build-direct "$VERSION" 1 0 0 0 0 0
    cd "$VV8_DIR/tests"
    ./run.sh python:3-bookworm trace-apis-obj
fi
