#!/usr/bin/env bash
set -euo pipefail

# We clean up manually rather than using the `clean` option because we have a `go.mod` that would be removed by clean.

# Delete all go files.
find go -type f -name '*.go' -delete

# Delete all jsonschema files.
rm -rf jsonschema
