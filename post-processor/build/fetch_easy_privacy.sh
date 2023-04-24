#!/bin/sh
set -ex
if [ ! -f "./artifacts/easylist.txt" ]; then
  curl https://easylist.to/easylist/easylist.txt -o ./artifacts/easylist.txt
fi
if [ ! -f "./artifacts/easyprivacy.txt" ]; then
    curl https://easylist.to/easylist/easyprivacy.txt -o ./artifacts/easyprivacy.txt
fi