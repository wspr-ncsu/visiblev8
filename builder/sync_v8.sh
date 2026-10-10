#!/bin/bash
set -e

export PATH="/work/depot_tools:$PATH"
export DEPOT_TOOLS_UPDATE=0

cd /work
cat > .gclient << 'EOF'
solutions = [
  {
    "name": "v8",
    "url": "https://chromium.googlesource.com/v8/v8.git",
    "deps_file": "DEPS",
    "managed": False,
    "custom_deps": {},
  },
]
EOF

echo "Running gclient sync for V8 standalone..."
gclient sync --no-history --shallow
echo "gclient sync completed successfully!"
