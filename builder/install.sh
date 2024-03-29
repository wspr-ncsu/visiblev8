#!/bin/bash
set -ex

if [ $TARGETPLATFORM == 'linux/amd64' ]; then
    dpkg -i "/artifacts/$PACKAGE_NAME_AMD64" || true
    apt update && apt install -f --yes
    dpkg -i "/artifacts/$PACKAGE_NAME_AMD64"
    rm "/artifacts/$PACKAGE_NAME_AMD64"
else
    echo "$TARGETPLATFROM"
    dpkg -i "/artifacts/$PACKAGE_NAME_ARM64" || true
    apt update && apt install -f --yes
    dpkg -i "/artifacts/$PACKAGE_NAME_ARM64"
    rm "/artifacts/$PACKAGE_NAME_ARM64"
fi