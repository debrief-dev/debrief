#!/bin/bash
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 0.1.0"
  exit 1
fi

VERSION="$1"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
README="$SCRIPT_DIR/../README.md"

# Update version in URL path: releases/download/<old_version>/ -> releases/download/<new>/
sed -i '' "s|releases/download/[^/]*/|releases/download/${VERSION}/|g" "$README"

# Update version in filenames: debrief-<platform>-<old_version>-<arch> -> debrief-<platform>-<new>-<arch>
sed -i '' "s|debrief-\([a-z-]*\)[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*-|debrief-\1${VERSION}-|g" "$README"

echo "Updated README.md links to version ${VERSION}"
grep -o 'releases/download/[^)]*' "$README" | head -3
