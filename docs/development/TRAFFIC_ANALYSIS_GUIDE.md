# Snapchat Traffic Analysis Guide

## 🎯 Goal
Capture real Snapchat API traffic to reverse-engineer the authentication endpoints and request signing.

## 📱 Prerequisites
- ✅ mitmproxy installed (done!)
- iPhone or Android device with Snapchat installed
- Mac and phone on same Wi-Fi network
- Snapchat test account: `testthebridge`

## 🚀 Step-by-Step Instructions

### Step 1: Get Your Mac's IP Address

Run this command:
```bash
ifconfig | grep "inet " | grep -v 127.0.0.1
```

Look for something like: `inet 192.168.1.xxx`
**Write this down!** You'll need it for your phone.

### Step 2: Start mitmproxy

Open a terminal and run:
```bash
mitmproxy --set block_global=false
```

You'll see an interface that looks like a terminal app.
Leave this running!

### Step 3: Configure Your iPhone

1. **Open Settings** → **Wi-Fi**
2. Tap the **(i)** icon next to your connected Wi-Fi
3. Scroll down to **HTTP Proxy**
4. Select **Manual**
5. Enter:
   - **Server**: Your Mac's IP (from Step 1)
   - **Port**: `8080`
   - **Authentication**: Off
6. Tap **Save**

### Step 4: Install mitmproxy Certificate

This is CRITICAL for HTTPS interception:

1. **On iPhone, open Safari**
2. Go to: **http://mitm.it**
3. You'll see "If you can see this, traffic is not passing through mitmproxy"
   - That's OK! The certificate installer will still work
4. Tap **Get mitmproxy-ca-cert.pem** (iOS option)
5. Tap **Allow** when prompted
6. **Close Safari**
7. Go to **Settings** → **Profile Downloaded**
8. Tap **Install** (top right)
9. Enter your passcode
10. Tap **Install** again (2-3 times)
11. Tap **Done**

### Step 5: Trust the Certificate

1. **Settings** → **General** → **About**
2. Scroll to bottom → **Certificate Trust Settings**
3. Toggle ON: **mitmproxy**
4. Tap **Continue** on warning

### Step 6: Capture Snapchat Traffic! 🎯

Now the magic happens:

1. **On your Mac**, watch the mitmproxy terminal
2. **On iPhone**, open **Snapchat**
3. **If already logged in**, log out first:
   - Tap profile icon (top left)
   - Tap gear icon (top right)
   - Scroll down → Log Out
4. **Log back in** with: `testthebridge` / `Shitfuck31!`
5. **Watch mitmproxy** - you'll see LOTS of requests!

### Step 7: Find the Login Endpoint

In mitmproxy:

1. Look for requests to `app.snapchat.com` or similar
2. Look for POST requests (they'll have `POST` in the URL)
3. Look for endpoints like:
   - `/loq/login`
   - `/bq/login`
   - `/ph/login`
   - Or anything with "login", "auth", "session"

### Step 8: Examine Request Details

When you find a promising request:

1. Use **arrow keys** to select it
2. Press **Enter** to see details
3. Press **Tab** to switch between Request/Response
4. Look for:
   - **URL**: The full endpoint
   - **Headers**: Especially X-Snapchat-* headers
   - **Body**: The JSON payload
   - **Response**: What Snapchat returned

### Step 9: Save Everything!

For each important request:

1. Select it in mitmproxy
2. Press **e** (edit/view)
3. Press **W** (capital W) to save to file
4. Or just take screenshots/notes!

## 🔍 What to Look For

### Critical Information:

1. **Login Endpoint**:
   ```
   POST https://app.snapchat.com/????/login
   ```

2. **Request Headers**:
   ```
   X-Snapchat-Client-Auth-Token: ...
   X-Snapchat-UUID: ...
   X-Snapchat-Client-Version: ...
   User-Agent: ...
   ```

3. **Request Body**:
   ```json
   {
     "username": "...",
     "password": "...",
     "timestamp": ...,
     // etc
   }
   ```

4. **Response**:
   ```json
   {
     "auth_token": "...",
     "user_id": "...",
     // etc
   }
   ```

## 📊 After Capturing

Once you have the traffic captured:

1. Document the EXACT endpoint URL
2. Document ALL headers sent
3. Document the request body structure
4. Document any signing/hashing algorithms visible
5. We'll implement it in Go!

## 🐛 Troubleshooting

### "No traffic showing up"
- Make sure iPhone proxy is set correctly
- Verify Mac IP address is correct
- Check both devices on same Wi-Fi

### "SSL Certificate Error"
- Make sure you installed AND trusted the certificate
- Check Settings → General → About → Certificate Trust Settings

### "Can't load any apps"
- Proxy is working! But mitmproxy might not be running
- Restart mitmproxy

### "Snapchat says 'Connection Error'"
- This is good! It means Snapchat is detecting the proxy
- We need to see what it's trying to connect to
- Try opening another app first (Safari, Twitter) to verify proxy works

## 🎯 Alternative: mitmweb (Easier UI)

If you prefer a web interface:

```bash
mitmweb --set block_global=false
```

Then open: http://localhost:8081

This gives you a nice web UI to browse requests!

## ⏱️ Expected Timeline

- Setup (Steps 1-5): 15-20 minutes
- Capturing (Steps 6-8): 10-15 minutes
- Analysis (Step 9): 30-60 minutes
- Implementation: 1-2 hours

**Total: ~2-4 hours to working bridge!**

## 🆘 Need Help?

If anything doesn't work:
1. Take a screenshot
2. Share what error you're seeing
3. I'll help debug!

---

**Ready? Start with Step 1!** 🚀

Type "ready" when you've completed each step and I'll help you with the next one.
