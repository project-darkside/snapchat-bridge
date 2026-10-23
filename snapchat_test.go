package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSnapchatClient_generateAuthToken(t *testing.T) {
	config := &Config{
		Snapchat: struct {
			BaseURL    string `yaml:"base_url"`
			UserAgent  string `yaml:"user_agent"`
			DeviceID   string `yaml:"device_id"`
			AppVersion string `yaml:"app_version"`
			ClientAuth string `yaml:"client_auth"`
			RateLimit  struct {
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

func TestRateLimiter_RefreshesAfterWait(t *testing.T) {
	rateLimiter := &RateLimiter{maxReqs: 1, window: 10 * time.Millisecond}
	if err := rateLimiter.Wait(); err != nil {
		t.Fatal(err)
	}
	firstRequest := rateLimiter.requests[0]
	if err := rateLimiter.Wait(); err != nil {
		t.Fatal(err)
	}
	if len(rateLimiter.requests) != 1 {
		t.Fatalf("Expected only the newly admitted request, got %d", len(rateLimiter.requests))
	}
	if rateLimiter.requests[0].Before(firstRequest.Add(rateLimiter.window)) {
		t.Error("The admitted request must be timestamped after the wait")
	}
}

func TestRateLimiter_InvalidLimits(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		maxReqs int
		window  time.Duration
	}{
		{name: "zero requests", maxReqs: 0, window: time.Second},
		{name: "negative requests", maxReqs: -1, window: time.Second},
		{name: "zero window", maxReqs: 1, window: 0},
		{name: "negative window", maxReqs: 1, window: -time.Second},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			rateLimiter := &RateLimiter{maxReqs: testCase.maxReqs, window: testCase.window}
			if err := rateLimiter.Wait(); err == nil {
				t.Error("Invalid limits must return an error")
			}
		})
	}
}

func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	const requestCount = 6
	rateLimiter := &RateLimiter{maxReqs: 1, window: 5 * time.Millisecond}
	start := make(chan struct{})
	results := make(chan error, requestCount)
	for range requestCount {
		go func() {
			<-start
			results <- rateLimiter.Wait()
		}()
	}
	startedAt := time.Now()
	close(start)
	for range requestCount {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if time.Since(startedAt) < time.Duration(requestCount-1)*rateLimiter.window {
		t.Error("Concurrent requests exceeded the configured admission rate")
	}
	if len(rateLimiter.requests) > rateLimiter.maxReqs {
		t.Error("The active request window exceeds the configured capacity")
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
			BaseURL    string `yaml:"base_url"`
			UserAgent  string `yaml:"user_agent"`
			DeviceID   string `yaml:"device_id"`
			AppVersion string `yaml:"app_version"`
			ClientAuth string `yaml:"client_auth"`
			RateLimit  struct {
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
	signer := newMockLoginSigner(t, server.URL+"/bq/login")
	client.picaboo.baseURL = signer.URL + "/"
	err := client.Login("testuser", "testpass")

	if err != nil {
		t.Fatalf("Login should succeed with mock server: %v", err)
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
			BaseURL    string `yaml:"base_url"`
			UserAgent  string `yaml:"user_agent"`
			DeviceID   string `yaml:"device_id"`
			AppVersion string `yaml:"app_version"`
			ClientAuth string `yaml:"client_auth"`
			RateLimit  struct {
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
	signer := newMockLoginSigner(t, server.URL+"/bq/login")
	client.picaboo.baseURL = signer.URL + "/"
	err := client.Login("wronguser", "wrongpass")

	if err == nil {
		t.Fatal("Login should fail with invalid credentials")
	}

	if !strings.Contains(err.Error(), "Invalid credentials") {
		t.Errorf("Error should mention invalid credentials, got: %v", err)
	}
}

func TestSnapchatClient_Login_RejectsInvalidResponses(t *testing.T) {
	const sensitiveBody = "response-body-secret"
	tests := []struct {
		name      string
		status    int
		body      string
		want      string
		truncated bool
	}{
		{"http failure", http.StatusUnauthorized, `{"status":"success","auth_token":"response-body-secret","user_id":"user","expires_in":3600}`, "HTTP status 401", false},
		{"invalid json", http.StatusOK, sensitiveBody, "failed to decode auth response", false},
		{"read failure", http.StatusOK, sensitiveBody, "failed to read auth response", true},
		{"missing token", http.StatusOK, `{"status":"success","user_id":"user","expires_in":3600}`, "incomplete session", false},
		{"missing user", http.StatusOK, `{"status":"success","auth_token":"response-body-secret","expires_in":3600}`, "incomplete session", false},
		{"missing expiry", http.StatusOK, `{"logged":true,"auth_token":"response-body-secret","user_id":"user"}`, "incomplete session", false},
		{"negative expiry", http.StatusOK, `{"status":"success","auth_token":"response-body-secret","user_id":"user","expires_in":-1}`, "incomplete session", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.truncated {
					w.Header().Set("Content-Length", "1024")
				}
				w.WriteHeader(test.status)
				w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)
			config := &Config{}
			config.Snapchat.RateLimit.Requests = 30
			config.Snapchat.RateLimit.Window = 60
			client := NewSnapchatClient(config)
			signer := newMockLoginSigner(t, server.URL)
			client.picaboo.baseURL = signer.URL + "/"
			var logs bytes.Buffer
			previousOutput := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(previousOutput) })

			err := client.Login("testuser", "testpass")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Expected %q error, got %v", test.want, err)
			}
			if client.sessionData != nil || client.authToken != "" || client.username != "" {
				t.Fatal("Invalid response must not establish a session")
			}
			if strings.Contains(err.Error(), sensitiveBody) || strings.Contains(logs.String(), sensitiveBody) {
				t.Fatal("Authentication response body was exposed in an error or log")
			}
		})
	}
}

func newMockLoginSigner(t *testing.T, targetURL string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/login" {
			http.NotFound(w, r)
			return
		}
		var request PicabooLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Invalid signer request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PicabooResponse{
			Seq:    request.Seq,
			URL:    targetURL,
			Params: map[string]interface{}{"username": request.Username, "password": request.Password},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestSnapchatClient_SendMessage_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loq/conversation_post_messages" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{
		Snapchat: struct {
			BaseURL    string `yaml:"base_url"`
			UserAgent  string `yaml:"user_agent"`
			DeviceID   string `yaml:"device_id"`
			AppVersion string `yaml:"app_version"`
			ClientAuth string `yaml:"client_auth"`
			RateLimit  struct {
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
		t.Fatalf("SendMessage should succeed: %v", err)
	}

	if message == nil {
		t.Fatal("Message should not be nil")
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
		now.Add(-2 * time.Hour),    // Too old
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
