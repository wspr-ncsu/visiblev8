#!/bin/sh

REV="$1"
if [ -z "$REV" ]; then
	REV="origin/master"
fi

cd "$WORKSPACE"
if [ ! -d "depot_tools" ]; then
	# No existing depot_tools yet; make sure that's installed (inside the workspace, for stupid reasons [not mine])
	echo "No depot_tools/ directory; verifying prerequisits and fetching depot_tools..."
	git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git "$WORKSPACE/depot_tools"
	export PATH="$PATH:$WORKSPACE/depot_tools"
	depot_tools/gclient >/dev/null || exit 1
else
	export PATH="$PATH:$WORKSPACE/depot_tools"
fi

if [ ! -d "src" ]; then
	# No existing fetch--do that first
	echo "No 'src/' directory; fetching chromium..."
	fetch --nohooks --no-history chromium || exit 1
fi
cd "$WORKSPACE/src"

echo "Checking out $REV and synching source dependencies..."
git fetch --quiet origin $REV
git checkout $REV || exit 1
echo "Syncing dependencies..."
"$SETUP/fsync.py" || exit 1

echo "Installing minimal build tool dependencies..."
build/install-build-deps.sh --no-syms --no-arm --no-chromeos-fonts --no-nacl --no-prompt || exit 1
echo "[Re-]Running post-checkout hook scripts..."
gclient runhooks || exit 1

