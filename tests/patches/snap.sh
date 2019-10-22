#!/bin/sh

PATCHES_DIR=$(realpath $(dirname $0))

usage() {
	echo "usage: ./snap.sh CHROMIUM_ROOT DIFF_FILE1 [DIFF_FILE2 [...]]"
	exit 1
}

CHROME_DIR="$1"
if [ -z "$CHROME_DIR" ]; then
	usage
elif [ ! -d "$CHROME_DIR" -o ! -d "$CHROME_DIR/.git" -o ! -r "$CHROME_DIR/DEPS" ]; then
	echo "'$CHROME_DIR' does not appear to be the root of a Chromium checkout...";
	echo
	usage
fi

# Check .git for the checkout commit hash
CHROME_COMMIT=$(cat "$CHROME_DIR/.git/HEAD") || exit 1

# This part is tricky/hacky, but works for now
PREFIX=$(echo "$CHROME_COMMIT" | cut -c-3)
VERSION=$(wget -O- "https://storage.googleapis.com/chromium-find-releases-static/$PREFIX.html" 2>/dev/null \
	| grep -o "<script>data={.*}</script>" \
	| grep -o "{.*}"  \
	| jq -r ".[\"${CHROME_COMMIT}\"][0]") || exit 1

SNAP_PREFIX=$(echo "$CHROME_COMMIT" | cut -c-20) || exit 1
mkdir "$PATCHES_DIR/$SNAP_PREFIX" || exit 1

echo "Chrome $VERSION" > "$PATCHES_DIR/$SNAP_PREFIX/version.txt"
echo "Commit $CHROME_COMMIT" >> "$PATCHES_DIR/$SNAP_PREFIX/version.txt"

DIFF_FILE="$2"
while [ -r "$DIFF_FILE" ]; do
	cp "$DIFF_FILE" "$PATCHES_DIR/$SNAP_PREFIX/$(basename $DIFF_FILE)"
	shift; DIFF_FILE="$2";
done
