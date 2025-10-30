#!/bin/bash

# Quick test script for Snapchat Bridge login
# Usage: ./test_snapchat_login.sh [username] [password]

USERNAME="${1:-testthebridge}"
PASSWORD="${2}"

if [ -z "$PASSWORD" ]; then
    echo "❌ Error: Password required"
    echo "Usage: $0 [username] [password]"
    echo "Example: $0 testthebridge mypassword123"
    exit 1
fi

echo "🚀 Testing Snapchat Bridge Login"
echo "================================"
echo "Username: $USERNAME"
echo ""

# Create a simple Go test program
cat > /tmp/test_snap_login.go << 'EOTEST'
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const STATIC_TOKEN = "m198sOkJEn37DjqZ32lpRu76xmw288xSQ9"

type AuthResponse struct {
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	ExpiresIn    int64  `json:"expires_in"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
	Logged       bool   `json:"logged"`
}

func generateDeviceTokens() (string, string) {
	dtoken1i := make([]byte, 32)
	dtoken1v := make([]byte, 32)
	rand.Read(dtoken1i)
	rand.Read(dtoken1v)
	return hex.EncodeToString(dtoken1i), hex.EncodeToString(dtoken1v)
}

func generateDeviceSignature(username, password string, timestamp int64, requestToken, dtoken1v string) string {
	data := fmt.Sprintf("%s|%s|%d|%s", username, password, timestamp, requestToken)
	h := hmac.New(sha256.New, []byte(dtoken1v))
	h.Write([]byte(data))
	hash := hex.EncodeToString(h.Sum(nil))
	return hash[:20]
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: test_snap_login username password")
		os.Exit(1)
	}

	username := os.Args[1]
	password := os.Args[2]

	dtoken1i, dtoken1v := generateDeviceTokens()
	timestamp := time.Now().Unix()
	requestToken := ""

	dsig := generateDeviceSignature(username, password, timestamp, requestToken, dtoken1v)

	fmt.Println("📱 Generated Device Tokens:")
	fmt.Printf("   dtoken1i: %s\n", dtoken1i[:20]+"...")
	fmt.Printf("   dtoken1v: %s\n", dtoken1v[:20]+"...")
	fmt.Printf("   dsig: %s\n", dsig)
	fmt.Println()

	payload := map[string]interface{}{
		"username":  username,
		"password":  password,
		"dtoken1i":  dtoken1i,
		"dtoken1v":  dtoken1v,
		"dsig":      dsig,
		"timestamp": timestamp,
		"ts":        timestamp * 1000,
		"seq":       1,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("❌ Failed to marshal payload: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🔐 Attempting login to Snapchat API...")
	fmt.Printf("   Endpoint: https://app.snapchat.com/bq/login\n")
	fmt.Printf("   Payload: %s\n", string(jsonData))
	fmt.Println()

	req, err := http.NewRequest("POST", "https://app.snapchat.com/bq/login", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ Failed to create request: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("User-Agent", "Snapchat/11.45.0.33 CFNetwork/1240.0.4 Darwin/20.6.0")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("X-Snapchat-Client-Version", "11.45.0.33")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	fmt.Printf("📥 Response Status: %d\n", resp.StatusCode)
	fmt.Printf("📥 Response Headers:\n")
	for k, v := range resp.Header {
		fmt.Printf("   %s: %v\n", k, v)
	}
	fmt.Println()
	fmt.Printf("📥 Response Body:\n%s\n", string(body))
	fmt.Println()

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		fmt.Printf("⚠️  Warning: Could not parse response as JSON: %v\n", err)
		os.Exit(1)
	}

	if authResp.Status == "success" || authResp.Logged {
		fmt.Println("✅ Login successful!")
		fmt.Printf("   User ID: %s\n", authResp.UserID)
		if authResp.AuthToken != "" {
			fmt.Printf("   Auth Token: %s...\n", authResp.AuthToken[:20])
		}
	} else {
		fmt.Printf("❌ Login failed: %s\n", authResp.Message)
		os.Exit(1)
	}
}
EOTEST

echo "📦 Compiling test program..."
cd /tmp && go build -o test_snap_login test_snap_login.go

if [ $? -ne 0 ]; then
    echo "❌ Failed to compile test program"
    exit 1
fi

echo ""
./test_snap_login "$USERNAME" "$PASSWORD"
