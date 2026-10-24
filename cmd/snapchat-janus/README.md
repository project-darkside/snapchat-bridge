# snapchat-janus

A diagnostic CLI for the Snapchat **Janus password-login RPC** recovered from
the installed Android `14.29.0.50` build. It assembles the `n070` password
request, sends it over the verified `aws.api.snapchat.com:443` destination, and
decodes the `o070` response envelope and success payload.

> **Status: reverse-engineering prototype.** Device attestation, Fidelius key
> generation, and server acceptance of the assembled request are **unverified**.
> This tool exists to iterate on the wire format; it is not a supported login
> client. **Do not send real account credentials to it.** Credentials belong
> only in a local account test after the full request is server-accepted.

The wire format is implemented in [`internal/janus`](../../internal/janus) so the
same encoder/decoder can later be folded into the bridge login path.

## Build

```powershell
go build -o snapchat-janus.exe ./cmd/snapchat-janus
```

## Usage

```powershell
# 1. Build the request offline and print it (no network).
.\snapchat-janus.exe --username bridge-test-user `
    --password-file .\pw.txt --dump-request

# 2. Send a request using recovered raw sub-messages.
.\snapchat-janus.exe --username my-user `
    --password-file .\pw.txt `
    --fidelius-init-file .\v0u.bin `
    --cof-metadata-file .\c3e.bin `
    --prerequisites-file .\fs60.bin `
    --raw

# 3. Build prerequisites from native attestation output only.
.\snapchat-janus.exe --email user@example.com `
    --password-file .\pw.txt `
    --attestation-file .\attestation-output.bin --raw
```

The password is read from, in order: `--password-file`, the
`SNAPCHAT_PASSWORD` environment variable, `--password` (discouraged: visible in
the process list), or a `--spec` file. It is never echoed as plaintext, but the
`--dump-request` hex/base64 output **contains the password in reversible
encoding**, so only dump synthetic fixtures.

### Credentials file

`--password-file` may contain either a bare password or `username:password`.
When it contains a `:` the part before the **first** `:` is used as the
username and everything after it is the password (usernames never contain `:`,
passwords may). With that form, `--username` can be omitted:

```powershell
# pw.txt contains: mybridge01:the-password
.\snapchat-janus.exe --password-file .\pw.txt --raw
```

An explicit `--username`, `--email`, or `--phone` always overrides the username
part of the file; the file's password is still used unless `--password` is
given. A file without a `:` contributes only the password, so an identifier
flag is still required.

## Proxy (Tor)

By default every network call is tunneled through the SOCKS5 proxy
`127.0.0.1:9150` (Tor's SocksPort), so Snapchat sees the proxy's exit address
instead of yours:

```powershell
.\snapchat-janus.exe --username my-user --password-file .\pw.txt `
    --proxy 127.0.0.1:9150 --raw
```

The destination host name (`aws.api.snapchat.com`) is sent to the proxy in the
SOCKS5 `CONNECT` request, so DNS resolution happens at the proxy — no local DNS
lookup or plaintext connection to Snapchat leaks your IP. TLS still pins the
server identity to `aws.api.snapchat.com`, independent of the transport path.
Use `--proxy ""` to disable proxying explicitly (e.g. when Tor is not running).
`--dump-request` and `--request-out` never open a network connection, so they
work without a proxy.

### Flags

| Flag | Meaning |
| --- | --- |
| `--username`, `--email`, `--phone` | Exactly one identifier (n070 oneof fields 1-3). |
| `--password-file` | Read the password from a file (`-` for stdin); `username:password` form also provides the username. |
| `--password` | Password inline (discouraged). |
| `--spec` | JSON request spec file (`-` for stdin). |
| `--num-attempts` | n070 field 5; omitted when 0. |
| `--password-source` | n070 field 9; omitted when 0. |
| `--whatsapp` | n070 field 11 value. |
| `--set-whatsapp` | Explicitly include n070 field 11. |
| `--fidelius-init-file` | Pre-encoded `v0u` bytes (hex/base64/raw). |
| `--cof-metadata-file` | Pre-encoded `c3e` bytes (hex/base64/raw). |
| `--prerequisites-file` | Pre-encoded `fs60` bytes (hex/base64/raw). |
| `--attestation-file` | Native attestation output, placed in `fs60` field 11. |
| `--user-agent` | HTTP/2 user agent (recovered Android format). |
| `--proxy` | SOCKS5 proxy address for **all** network calls (default `127.0.0.1:9150`; empty disables). |
| `--dump-request` | Print the request hex/base64 and exit (no network). |
| `--request-out` | Write request bytes to a file. |
| `--raw` | Print the raw response hex. |
| `--timeout` | RPC timeout (default 10s). |

### JSON spec

A `--spec` file assembles a full request from captured values:

```json
{
  "identifier_kind": "username",
  "identifier": "bridge-test-user",
  "password": "not-a-real-password",
  "num_attempts": 1,
  "password_source": 0,
  "whatsapp": false,

  "fidelius_init": "hex-or-base64 of pre-encoded v0u",
  "cof_metadata":  "hex-or-base64 of pre-encoded c3e",
  "prerequisites": "hex-or-base64 of pre-encoded fs60",
  "attestation":   "hex-or-base64 native attestation output",

  "fidelius": {
    "n1u_h": ["base64", "..."],
    "n1u_e": "base64",
    "r241_f": "base64",
    "r241_c": "base64",
    "r241_e": "base64",
    "r241_i": 0
  }
}
```

When `fidelius` is present it reproduces `vo3.apply` case 28: `v0u` field 1 is
the base64-decoded `n1u_h` entries, field 2 is the `k5u` built from the `r241`
values, and field 3 is `n1u_e`. `k5u` field 1 is `r241_f[26:91)` (the
`mor0.d`/`uh3.w1` byte range), fields 2-4 are `r241_c`, `r241_e`, and `r241_i`.

## Recovered wire format

Request `n070` → `/snapchat.janus.api.LoginService/LoginWithPassword`:

| Field | Type | Meaning |
| --- | --- | --- |
| 1 / 2 / 3 | string | username / email / phone identifier oneof |
| 4 | string | password |
| 5 | varint | attempt metadata (`numAttempts`) |
| 7 | message `v0u` | Fidelius initialization |
| 8 | message `c3e` | COF configuration-sync metadata |
| 9 | varint | password source |
| 10 | message | phone-context oneof alternative |
| 11 | bool | WhatsApp installed |
| 15 | message `fs60` | prerequisites (field 11 = native attestation) |

Response `o070`: field 1 = application status (1 = success), fields 2-13 =
status-specific oneof payload, field 14 = optional bytes. Status 1 payload is
`h18` → `b841` → `ffu0` → repeated `ivs0` scoped tokens.

### Native attestation request (`afx`)

The attestation boundary is `yblj.prh.a([B)[B]` in `libscplugin.so`; its input
is the serialized `afx` request (`ydx` in the 14.30.0.39 APK). The wire format
is implemented in [`internal/janus/attestation.go`](../../internal/janus/attestation.go):

| Field | Type | Meaning |
| --- | --- | --- |
| 1 | string | RPC path |
| 2 | string | (unknown) |
| 3 | varint | attestation type (`1` = LOGIN) |
| 4 | message `fg3` | metadata |
| 5 / 6 | bytes | (unknown) |
| 7 | varint | (unknown) |

`fg3` is `repeated kte` (field 1) plus varints (fields 2-5); `kte` is repeated
strings (fields 1-2) plus varints (fields 3-4). The `CommandAttestor` interface
is the Option-A oracle plumbing: it feeds the serialized `afx` to a command
(an on-device `prh.a` helper via `adb`) and returns the blob as stdout. The
field *values* the APK's builder fills in (path variant, metadata, extra
strings) are not yet recovered.

Provenance: [PASSWORD_LOGIN_FLOW.md](../../docs/development/PASSWORD_LOGIN_FLOW.md)
and [FINDINGS.md](../../docs/development/FINDINGS.md). The minimal
identifier/password fixtures match the APK `MessageNano` output byte-for-byte
(see `internal/janus/janus_test.go`).

## What is still unknown

- Fresh Fidelius key generation (`r241` creation) and the
  HMAC-SHA256-derived key semantics (`r241.a()` = HMAC-SHA256(`a`, `c`)).
- Native attestation input semantics and server acceptance of any output.
- Exact `c3e` setter encodings and `fs60` nested messages (`wgg0`, `q8p`).
- Token timing units, expiry rules, refresh, and challenge continuation RPCs.

Until those are recovered, a response with application status 1 must not be
persisted as a bridge session without successful `DecodeSuccess`.
