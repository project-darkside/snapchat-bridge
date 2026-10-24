# Snapchat Bridge Progress Summary

Updated: 2026-10-24

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

## Static recovery — 2026-10-24 (fs60 assembly chain)

Path-2 (no-root) work this session recovered the full `fs60`/`n070` assembly
chain from the installed 14.29.0.50 build and verified the on-device oracle on
the Pixel 5 (same build, `versionName=14.29.0.50`, versionCode 322522).

- `su60` (`BuildLoginHeaderParams`) ctor is
  `(String instanceUuid, List sequenceIds, int[] flags, byte[] clientAttestation,
  String cloudAccountId, rxf0 deviceTokenId)`.
- `vd50.r` (RxJava `Function6`, `this.a==2`) zips six sources into `su60`;
  `vd50.r`'s default case builds `ndw0` instead (a non-login shape).
- `xu60.a` assembles the six-way zip: `tc41.b()` (instanceUuid), the integrity
  single, the COF sequence-id single, the attestation single (`qb00` →
  `prh.a`), `d(eg8)` (cloud account), and `vu60` (`rxf0.b(rz4.g())`, device
  token).
- `eq60` maps `su60` → `fs60` (`eq60.apply` case 2). Its login ctor is
  `(x420, nr60, xu60, String, String, String)`; the three strings are NOT
  fixed literals — they are `nr60.a`, `nr60.b`, and the third string argument
  to `xu60.a`.
- `eq60.apply` case 0 is the reverse map: `fs60` → `rz2` (the intermediate
  DTO), confirming `fs60.o0`/`r0`/`n0` feed `rz2.X`/`Y`/`Z`.
- The login variant `xq60.o` (`x420.VERIFY_ODLV_PATH`) calls `xu60.a` with the
  third metadata string set to `cv21.a().toString()` — a fresh per-request
  UUID, not a constant. `nr60` is created by `v3h0.a()`:
  `new nr60(cz60.b, cz60.d, yv60.b(), yv60.v)` (fallback
  `BuildConfig.APPLIVERY_SECRET` for `nr60.d`).

### fs60 field map (re-verified against fs60.writeTo)

| field | name | type | source |
| --- | --- | --- | --- |
| 1 | b | string | `rod.b()` device identifier |
| 2 | c | string | `nr60.a` |
| 3 | t | string | `nr60.b` |
| 4 | I | string | `nr60.d` login attempt id (fallback `BuildConfig.APPLIVERY_SECRET`) |
| 5 | X | string | third metadata string (`xu60.a` argument) |
| 6 | Y | string | `su60.a` instance uuid |
| 7 | Z | wgg0 | `su60.c` flags (COF sequence ids, repeated int32 field 1) |
| 8 | m0 | string | `su60.e` cloud account id |
| 10 | n0 | q8p | `rxf0.j()` device token id (string field 1), omitted when null |
| 11 | o0 | bytes | `su60.d` client attestation |
| 12 | p0 | string | (unused in login) |
| 13 | q0 | string | (unused in login) |
| 14 | r0 | bytes[] | `su60.b` sequence ids (serialized messages) |

`wgg0` = repeated int32 field 1 (unpacked). `q8p` = string field 1. Both are
now modeled and tested in [messages.go](internal/janus/messages.go) (`Wgg0`,
`Q8P`, `NewLoginPrerequisites`) with
[prerequisites_test.go](internal/janus/prerequisites_test.go).

### Fidelius key wrap/unwrap (recovered 2026-10-24)

- `r241.<init>(masterKey, privateDER, version, publicDER)` sets fields
  `a`/`b`/`c`/`t`; `e()=a` (iwek/master key), `d()=b` (PKCS#8 private),
  `f()=c` (SPKI public), `i()=t` (version). `r241.a()` = HMAC-SHA256(masterKey,
  publicDER) — the "hashed public key" label. `r241.j()` serializes to the
  `q700` message (fields h/i/j/k). `writeToParcel` writes only a/b/c/t.
- `p241.a(Integer, String, String, String)` parses an r241 from 4 stored
  strings (version + three `vh21.Y`-decoded byte[]s); returns null when any
  string is empty.
- `fs91.b(label, masterKey, ciphertext)` = UNWRAP (decrypt): AES key =
  `HKDF(masterKey, salt="", info=label+"key")[0:16]` (AES-128), nonce =
  `HKDF(masterKey, salt="", info=label+"nonce")[0:12]`, then
  `r7i.a(key, nonce, ciphertext, 32, null)` (AES-GCM decrypt).
- `fs91.c(label, masterKey, plaintext)` = WRAP (encrypt): same HKDF derivation,
  then `r7i.b(...)` (AES-GCM encrypt). `fs91.a(masterKey, q0u)` = full load:
  `fs91.b(label, masterKey, q0u.b)` → private scalar → EC key → `r241`.
- `r7i.a` = decrypt, `r7i.b` = encrypt (AES-GCM via `kdw.d(bool, mvd0(...))`);
  `r7i.d` = encrypt with a random 12-byte IV returning `iv||ciphertext`;
  `r7i.c` = decrypt of that form. The int argument is the GCM tag size (32 in
  the Fidelius path, 128 in the `r7i.c`/`r7i.d` path).

### Fidelius key generation (recovered 2026-10-24)

Fresh keygen is `r5u.a`: a 32-byte SecureRandom master key + a fresh P-256
keypair, version 10 (9 only when a config flag is set), yielding
`r241(masterKey, PKCS8 private, 10, SPKI public)`. `vo3.apply` case 28 then
assembles `v0u`: field 1 = the hashed public key as the sole device user id,
field 2 = `k5u` (point / hashed public key / master key / version), field 3 =
the 9-byte Android ID. The hashed public key is `r241.a()` =
HMAC-SHA256(masterKey, SPKI public DER) — it is `k5u` field 2 and the device
user id. This is implemented and tested in
[fidelius.go](internal/janus/fidelius.go) (`computeHashedPublicKey`,
`AndroidIDBytes`, `NewFideliusInitFromKey`, `FideliusKeyVersion = 10`).

The device identifier (`fs60` field 1, `rod.b()`) is the Snapchat daily client
id: `rod` is implemented by `yod` (`ClientIdProviderV2Impl`, a fresh UUID) and
`td60` (lock-screen-mode only), so a freshly generated UUID is the correct
value for a fresh login.

### Still unknown (literal runtime values)

- `cz60.b` / `cz60.d` (`nr60.a` / `nr60.b`) and `yv60.v` (`nr60.d`) — the
  login-config object fields populated at config-init time (deep `gz60.i()`
  → `cz60` chain); likely app/device metadata strings.
- `passwordSource` (n070 field 9) and `numAttempts` (field 5) values — the
  wire map is verified (`n070.writeTo`), but the expected non-zero values for
  a password login are not yet pinned down.
- Fresh-login defaults for the COF sequence/feature flags (`wgg0`), cloud
  account id, device token id, and sequence ids (expected empty or null on
  first login). The third metadata string, login attempt id, instance uuid,
  and client id are fresh UUIDs and are generated client-side.

### End-to-end result (2026-10-24)

`TestEndToEndLogin` (opt-in, [e2e_login_test.go](internal/janus/e2e_login_test.go))
mints a real 4227-byte attestation blob, assembles a 4576-byte `n070` request
with best-guess placeholders, and sends it over Tor: Snapchat returns
`InvalidArgument / invalidRequest` (generic). The harness and wire format are
sound; the placeholder values above are what the validator rejects.

### Verified this session

- `go test ./internal/janus/` and `go vet ./internal/janus/` pass.
- `TestAdbAttestorLiveProbe` minted a 4227-byte attestation blob on the Pixel 5
  via the no-root `app_process64` oracle (Go `AdbAttestor` → `prh.a`).

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
APK) plus an `Attestor` interface, `CommandAttestor` (stdin/stdout oracle
plumbing) and `AdbAttestor` (file-staged on-device oracle). Remaining routes,
in time-to-working-login order:

1. **A — device oracle (built, verified 2026-10-23).** An on-device
   `app_process64` helper (`AttestationOracle`, reflection-only, no android.jar
   needed to compile) loads `libscplugin.so`, calls `yblj.prh.a(afx)`, and
   returns the blob. A full LOGIN afx (835 bytes: field 2 = path, field 3 = 1,
   field 4 = fg3, field 5 = empty) minted a fresh 4.5 KB blob whose SHA-256
   differs from the minimal-afx output, proving the recovered fg3 metadata is
   incorporated. `AdbAttestor` drives it (file staging, since adb shell does
   not reliably forward binary stdin to app_process64). The CLI already accepts
   the blob via `--attestation-file`. **Still open: send a full login request
   and confirm server acceptance** (blocked on the Fidelius/COF fields below).
2. **C — instrument `prh.a`:** interpose libc (`getrandom`, `clock_gettime`,
   file reads) around `prh.a` to learn the exact entropy/key inputs; feeds B.
3. **B — reimplement in Go:** Ghidra-headless decompile of `prh.a` + callees in
   the Linux Docker environment; drop the phone dependency.

### Recovered afx builder field values (14.30.0.39 APK)

Recovered 2026-10-23 from the `ydx` callers. `ydx` is the `afx` request; its
setters are `e(String)`→field 1, `d(String)`→field 2, `g(int)`→field 3, direct
`I`→field 4, `a(byte[])`→field 5, `c(byte[])`→field 6, direct `Z`→field 7. The
native boundary on the Java side is `fl.c.h(byte[])` (14.30.0.39; the installed
14.29.0.50 equivalent is `yblj.prh.a(byte[])`).

- **Builder** `n6o0` ("SCArgosServiceImpl"). `c(path)` is the LOGIN helper and
  calls `b(path, 1, null, new byte[0], null)`, so a LOGIN afx is exactly:
  field 2 = request path, field 3 = 1, field 4 = `fg3` metadata, field 5 =
  empty byte[] (presence bit set), fields 1/6/7 absent.
- **Endpoint classification** `nji0`: exact `/loq/register_v2` and prefix
  `/snapchat.janus.api/RegistrationService/` → type 2 (REGISTER); prefix
  `/snapchat.janus.api.LoginService/` → type 1 (LOGIN); otherwise type 0
  (ARGOS). `kld` (`ClientAttestationInterceptor`) attaches the result as HTTP
  header `x-snapchat-att` = Base64 of `n6o0.b(...)`.
- **`fg3` metadata** (`dc81.q`, when Argos enabled): three hardcoded `kte`
  entries — (1) b = `/boosts-prod/`, `/readreceipt-indexer/`,
  `/streaming-collector/`, `/suggest_friend_`,
  `/snapchat.friending.server.FriendAction/`,
  `/snapchat.telephony.api.PhoneEnrollmentService/`; c = `/events_batch`,
  `/snapchat.friending.server.ContactBook/FullSyncContactBookUpload`; t=5;
  I=true. (2) b = `/messagingcoreservice.MessagingCoreService/`; t=1; I=false.
  (3) music: b = the four ig3.e music service paths, c = the thirteen ig3.f
  music method names; t=5; I=true. A fourth `kte` (b = the
  `ARGOS_SUPPORTED_ENDPOINTS_PREFIXES` value split on `,`, t=5) is appended when
  that config is non-empty. `fg3` booleans/long come from the
  `ARGOS_CONFIGURATION` bytes (default empty); when Argos is disabled `fg3` is
  empty.
- **Wire-format fix**: `fg3` fields 2/3/5 and `kte` field 4 are booleans (not
  int32). [`attestation.go`](internal/janus/attestation.go) now models
  MessageNano presence bits and exports `NewLoginAfxRequest(path)` with the
  recovered constants.

### Recovered c3e (COF metadata, n070 field 8) and v0u (Fidelius init, field 7)

Recovered 2026-10-23 from the installed 14.29.0.50 APK.

**c3e** is built by `xq60.a` from `mr60` = `LoginConfigs(cofSyncMetadata=b3e,
cofRoutingTag=String)`:

| c3e field | setter | value |
| --- | --- | --- |
| 1 | `e(String)` | cofRoutingTag |
| 2 | `c(String)` | cofSyncMetadata.etag (`b3e.a`) |
| 3 | `d()` | fixed constant **385** |
| 4 | `a(byte[])` | cofSyncMetadata.bitmap (`b3e.b`, byte[]) |

[`messages.go`](internal/janus/messages.go) previously had field 1/2 swapped and
treated field 3 as the bitmap; that is now corrected, with
`NewCOFMetadata(routingTag, etag, bitmap)` encoding the fixed 385. The
etag/bitmap/routing-tag values come from the COF config store (`jjd`, private
app data) — runtime values still needed from the device.

**v0u** is assembled by `vo3.apply` case 28 exactly as already encoded in
[`messages.go`](internal/janus/messages.go): field 1 = base64-decoded device
user ids (`n1u.h()`), field 2 = `k5u`, field 3 = Android ID bytes (`n1u.e()`).
`k5u`: field 1 = `r241.f()[26:91)` (65-byte uncompressed P-256 point),
field 2 = `r241.c()`, field 3 = `r241.e()` (iwek/master key), field 4 =
`r241.i()` (version).

**Key-material source (the blocker).** `fs91.a` unwraps the private scalar from
the device blockstore: HKDF(label = stored hashed-public-key + "key"/"nonce",
keyed by the master key) → AES-GCM decrypt of the stored wrapped scalar → an
EC `PrivateKey` over `lbr.d` (P-256). It returns
`r241(iwek=master key, in_beta=PKCS8 EC private key, out_beta=SPKI public,
version)`. The master key and the wrapped blockstore entry live in the app's
private data (`/data/data/com.snapchat.android/...`), so producing a real `v0u`
needs either root access to that storage or the equivalent key material.
The Android ID (`v0u` field 3) is obtainable without root via
`adb shell settings get secure android_id`, and its exact byte encoding is now
known (`lwf` case 1 → `xs91`): hex-decode the 16-char string to 8 bytes
(`xs91.a`, left-padding with `0` via `xs91.c` if shorter), then prepend a
single `0x07` byte (`xs91.d`) — **9 bytes total**. The device-specific value
is read from the attached device and is not committed.

### Recovered fs60 (prerequisites, n070 field 15) — the real blocker

Recovered 2026-10-24. The Go `PasswordRequest`/`n070` field map was re-verified
against the actual `n070.writeTo` (14.29.0.50) and is **correct**: 1/2/3 =
identifier oneof, 4 = password, 5 = attempts, 7 = `v0u`, 8 = `c3e`, 9 =
password source, 11 = WhatsApp bool, 15 = `fs60`.

The reason a login still returns `InvalidArgument/invalidRequest` is that
**`fs60` is not just field 11 (attestation)**. `fs60` has 14 fields, and
`eq60` (the login request mapper) populates most of them:

| fs60 field | source | meaning |
| --- | --- | --- |
| 1 | `rod.b()` | device/app identifier (provider-backed) |
| 2-3, 5 | `eq60` ctor strings | request metadata |
| 4 | `nr60.d` | loginAttemptId (per-request UUID) |
| 6 | `su60.a` | instanceUuid (per-request UUID) |
| 7 | `wgg0.a` = `su60.c` | int[] feature/session flags |
| 8 | `su60.e` | cloudAccountId |
| 10 | `q8p.b` = `su60.f.j()` | deviceTokenId |
| 11 | `su60.d` | clientAttestation (the native `prh.a` blob) |
| 14 | `su60.b` | sequenceIds (repeated serialized messages) |

`su60` is literally named `BuildLoginHeaderParams(instanceUuid, sequenceIds,
int[], clientAttestation, cloudAccountId, deviceTokenId)`. `rz2` is the
intermediate login DTO the mapper assembles before it is converted to `n070`:
field 1 = `mz2` (identifier), field 2 = `lz2` (`c3e` + `v0u`), field 3 =
`hs60` (password oneof), 4/5/6 = bytes/repeated bytes, 7 = `q8p`, 8 = string,
9 = bool, 10 = `qhs0`.

Most fs60 values are per-request runtime values (UUIDs, COF sequence ids,
feature flags) plus the COF/Fidelius key material. A synthetic probe (fresh
P-256 key + HMAC-SHA256(iwek, SPKI) + empty device-user-ids + empty COF + real
attestation blob) over Tor still returned the generic `invalidRequest`, as
expected with an incomplete fs60. The definitive capture is to root the device
and Frida-hook `n070.writeTo`/`eq60.apply` for a real login, or extract the
stored key material + COF config.

## Next Steps

1. ~~**Recover the `afx` builder field values.**~~ Done 2026-10-23 (see
   "Recovered afx builder field values" above). `NewLoginAfxRequest(path)` in
   [`attestation.go`](internal/janus/attestation.go) encodes the full LOGIN afx.
2. ~~**Build the device attestation oracle (Option A).**~~ Done 2026-10-23:
   on-device `AttestationOracle` helper + [`adb_attestor.go`](internal/janus/adb_attestor.go)
   (`AdbAttestor`) verified against the Pixel 8. Remaining: send a full login
   request (with Fidelius `v0u` and COF `c3e`) and confirm the server accepts
   the minted blob.
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
6. **Complete the request prerequisites (`fs60`).** Now that the full 14-field
   map is known (see above), recover each value: `rod.b()`, the `eq60` string
   params, `nr60.d` (loginAttemptId), `su60.a` (instanceUuid), `wgg0.a` (int[]
   flags), `su60.e` (cloudAccountId), `su60.f.j()` (deviceTokenId), and
   `su60.b` (sequenceIds). Root + Frida on the Pixel 5 is the surest way to
   capture the real values plus `v0u`/`c3e` for a live login; alternatively
   recover `rod`/`su60` builders and the `rz2`→`n070` converter statically.
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
