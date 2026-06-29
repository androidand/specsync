#!/bin/sh
# Stamp the current npm package version into index.html.
# CF Pages build command: cd site && sh build.sh
set -e
VERSION=$(node -e "console.log(require('../npm/package.json').version)")
sed -i.bak "s/v0\.2\.1/v${VERSION}/g" index.html && rm -f index.html.bak
echo "site: stamped version ${VERSION}"
