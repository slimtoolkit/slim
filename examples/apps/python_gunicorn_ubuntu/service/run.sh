#!/bin/bash
set -e

echo "python service: starting..."
gunicorn -b 0.0.0.0:9000 -k gevent --keep-alive 70 -t 90 --error-logfile gerrors.log --access-logfile gaccess.log "server:create_api()"
