#!/bin/sh
# Main driver script for building Chromium components

# Parse arguments: [--clean] [--idl] OUT_DIR TARGET1 [TARGET2 [...]]
CLEAN=n
IDL=n
if [ "$1" = "--clean" ]; then
	CLEAN=y
	shift
fi
if [ "$1" = "--idl" ]; then
	IDL=y
	shift
fi
OUTDIR="$1"
shift

# Dump IDL if so desired
if [ "$IDL" = "y" ]; then
	echo "Dumping IDL found inside '$WORKSPACE/src' to '$OUTDIR/idldata.json'..."
	./dump_idl.py "$WORKSPACE/src" > "$OUTDIR/idldata.json"
fi

# Assert >0 targets
if [ "$#" -eq 0 ]; then
	echo "ERROR: must specify at least one target to build"
	exit 1;
fi

# Set up path to include depot_tools
export PATH="$PATH:$WORKSPACE/depot_tools"
cd "$WORKSPACE/src" || exit 1

# Generate a build configuration with our custom arguments
echo "Configuring build in '$OUTDIR'..."
gn gen "$OUTDIR" || exit 1
cd "$OUTDIR"

if [ "$CLEAN" = "y" ]; then
	echo "Cleaning old intermediate build artifacts..."
	ninja -t clean || exit 1
fi

# Build chrome (and its tests, and v8_shell, and v8's unittests)
echo "Selected targets:"
for t in "$@"; do
	printf "\t%s\n" "$t"
done

echo "Kicking off build..."
ninja "$@" || exit 1

echo "DONE"

