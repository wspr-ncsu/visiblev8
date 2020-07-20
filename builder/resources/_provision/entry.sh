#!/bin/sh

# Install baseline prerequisites
# (yes, the juicebox hacksters at Google demand *both* bzip and lbzip binaries)
echo "Installing baseline prerequisites..."
apt update && apt -y install \
	curl \
	git \
	bzip2 \
	lbzip2 \
	lsb-release \
	pkg-config \
	python \
	python-pip \
	python3 \
	python3-pip \
	sudo \
	xz-utils \
	openjdk-8-jdk

# Make sure the "builder" user exists (with whatever UID the workspace is owned by)
UID=$(stat -c "%u" "$WORKSPACE")
useradd -d "$WORKSPACE" -u $UID builder

# Make sure this user can sudo (headless), because depot_tools is stupid like that...
echo 'builder ALL=(ALL:ALL) NOPASSWD:ALL' | sudo EDITOR='tee -a' visudo

# And run the rest of the provisioning as that user...
argv=$(printf "'%s' " "$@")
exec su builder -c "$SETUP/setup.sh $argv"

