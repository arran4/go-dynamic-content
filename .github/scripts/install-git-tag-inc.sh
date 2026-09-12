#!/bin/bash
set -euo pipefail
VERSION="1.3.10"
ARCHIVE="git-tag-inc_${VERSION}_linux_amd64.tar.gz"
EXPECTED_SHA256="4fab8594ffff76ef99cb911baf0fe3126e4674a4fda2194d33d4f37993f82d9d"
curl -sSfL -o "$ARCHIVE" "https://github.com/arran4/git-tag-inc/releases/download/v${VERSION}/${ARCHIVE}"
echo "${EXPECTED_SHA256}  ${ARCHIVE}" | sha256sum -c -
tar -xzf "$ARCHIVE" git-tag-inc
sudo mv git-tag-inc /usr/local/bin/git-tag-inc
rm "$ARCHIVE"
git-tag-inc --version
