package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSnapchatClient_generateAuthToken(t *testing.T) {
	config := &Config{
		Snapchat: struct {
			BaseURL     string `yaml:"base_url"`
			UserAgent   string `yaml:"user_agent"`
			DeviceID    string `yaml:"device_id"`
			AppVersion  string `yaml:"app_version"`
			ClientAuth  string `yaml:"client_auth"`
			RateLimit   struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			} `yaml:"rate_limit"`
		}{
			AppVersion: "12.51.0.35",
			RateLimit: struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			}{
				Requests: 30,
				Window:   60,
			},
		},
	}

	client := NewSnapchatClient(config)
	token := client.generateAuthToken("testuser", "testpass")

	if token == "" {
		t.Error("Generated token should not be empty")
	}

	if !strings.HasPrefix(token, "sc-auth-v1:") {
		t.Error("Token should start with sc-auth-v1: prefix")
	}

	parts := strings.Split(token, ":")
	if len(parts) != 4 {
		t.Errorf("Token should have 4 parts separated by colons, got %d", len(parts))
	}
}

func TestSnapchatClient_generateDeviceID(t *testing.T) {
	deviceID := generateDeviceID()

	if deviceID == "" {
		t.Error("Device ID should not be empty")
	}

	validDevices := []string{"iPhone14,2", "iPhone13,4", "iPhone12,3", "iPad13,1"}
	found := false
	for _, valid := range validDevices {
		if deviceID == valid {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Device ID should be one of the valid devices, got: %s", deviceID)
	}
}

func TestSnapchatClient_generateMessageID(t *testing.T) {
	id1 := generateMessageID()
	id2 := generateMessageID()

	if id1 == "" || id2 == "" {
		t.Error("Message IDs should not be empty")
	}

	if id1 == id2 {
		t.Error("Message IDs should be unique")
	}

	if !strings.Contains(id1, "_") {
		t.Error("Message ID should contain underscore separator")
	}
}

func TestSnapchatClient_obfuscateToken(t *testing.T) {
	config := &Config{}
	client := NewSnapchatClient(config)

	testToken := "test123456"
	obfuscated := client.obfuscateToken(testToken)

	if obfuscated == "" {
		t.Error("Obfuscated token should not be empty")
	}

	if obfuscated == testToken {
		t.Error("Obfuscated token should be different from original")
	}

	if len(obfuscated) == 0 {
		t.Error("Obfuscated token should have content")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rateLimiter := &RateLimiter{
		maxReqs: 2,
		window:  time.Second,
	}

	start := time.Now()

	err1 := rateLimiter.Wait()
	if err1 != nil {
		t.Errorf("First request should not be rate limited: %v", err1)
	}

	err2 := rateLimiter.Wait()
	if err2 != nil {
		t.Errorf("Second request should not be rate limited: %v", err2)
	}

	err3 := rateLimiter.Wait()
	if err3 != nil {
		t.Errorf("Third request should be delayed but not error: %v", err3)
	}

	elapsed := time.Since(start)
	if elapsed < 800*time.Millisecond {
		t.Error("Rate limiter should have introduced delay")
	}
}

func TestSnapchatClient_Login_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bq/login" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "success",
				"auth_token": "mock_auth_token",
				"refresh_token": "mock_refresh_token",
				"user_id": "mock_user_id",
				"expires_in": 3600
			}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{
		Snapchat: struct {
			BaseURL     string `yaml:"base_url"`
			UserAgent   string `yaml:"user_agent"`
			DeviceID    string `yaml:"device_id"`
			AppVersion  string `yaml:"app_version"`
			ClientAuth  string `yaml:"client_auth"`
			RateLimit   struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			} `yaml:"rate_limit"`
		}{
			BaseURL:    server.URL,
			UserAgent:  "TestAgent",
			AppVersion: "12.51.0.35",
			RateLimit: struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			}{
				Requests: 30,
				Window:   60,
			},
		},
	}

	client := NewSnapchatClient(config)
	err := client.Login("testuser", "testpass")

	if err != nil {
		t.Errorf("Login should succeed with mock server: %v", err)
	}

	if client.username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", client.username)
	}

	if client.authToken != "mock_auth_token" {
		t.Errorf("Expected auth token 'mock_auth_token', got '%s'", client.authToken)
	}
}

func TestSnapchatClient_Login_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bq/login" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "error",
				"message": "Invalid credentials"
			}`))
		}
	}))
	defer server.Close()

	config := &Config{
		Snapchat: struct {
			BaseURL     string `yaml:"base_url"`
			UserAgent   string `yaml:"user_agent"`
			DeviceID    string `yaml:"device_id"`
			AppVersion  string `yaml:"app_version"`
			ClientAuth  string `yaml:"client_auth"`
			RateLimit   struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			} `yaml:"rate_limit"`
		}{
			BaseURL:    server.URL,
			UserAgent:  "TestAgent",
			AppVersion: "12.51.0.35",
			RateLimit: struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			}{
				Requests: 30,
				Window:   60,
			},
		},
	}

	client := NewSnapchatClient(config)
	err := client.Login("wronguser", "wrongpass")

	if err == nil {
		t.Error("Login should fail with invalid credentials")
	}

	if !strings.Contains(err.Error(), "Invalid credentials") {
		t.Errorf("Error should mention invalid credentials, got: %v", err)
	}
}

func TestSnapchatClient_SendMessage_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bq/send" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{
		Snapchat: struct {
			BaseURL     string `yaml:"base_url"`
			UserAgent   string `yaml:"user_agent"`
			DeviceID    string `yaml:"device_id"`
			AppVersion  string `yaml:"app_version"`
			ClientAuth  string `yaml:"client_auth"`
			RateLimit   struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			} `yaml:"rate_limit"`
		}{
			BaseURL:    server.URL,
			UserAgent:  "TestAgent",
			AppVersion: "12.51.0.35",
			RateLimit: struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			}{
				Requests: 30,
				Window:   60,
			},
		},
	}

	client := NewSnapchatClient(config)
	client.username = "testuser"
	client.authToken = "test_token"

	message, err := client.SendMessage("recipient", "Hello World!")

	if err != nil {
		t.Errorf("SendMessage should succeed: %v", err)
	}

	if message == nil {
		t.Error("Message should not be nil")
	}

	if message.Content != "Hello World!" {
		t.Errorf("Expected content 'Hello World!', got '%s'", message.Content)
	}

	if message.FromUser != "testuser" {
		t.Errorf("Expected from_user 'testuser', got '%s'", message.FromUser)
	}

	if message.ToUser != "recipient" {
		t.Errorf("Expected to_user 'recipient', got '%s'", message.ToUser)
	}
}

func TestFilterOldRequests(t *testing.T) {
	now := time.Now()
	cutoff := now.Add(-time.Hour)

	requests := []time.Time{
		now.Add(-2 * time.Hour), // Too old
		now.Add(-30 * time.Minute), // Keep
		now.Add(-10 * time.Minute), // Keep
	}

	filtered := filterOldRequests(requests, cutoff)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 requests after filtering, got %d", len(filtered))
	}

	for _, req := range filtered {
		if req.Before(cutoff) {
			t.Error("Filtered requests should all be after cutoff")
		}
	}
}