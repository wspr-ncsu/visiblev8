# VisibleV8 Porting & Patching Walk-Through Guide

This guide details the end-to-end engineering process for updating VisibleV8 (VV8) patchsets to support new versions of Chromium and the V8 JavaScript engine. It uses the transition from **Chrome 147** to **Chromium 155.0.8059.30** as a concrete reference walkthrough.

---

## 1. Overview & Architecture

VisibleV8 instruments the V8 JavaScript engine and the Chromium content renderer to capture fine-grained execution traces:
- **API calls** (`c`): DOM and platform API function/constructor invocations.
- **Property accesses** (`g`): Named and keyed property reads.
- **Property assignments** (`s`): Named and keyed property writes and mutation operations (`++`, `--`, compound assignments).
- **Keyed membership tests** (`h`): The `in` operator (`prop in obj`).
- **Script lifecycle & genealogy** (`$`): Script IDs, compilation origins, script source code, and parent-child evaluation relationships.
- **Security origin transitions** (`@`): Dynamic URL and security origin changes across execution contexts.
- **Android Gin Bridge calls**: Direct Java-to-JavaScript bridge interactions within Android WebView (`SystemWebView`).

### Repository Structure & Patch Organization
The repository maintains versioned patches under `patches/<20-char-commit-hash>/`:
- **`version.txt`**: Records the target Chrome version (Line 1: `Chrome <version>`) and full 40-character Chromium git commit SHA (Line 2: `Commit <hash>`).
- **`chrome-sandbox.diff`**: Applied to the Chromium source tree (`src/`). Disables renderer process sandboxing and instruments Android Gin Java bridge bindings.
- **`trace-apis.diff`**: Applied to the V8 source tree (`src/v8/`). Implements AST bytecode generation hooks, builtin call interception, safe property inspection, and thread-local trace logging.
- **`patches/trace-apis.diff`** (root): The active development head patch applied to current work.

### Patch Directory Naming Convention
VisibleV8 names each version directory in `patches/` using the **first 20 hexadecimal characters of the Chromium release tag commit hash**.

For **Chromium 155.0.8059.30**:
- **Chromium Tag**: `155.0.8059.30`
- **Chromium Commit**: `a17dbb3e325efe798bb372f32370ee7f67093bc5`
- **V8 Commit**: `82b65fdd7b5847a41e76c9eeb04351ebec267785`
- **Target Directory**: `patches/a17dbb3e325efe798bb3/`

---

## 2. Anatomy of the Patches

### A. Chromium Root Patches (`chrome-sandbox.diff`)

Applied from the root of the Chromium checkout (`src/`):

| Target File | Purpose in VisibleV8 |
|---|---|
| `content/renderer/renderer_main.cc` | Hardcodes `need_sandbox = false;` so desktop renderer processes can create and write trace log files to disk without sandbox violations. |
| `base/android/java/src/org/chromium/base/process_launcher/BindService.java` | Disables Android renderer isolation by forcing `supportVariableConnections()` to return `false`. |
| `chrome/android/java/AndroidManifest.xml` | Sets `isolatedProcess="false"` and `externalService="false"` for sandboxed renderer services. |
| `content/renderer/java/gin_java_function_invocation_helper.cc` | Intercepts Java function invocations across the WebView Gin bridge and dispatches argument metadata to `v8::visv8_log_java_api_call`. |
| `content/renderer/java/gin_java_method_invocation_helper.cc` | Intercepts Java object method invocations crossing the Gin bridge and passes method metadata to the logger. |
| `third_party/blink/renderer/modules/remote_objects/remote_object.cc` | Adds `<base/logging.h>` for Gin remote object tracing support. |
| `third_party/blink/renderer/platform/bindings/v8_binding.h` | Includes public VisibleV8 header `v8/include/v8-visiblev8.h` into Blink bindings. |

### B. V8 Patches (`trace-apis.diff`)

Applied from the root of the V8 submodule (`src/v8/`):

| Subsystem / File | Purpose in VisibleV8 |
|---|---|
| **Build Configuration** (`BUILD.gn`, `BUILD.bazel`) | Adds `src/runtime/runtime-utils.cc` to `v8_base_without_compiler`, forces `v8_enable_lazy_source_positions = false` so source positions are available during compilation, and defines the preprocessor macro `VV8_TRACE_PROPERTIES`. |
| **Public C++ API** (`include/v8-visiblev8.h`, `src/api/api.cc`) | Exports C++ logging APIs (`visv8_log_java_api_call`, `visv8_log_java_prop_get`, `visv8_log_java_prop_set`, `visv8_tls_init`) and handles conversions between public `v8::Local` handles and internal `DirectHandle` / `Tagged` pointers. |
| **Engine Initialization** (`src/init/v8.cc`) | Injects a call to `visv8_tls_init()` inside `V8::Initialize()` to allocate thread-local storage slots before isolates are spun up. |
| **Bytecode Generator** (`src/interpreter/bytecode-generator.cc`) | Injects runtime tracing calls during AST traversal: `Runtime::kTracePropertyLoad` for named/keyed reads, `Runtime::kTracePropertyStore` for assignments and count operations (`++`/`--`), and `Runtime::kTraceFunctionCall` for calls. |
| **Inline Caches** (`src/ic/accessor-assembler.cc`) | Injects `CallRuntime(Runtime::kVV8TraceKeyedHasIC, context, receiver, name)` into `GenerateKeyedHasIC()` to log the `in` operator (`prop in obj`). |
| **Builtin Hooks** (`src/builtins/builtins-call-gen.cc`, `builtins-api.cc`, `builtins-function.cc`, `builtins-global.cc`, `builtins-reflect.cc`, `reflect.tq`) | Hooks `HandleApiCallOrConstruct`, `HandleApiConstruct`, `CreateDynamicFunction` (`Function(...)`), `GlobalEval`, and `Reflect.get`/`Reflect.set`. Builtin `ReflectGet` in Torque (`reflect.tq`) calls out to `TracePropertyLoad`. |
| **Direct Eval Interception** (`src/runtime/runtime-compiler.cc`) | Intercepts `Runtime_ResolvePossiblyDirectEval` to log direct `eval(...)` invocations. |
| **JIT Optimization Bypass** (`src/compiler/js-call-reducer.cc`) | Short-circuits `JSCallReducer::ReduceCallApiFunction` by returning `NoChange()`. This prevents TurboFan from inlining API callbacks into JIT code, ensuring every API invocation flows through the instrumented trampoline. |
| **Safe Property Access** (`src/objects/objects.cc`, `objects.h`, `objects-inl.h`) | Implements `Object::VV8GetPropertyNoSideEffects` and `Object::VV8GetPropertyOrElementWithNoSideEffects` to safely inspect object properties during trace formatting without triggering JavaScript getters or proxy traps. |
| **Logging Runtime Engine** (`src/runtime/runtime-utils.cc`, `runtime-utils.h`, `runtime.cc`, `runtime.h`, `runtime-test.cc`) | Implements thread-local logging state (`VisV8TlsData`, `VisV8Logger`, `VisV8Context`), fast stringification (`visv8_to_string`), script origin tracking (`@`), script genealogy (`$`), and automatic log rotation at 1GB file boundaries. |

---

## 3. Porting Strategy & Conflict Resolution Workflow

When migrating patchsets between versions, follow this systematic workflow:

```
[Baseline Patch (Chrome 147)] ──> [git apply --reject] ──> [Analyze *.rej Files]
                                                                  │
                                                                  ▼
[Clean Git Tree] <── [Verify with v8_shell] <── [Resolve Conflicts Manually]
       │
       ▼
[Generate New Diffs] ──> [Snapshot into patches/<hash>/] ──> [Run Regression Tests]
```

### The Fast-Feedback Development Loop
Full browser builds take hours. When porting patches, **do not build `chrome` immediately**. Use this tiered verification process:
1. **Torque & AST Generation**: Verify that `reflect.tq` compiles.
2. **`v8_shell`**: Compiles in 2–5 minutes. Verifies public APIs, runtime functions, AST bytecode generator modifications, and linking.
3. **`d8`**: Validates the full V8 standalone engine, builtins, and snapshot generation.
4. **`chrome`**: Once `d8` links and runs cleanly, proceed to the full browser build.

### Debugging Failed Automated Builds (`make patch-debug`)
If an automated build fails during `patch -p1` in `builder/build-direct.sh`:
```bash
cd builder
# Commit the stopped container and drop into an interactive debug shell
make patch-debug
```
Inside the container, inspect where the patch failed and resolve rejects directly.

---

## 4. Step-by-Step Patching Walk-Through

### Prerequisites
- Linux host or Docker (AMD64 / x86_64).
- 100+ GB free disk space.
- Multi-core CPU (16–32+ cores recommended).

### Step 1: Look Up Upstream Commit Hashes
Use the Chromium Dash API to fetch release metadata:
```bash
# Fetch latest stable release info for Linux / Android
curl -s "https://chromiumdash.appspot.com/fetch_releases?channel=Stable&platform=Linux&num=1&offset=0" | jq .
```
Identify the key release fields:
- `version`: `155.0.8059.30`
- `hashes.chromium`: `a17dbb3e325efe798bb372f32370ee7f67093bc5`
- `hashes.v8`: `82b65fdd7b5847a41e76c9eeb04351ebec267785`

> [!TIP]
> You can also determine the exact V8 revision used by any Chromium checkout by inspecting `src/DEPS`:
> ```bash
> grep -A 2 "'v8_revision':" src/DEPS
> ```

### Step 2: Launch the Builder Container
From the VisibleV8 repository root:
```bash
cd builder

# Build the builder base image
docker build --platform linux/amd64 -t build-direct -f build-direct.dockerfile .

# Launch interactive shell with repository mounts
docker run --platform linux/amd64 -it \
  -v "$(pwd)/artifacts:/artifacts" \
  -v "$(pwd)/build:/build" \
  -v "$(dirname "$(pwd)"):/build/visiblev8" \
  --entrypoint /bin/bash build-direct
```

### Step 3: Check Out Target Chromium Source
Inside the build container:
```bash
VERSION="155.0.8059.30"
WD="/build/$VERSION"
mkdir -p "$WD" && cd "$WD"

# Set up depot_tools
[ ! -d /tmp/depot_tools ] && git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git /tmp/depot_tools
export PATH="$PATH:/tmp/depot_tools"

# Configure gclient for Android and Linux targets
cat > .gclient << 'EOF'
solutions = [
  { "name"        : 'src',
    "url"         : 'https://chromium.googlesource.com/chromium/src.git',
    "deps_file"   : 'DEPS',
    "managed"     : False,
    "custom_deps" : {},
    "custom_vars" : { "checkout_pgo_profiles": True },
  },
]
target_os = [ 'android' ]
EOF

# Clone Chromium src at target release tag
[ ! -d "$WD/src" ] && git clone --depth 4 --branch "$VERSION" https://chromium.googlesource.com/chromium/src "$WD/src"

cd "$WD/src"
# Sync dependencies and submodules (including src/v8)
gclient sync -D --force --reset --with_branch_heads
```

### Step 4: Apply Baseline Patches & Resolve Rejects
Use the most recent stable baseline patches (e.g., Chrome 147 from `patches/5a40bc538bcbf1b9f4010/`):

#### 4.1 Apply Sandbox Patch (Chromium root `src/`)
```bash
cd "$WD/src"
git apply --reject --whitespace=fix /build/visiblev8/patches/5a40bc538bcbf1b9f4010/chrome-sandbox.diff
```
Inspect any rejected hunks:
```bash
find . -name "*.rej"
```
Common files needing manual resolution in `src/`:
- `content/renderer/renderer_main.cc`: Locate where `need_sandbox` is initialized and set `need_sandbox = false;`.
- `content/renderer/java/gin_java_function_invocation_helper.cc`: Ensure Gin invocation argument structures match the current Chromium IPC conventions.

Delete `.rej` files once resolved:
```bash
find . -name "*.rej" -delete
```

#### 4.2 Apply V8 Patch (`src/v8/`)
```bash
cd "$WD/src/v8"
git apply --reject --whitespace=fix /build/visiblev8/patches/5a40bc538bcbf1b9f4010/trace-apis.diff
```
Locate all rejects:
```bash
find . -name "*.rej"
```

Resolution checklist for key V8 files:
1. **`src/interpreter/bytecode-generator.cc`**:
   - Locate `VisitPropertyLoad`, `VisitAssignment`, `VisitCompoundAssignment`, `VisitCountOperation`, and `VisitCall`.
   - Ensure the `Runtime::kTracePropertyLoad` and `Runtime::kTracePropertyStore` register sequences match the current AST node visitor signatures.
2. **`src/objects/objects.cc`**:
   - Verify `VV8GetPropertyNoSideEffects`. Compare against `src/objects/lookup.h` to ensure all `LookupIterator::State` enum values are covered.
3. **`src/builtins/builtins-call-gen.cc`**:
   - In `HandleApiCallOrConstruct`, ensure the CodeStubAssembler argument extraction aligns with upstream API calling conventions.
4. **`src/builtins/reflect.tq`**:
   - Ensure Torque types in `ReflectGet` match upstream definitions.
5. **`src/ic/accessor-assembler.cc`**:
   - Confirm `CallRuntime(Runtime::kVV8TraceKeyedHasIC, ...)` sits cleanly inside `GenerateKeyedHasIC()`.
6. **`BUILD.gn`**:
   - Ensure `src/runtime/runtime-utils.cc` is listed under `v8_base_without_compiler` sources.

Delete `.rej` files once all conflicts are addressed:
```bash
find . -name "*.rej" -delete
```

### Step 5: Test Compile V8 and Chromium Targets
Generate GN release arguments:
```bash
cd "$WD/src"
mkdir -p out/Release

cat > out/Release/args.gn << 'EOF'
dcheck_always_on=false
is_debug=false
disable_fieldtrial_testing_config=true
is_official_build=true
enable_linux_installer=true
is_component_build=false
use_thin_lto=false
is_cfi=false
chrome_pgo_phase=0
v8_use_external_startup_data=true
EOF

gn gen out/Release
```

#### 5.1 Fast Build: `v8_shell` (Minutes)
Verify V8 syntax, headers, runtime calls, and bytecode generation before committing to a full browser build:
```bash
autoninja -C out/Release v8_shell
```
Fix any compilation errors under `-Werror` (e.g., handle types, missing includes, or unhandled enum switches).

#### 5.2 Standalone Engine: `d8` and Unit Tests
```bash
autoninja -C out/Release d8 v8_unittests
```

#### 5.3 Full Chromium Browser Build
```bash
autoninja -C out/Release chrome chrome/installer/linux:stable_deb web_idl_database
```

### Step 6: Create the Version Snapshot

#### Option A: Using `patches/snap.sh`
```bash
cd /build/visiblev8/patches
./snap.sh "$WD/src" /tmp/chrome-sandbox.diff /tmp/trace-apis.diff
```

#### Option B: Manual Snapshot Creation
Export clean diffs from the modified trees:
```bash
# Export V8 patch
cd "$WD/src/v8"
git diff > /tmp/trace-apis.diff

# Export Chromium sandbox patch (excluding v8 submodule)
cd "$WD/src"
git diff ':!v8' > /tmp/chrome-sandbox.diff

# Determine 20-character commit prefix
COMMIT_PREFIX="a17dbb3e325efe798bb3"
PATCH_DIR="/build/visiblev8/patches/$COMMIT_PREFIX"
mkdir -p "$PATCH_DIR"

cp /tmp/chrome-sandbox.diff "$PATCH_DIR/chrome-sandbox.diff"
cp /tmp/trace-apis.diff "$PATCH_DIR/trace-apis.diff"

# Write version metadata (Strict format required by build-direct.sh!)
cat > "$PATCH_DIR/version.txt" << 'EOF'
Chrome 155.0.8059.30
Commit a17dbb3e325efe798bb372f32370ee7f67093bc5
EOF

# Update root development head patch
cp /tmp/trace-apis.diff /build/visiblev8/patches/trace-apis.diff
```

> [!IMPORTANT]
> The format of `version.txt` must match exactly:
> - Line 1: `Chrome <version>` (e.g., `Chrome 155.0.8059.30`)
> - Line 2: `Commit <40-char-sha>`
> `builder/build-direct.sh` uses `grep Chrome $VV8/patches/*/version.txt` and `awk '{print $2}'` to discover the active patch directory.

### Step 7: Run Regression Tests
Verify that generated traces adhere to VisibleV8 specifications:
```bash
cd /build/visiblev8/builder
# Run test suite against the built image
../tests/run.sh -x <docker-image-name> trace-apis-obj
```

#### Understanding the Test Suite
- `tests/run.sh` executes each test script in `tests/src/*.js` using `vv8-shell`.
- Generates log files: `vv8-*-vv8-shell-*.0.log`.
- `tests/logs/relabel.py` normalizes memory addresses and volatile object IDs.
- Normalized output is diffed against expected output in `tests/logs/trace-apis-obj/*.log`.

Verify the generated logs produce valid format entries:
- `c<call_site>:<target>:<receiver>:<args...>`: API function invocations.
- `g<call_site>:<receiver>:<property>`: Property reads.
- `s<call_site>:<receiver>:<property>:<value>`: Property assignments.
- `h<call_site>:<receiver>:<property>`: Keyed membership (`prop in obj`).
- `$<script_id>:<parent_id>:<name>:<source>`: Script source and genealogy.
- `@<url>:<security_token>`: Origin transitions.

---

## 5. Build Automation & Release Verification

VisibleV8's automated build script (`builder/build-direct.sh`) dynamically resolves the patchset using:
```bash
grep Chrome $VV8/patches/*/version.txt | grep 155.0.8059.30
```
When this matches, `build-direct.sh` selects `patches/a17dbb3e325efe798bb3/` as the active patchset.

### Triggering Full Multi-Platform Builds
You can trigger a full build for Linux, ARM64, and Android targets via:
```bash
cd builder
make build VERSION=155.0.8059.30 ANDROID=1 ARM=1 WEBVIEW=1 IDLDATA=1
```

### Generated Artifacts
Resulting packages are saved to `builder/artifacts/155.0.8059.30/`:
- `chrome-vv8-amd64-155.0.8059.30`: VisibleV8 desktop Chromium executable (x86_64).
- `vv8-shell-amd64-155.0.8059.30`: Lightweight standalone V8 shell for testing.
- `google-chrome-stable_*_amd64.deb`: Debian package for easy installation.
- `ChromePublic-vv8-155.0.8059.30.apk`: Android Chromium browser APK.
- `SystemWebView-vv8-155.0.8059.30.apk`: Android System WebView with Gin bridge instrumentation.
- `idldata.json`: Full dump of Blink Web IDL interfaces and properties.

---

## 6. Troubleshooting & Common Pitfalls

| Error / Symptom | Root Cause | Fix |
|---|---|---|
| `error: enumeration value '...' not handled in switch [-Werror=switch]` | New enum variant added upstream in `LookupIterator::State` (`src/objects/lookup.h`). | Add an explicit case handling the new state in `Object::VV8GetPropertyNoSideEffects` in `src/objects/objects.cc` (usually returning `isolate()->factory()->undefined_value()`). |
| `no matching member function for call to 'length'` | `contents->length()` changed return type to `SafeHeapObjectSize`. | Update call site in `src/runtime/runtime-utils.cc` to `(contents->length()).value()`. |
| `cannot convert 'v8::internal::Handle<T>' to 'v8::internal::DirectHandle<T>'` | Upstream transitioned function parameters from `Handle` to `DirectHandle`. | Update the local handle declaration in `src/runtime/runtime-utils.cc` or `src/api/api.cc` to `DirectHandle<T>`. |
| `Patching Chromium ... failed. Exiting!` during `make build` | Line numbers shifted or context changed between baseline and target release. | Run `make patch-debug` from `builder/` to enter the container, inspect `*.rej`, apply fixes, and re-export the diff. |
| Test failure: `diff filtered_actual.log filtered_expected.log` | New built-in properties or prototype pollution altering trace output order. | Inspect the failure diff output from `relabel.py`. If trace semantics are correct but order/builtin properties shifted upstream, update `tests/logs/trace-apis-obj/<test>.log`. |
