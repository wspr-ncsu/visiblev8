# Chromium Build Tool

The program `tool.py` simplifies the process of checking out Chromium source code, setting up Depot tools and other required software, configuring Chromium's build system, building Chromium components, and packaging build artifacts for deployment via Docker container.

Its use is *not* required: feel free to follow the standard instructions for building Chromium from scratch, if you prefer.

## Requirements

You need Python 3 (e.g., 3.7.4).  You also need Docker and the Docker CLI tools installed and on your PATH.
(`tool.py` has been developed, tested, and used exclusively on Linux, so you might consider Linux a requirement as well...)

## Concepts

The tool clones Chromium source code files (and Depot tools scripts/etc.) into a user-specified "chrome source tree" working directory, denoted **$WD** in our examples.
Build artifacts are stored inside **$WD** as well, although this can be customized.

Chromium's build process relies on a lot of tools (some custom, some standard).
In fact, on Linux, Chromium's build process automatically installs a lot of packages, and assumes a particular Ubuntu distribution/version.
To avoid having to use a particular Linux distro on the host machine, and to avoid cluttering the host up, `tool.py` performs all fetch and build steps inside a Docker container created on-demand from a base Ubuntu image, customized with the tools required for that particular Chromium version, and snapshotted for use on future builds of that version.

## Uses

`tool.py` comprises multiple distinct commands within one program in the style of modern CLI tools like `git` or `npm`.  Full documentation is available via the `--help` CLI option, but we provide a quick summary of the tool's uses here.

`checkout <COMMIT_HASH>` initializes **$WD** (creating it if necessary), performs a clone of the target Chromium commit hash, and ensures all software required to build that version is installed in a per-version Docker container image.

`shell` invokes a standard, interactive Linux shell inside the Docker build container for the currently-checked-out version of Chromium.  Useful for manual builds and/or debugging the build system.

`build <TARGET1> [<TARGET2> ...]` handles build system configuration and invocation to build the list of provided targets.  Supports some meta-targets like `@v8` (building just the V8 shell and unittest programs), `@chrome` (building the Chromium browser and its Debian installer [which works only on `--release` builds]), and `@std` (the union of `@v8` and `@chrome`) in addition to all `ninja`-recognized build target names.

`install` is a convenience tool for installing Chromium (and/or V8 shell) build artifacts from **$WD** into a Docker container image.  The default base image is a Node.js LTS image, useful for running Puppeteer-based Node apps with custom Chromium builds.

