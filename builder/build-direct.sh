#!/bin/bash
set -e
set -x
get_latest_patch_version() {
    grep Chrome $VV8/patches/*/version.txt | awk '{print $2}' | sort -n | tail -n 1
}

get_latest_patch_file() {
    grep $LAST_PATCH $VV8/patches/*/version.txt | awk '{print $1}' | sort -n | tail -n 1 | sed "s/:Chrome//" | sed "s/version.txt/trace-apis.diff/"
}

get_latest_stable_version() {
    curl -s https://omahaproxy.appspot.com/linux
}

VV8="$(pwd)/visiblev8"
#FIXME 
[ ! -d $VV8 ] && git clone https://github.com/kapravel/visiblev8.git $VV8


if [ -z "$1" ]
  then
    echo "No Chrome version supplied. Will use the latest stable version."
    VERSION="$(get_latest_stable_version)"
    echo "Latest Chrome stable version is $VERSION"
else
    VERSION=$1
fi

WD="$(pwd)/$VERSION"
DP="$(pwd)/depot_tools"


LAST_PATCH="$(get_latest_patch_version)"
echo $LAST_PATCH;
LAST_PATCH_FILE="$(get_latest_patch_file)"
echo $LAST_PATCH_FILE
LAST_STABLE="$(get_latest_stable_version)"
echo $LAST_STABLE;

# checkout the stable chrome version and its dependencies
[ ! -d $WD/src ] && git clone --depth 4 --branch $VERSION https://chromium.googlesource.com/chromium/src.git $WD/src
[ ! -d depot_tools ] && git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
export PATH="$PATH:${DP}"
cd $WD/
# gclient config https://chromium.googlesource.com/chromium/src.git
cat >.gclient <<EOL
solutions = [
  { "name"        : 'src',
    "url"         : 'https://chromium.googlesource.com/chromium/src.git',
    "deps_file"   : 'DEPS',
    "managed"     : False,
    "custom_deps" : {
    },
    "custom_vars": {},
  },
]
EOL
cd $WD/src
./build/install-build-deps.sh
gclient sync -D --force --reset --with_branch_heads # --shallow --no-history
gn gen out/Default

# patching
cd $WD/src/v8
echo "Using $LAST_PATCH_FILE to patch V8"
patch -p1 <$LAST_PATCH_FILE
cd $WD/src

# building
autoninja -C out/Default chrome v8_shell v8/test/unittests

# copy artifacts
cp out/chrome out/v8_shell /artifacts