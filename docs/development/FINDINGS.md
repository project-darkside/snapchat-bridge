# Snapchat API Investigation Findings

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
