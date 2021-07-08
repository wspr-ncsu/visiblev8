# VisibleV8 Sandbox Compatibility Experiment

This branch/directory contains a proof-of-concept demo of logging from inside V8 C++ code via TCP sockets that are opened _inside_ the Chrome sandbox using modified Chrome browser IPC handlers.

This demo is _not_ a working VisibleV8 build; it simply logs a "hello world" kind of message from within the V8 engine's per-process init code (so you see a message for each renderer process that boots up the V8 engine).

## Building

Apply the `chrome-demo-patch.diff` patch to `chromium/src` in a Chrome 91 (commit `c1e1dff6f551c4aab8578ec695825cc9b27d51e6`) checkout.
Apply the `v8-demo-patch.diff` patch to `chromium/src/v8/src` in that same checkout.


## Comments

The critical pieces are:

* `content/browser/sandbox_ipc_linux.cc`: browser-side (non-sandboxed) handler (in `SandboxIPCHandler::HandleRequestFromChild`) that dispatches requests
    * `base::Pickle` serializeation used; first "int" member is the kind of command invoked
    * I added a `sandbox::policy::SandboxLinux::VV8_LOG_SOCKET_ACCESS` enum value in `sandbox/policy/linux/sandbox_linux.h`
    * not a lot of error handling/logging in this code path, even in the existing code...
    * ...used the existing `SandboxIPCHandler::SendRendererReply(...)` helper to send back the open socket FD [if possible]

* `content/common/zygote/sandbox_support_linux.cc`: client-side (sandboxed) client example code with helper for finding the IPC socket correctly
    * added an `__attribute__((visibility("default"))) extern "C" int vv8_connect_to_logging_server(void) { ... }` function here that uses `base::UnixDomainSocket` to do the IPC juju
    * basically just cloned from the existing memory-mapping-request-IPC examle in that same file
    * this is compiled/linked into the Chrome binary, and its visibility/linkage means that it can be called from our code in V8 via a simple `extern "C"` declaration

* `v8/src/init/v8.cc`: [or wherever needed inside the V8 build] added an `extern "C" int vv8_connect_to_logging_server(void) __attribute__((weak));`
    * this results in a function pointer; if it's NULL, we don't have the `vv8_connect_to_logging_server()` function available and have to try direct socket/connect stuff
    * if it is non-NULL, it contains the address of the `vv8_connect_to_logging_server()` function defined above in the `sandbox_support_linux.cc` file; we call this to do the IPC and get our logging socket back for us

