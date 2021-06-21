#!/bin/sh

LOGFILE="$1"
if [ ! -r "$LOGFILE" ]; then
	echo "usage: $0 LOGFILE"
	exit 1
fi

./vv8-post-processor "$LOGFILE" features | grep "script_creation" > creations.txt || exit 1
./vv8-post-processor "$LOGFILE" causality  | grep "script_causality" > causalities.txt || exit 1
jq -r '.[1].script_hash' creations.txt | sort -u > created_hashes.txt || exit 1
jq -r '.[1].child_hash' causalities.txt | sort -u > caused_hashes.txt || exit 1

echo
echo "--------------------------------------"
echo "Hash mismatches (if any):"
comm -3 created_hashes.txt caused_hashes.txt
echo "--------------------------------------"

