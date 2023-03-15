#!/bin/bash
source .env

RELEASES=$(curl -s -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer $GITHUB_TOKEN"\
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/kapravel/visiblev8/releases)

LAST_RELEASE=$(echo $RELEASES | jq -r .[0].tag_name | cut -d{ -f1)
echo "Last VisibleV8 build is:  $LAST_RELEASE"

get_latest_stable_version() {
    curl -s https://omahaproxy.appspot.com/linux
}

VERSION="$(get_latest_stable_version)"
echo "Latest Chrome stable version is:  $VERSION"

if [ "$LAST_RELEASE" == "$VERSION" ]; then
    echo "Latest release is already up to date"
else
    echo "New release is available"
    echo "Building VisibleV8 for $VERSION"
    make build VERSION=$VERSION DEBUG=0 PUBLISH_ASSETS=1 TESTS=0
    if [ $? -eq 0 ]; then
        echo "Done building VisibleV8 for $VERSION"
        curl -X POST -H 'Content-type: application/json' --data '{"text":"VisibleV8 build for version '$VERSION' has been successful!"}' $SLACK_WEBHOOK
    else
        echo "Failed to build VisibleV8 for $VERSION"
        curl -X POST -H 'Content-type: application/json' --data '{"text":"VisibleV8 build for version '$VERSION' failed. Check the latest logs for errors."}' $SLACK_WEBHOOK
    fi
    echo "Done building VisibleV8 for $VERSION"
fi