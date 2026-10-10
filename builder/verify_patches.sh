#!/bin/bash
set -e

VV8="/build/visiblev8"
VERSION="155.0.8059.30"

LAST_PATCH=$(grep Chrome $VV8/patches/*/version.txt | grep $VERSION | awk '{print $2}' | sort -V | tail -n 1)
LAST_V8_PATCH_FILE=$(grep $LAST_PATCH $VV8/patches/*/version.txt | awk '{print $1}' | sort -V | tail -n 1 | sed "s/:Chrome//" | sed "s/version.txt/trace-apis.diff/")
LAST_CHROME_SANDBOX_PATCH_FILE=$(grep $LAST_PATCH $VV8/patches/*/version.txt | awk '{print $1}' | sort -V | tail -n 1 | sed "s/:Chrome//" | sed "s/version.txt/chrome-sandbox.diff/")

echo "=== PATCH RESOLUTION TEST ==="
echo "LAST_PATCH: $LAST_PATCH"
echo "LAST_V8_PATCH_FILE: $LAST_V8_PATCH_FILE"
echo "LAST_CHROME_SANDBOX_PATCH_FILE: $LAST_CHROME_SANDBOX_PATCH_FILE"

test -f "$LAST_V8_PATCH_FILE" || { echo "ERROR: V8 patch file missing"; exit 1; }
test -f "$LAST_CHROME_SANDBOX_PATCH_FILE" || { echo "ERROR: Chrome sandbox patch file missing"; exit 1; }

echo "=== DRY-RUN V8 PATCH TEST ==="
cd /work/v8
git checkout 82b65fdd
patch -p1 --dry-run < "$LAST_V8_PATCH_FILE"
echo "V8 patch dry-run SUCCESS"

echo "=== DRY-RUN CHROME SANDBOX PATCH TEST ==="
cd /work/chromium
patch -p1 --dry-run < "$LAST_CHROME_SANDBOX_PATCH_FILE"
echo "Chrome sandbox patch dry-run SUCCESS"

echo "=== ALL VERIFICATIONS PASSED SUCCESSFULLY ==="
