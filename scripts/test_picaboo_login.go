//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"
)

// Test login with Picaboooo integration
func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: test_picaboo_login <username> <password>")
		os.Exit(1)
	}

	username := os.Args[1]
	password := os.Args[2]

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🚀 Testing Snapchat Login via Picaboooo")
	fmt.Println("========================================")
	fmt.Printf("Username: %s\n", username)
	fmt.Println()

	// Create config
	config := &Config{}
	config.Snapchat.BaseURL = "https://app.snapchat.com"
	config.Snapchat.UserAgent = "Snapchat/11.45.0.33 CFNetwork/1240.0.4 Darwin/20.6.0"
	config.Snapchat.DeviceID = "iPhone13,4"
	config.Snapchat.AppVersion = "11.45.0.33"
	config.Snapchat.RateLimit.Requests = 10
	config.Snapchat.RateLimit.Window = 60

	// Create client
	fmt.Println("📱 Creating Snapchat client with Picaboooo...")
	client := NewSnapchatClient(config)
	fmt.Printf("   Device Token ID: %s...\n", client.dtoken1i[:20])
	fmt.Printf("   Device Token Verifier: %s...\n", client.dtoken1v[:20])
	fmt.Println()

	// Attempt login
	fmt.Println("🔐 Attempting login...")
	err := client.Login(username, password)
	if err != nil {
		fmt.Printf("\n❌ Login failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✅ Login successful!")
	fmt.Println()
	fmt.Println("📊 Session Data:")
	fmt.Printf("   Username: %s\n", client.sessionData.Username)
	fmt.Printf("   User ID: %s\n", client.sessionData.UserID)
	if len(client.sessionData.AuthToken) > 20 {
		fmt.Printf("   Auth Token: %s...\n", client.sessionData.AuthToken[:20])
	} else {
		fmt.Printf("   Auth Token: %s\n", client.sessionData.AuthToken)
	}
	fmt.Println()

	fmt.Println("🎉 Success! The bridge is working!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Test sending a message")
	fmt.Println("  2. Test receiving messages")
	fmt.Println("  3. Integrate with Matrix bridge")
}
