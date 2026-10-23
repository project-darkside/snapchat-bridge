# Snapchat API Investigation Findings

## APK Evidence Update: October 22, 2026

The bridge is not verified to authenticate or exchange messages with current
Snapchat. The historical conclusions below are not compatibility evidence.
In particular, neither the local HMAC signing scheme nor the legacy signing
service has been validated against the supplied APK. Do not send real account
credentials to that third-party service on the strength of the old notes.

### Installed Android 14.29.0.50

The connected Pixel 8 has **14.29.0.50**, not the supplied 14.30 Beta.
Do not reuse obfuscated names or native offsets across these builds.
Public installed APKs and local observations are retained under
`%LOCALAPPDATA%\REA\device-snapchat-20261022-154124`:

- Base APK SHA256:
    `e4d9d4aed7f7b354e8123e0e0ccdd344a6b2e0d023a36e12f31f9cc427ff33d7`.
- ARM64 split SHA256:
    `08752341456aa87d6852ebba2cbfff884ebc60a2d2213ba577c25f9925c7219f`.
- Original `libscplugin.so` SHA256:
    `23e1a5bb7cc56bae0d1ce3c664d2504ee6f0e65935733c5967d1562d993c9dd8`.

An isolated, shell-owned Android process successfully loaded that unmodified
native library. Original APK initialization, Android contexts, counter owner,
and protobuf serializer produced a 54-byte LOGIN attestation input and a
4,515-byte native output. Output SHA256:
`dc4053b25e6a52fa1eb64f3d48a07f95cd926d3926277f29a77a82bf93a5e6eb`.
See `rootless-attestation-looper-call.txt` and
`runtime-probe-observation.json`. This was **not** execution as Snapchat's app
UID, did not include all metadata from the full wrapper, and does not prove
that Snapchat accepts the resulting payload. `server_acceptance_tested` is false.
Frida discovery works, but attaching to the installed app failed.

`LoginWireProbe.java` used original APK `MessageNano` serialization and parsing
to round-trip username, email, and phone password requests containing only
fixed, synthetic credentials. `login-wire-observation.txt` retains the bytes.
The Go fixtures in `janus_test.go` match all three byte-for-byte.
The minimal credential wire format is:

| Field | Meaning |
| --- | --- |
| 1, 2, or 3 | Username, email, or phone identifier oneof |
| 4 | Password |
| 15 | `fs60` prerequisite message; its field 11 carries native attestation |

The actual application builder also supplies attempt/source flags, key-related
field 7, and metadata field 8. The minimal fixtures do not establish that those
can be omitted from an accepted login.

Recovered transport and response evidence:

- `z2k.j`, selector 16, configures `aws.api.snapchat.com` for `LoginService`:
    `ev_a97916fdfa107c63552033830a8c1346b1d08bdb73ec137352c888bd3d02eea6`.
    `mq60.subscribe` calls
    `/snapchat.janus.api.LoginService/LoginWithPassword`.
    Login sets `requireAuth=false`; the channel also sets
    `requiresAttestation=false`. This does not remove the attestation contained
    in the protobuf prerequisites. The channel supplies its own user-agent prefix.
- `n070.writeTo` verifies the credential fields:
    `ev_26209fce817f97b601f324f4d17e8c2914dc44ae6361b26ad944b50e00155c1b`.
    `fs60.writeTo` verifies the prerequisite fields:
    `ev_b4bffce443fbb58a25ebc25e1f04da383d249cf9f1fba4a461356ff8d4acbf85`.
- `vq61.apply`'s two-argument overload generates language, request-created-at,
    optional route and pre-auth token headers:
    `ev_8fa69cf28880216e2888a60b1978bb857da98f754b09c68311171faa48c46f70`.
- `o070.writeTo` puts application status in field 1 and the success oneof in
    field 2 (`h18`):
    `ev_8b91d068392630ba76b1dde7d7195ef8c9df68d0efea940bb90667f4f9fadf74`.
    Status 1 enters success processing; other statuses include challenges and
    integrity retries. Transport success alone must not establish a session.
- Success storage consumes `h18.b` (`b841`), whose field 6 is `ffu0`.
    `ffu0` contains a primary string and repeated `ivs0` scoped-token messages.
    The scoped-token conversion reads token, scope, and lifetime/timing fields:
    `ev_fb7cccb454e5379402f0bdcbd945a27846d7caf3052d090f520b7ef8e7e590b3`.
    A complete accepted-session decoder and persistence integration are not yet
    implemented.

Credential-free empty gRPC requests reached this host/path over HTTP/2 and
returned HTTP 200, `Content-Type: application/grpc`, gRPC status 3, and
`user agent is required`. Supplying a generic bridge-probe HTTP user agent did
not change that response. This establishes reachability and application
validation, not the correct application user-agent format or successful login.
No real credentials, account identifier, or native output were sent.

`janus.go` now contains the verified minimal request encoder and a separate
response-envelope parser. Its parser preserves application status and oneof
selection, rejects missing success payloads and malformed envelopes, and does
not claim to validate or install a session. These components are **not wired
into the legacy login path**. Full device prerequisites, key material, native
transport metadata, challenge handling, and modern messaging remain unresolved.
The bridge is still **not ready for a proper-account end-to-end test**.

### Supplied Android 14.30.0.39 Beta

REA/JADX inspected the supplied `com.snapchat.android` 14.30.0.39 Beta base APK:

- APK SHA256: `f9b88179d95a07dbb24f202606edabbf79d9c732ddd33c562153140116cd7e4e`.
- `defpackage.bt60.d0` starts `login:overall` and classifies the identifier as
    username, email, or phone. Evidence:
    `ev_1a844ec1eab00ecd91200630662bc8353249722d54e55d887773f321b053eb15`.
- `defpackage.c420.<clinit>` defines `PASSWORD_LOGIN_PATH` as
    `/snapchat.janus.api.LoginService/LoginWithPassword`. Evidence:
    `ev_f0acbd6995e9fd6e4c2497e50f2ca57ba05137b1cecb14034c9a12f28d860957`.
- `defpackage.zo60.q` uses that path and the `ht60.a` prerequisite builder.
    Evidence: `ev_da57a9f3f27e6b868932063ae636792ae3f0ec93c134dfeaeb1ea465cca7d6e9`.
- `defpackage.ht60.a` prepares `login:request:integrity` and
    `login:request:attestation`. Its SafetyNet operation is conditional on a
    non-null byte-array argument; this does not prove which checks a live
    server requires. Evidence:
    `ev_529bc3d911787c66264bd31c3cc43d2823202e6bb576244710e438bd55a4722c`.
- Its attestation callable (`defpackage.laz.call`, selector 29) delegates to
    `n6o0.c(path)`. That method requests a login attestation payload through
    `n6o0.b(path, 1, null, new byte[0], null)`. Evidence:
    `ev_21d4e57ac8fb380d0e191b6145057f74d540b1e79087561848a240ab2fe77a14`
    and `ev_81dc51e6efc211c0a1cf8f486f53b5132f7a898ba05fb632debb4392b7ecf22b`.
- `n6o0.b` serializes a `ydx` message with `MessageNano.toByteArray`, calls
    `q6o0.a()`, and passes the serialized bytes to `c.h(byteArray)`.
    `q6o0.a()` loads the native library `scplugin`. Evidence:
    `ev_bd6385a7eed84067e0733ecd3f82d51f1926a635eefbb879e845e3094f7f9988`
    and `ev_c99cbdbabedcaa5ba8f48206811d7afff589127269bf336d2572ab04c3151a69`.
- The supplied ARM64 split contains `lib/arm64-v8a/libscplugin.so` (2,501,200
    bytes), SHA256
    `d7cb350a0a81a01ce9b281ac7f4d5071f4055bcdf74bbcd17bdba80b9cccdeb1`.
    REA extracted it without executing it. Extraction evidence:
    `ev_6cee847de2848e25d71935ea5b8f5107017e6bf445706295ea2ac84ea6e6a351`.

All evidence IDs above are retained in the local export
`%LOCALAPPDATA%\REA\snapchat-native-evidence-2026-10-22.json`.

Additional static inspection establishes two fields of the native attestation
input. `ydx.d(path)` assigns `c` and sets presence bit 2; `ydx.writeTo` writes
`c` as field 2. `ydx.g(type)` assigns `t` and sets presence bit 4; `writeTo`
writes `t` as field 3. Combined with `n6o0.c`, the LOGIN request therefore
contains the RPC path in field 2 and request type `1` in field 3. This does not
recover the native output or the outer login request. Evidence:

- Serializer: `ev_7bb01218bfd3d0de5c7e01ffb63cc1464110f1f6cfb6234a2af6c441beb2f119`.
- Path setter: `ev_20212d56ec11ac22608a341f23b09de6e57425948894a3c0e7b1d86d5e479a2a`.
- Type setter: `ev_2bcf2d8e2caf0a25c88ca892c1b58c4c63235f2895c6fc39124d9237b6c02fe3`.

These three complete CLI Evidence records are saved separately under
`%LOCALAPPDATA%\REA\linux\attestation-request-{serializer,path,type}.json`.

The outer login builder is also partially recovered. `o550.apply`, selector 4,
constructs `dz60`: email and phone identifiers select fields 2 and 3 respectively,
and the `hq60` prerequisite result is assigned to `n0`. `dz60.writeTo` serializes
`n0` as nested field 15. Other credential and metadata fields remain to be
assigned verified semantic names. Evidence:

- Builder: `ev_b6fc6f8ea1a377ded9a5a1171f3e8b02eef1f0e612b8962af874f9643a3c1652`.
- Serializer: `ev_e9459f7e1b0a7d7facd2afa0e37c99a0479fc9fff3e7728f994e77921ec445c8`.

These CLI records are saved as `login-request-builder.json` and
`login-request-serializer.json` under `%LOCALAPPDATA%\REA\linux`.

Full-class inspection with the repaired JADX engine resolves `n6o0`'s `c`
import to `fl.c`. REA separately verifies that `fl.c.h` has descriptor `([B)[B`
and is native, with no Java body. Evidence:

- Class: `ev_3a30c0b751c2c2a87464c6434ff4b4790b7ae02ede999e24020c6bf48afc4cd1`.
- Method: `ev_d9241b9951cbee0ca039fd2eeb37edc5e41c21bc194f0454e15d1d935e433cbe`.

These CLI records are saved as `attestation-native-owner.json` and
`attestation-native-method.json` under `%LOCALAPPDATA%\REA\linux`.

Native analysis now runs through REA 6.7.0 and Ghidra 12.1 in a native Linux
ARM64 Docker image, `snapchat-rea:6.7.0-ghidra12.1-arm64`. The default profile
reached REA's 330-second startup deadline during switch analysis. A supported
retry with `REA_GHIDRA_DISABLED_ANALYZERS=Decompiler Switch Analysis` completed
import and exposed `JNI_OnLoad` at `0x227f68`. This reduced profile is recorded
in the Evidence; it does not establish complete native control-flow coverage.

Raw-byte inspection finds `fl/c` at `0x185108`, referenced at `0x228074` and
`0x22b080`. Inspection of the registration setup and relocated pointer at
`0x365e00` gives `0x22cac4` as a candidate implementation of `h`. This binding
is a static derivation, not a runtime observation. Both `JNI_OnLoad` and that
candidate have unresolved indirect branches in their decompiler output; the
attestation algorithm, initialization requirements, and output are not recovered.
Addresses here are Ghidra image addresses, not live Android process addresses.

- JNI entry: `ev_6daa3a0c5ef711c4b93c06f2a1070cfdfb2ba35857d2a57e59151322076b0cf2`.
- Class-name bytes: `ev_eb62f99589cdf97ac5e60bb357a6bb56954d5aac36cf378777e0f3e08c58b32b`.
- Class-name references: `ev_d54a60ec99e479905304f60dd40ca19423a5cdd58c47514a11f4ccc318b84bdf`.
- Registration bytes: `ev_be42d8da157636f96ce7c4893f3807a2fe25abb48674adffec7304fbafe44d52`.
- Relocated pointer: `ev_e91e9291f078c68ab1ea08d4b9d5d0a028aa24e52163070664db940068e7b053`.
- Candidate pseudocode: `ev_cc961be701198f8cff86e77241418099cf9f8c78dbc2a27ac5b9d62de1fff991`.

The native session was exported before closing to
`%LOCALAPPDATA%\REA\linux\native-jni-noswitch-2026-10-22.json`: 13 Evidence
records and one open question,
`unk_9221a97b6e663221d9c1f3692afa7611c3de72fd1953b5913da565cd2e1515e4`.
An authorized Android runtime observation is the next discriminating check for
the candidate binding and byte-level contract in this 14.30 build. It has not
been performed for this build; the 14.29 runtime result above is separate.

These are static observations, not a working replacement protocol. Changing
the URL alone cannot adapt the bridge's legacy JSON request to this contract.
The complete request serialization, native attestation implementation, live
transport destination, challenge handling, and session response still need
verification. No current message protocol or successful live login has been
established.

Local login tests now use an in-process signing fixture and reject HTTP errors,
unreadable or invalid JSON bodies, and incomplete sessions. Passing these tests
does not establish Snapchat compatibility.

## Historical Notes: October 29, 2025

**Date:** October 29, 2025
**Status:** 🔴 BLOCKED - Need Real API Endpoints

## Current Situation

We attempted to login to Snapchat using `/bq/login` endpoint and got **404 Not Found**.

### Test Results:
```
Endpoint: https://app.snapchat.com/bq/login
Status: 404 Not Found
Response: HTML error page
Headers: X-Snapchat-Notice: Snapchat Private APIs - Unauthorized use is prohibited.
```

## The Problem

The PHP SDK we're referencing uses an **external Picaboooo service** for authentication:
- Picaboooo acts as a signing service
- It returns the actual Snapchat API URL + signed headers + params
- We don't have access to Picaboooo or know what URLs it returns

### PHP SDK Flow:
```php
$requestData = $picaboo->getAuthenticatedLogin(username, password, tokens...);
$url = $requestData["url"];          // ← We don't know this!
$headers = $requestData["headers"];  // ← Or these!
$params = $requestData["params"];    // ← Or these!
```

## What We Know

1. ✅ Base URL is correct: `https://app.snapchat.com`
2. ✅ Our device token generation works
3. ✅ Our device signature (HMAC-SHA256) works
4. ❌ The endpoint `/bq/login` doesn't exist (or requires different auth)
5. ❌ We don't know the real login endpoint

## Possible Solutions

### Option 1: Use Picaboooo Service (EASIEST) ⭐
**Effort:** Low | **Risk:** Low | **Timeline:** 1-2 hours

- Use the existing Picaboooo service at `https://picaboooo.com/v1/`
- Make our bridge depend on this external service
- Pro: Will definitely work
- Con: External dependency, may not be reliable long-term

```go
// Call Picaboooo to get signed request
response := POST("https://picaboooo.com/v1/login", {
    username, password, dtoken1i, dtoken1v
})

// Use the returned URL and headers
url := response["url"]
headers := response["headers"]
// Make actual Snapchat API call
```

### Option 2: Traffic Analysis (MEDIUM)
**Effort:** Medium | **Risk:** Medium | **Timeline:** 1-2 days

- Set up mitmproxy or Charles Proxy
- Install Snapchat app on device/emulator
- Configure device to route through proxy
- Capture real login traffic
- Reverse-engineer the actual endpoints and signing

**Steps:**
```bash
# Install mitmproxy
brew install mitmproxy

# Run proxy
mitmproxy --mode transparent

# Configure iOS/Android device to use proxy
# Login to Snapchat through app
# Analyze captured traffic
```

### Option 3: Find Existing Implementation (FASTEST IF EXISTS)
**Effort:** Low | **Risk:** Low | **Timeline:** 1-2 hours

- Search GitHub for Snapchat API implementations in other languages
- Look for `snapchat-api`, `snapchat-client`, etc.
- Check if anyone has reverse-engineered the endpoints
- Port the logic to Go

**Search queries:**
- "snapchat api unofficial"
- "snapchat private api login"
- "snapchat api reverse engineering"

### Option 4: Continue Reverse Engineering (HARD)
**Effort:** High | **Risk:** High | **Timeline:** Unknown

- Try different endpoint combinations:
  - `/loq/login`
  - `/ph/login`  
  - `/account/login`
  - `/v2/login`
- Try different signing algorithms
- Analyze Snapchat app binary
- Decompile and reverse-engineer

## Recommendation

**I recommend Option 1 (Use Picaboooo) to unblock immediately**, then work on Option 2 (Traffic Analysis) in parallel for long-term solution.

### Why?
1. We're 95% complete - just need login working
2. Picaboooo exists and works (PHP SDK uses it)
3. Can switch to self-hosted signing later
4. Gets us to a working demo FAST

### Implementation Plan (Option 1):
```go
// 1. Add Picaboooo client
type PicabooClient struct {
    baseURL string // "https://picaboooo.com/v1/"
    httpClient *http.Client
}

// 2. Call Picaboooo for login
func (p *PicabooClient) GetAuthenticatedLogin(username, password, dtoken1i, dtoken1v string) (*SignedRequest, error) {
    // POST to https://picaboooo.com/v1/login
    // Return url, headers, params
}

// 3. Use in Login()
func (c *SnapchatClient) Login(username, password string) error {
    signed, err := c.picaboo.GetAuthenticatedLogin(username, password, c.dtoken1i, c.dtoken1v)
    // Use signed.URL instead of hardcoded /bq/login
    // Use signed.Headers
    // Make request
}
```

## Next Steps

**DECISION NEEDED:**  
Which option should we pursue?

1. ✅ **Option 1** - Use Picaboooo (fastest, gets us working)
2. 🔍 **Option 2** - Traffic analysis (educational, long-term)
3. 🔎 **Option 3** - Find existing code (might not exist)
4. 🧪 **Option 4** - Full reverse engineering (slow, uncertain)

**My vote:** Start with Option 1, then do Option 2 if time permits.

---

## Additional Notes

- The 404 response includes `X-Snapchat-Notice` header, meaning Snapchat's API is responding
- The `X-Snapchat-Request-Id` suggests request reached their servers
- Server is "Google Frontend" - Snapchat likely uses Google Cloud
- The fact we got 404 (not 401/403) suggests endpoint doesn't exist, not that auth failed

## Resources

- Picaboooo: https://picaboooo.com/v1/
- PHP SDK: Already in docs/SnapchatSDK.txt
- mitmproxy: https://mitmproxy.org/
- Charles Proxy: https://www.charlesproxy.com/
