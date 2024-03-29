#!/bin/bash
set -ex
docker build -t 'vv8postprocessor-test-log-docker' .
docker run -it -u 0 --entrypoint /bin/bash -v $(pwd):/app:rw --rm vv8postprocessor-test-log-docker /app/get-logs.sh