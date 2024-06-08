#!/bin/bash
set -ex
cd vv8-test/
python3 -m http.server 8080 &
sleep 1
mkdir -p /app/logs
rm -rf /app/logs/*
cd /app/logs
ls -lah
/opt/chromium.org/chromium/chrome --no-sandbox --disable-setuid-sandbox --headless --screenshot  --virtual-time-budget=30000 --user-data-dir=/tmp --disable-dev-shm-usage 'http://dawn.com'
rm -rf /app/logs/*.png
