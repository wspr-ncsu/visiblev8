#!/bin/sh
set -ex
if [ ! -d "./tracker-radar" ]; then
    git clone https://github.com/duckduckgo/tracker-radar.git
fi
./build/agg_entity_hashmap.py