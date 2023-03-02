# VisibleV8

Patches and build tools (with some tests) for turning Chromium into VisibleV8.

The core patches are architecture and platform agnostic, but some of the logging code currently has implementation-detail dependencies on Linux.
The [optional] build system is definitely Linux-specific.

## Quick Start (updated March 2023)
(These instructions are for building VV8 on Chromium 104. Find commit hashes of other versions [here](http://omahaproxy.appspot.com/), but make sure there's a matching patchset in `patches/` in this repository.)

* Make sure you have [Docker](https://docs.docker.com/install/) and [Python 3](https://www.python.org/downloads/) and a lot of free disk space (e.g., 50GiB) for downloading and building Chromium
* Clone this repository *(we will call the cloned working directory **$VV8**)*
* Run `sudo ./run.sh 104.0.5112.79` from *inside* `$VV8/builder` *

* Success !! You can find the `.deb` file inside `$VV8/builder/artifacts` *


## (Deprecated) Quick Start

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
