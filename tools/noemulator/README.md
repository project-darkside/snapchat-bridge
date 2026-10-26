# NoEmulator capture tooling

Reverse-engineering harness used to capture the Snapchat password-login
request from a rooted Android VM so the Go encoder in `internal/janus` can be
validated byte-for-byte.

This directory contains two things:

- `vm/` — scripts that boot and control the rooted NoEmulator LineageOS VM.
- `capture/` — Frida hooks and drivers that dump the plaintext login request.

The captured output is written to `capture/captures/` and is **gitignored**:
it contains the disposable test credentials and device attestation blobs.

## What this proved

`hook_grpc_all.js` captures the login request at the point it enters native
code. The transport is **gRPC `UnifiedGrpcService.unaryCall`** to
`/snapchat.janus.api.LoginService/LoginWithPassword`, with the `n070` protobuf
as a `java.nio.ByteBuffer` body. Feeding the captured field values back through
`PasswordRequest.Marshal` reproduces the request byte-for-byte (5987/5987
bytes) — the Go encoder is byte-identical to the APK. See
[docs/development/PASSWORD_LOGIN_FLOW.md](../../../docs/development/PASSWORD_LOGIN_FLOW.md).

## Prerequisites

- Windows 11 ARM64 host with WHPX enabled.
- The NoEmulator lab at `%LOCALAPPDATA%\NoEmulatorLab` (override with `-Lab`),
  with `runtime\clangarm64\bin\qemu-system-aarch64.exe` and the LineageOS
  image under `lineage\LineageOS_on_arm64.utm\Data`.
- Android platform-tools on `adb` (or `ADB` env var).
- Python 3 with `frida` installed (`pip install frida`).

The VM is rooted (KernelSU, `su -c <cmd>` works directly) and spoofs a Pixel 8
with no Google Play Services.

## Boot the VM

```powershell
powershell -File tools/noemulator/vm/Start-SnapchatGuest.ps1
# QEMU_PID <pid>
# PREFIX <log prefix>
# WORK   <overlay dir>
```

This creates a child overlay `snapchat-test` backed by `port-test` (which must
already exist with Snapchat installed and rooted) and forwards:

- adb: `127.0.0.1:15555` (guest `5555`)
- QMP: `127.0.0.1:15556`

Verify it is up:

```powershell
adb -s 127.0.0.1:15555 shell id
# uid=0(root) ... context=u:r:ksu:s0
```

QMP control (screenshot, synthetic taps, hostfwd):

```powershell
powershell -File tools/noemulator/vm/Invoke-LineageMonitor.ps1 `
  -Command screendump -Arguments @{filename='C:\tmp\screen.png';format='png'}
```

## Start frida-server

frida 17.x is used (no Java bridge, but native hooks work). Start it inside the
guest and forward the port:

```powershell
adb -s 127.0.0.1:15555 shell "su -c 'nohup /data/local/tmp/frida-server -l 0.0.0.0:27042 >/data/local/tmp/frida.log 2>&1 &'"
# Then, if 27043 is not already forwarded by QEMU:
powershell -File tools/noemulator/vm/Invoke-LineageMonitor.ps1 `
  -Command human-monitor-command `
  -Arguments @{'command-line'='hostfwd_add tcp:127.0.0.1:27043-:27042'}
```

The drivers connect to `127.0.0.1:27043` by default (`FRIDA_HOST`).

## Capture the login request

1. Launch Snapchat and reach the login form. The "Device not compatible (no
   Google Play services)" modal is dismissible (tap OK ≈ `(800,443)`), then tap
   "Already have an account? Log In" ≈ `(640,680)`. Screen is 1080x2400.

2. Start the capture:

   ```powershell
   python tools/noemulator/capture/run_grpc_all.py 60
   ```

3. Enter credentials and tap Log In. The fields are at `username (640,157)`,
   `password (640,279)`, and the Log In button at `(640,360)` (keyboard closed).
   `input text` works for alphanumeric credentials.

4. Watch the driver output for:

   ```text
   EVT {'kind': 'grpc_unaryCall', 'path': '/snapchat.janus.api.LoginService/LoginWithPassword', 'body': {'len': ..., 'hex': '...'}}
   ```

   The full body hex is appended to `capture/captures/grpc_all.txt`.

## Notes and caveats

- The request body arrives with `position == limit`, so the hook reads it via
  `limit()` + absolute `get`, not `remaining()`.
- `NetworkApi.submit` and `NativeNetworkApi.submit` only carry analytics and
  metrics; the login itself is gRPC `unaryCall`.
- `hook_debug_jni.js` / `run_debug_jni.py` dump the JNIEnv function table and
  raw arguments — useful when adding hooks for other Djinni `CppProxy` methods.
  Djinni `CppProxy` native methods are **static** and take `jlong nativeRef`
  as the first argument, so the first method argument is `args[3]`.
- `dump_libclient_symbols.py` / `dump_net_exports.py` enumerate the
  `libclient.so` exports to find additional hook targets.
- The VM freezes every few hours; reboot with `Start-SnapchatGuest.ps1` if adb
  stops responding.
