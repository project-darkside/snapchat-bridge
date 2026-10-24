# Snapchat Bridge Progress Summary

Updated: 2026-10-23

## Current Status

The bridge is **not ready for real Snapchat account testing**. Modern login
components exist but are not connected to the active Matrix login flow. No real
account credentials have been used. Passing local tests and reaching Snapchat's
request validator do not demonstrate successful authentication or messaging.

Use this checkpoint and [current findings](docs/development/FINDINGS.md) instead
of the completion percentages in the historical
[handoff](docs/development/HANDOFF.md).

The [REA login/session report](docs/development/PASSWORD_LOGIN_FLOW.md) explains
the native attestation boundary, Fidelius initialization, challenge dispatch,
and token persistence with method-level evidence and remaining unknowns.

## Handoff Snapshot — 2026-10-23

**Goal.** Reverse-engineer Snapchat password login and extract the native
attestation into a Go library so this Matrix bridge can log in without leaking
the user's IP (all traffic must go through Tor SOCKS5 `127.0.0.1:9150`).

**What the login needs (the three "missing" n070 fields).**
- Field 7 `v0u` — Fidelius init: secp256r1 (P-256). `k5u` field 1 = X.509 SPKI
  DER bytes `[26:91)` (65-byte uncompressed point); `v0u` field 3 = Android ID.
  Fresh keygen is EC P-256; the private scalar is HKDF + AES-GCM-unwrapped from
  the device blockstore (`fs91.b/c` + `r7i`). `r241` creation is still unknown.
- Field 8 `c3e` — COF metadata: ETag + bitmap + routing tag (not a credential).
- Field 15 `fs60` — prerequisites; its field 11 is the native attestation blob.

**Attestation is software (proven at runtime).** The boundary is
`yblj.prh.a([B)[B]` in `libscplugin.so`. An on-device probe that snapshotted
`/proc/self/maps` around `prh.a()` saw zero new libraries load, and repeated
calls returned different-size outputs with distinct SHA-256s (freshness, not a
device cert). `libscplugin.so` links only libc/libm/libdl/libz; no TEE/Keymaster.
`libkameleon.so` is a red herring: a string inside `libscplugin.so`, absent from
every APK, 0 DEX hits, 0 device filesystem hits, not loaded on first load, and
not on the attestation path.

**`afx` attestation-request wire format (implemented in Go).** Input to
`prh.a` is the serialized `afx` message (`ydx` → nested `fg3` → repeated `kte`
in the 14.30.0.39 APK):

| msg | field | type | note |
| --- | --- | --- | --- |
| `ydx` (afx) | 1 | string | RPC path |
| | 2 | string | unknown |
| | 3 | varint | type (`1` = LOGIN) |
| | 4 | msg `fg3` | metadata |
| | 5 / 6 | bytes | unknown |
| | 7 | varint | unknown |
| `fg3` | 1 | repeated `kte` | metadata entries |
| | 2-5 | varint | unknown |
| `kte` | 1-2 | repeated string | unknown |
| | 3-4 | varint | unknown |

`internal/janus/attestation.go` marshals all three and exposes `Attestor` +
`CommandAttestor` (oracle plumbing). **Still unknown: the field *values* the
APK's builder fills in** — recover them from the caller of `ydx`'s setters.

**Environment / tooling.**
- Device: Pixel 8 (`shiba`), Android 15 API 35, production build, **non-rooted**.
- adb: `C:\Users\Peter\AppData\Local\Android\Sdk\platform-tools\adb.exe`.
- REA CLI: `%LOCALAPPDATA%\REA\runtime\node_modules\rea-agents\scripts\rea.mjs`
  (node `%LOCALAPPDATA%\REA\node-v24.17.0-win-x64\node.exe`); JADX via
  `REA_JADX_MCP_JAR=%LOCALAPPDATA%\REA\jadx-headless-mcp-0.7.1-all.jar`.
- frida injection is **blocked** on non-rooted Android 15 (memfd `/proc/PID/fd/N`
  dlopen fails for spawn and attach).
- Workaround that worked: **self-instrumented probe** — pull framework.jar /
  core-oj.jar / core-libart.jar from the device, compile a Java helper with
  javac 21, dex it with `r8-8.3.37.jar`
  (`com.android.tools.r8.D8 --min-api 26 --lib ...`), run it via
  `app_process64` with Snapchat's `base.apk` on CLASSPATH.
- Ghidra native (arm64 ELF) analysis needs the Linux Docker env in
  `%LOCALAPPDATA%\REA\linux\` (`Dockerfile`; Docker Desktop must be running).
  Windows Ghidra P0 is PE-only; `kuna` is not configured.

**Session artifacts** (not committed): `c:/Users/Peter/.copilot/session-state/8026201d-92c8-4aef-8af8-1932ed5b8f2c/` —
`checkpoints/`, `files/MapsProbe.java` (the probe that proved software
attestation), `files/Sleeper.java`, frida scripts. Evidence:
`%LOCALAPPDATA%\REA\device-snapchat-20261022-154124\` (APKs, `libscplugin.so`,
probe zips, `attestation-input/output.bin`, ~60 REA records) and
`%LOCALAPPDATA%\REA\linux\` (Ghidra exports incl. `attestation-*.json`).

## Completed Progress

- Hardened legacy login response handling: reject failed HTTP responses,
  malformed bodies, and incomplete sessions; removed raw response logging.
- Made the rate limiter concurrency-safe and corrected admission timestamps.
- Replaced external-service login tests with local fixtures and added regression
  coverage for response failures and concurrent rate limiting.
- Added [Janus protobuf handling](janus.go): minimal password requests matching
  synthetic requests serialized by the installed APK, plus fail-closed response
  envelope parsing that preserves challenge and unknown statuses.
- Added [Janus gRPC transport](janus_transport.go) using standard Go gRPC, TLS
  verification, a fixed official destination, bounded calls, allowlisted
  metadata, and sanitized RPC errors.
- Added [Janus tests](janus_test.go) for wire fixtures, malformed messages, local
  gRPC contracts, metadata, and error redaction. The live probe is opt-in.
- Repaired the local REA/JADX tooling and retained APK evidence outside the
  repository. Original native code ran in an isolated phone-side probe, but its
  output has not been accepted by Snapchat or submitted with credentials.

## Verified Protocol Boundaries

- Installed build: Android Snapchat **14.29.0.50**, distinct from the supplied
  14.30.0.39 APK. Do not mix their obfuscated symbols or native addresses.
- Official destination: `aws.api.snapchat.com:443`.
- Password RPC: `/snapchat.janus.api.LoginService/LoginWithPassword`.
- APK-format user agent plus an empty, credential-free request reaches the
  official validator and returns gRPC `InvalidArgument` / `invalidRequest`.
  This is transport reachability only, not a working login.
- Request field 7 is Fidelius initialization material, accompanied locally by
  private user-key context required by later processing.
- Request field 8 is COF configuration-sync metadata: ETag, bitmap, and routing
  tag. It is not another device credential.
- The active login and message paths still use legacy APIs and an unverified
  legacy signing service. Do not provide real credentials to that service.

## Native Attestation & libkameleon (definitive status)

- The attestation boundary is `yblj.prh.a([B)[B]` in `libscplugin.so`. It
  builds the `afx` blob that becomes `fs60` field 11 / `n070` field 15.
- **It is software, not hardware.** An isolated on-device probe that snapshots
  `/proc/self/maps` around `prh.a()` observed zero newly loaded libraries, and
  repeated calls returned different-sized outputs with distinct SHA-256s
  (freshness/timestamp, not a device certificate). There is no TEE/Keymaster
  path; `libscplugin.so` links only libc/libm/libdl/libz.
- `libscplugin.so` contains the string `libkameleon.so`, but `libkameleon.so`:
  is absent from every shipped APK, absent from the Java bytecode (0 DEX hits),
  absent from a full device filesystem sweep, and absent from the first-load
  `nativeloader` sequence (`libferrite-launcher-4fae2d0.so` then `libclient.so`
  load directly from the split APKs). It is not on the attestation path; the
  name is a native-internal, likely conditional/legacy `dlopen` target.
- Consequence: the "closed path" is obfuscation plus an unknown signing key,
  not hardware. It is reproducible in principle from `libscplugin.so`.

### Attestation into Go: plan

[`internal/janus/attestation.go`](internal/janus/attestation.go) now implements
the recovered `afx` request wire format (`ydx`/`fg3`/`kte` in the 14.30.0.39
APK) plus an `Attestor` interface and `CommandAttestor` oracle plumbing. Three
routes remain, in time-to-working-login order:

1. **A — device oracle:** a small on-device helper (reusing the `app_process64`
   probe technique) loads `libscplugin.so`, calls `prh.a(afx)`, and returns the
   blob over `adb`; `CommandAttestor` consumes it. Validate server acceptance
   end-to-end before building further.
2. **C — instrument `prh.a`:** interpose libc (`getrandom`, `clock_gettime`,
   file reads) around `prh.a` to learn the exact entropy/key inputs; feeds B.
3. **B — reimplement in Go:** Ghidra-headless decompile of `prh.a` + callees in
   the Linux Docker environment; drop the phone dependency.

The `afx` builder *field values* (path variant, `fg3` metadata, extra strings)
are still unknown; recover them from the APK caller of `ydx`'s setters before
A or B can mint a server-accepted blob.

## Next Steps

1. **Recover the `afx` builder field values.** Find the APK caller of `ydx`'s
   setters (`e`/`d`/`g` + the `fg3`/`kte` population) to learn the exact path
   string, `fg3` metadata, and extra strings. This unblocks both the oracle and
   the Go reimplementation.
2. **Build the device attestation oracle (Option A).** Extend the
   `app_process64` probe into a helper that loads `libscplugin.so`, calls
   `prh.a(afx)`, and returns the blob (file or socket); drive it from
   `CommandAttestor`. Then send a full login request and check server
   acceptance of the minted blob.
3. **Instrument `prh.a` (Option C).** Interpose libc (`getrandom`,
   `clock_gettime`, file reads) around `prh.a` to learn the exact entropy/key
   inputs feeding the signature.
4. **Reimplement `prh.a` in Go (Option B).** Ghidra-headless decompile of
   `prh.a` + callees in the Linux Docker env; drop the phone dependency.
5. **Finish Fidelius key initialization.** Resume at `vo3.apply` case 28, which
   builds `a3u(v0u, r241)`. Recover fresh key generation, algorithms, encodings,
   and persistence requirements before choosing a crypto implementation.
   `r241.f()` returns stored bytes; `mor0.d()` copies the end-exclusive range
   `[26, 91)`, producing 65 bytes. The crypto algorithm and key encoding remain
   unverified. `p241.b(byte[])` parses existing keys; it does not generate them.
   The inspected `oz4.call` branch processes successful-login key material; it
   does not establish how fresh login keys are generated.
6. **Complete the request prerequisites.** Follow `xu60.a` and the `fs60`
   builder, including original native-provider inputs. Minimal isolated native
   output is not proof of a complete or server-accepted prerequisite. Verify
   the `c3e` setters before encoding the COF metadata from `mr60` / `b3e`.
7. **Decode usable sessions and challenges.** Follow `o070` success field 2
   (`h18`) through `rz4.c`, token storage, and Fidelius processing. Verify token
   scopes, expiry units, refresh rules, retries, and challenge continuations.
   An envelope with status 1 alone must never become a persisted session.
8. **Integrate modern authentication safely.** Wire the complete request,
   transport, and accepted-session handling into the active `bridgev2` login
   flow, replacing credential forwarding to the legacy signing service. Add
   local contract tests for persistence, restart, cancellation, and failures.
9. **Implement current messaging and lifecycle behavior.** Recover current
   conversation identifiers and send/receive APIs; connect Matrix portals,
   polling or streaming, cancellation, and durable message deduplication.
10. **Run account testing only after those prerequisites are implemented.**
    Supply credentials locally, never in chat or to an unverified third party.
    Validate Matrix-to-Snapchat delivery, a reply exactly once in the same
    Matrix room, and both directions after restarting the bridge.

## Verification

Checkpoint checks on 2026-10-22: the full local race suite passed, dependency
checksums verified, and the summary's local links were checked. No live account
login or end-to-end messaging test was performed.

Run the full local race suite from the repository root in PowerShell:

```powershell
docker run --rm `
    --mount "type=bind,source=$((Get-Location).Path),target=/src,readonly" `
    --mount type=volume,source=snapchat-bridge-gomod,target=/go/pkg/mod `
    --mount type=volume,source=snapchat-bridge-gocache,target=/root/.cache/go-build `
    --workdir /src --env CGO_ENABLED=1 golang:1.24-bookworm `
    go test -mod=readonly -tags goolm -race -count=1 ./...
```

For focused coverage, replace the final command with
`go test -mod=readonly -tags goolm -race -count=1 -run '^TestJanus' .`.
`TestJanusTransportLiveProbe` is skipped unless
`SNAPCHAT_JANUS_TRANSPORT_PROBE=1` is explicitly supplied to the container. It
sends only an empty request and synthetic profile fields, not account data or
phone-derived native output. Do not repeat it merely for reassurance.

See [testing guidance](docs/TESTING.md) for environment details. Local tests do
not cover accepted live login or end-to-end Snapchat messaging.

## Investigation Artifacts

Installed-build evidence and probe files are retained locally under
`%LOCALAPPDATA%\REA\device-snapchat-20261022-154124`. Resume with
`password-fidelius-init-creation.json`,
`password-fidelius-userkeys-callers.json`, and the client-metadata label records.
The REA CLI's decompiled method text is in `.normalized_result.source.text`.
Keep APKs, native blobs, credentials, device identifiers, and captures out of
Git. Clean only the exact owned files in the dedicated phone probe directory
when they are no longer needed.

A diagnostic CLI for the recovered Janus password-login wire format now lives
under [`cmd/snapchat-janus`](cmd/snapchat-janus) with the reusable encoder,
transport, and response decoder in [`internal/janus`](internal/janus). It
assembles the `n070` request from captured sub-messages, sends it to
`aws.api.snapchat.com:443`, and decodes the `o070` envelope and success
payload, so the login path can be iterated on without a Matrix server. It does
not remove the Fidelius key-generation or native-attestation unknowns and must
not be used with real credentials.
