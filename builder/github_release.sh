#!/usr/bin/env bash
source .env

# Set variables
REPO="wspr-ncsu/visiblev8"
GIT_COMMIT=$(git rev-parse --short HEAD)
# visiblev8_ab475ac-112.0.5615.165
TAG="visiblev8_$GIT_COMMIT-$1"
NAME="visiblev8_$GIT_COMMIT-$1"
BODY="This is the release for VisibleV8 commit $GIT_COMMIT based on Chromium $1."
FILE="$1"

cd artifacts

# Create a release
RELEASE=$(curl -s -X POST \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tag_name\": \"$TAG\", \"target_commitish\": \"master\", \"name\": \"$NAME\", \"body\": \"$BODY\", \"draft\": false, \"prerelease\": false}" \
  "https://api.github.com/repos/$REPO/releases")
echo $RELEASE

# Extract the upload_url value
UPLOAD_URL=$(echo $RELEASE | jq -r .upload_url | cut -d{ -f1)
if [ "$UPLOAD_URL" == "null" ]; then
    echo "Error: Failed to create release"
    exit 1
fi

# Zip the asset file
tar -czvf $FILE.tar.gz $FILE/*.deb $FILE/*.pickle $FILE/*.json $FILE/*.apk

# Upload the asset file
curl -X POST \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Content-Type: application/gzip" \
  --data-binary @$FILE.tar.gz \
  "$UPLOAD_URL?name=$FILE.tar.gz&label=$FILE.tar.gz"