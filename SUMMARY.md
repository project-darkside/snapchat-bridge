# Snapchat Bridge Progress Summary

Updated: 2026-10-22

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

## Next Steps

1. **Finish Fidelius key initialization.** Resume at `vo3.apply` case 28, which
   builds `a3u(v0u, r241)`. Recover fresh key generation, algorithms, encodings,
   and persistence requirements before choosing a crypto implementation.
  `r241.f()` returns stored bytes; `mor0.d()` copies the end-exclusive range
  `[26, 91)`, producing 65 bytes. The crypto algorithm and key encoding remain
  unverified. `p241.b(byte[])` parses existing keys; it does not generate them.
   The inspected `oz4.call` branch processes successful-login key material; it
   does not establish how fresh login keys are generated.
2. **Complete the request prerequisites.** Follow `xu60.a` and the `fs60`
   builder, including original native-provider inputs. Minimal isolated native
   output is not proof of a complete or server-accepted prerequisite. Verify
   the `c3e` setters before encoding the COF metadata from `mr60` / `b3e`.
3. **Decode usable sessions and challenges.** Follow `o070` success field 2
   (`h18`) through `rz4.c`, token storage, and Fidelius processing. Verify token
   scopes, expiry units, refresh rules, retries, and challenge continuations.
   An envelope with status 1 alone must never become a persisted session.
4. **Integrate modern authentication safely.** Wire the complete request,
   transport, and accepted-session handling into the active `bridgev2` login
   flow, replacing credential forwarding to the legacy signing service. Add
   local contract tests for persistence, restart, cancellation, and failures.
5. **Implement current messaging and lifecycle behavior.** Recover current
   conversation identifiers and send/receive APIs; connect Matrix portals,
   polling or streaming, cancellation, and durable message deduplication.
6. **Run account testing only after those prerequisites are implemented.**
   Supply credentials locally, never in chat or to an unverified third party.
   Validate Matrix-to-Snapchat delivery, a reply exactly once in the same Matrix
   room, and both directions after restarting the bridge.

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