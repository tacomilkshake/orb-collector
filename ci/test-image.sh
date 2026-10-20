#!/usr/bin/env bash
set -euo pipefail
docker run --rm --network none "$1" --help | grep -q 'Available Commands'
docker run --rm --network none "$1" version
