//go:build test_login
// +build test_login

package main

import (
	"fmt"
	"log"
	"os"
)

// Simple standalone test program to verify Snapchat login works
// Build with: go build -tags test_login -o test_login test_login.go snapchat.go config.go
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Get credentials from environment or args
	username := os.Getenv("SNAPCHAT_USERNAME")
	password := os.Getenv("SNAPCHAT_PASSWORD")

	if username == "" && len(os.Args) > 1 {
		username = os.Args[1]
	}
	if password == "" && len(os.Args) > 2 {
		password = os.Args[2]
	}

	if username == "" || password == "" {
		fmt.Println("Usage: ./test_login <username> <password>")
		fmt.Println("Or set SNAPCHAT_USERNAME and SNAPCHAT_PASSWORD environment variables")
		os.Exit(1)
	}

	// Create a test config
	config := &Config{}
	config.Snapchat.BaseURL = "https://app.snapchat.com"
	config.Snapchat.UserAgent = "Snapchat/11.45.0.33 CFNetwork/1240.0.4 Darwin/20.6.0"
	config.Snapchat.DeviceID = "iPhone13,4"
	config.Snapchat.AppVersion = "11.45.0.33"
	config.Snapchat.ClientAuth = "your_client_auth_token_here"
	config.Snapchat.RateLimit.Requests = 10
	config.Snapchat.RateLimit.Window = 60

	fmt.Println("🚀 Testing Snapchat Bridge Login")
	fmt.Println("================================")
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Base URL: %s\n", config.Snapchat.BaseURL)
	fmt.Println()

	// Create client
	fmt.Println("📱 Creating Snapchat client...")
	client := NewSnapchatClient(config)
	fmt.Printf("   Device ID: %s\n", client.deviceID)
	fmt.Printf("   Device Token Identifier: %s\n", client.dtoken1i)
	fmt.Printf("   Device Token Verifier: %s\n", client.dtoken1v)
	fmt.Println()

	// Attempt login
	fmt.Println("🔐 Attempting login...")
	err := client.Login(username, password)
	if err != nil {
		fmt.Printf("❌ Login failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Login successful!")
	fmt.Println()
	fmt.Println("📊 Session Data:")
	fmt.Printf("   Username: %s\n", client.sessionData.Username)
	fmt.Printf("   User ID: %s\n", client.sessionData.UserID)
	fmt.Printf("   Auth Token: %s...\n", client.sessionData.AuthToken[:20])
	fmt.Printf("   Device ID: %s\n", client.sessionData.DeviceID)
	fmt.Println()

	// Test getting messages
	fmt.Println("💬 Testing message retrieval...")
	messages, err := client.GetMessages()
	if err != nil {
		fmt.Printf("⚠️  Failed to get messages: %v\n", err)
	} else {
		fmt.Printf("✅ Retrieved %d messages\n", len(messages))
		for i, msg := range messages {
			if i < 5 { // Show first 5 messages
				fmt.Printf("   %d. From: %s, Content: %s\n", i+1, msg.FromUser, msg.Content)
			}
		}
	}
	fmt.Println()

	fmt.Println("🎉 All tests passed!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Try sending a test message")
	fmt.Println("  2. Check if messages appear in the Snapchat app")
	fmt.Println("  3. Send a message from Snapchat and check if bridge receives it")
}
