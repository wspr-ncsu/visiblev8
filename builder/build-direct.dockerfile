# docker build --platform linux/amd64 -t build-direct -f build-direct.dockerfile .
# docker run --rm -v $(pwd)/artifacts:/artifacts  build-direct
FROM ubuntu:bionic

RUN apt update
RUN apt install -y git curl lsb-release sudo apt-utils python3.8
RUN update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.8 1
# speed up the process of build-dependencies by caching the current dependencies
RUN apt install -y binutils binutils-aarch64-linux-gnu binutils-arm-linux-gnueabihf binutils-mips64el-linux-gnuabi64 binutils-mipsel-linux-gnu bison bzip2 cdbs curl dbus-x11 devscripts dpkg-dev elfutils fakeroot flex git-core gperf libasound2 libasound2-dev libatk1.0-0 libatspi2.0-0 libatspi2.0-dev libbluetooth-dev libbrlapi-dev libbz2-1.0 libbz2-dev libc6 libc6-dev libcairo2 libcairo2-dev libcap-dev libcap2 libcups2 libcups2-dev libcurl4-gnutls-dev libdrm-dev libdrm2 libelf-dev libevdev-dev libevdev2 libexpat1 libffi-dev libfontconfig1 libfreetype6 libgbm-dev libgbm1 libgl1 libglib2.0-0 libglib2.0-dev libglu1-mesa-dev libgtk-3-0 libgtk-3-dev libinput-dev libinput10 libjpeg-dev libkrb5-dev libnspr4 libnspr4-dev libnss3 libnss3-dev libpam0g libpam0g-dev libpango-1.0-0 libpci-dev libpci3 libpcre3 libpixman-1-0 libpng16-16 libpulse-dev libpulse0 libsctp-dev libspeechd-dev libspeechd2 libsqlite3-0 libsqlite3-dev libssl-dev libstdc++6 libudev-dev libudev1 libuuid1 libva-dev libvulkan-dev libvulkan1 libwayland-egl1-mesa libwww-perl libx11-6 libx11-xcb1 libxau6 libxcb1 libxcomposite1 libxcursor1 libxdamage1 libxdmcp6 libxext6 libxfixes3 libxi6 libxinerama1 libxkbcommon-dev libxrandr2 libxrender1 libxshmfence-dev libxslt1-dev libxss-dev libxt-dev libxtst-dev libxtst6 locales mesa-common-dev openbox p7zip patch perl pkg-config rpm ruby snapcraft subversion uuid-dev wdiff x11-utils xcompmgr xz-utils zip zlib1g zstd

# RUN --mount=type=tmpfs,destination=/build,tmpfs-mode=1770,tmpfs-size=53687091200
RUN mkdir /build
WORKDIR /build
COPY build-direct.sh /tmp

CMD /tmp/build-direct.sh
