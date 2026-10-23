# Testing Guide - Snapchat Bridge

Current Snapchat authentication and message compatibility are not verified.
The unit tests use local fixtures, not Snapchat's live service. Do not submit
real credentials to the legacy signing service on the strength of these tests.
See [current APK findings](development/FINDINGS.md) for the protocol gaps.

### Current Janus Coverage

`janus_test.go` checks minimal password-request bytes against three synthetic
requests serialized and round-tripped by the installed Android 14.29.0.50 APK.
It also checks invalid request inputs and response-envelope handling: malformed
fields, missing success payloads, unknown statuses, and protobuf oneof ordering.
Run the focused tests with:

```bash
CGO_ENABLED=1 go test -mod=readonly -tags goolm -race -run '^TestJanus' .
```

These tests do not cover a complete device prerequisite, accepted session,
live login, or Snapchat messaging. The new encoder and envelope parser are not
connected to the legacy login path. Do not use the account login instructions
below as evidence that the current bridge is ready for credentials.

## Test Environment Setup

### 1. Create Test Snapchat Account

⚠️ **Important**: Use a throwaway account for testing, not your personal account.

1. Go to https://accounts.snapchat.com/signup
2. Create new account with fake/temp credentials
3. Complete any verification steps
4. Note down username and password

### 2. Setup Test Matrix Account

1. Create test account on your Matrix homeserver
2. Use a client like Element to verify it works
3. Note down username and password

## Unit Tests

### Run Encryption Tests

```bash
cd /Users/minibrain/Desktop/snapchat-bridge

# Run all crypto tests
go test -tags nocrypto -v crypto_test.go crypto.go

# Expected output:
# === RUN   TestMediaEncryption
#     crypto_test.go:40: ✅ Encryption/decryption test passed!
# --- PASS: TestMediaEncryption (0.00s)
# === RUN   TestEncryptFile
#     crypto_test.go:86: ✅ File encryption test passed! Encrypted 10256 bytes
# --- PASS: TestEncryptFile (0.00s)
# === RUN   TestPKCS7Padding
#     crypto_test.go:124: ✅ PKCS7 padding test passed!
# --- PASS: TestPKCS7Padding (0.00s)
# PASS
```

### Run All Tests

```bash
CGO_ENABLED=1 go test -mod=readonly -tags goolm -v ./...
```

The `goolm` tag selects mautrix's pure-Go Matrix encryption implementation,
avoiding the external `libolm` headers. SQLite still needs CGO and a C compiler.
On Windows without a suitable compiler, run this from the repository root in
PowerShell with Docker Desktop using Linux containers:

```powershell
docker run --rm `
    --mount "type=bind,source=$((Get-Location).Path),target=/src,readonly" `
    --mount type=volume,source=snapchat-bridge-gomod,target=/go/pkg/mod `
    --mount type=volume,source=snapchat-bridge-gocache,target=/root/.cache/go-build `
    --workdir /src --env CGO_ENABLED=1 golang:1.24-bookworm `
    go test -mod=readonly -tags goolm ./...
```

## Integration Tests

### Test 1: Bridge Startup

**Objective**: Verify bridge starts correctly

```bash
./snapchat-bridge -config config.yaml
```

**Expected**:
- ✅ No error messages
- ✅ "Starting Snapchat bridge" message
- ✅ "Listening on 0.0.0.0:29318" message
- ✅ No crashes

**Debug**:
- Check config.yaml syntax
- Verify tokens match in registration.yaml
- Check port 29318 is available

### Test 2: Homeserver Registration

**Objective**: Verify Matrix homeserver recognizes the bridge

1. Check homeserver logs for appservice registration
2. Verify no errors related to snapchat bridge

**Expected**:
- ✅ Homeserver logs show appservice loaded
- ✅ No token mismatch errors

**Debug**:
- Restart homeserver after adding registration
- Check registration.yaml path in homeserver config
- Verify tokens match exactly

### Test 3: Bot Accessibility

**Objective**: Verify bridge bot is accessible

1. Open Matrix client (Element)
2. Start chat with `@snapchatbot:your-domain.com`
3. Send any message

**Expected**:
- ✅ Bot responds or shows as online
- ✅ No "User not found" errors

**Debug**:
- Check bridge is running
- Verify bot localpart matches registration
- Check homeserver domain matches config

### Test 4: Authentication Flow

**Objective**: Test Snapchat login

1. Message bridge bot: `!sc login test-username test-password`
2. Wait for response

**Expected Scenarios**:

**Success**:
```
✅ Successfully logged in as test-username
Your conversations will sync shortly
```

**Wrong Credentials**:
```
❌ Authentication failed: Invalid username or password
```

**API Error**:
```
❌ Auth request failed: <error details>
```

**Debug**:
- Check bridge logs for detailed errors
- Verify Snapchat credentials
- Check rate limiting isn't triggered
- Review request signing implementation

### Test 5: Message Sending (Matrix → Snapchat)

**Objective**: Send message from Matrix to Snapchat

**Setup**:
1. Login to bridge (Test 4)
2. Create DM with a Snapchat contact
3. Send test message

**Expected**:
- ✅ Message sends without error
- ✅ Bridge logs show message sent
- ✅ Message appears in Snapchat app

**Debug**:
- Check conversation ID generation
- Verify signed request generation
- Check rate limiting
- Review send message payload

### Test 6: Message Receiving (Snapchat → Matrix)

**Objective**: Receive message from Snapchat in Matrix

**Setup**:
1. Send message from Snapchat app to bridge user
2. Wait for sync

**Expected**:
- ✅ Message appears in Matrix room
- ✅ Sender information correct
- ✅ Timestamp accurate

**Debug**:
- Check polling interval
- Verify session is active
- Check message parsing
- Review sync logic

### Test 7: Session Persistence

**Objective**: Verify session survives restart

1. Login to bridge
2. Stop bridge: `Ctrl+C`
3. Restart bridge
4. Try sending message

**Expected**:
- ✅ No need to login again
- ✅ Messages work immediately
- ✅ Session data restored

**Debug**:
- Check database file exists
- Verify session data structure
- Review persistence code

### Test 8: Rate Limiting

**Objective**: Verify rate limiting works

1. Send many messages quickly (>10 in 60 seconds)
2. Observe behavior

**Expected**:
- ✅ Messages throttled after limit
- ✅ No bridge crash
- ✅ Log shows rate limit applied

**Debug**:
- Check rate limit config
- Review rate limiter implementation
- Adjust limits if needed

### Test 9: Error Handling

**Objective**: Verify graceful error handling

**Test cases**:

1. **Invalid Snapchat username**
   ```
   !sc login invalid-user wrong-pass
   ```
   Expected: Clear error message, no crash

2. **Network failure**
   - Disconnect internet
   - Try sending message
   Expected: Error message, retry logic

3. **Invalid command**
   ```
   !sc invalidcommand
   ```
   Expected: Help message or "Unknown command"

### Test 10: Media Encryption (Unit Test)

**Objective**: Verify encryption works correctly

Already covered by crypto_test.go, but manual verification:

```go
// Create test file
echo "Test data" > test.jpg

// Try encrypting
encryptor, _ := NewMediaEncryption()
data, _ := ioutil.ReadFile("test.jpg")
encrypted, metadata, _ := encryptor.EncryptFile(data)

// Verify
- encrypted != data ✅
- metadata has key, iv, hash ✅
- decryption returns original ✅
```

## API Testing

### Test Snapchat API Endpoints

Use verbose logging to verify:

1. **Login Endpoint** (`/bq/login`)
   - Request has proper headers ✅
   - Request has signed parameters ✅
   - Response parsed correctly ✅

2. **Send Message** (`/loq/conversation_post_messages`)
   - Message format correct ✅
   - Headers signed ✅
   - Response successful ✅

3. **Get Messages** (`/loq/conversations`)
   - Request signed ✅
   - Response parsed ✅
   - Messages extracted ✅

## Performance Testing

### Load Test

**Objective**: Verify bridge handles multiple users

1. Create multiple Matrix accounts
2. Login each to bridge
3. Send messages simultaneously

**Metrics**:
- Response time
- Memory usage
- CPU usage
- Error rate

**Tools**:
```bash
# Monitor resources
top -p $(pgrep snapchat-bridge)

# Check memory
ps aux | grep snapchat-bridge
```

## Debugging Tools

### Enable Debug Logging

In config.yaml:
```yaml
logging:
  level: debug
  file: bridge.log
```

### Check Logs

```bash
# Tail bridge logs
tail -f bridge.log

# Filter for errors
grep ERROR bridge.log

# Filter for specific user
grep "username" bridge.log
```

### Network Inspection

```bash
# Capture traffic (requires mitmproxy)
mitmproxy -p 8080

# Configure bridge to use proxy
export HTTP_PROXY=http://localhost:8080
export HTTPS_PROXY=http://localhost:8080
./snapchat-bridge -config config.yaml
```

## Test Results Template

```markdown
# Test Results - [Date]

## Environment
- Bridge Version: [version]
- Go Version: [go version]
- OS: [macOS/Linux/etc]
- Matrix Server: [Synapse/Dendrite]

## Test Results

| Test | Status | Notes |
|------|--------|-------|
| Build | ✅/❌ | |
| Startup | ✅/❌ | |
| Bot Accessible | ✅/❌ | |
| Login | ✅/❌ | |
| Send Message | ✅/❌ | |
| Receive Message | ✅/❌ | |
| Session Persistence | ✅/❌ | |
| Rate Limiting | ✅/❌ | |
| Error Handling | ✅/❌ | |

## Issues Found
1. [Description]
2. [Description]

## Performance
- Memory Usage: [XX MB]
- CPU Usage: [XX%]
- Response Time: [XX ms]

## Recommendations
- [ ] [Action item]
- [ ] [Action item]
```

## Automated Testing Script

```bash
#!/bin/bash
# test.sh - Automated bridge testing

echo "🧪 Snapchat Bridge Test Suite"
echo "================================"

# Test 1: Build
echo "1. Testing build..."
go build -tags nocrypto -o snapchat-bridge
if [ $? -eq 0 ]; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi

# Test 2: Unit tests
echo "2. Running unit tests..."
go test -tags nocrypto -v crypto_test.go crypto.go
if [ $? -eq 0 ]; then
    echo "✅ Unit tests passed"
else
    echo "❌ Unit tests failed"
    exit 1
fi

# Test 3: Config generation
echo "3. Testing config generation..."
./snapchat-bridge -generate-config > /dev/null 2>&1
if [ -f config.yaml ]; then
    echo "✅ Config generated"
else
    echo "❌ Config generation failed"
    exit 1
fi

echo ""
echo "================================"
echo "✅ All automated tests passed!"
echo ""
echo "Next: Run manual integration tests"
```

Make executable and run:
```bash
chmod +x test.sh
./test.sh
```

## Checklist for Production

Before deploying to production:

- [ ] All unit tests pass
- [ ] Authentication works with real account
- [ ] Can send messages
- [ ] Can receive messages
- [ ] Session persists across restarts
- [ ] Rate limiting works
- [ ] Error handling is graceful
- [ ] Logs are clean (no spam)
- [ ] Memory usage is stable
- [ ] No memory leaks
- [ ] Security review complete
- [ ] Documentation is accurate
- [ ] Backup strategy in place

## Next Steps

1. Run all tests in order
2. Document any failures
3. Fix issues found
4. Repeat until all tests pass
5. Deploy to production
6. Monitor in production
7. Create demo video

---

**Good luck with testing!** 🎉
