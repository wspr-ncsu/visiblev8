#!/bin/sh
if [ ! -d "./tracker-radar" ]; then
    git clone https://github.com/duckduckgo/tracker-radar.git
fi
./agg_entity_hashmap.py