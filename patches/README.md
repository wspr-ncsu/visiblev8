# VisibleV8 Patchsets

The essence of VisibleV8 is a set of patches adding fine-grained instrumentation and logging logic to the V8 JavaScript engine used inside the Chromium web browser.

Our patches are designed to be as minimal as possible, to facilitate maintenance as V8 evolves.  But even completely unrelated changes to V8 code can break the automated patch-application process in some cases, so to reproduce old builds, we keep a set of per-Chromium-release-tag patches.

The patch files in the root of this directory comprise the current "development head" of the VisibleV8 patches.  New features are added and tested here first, against whatever version of Chromium/V8 the developer was targetting at the moment (probably the most recent stable release).

The strangely-named subdirectories (e.g., `ff70b961e190bd36db35`) contain "versioned" patchsets known to apply cleanly to the version of V8 bundled in the corresponding Chromium release tag (e.g., Chrome 69.0.3497.100).  The directory name consists of the first 20 characters of the Chromium release tag's Git hash.

VisibleV8 itself is changing over time.  There exist patchsets that have not been maintained with subsequent Chromium releases because they represented developmental dead-ends.  Unmaintained patchsets are retained for historical reference.

## Taking a New Version Snapshot (for VV8 Developers)

The `snap.sh` Bourne shell script automates the process of taking a per-version snapshot of VV8 patchset files for a given checkout of Chromium.  Run it without arguments for brief usage documentation.

Requirements: Bourne-like shell, standard POSIX CLI utilities, `wget`, `jq`

