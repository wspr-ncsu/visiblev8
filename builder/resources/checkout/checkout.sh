#!/bin/sh
# Hard-sync a Chromium source tree to a particular revision
# Should be executed inside a container that has all necessary tools pre-installed,
# but makes no assumptions about the presence of depot_tools/ or src/ inside the
# root workspace (i.e., can check everything out from scratch if needed).

REV="$1"
if [ -z "$REV" ]; then
	REV="origin/master"
fi

cd "$WORKSPACE"
if [ ! -d "depot_tools" ]; then
	# No existing depot_tools yet; make sure that's installed (inside the workspace, for stupid reasons [not mine])
	echo "No depot_tools/ directory; verifying prerequisits and fetching depot_tools..."
	git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git "$WORKSPACE/depot_tools"
	depot_tools/gclient >/dev/null || exit 1
fi
export PATH="$PATH:$WORKSPACE/depot_tools"

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

# ONLY significant change from _provision is here: we no longer run the "install build dependencies" script, since _provision has already done that for us.

echo "[Re-]Running post-checkout hook scripts..."
gclient runhooks || exit 1

