[![Build VisibleV8](https://github.com/wspr-ncsu/visiblev8/actions/workflows/chromium-build.yaml/badge.svg)](https://github.com/wspr-ncsu/visiblev8/actions/workflows/chromium-build.yaml) [![Build VV8 postprocessors](https://github.com/wspr-ncsu/visiblev8/actions/workflows/post-processor.yaml/badge.svg)](https://github.com/wspr-ncsu/visiblev8/actions/workflows/post-processor.yaml) [![GitHub release](https://img.shields.io/github/release/wspr-ncsu/visiblev8?include_prereleases=&color=blue&label=Latest%20release)](https://github.com/wspr-ncsu/visiblev8/releases/latest) 

[![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/wspr-ncsu/visiblev8/total?label=Downloads%20for%20VisibleV8)](https://github.com/wspr-ncsu/visiblev8/releases/) [![Docker Pulls for vv8](https://img.shields.io/docker/pulls/visiblev8/vv8-base?label=Docker%20pulls%20for%20vv8)](https://hub.docker.com/r/visiblev8/vv8-base) [![Docker Pulls](https://img.shields.io/docker/pulls/visiblev8/vv8-postprocessors?label=Docker%20pulls%20for%20postprocessors)](https://hub.docker.com/r/visiblev8/vv8-postprocessors)

# VisibleV8

VisibleV8 is a custom variant of the V8 JavaScript engine that logs all JavaScript API calls and their arguments to a trace log. Maintained and distributed as a minimally-invasive and maintainable patchset, VisibleV8 captures and logs the following activities in plaintext logs:


* All JS function calls that cross the JS/hosting-application boundary (i.e., calls to what V8 internally dubs “API” functions, like window.alert in a browser)
* Named (e.g., foo.bar) or keyed (e.g., foo["bar"]) property lookups for which the receiver (e.g., foo) refers to an object defined by the hosting application or the global object
* Assignments to named or keyed property expressions involving receiver objects defined by the hosting application (or the global object)
* Reflect.get and Reflect.set API access to properties on hosting application-defined objects

This repository provides the patches and build tools (with some tests) for turning Chromium into VisibleV8.

The core patches are architecture and platform agnostic, but some of the logging code currently has implementation-detail dependencies on Linux.
The [optional] build system is definitely Linux-specific.

## Quick Start
The best way to start is by using our docker image. The image is available on [Docker Hub](https://hub.docker.com/r/visiblev8/vv8-base) and can be used by running the following command:

```bash
docker run -it --entrypoint /bin/bash visiblev8/vv8-base:latest
``` 

This will start a docker container with the latest version of VisibleV8 installed. You can then run VisibleV8 by simply invoking the `chrome` binary, here's an example:

```bash
/opt/chromium.org/chromium/chrome --no-sandbox --headless --screenshot  --virtual-time-budget=30000 --user-data-dir=/tmp --disable-dev-shm-usage https://www.google.com
```
and the VisibleV8 logs will be available in the local directory. 

Alternatively, you can download Debian packages from the [releases](https://github.com/wspr-ncsu/visiblev8/releases) page and install them manually.

To run web-scale measurements using VisibleV8, you can use the [visiblev8-crawler](https://github.com/wspr-ncsu/visiblev8-crawler).

## VV8 for Android

This guide demonstrates how VisibleV8 can be used for mobile web measurements. Prebuild VisibleV8 `apk` files are available in our [artifact releases](https://github.com/wspr-ncsu/visiblev8/releases). Each instance of a website crawl will generate seperate log files per isolate. The generated files are written to the `/sdcard` directory. The following code demonstrates how ADB can be leveraged to pull the file once the crawl has finished.

```python
def chromeSequence(website):
  phone_id = '' # identify and set the correct phone-id using 'adb devices'
  baseCommand = "shell am start -a android.intent.action.VIEW -d"
  adbInitCommand = "adb -s {} ".format(phone_id)
  openChrome = adbInitCommand + f"shell am start -a android.intent.action.VIEW -d {website}"
  # closeTabCommand = adbInitCommand + "shell input tap 484 376"
  print("starting the chrome sequence")
  if os.system(openChrome) != 0:
      print("Failed openChrome")
      return
  else:
      print("opened chrome successfully")

def pull_log_file(website_filename):
    pull_cmd = f"""adb pull /sdcard/{website_filename} ."""
    print(pull_cmd)
    os.system(pull_cmd)
```

The first function `chromeSequence` will spawn an instance of Chromium and open the website specified. The vv8 runtime will instantly start logging and writing the logs to a separate file in the `/sdcard` directory. `pull_log_file` function can be used to pull the log files out of the android file system to your local machine. 

Note: The build process for Android has been simplified. Head over to `build/build_direct.sh` and set ANDROID=1 if you want to build the `apk` from scratch.

## Building VisibleV8
(These instructions are for building VV8 on Chromium 104. Find commit hashes of other versions [here](https://chromiumdash.appspot.com/fetch_releases?channel=Stable&platform=Linux&num=1&offset=0), but make sure there's a matching patchset in `patches/` in this repository.)

* Make sure you have [Docker](https://docs.docker.com/install/) and [Python 3](https://www.python.org/downloads/) and a lot of free disk space (e.g., 50GiB) for downloading and building Chromium
* Clone this repository *(we will call the cloned working directory **$VV8**)*
* Run `make build` from *inside* `$VV8/builder`, this will build the latest Chromium version with the VisibleV8 patches. You can also run `make build VERSION=104.0.5112.79` to build a specific version of Chromium, but keep in mind that we do not have patchsets for all versions of Chromium.
* You can find the `.deb` file inside `$VV8/builder/artifacts` and install it using `dpkg -i <path-to-deb-file>`
* (Optional) If you want to verify that VisibleV8 is producing logs as intended, you can run a set of regression tests using `../tests/run.sh -x $(docker ps -l --format={{.Image}}) trace-apis-obj`. The output will verify the output of the v8-shell against a set of pre-generated logs looking for differences between the logs.

## Log Output

VV8 produces trace logs in the browser's current working directory.
The current builds thus require the Chrome sandbox to be disabled (`--no-sandbox`) so VV8 can create and write to log files on demand.
**Note** that the default Docker images produced by the `install` step above do *not* include the `--no-sandbox` argument (or any arguments) to the entry-point, `chrome`.

## Project Contents

* The build tool source and resources (in `builder/`) simplifies building and installing custom Chromium variants
* The patchset directory (`patches/`) includes information on what Chromium versions are supported
* The tests directory (`tests/`) includes JS source and expected log files to help regression-test updates to VV8, and also contains documentation of the log format[s]

## Research Paper

You can read more about the details of our work in the following research paper:

**VisibleV8: In-browser Monitoring of JavaScript in the Wild** [[PDF]](https://kapravelos.com/publications/vv8-imc19.pdf)  
Jordan Jueckstock, Alexandros Kapravelos  
*Proceedings of the ACM Internet Measurement Conference (IMC), 2019*

If you use *VisibleV8* in your research, consider citing our work using this **Bibtex** entry:
``` tex
@conference{vv8-imc19,
  title = {{VisibleV8: In-browser Monitoring of JavaScript in the Wild}},
  author = {Jueckstock, Jordan and Kapravelos, Alexandros},
  booktitle = {{Proceedings of the ACM Internet Measurement Conference (IMC)}},
  year = {2019}
}
```

## (Deprecated) Building VisibleV8 manually

(These instructions are for building VV8 on Chromium 75.  Find commit hashes of other versions [here](http://omahaproxy.appspot.com/), but make sure there's a matching patchset in `patches/` in this repository.)

* Make sure you have [Docker](https://docs.docker.com/install/) and [Python 3](https://www.python.org/downloads/) and a lot of free disk space (e.g., 50GiB) for downloading and building Chromium
* Clone this repository *(we will call the cloned working directory **$VV8**)*
* Create an empty working directory on a device with enough space to check out and build Chromium *(we will call this directory **$WD**)*
* Run `$VV8/builder/tool.py -d $WD checkout 5afa96dadfe803e8a058d6ede0c9c3987405b8d8`
    * This will take a while: it has to check out all the code and run initial software installation steps
    * All tool installation will be captured in a Docker container image that can be reused for all future builds of this version of Chromium
* Run `patch -p1 <$VV8/patches/5afa96dadfe803e8a058/trace-apis.diff` from *inside* `$WD/src/v8` 
* Run `$VV8/builder/tool.py -d $WD build @std`
    * This will *really* take a while: it has to build all of Chromium and [Visible]V8, and V8's unit tests, and the Chromium installer Debian package
    * All these artifacts will be left in `$WD/src/out/Builder`
    * You can specify one or more of Chromium's Ninja build targets in place of our magic placeholder `@std` (e.g., `d8`)
* Optionally, run `$VV8/builder/tool.py -d $WD install` to create a new Docker image with the Chromium/VV8 build installed as the entry-point (for running the tests and/or building your own Puppeteer-based applications using Chromium/VV8 for instrumentation)


