#!/bin/sh
# Bootstrap/entrypoint script for driving builds of the Chromium source tree.
# Exists solely to solve the uid-remapping problem...

# Make sure the "builder" user is modified to use whatever UID the workspace is owned by
# (We're assuming a "builder" image already in place for us, here...)
UID=$(stat -c "%u" "$WORKSPACE")
usermod -d "$WORKSPACE" -u $UID builder

# And run the rest of the checkout as that user...
argv=$(printf "'%s' " "$@")
exec su builder -c "./build.sh $argv"
