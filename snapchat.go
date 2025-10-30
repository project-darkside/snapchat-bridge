package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

type SnapchatClient struct {
	config      *Config
	httpClient  *http.Client
	username    string
	authToken   string
	sessionData *SessionData
	deviceID    string
	rateLimiter *RateLimiter
}

type SessionData struct {
	Username    string `json:"username"`
	UserID      string `json:"user_id"`
	AuthToken   string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt   int64  `json:"expires_at"`
	DeviceID    string `json:"device_id"`
}

type RateLimiter struct {
	requests []time.Time
	maxReqs  int
	window   time.Duration
}

type SnapchatMessage struct {
	ID        string    `json:"id"`
	FromUser  string    `json:"from_user"`
	ToUser    string    `json:"to_user"`
	Content   string    `json:"content"`
	MediaURL  string    `json:"media_url,omitempty"`
	MediaType string    `json:"media_type,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Ephemeral bool      `json:"ephemeral"`
	ViewTime  int       `json:"view_time,omitempty"`
}

type SnapchatConversation struct {
	ID           string             `json:"id"`
	Participants []string           `json:"participants"`
	Messages     []*SnapchatMessage `json:"messages"`
	IsGroup      bool               `json:"is_group"`
	Name         string             `json:"name,omitempty"`
}

type AuthResponse struct {
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	ExpiresIn    int64  `json:"expires_in"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
}

type MessageResponse struct {
	Messages     []*SnapchatMessage `json:"messages"`
	Conversations []*SnapchatConversation `json:"conversations"`
	Status       string             `json:"status"`
	LastUpdate   int64              `json:"last_update"`
}

func NewSnapchatClient(config *Config) *SnapchatClient {
	return &SnapchatClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		deviceID: generateDeviceID(),
		rateLimiter: &RateLimiter{
			maxReqs: config.Snapchat.RateLimit.Requests,
			window:  time.Duration(config.Snapchat.RateLimit.Window) * time.Second,
		},
	}
}

func (c *SnapchatClient) Login(username, password string) error {
	if err := c.rateLimiter.Wait(); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	authToken := c.generateAuthToken(username, password)

	payload := map[string]interface{}{
		"username":     username,
		"password":     password,
		"device_id":    c.deviceID,
		"client_auth":  authToken,
		"app_version":  c.config.Snapchat.AppVersion,
		"timestamp":    time.Now().Unix(),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal auth payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.Snapchat.BaseURL+"/bq/login", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	c.setHeaders(req, authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.Status != "success" {
		return fmt.Errorf("authentication failed: %s", authResp.Message)
	}

	c.username = username
	c.authToken = authResp.AuthToken
	c.sessionData = &SessionData{
		Username:    username,
		UserID:      authResp.UserID,
		AuthToken:   authResp.AuthToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresAt:   time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second).Unix(),
		DeviceID:    c.deviceID,
	}

	log.Printf("Successfully authenticated user: %s", username)
	return nil
}

func (c *SnapchatClient) SendMessage(toUser, content string) (*SnapchatMessage, error) {
	if err := c.rateLimiter.Wait(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	messageID := generateMessageID()
	timestamp := time.Now()

	payload := map[string]interface{}{
		"id":         messageID,
		"to_user":    toUser,
		"content":    content,
		"timestamp":  timestamp.Unix(),
		"device_id":  c.deviceID,
		"auth_token": c.authToken,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.Snapchat.BaseURL+"/bq/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create send request: %w", err)
	}

	c.setHeaders(req, c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send failed with status %d: %s", resp.StatusCode, string(body))
	}

	message := &SnapchatMessage{
		ID:        messageID,
		FromUser:  c.username,
		ToUser:    toUser,
		Content:   content,
		Timestamp: timestamp,
		Ephemeral: false,
	}

	log.Printf("Sent message %s to %s", messageID, toUser)
	return message, nil
}

func (c *SnapchatClient) GetMessages() ([]*SnapchatMessage, error) {
	if err := c.rateLimiter.Wait(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	req, err := http.NewRequest("GET", c.config.Snapchat.BaseURL+"/bq/messages", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get messages request: %w", err)
	}

	q := req.URL.Query()
	q.Add("auth_token", c.authToken)
	q.Add("device_id", c.deviceID)
	q.Add("since", strconv.FormatInt(time.Now().Add(-24*time.Hour).Unix(), 10))
	req.URL.RawQuery = q.Encode()

	c.setHeaders(req, c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get messages request failed: %w", err)
	}
	defer resp.Body.Close()

	var msgResp MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, fmt.Errorf("failed to decode messages response: %w", err)
	}

	return msgResp.Messages, nil
}

func (c *SnapchatClient) SendSnap(toUser, mediaPath string, viewTime int) (*SnapchatMessage, error) {
	if err := c.rateLimiter.Wait(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	snapID := generateMessageID()
	timestamp := time.Now()

	payload := map[string]interface{}{
		"id":         snapID,
		"to_user":    toUser,
		"media_path": mediaPath,
		"view_time":  viewTime,
		"timestamp":  timestamp.Unix(),
		"device_id":  c.deviceID,
		"auth_token": c.authToken,
		"ephemeral":  true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal snap payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.Snapchat.BaseURL+"/bq/send_snap", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create send snap request: %w", err)
	}

	c.setHeaders(req, c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send snap request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send snap failed with status %d: %s", resp.StatusCode, string(body))
	}

	message := &SnapchatMessage{
		ID:        snapID,
		FromUser:  c.username,
		ToUser:    toUser,
		MediaURL:  mediaPath,
		MediaType: "image/jpeg",
		Timestamp: timestamp,
		Ephemeral: true,
		ViewTime:  viewTime,
	}

	log.Printf("Sent snap %s to %s", snapID, toUser)
	return message, nil
}

func (c *SnapchatClient) SetTyping(toUser string, typing bool) error {
	if err := c.rateLimiter.Wait(); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	payload := map[string]interface{}{
		"to_user":    toUser,
		"typing":     typing,
		"auth_token": c.authToken,
		"device_id":  c.deviceID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal typing payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.Snapchat.BaseURL+"/bq/typing", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create typing request: %w", err)
	}

	c.setHeaders(req, c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("typing request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (c *SnapchatClient) generateAuthToken(username, password string) string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()

	data := fmt.Sprintf("%s:%s:%s:%s:%s",
		username, password, c.deviceID, timestamp, nonce)

	hash := sha256.Sum256([]byte(data))

	obfuscated := c.obfuscateToken(hex.EncodeToString(hash[:]))

	return fmt.Sprintf("sc-auth-v1:%s:%s:%s", timestamp, nonce, obfuscated)
}

func (c *SnapchatClient) obfuscateToken(token string) string {
	xorKey := []byte("snapchat_secret_key_2024")
	result := make([]byte, len(token))

	for i := 0; i < len(token); i++ {
		result[i] = token[i] ^ xorKey[i%len(xorKey)]
	}

	return hex.EncodeToString(result)
}

func (c *SnapchatClient) setHeaders(req *http.Request, authToken string) {
	req.Header.Set("User-Agent", c.config.Snapchat.UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Snapchat-Client-Auth-Token", authToken)
	req.Header.Set("X-Snapchat-Client-Version", c.config.Snapchat.AppVersion)
	req.Header.Set("X-Snapchat-Device-ID", c.deviceID)
	req.Header.Set("X-Snapchat-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
}

func (rl *RateLimiter) Wait() error {
	now := time.Now()

	rl.requests = filterOldRequests(rl.requests, now.Add(-rl.window))

	if len(rl.requests) >= rl.maxReqs {
		oldestReq := rl.requests[0]
		waitTime := rl.window - now.Sub(oldestReq)
		if waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	rl.requests = append(rl.requests, now)
	return nil
}

func filterOldRequests(requests []time.Time, cutoff time.Time) []time.Time {
	var filtered []time.Time
	for _, req := range requests {
		if req.After(cutoff) {
			filtered = append(filtered, req)
		}
	}
	return filtered
}

func generateDeviceID() string {
	deviceTypes := []string{"iPhone14,2", "iPhone13,4", "iPhone12,3", "iPad13,1"}
	return deviceTypes[time.Now().Unix()%int64(len(deviceTypes))]
}

func generateMessageID() string {
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return fmt.Sprintf("%d_%x", timestamp, randomBytes)
}

func generateNonce() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func (c *SnapchatClient) Close() error {
	if c.sessionData != nil {
		log.Printf("Closing Snapchat session for user: %s", c.username)
	}
	return nil
}