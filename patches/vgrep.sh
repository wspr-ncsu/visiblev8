#!/bin/sh

VERSION_PREFIX="$1"

if [ -z "$VERSION_PREFIX" ]; then
	echo "usage: $0 VERSION_PREFIX"
	exit 1
fi

PATCH_ROOT=$(dirname "$0")
find "$PATCH_ROOT" -name "version.txt" -exec grep -H "^Chrome $VERSION_PREFIX" '{}' \;

