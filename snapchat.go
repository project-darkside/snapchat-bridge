package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Static token used when no AuthToken is available
	STATIC_TOKEN = "m198sOkJEn37DjqZ32lpRu76xmw288xSQ9"
)

// SignedRequest contains signed headers and parameters for API requests
type SignedRequest struct {
	Headers map[string]string      `json:"headers"`
	Params  map[string]interface{} `json:"params"`
	Seq     int                    `json:"seq"`
}

type SnapchatClient struct {
	config      *Config
	httpClient  *http.Client
	picaboo     *PicabooClient // Picaboooo signing service client
	username    string
	authToken   string
	sessionData *SessionData
	deviceID    string
	dtoken1i    string // Device Token Identifier
	dtoken1v    string // Device Token Verifier
	rateLimiter *RateLimiter
	seq         int // Sequence number for request signing
}

type SessionData struct {
	Username     string `json:"username"`
	UserID       string `json:"user_id"`
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	DeviceID     string `json:"device_id"`
}

type RateLimiter struct {
	mutex    sync.Mutex
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
	Logged       bool   `json:"logged"`
}

type MessageResponse struct {
	Messages      []*SnapchatMessage      `json:"messages"`
	Conversations []*SnapchatConversation `json:"conversations"`
	Status        string                  `json:"status"`
	LastUpdate    int64                   `json:"last_update"`
}

func NewSnapchatClient(config *Config) *SnapchatClient {
	client := &SnapchatClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		picaboo:  NewPicabooClient(), // Initialize Picaboooo client
		deviceID: generateDeviceID(),
		rateLimiter: &RateLimiter{
			maxReqs: config.Snapchat.RateLimit.Requests,
			window:  time.Duration(config.Snapchat.RateLimit.Window) * time.Second,
		},
	}

	// Generate device tokens
	client.dtoken1i, client.dtoken1v = generateDeviceTokens()

	return client
}

func (c *SnapchatClient) Login(username, password string) error {
	if err := c.rateLimiter.Wait(); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	picabooResp, err := c.picaboo.GetAuthenticatedLogin(username, password, c.dtoken1i, c.dtoken1v)
	if err != nil {
		return fmt.Errorf("legacy signing service authentication failed: %w", err)
	}

	// Prepare payload with Picaboooo params
	payload := make(map[string]interface{})
	for k, v := range picabooResp.Params {
		payload[k] = v
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal auth payload: %w", err)
	}

	// Use the URL provided by Picaboooo
	req, err := http.NewRequest("POST", picabooResp.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	// Set headers from Picaboooo
	for k, v := range picabooResp.Headers {
		req.Header.Set(k, v)
	}

	// Also set our base headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("authentication failed with HTTP status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	var authResp AuthResponse
	if err := json.Unmarshal(bodyBytes, &authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.Status != "success" && !authResp.Logged {
		return fmt.Errorf("authentication failed: %s (status: %s)", authResp.Message, authResp.Status)
	}
	if authResp.AuthToken == "" || authResp.UserID == "" || authResp.ExpiresIn <= 0 {
		return fmt.Errorf("authentication response contains an incomplete session")
	}

	c.username = username
	c.authToken = authResp.AuthToken
	c.sessionData = &SessionData{
		Username:     username,
		UserID:       authResp.UserID,
		AuthToken:    authResp.AuthToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second).Unix(),
		DeviceID:     c.deviceID,
	}

	log.Printf("✅ Successfully authenticated user: %s", username)
	return nil
}

func (c *SnapchatClient) SendMessage(toUser, content string) (*SnapchatMessage, error) {
	if err := c.rateLimiter.Wait(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Generate UUIDs for message identification
	messageID := generateMessageID()
	chatMessageID := generateMessageID()
	timestamp := time.Now().UnixMilli() // Milliseconds timestamp

	// Generate conversation ID (alphabetically sorted usernames joined by ~)
	conversationID := c.getConversationID(c.username, toUser)

	// Build chat message structure matching PHP SDK
	chatMessage := map[string]interface{}{
		"type":            "chat_message",
		"id":              messageID,
		"chat_message_id": chatMessageID,
		"seq_num":         1, // TODO: Track actual sequence numbers per conversation
		"timestamp":       timestamp,
		"header": map[string]interface{}{
			"from":    c.username,
			"to":      []string{toUser},
			"conv_id": conversationID,
		},
		"body": map[string]interface{}{
			"type": "text",
			"text": content,
		},
	}

	// Wrap in messages array as expected by the API
	payload := map[string]interface{}{
		"messages": []interface{}{chatMessage},
	}

	// Get signed request
	signed, err := c.getSignedRequest(c.username, c.authToken, "/loq/conversation_post_messages")
	if err != nil {
		return nil, fmt.Errorf("failed to generate signed request: %w", err)
	}

	// Merge signed params into payload
	for k, v := range signed.Params {
		payload[k] = v
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.Snapchat.BaseURL+"/loq/conversation_post_messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create send request: %w", err)
	}

	c.setHeaders(req, c.authToken)

	// Add signed headers
	for k, v := range signed.Headers {
		req.Header.Set(k, v)
	}

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
		Timestamp: time.UnixMilli(timestamp),
		Ephemeral: false,
	}

	log.Printf("Sent message %s to %s in conversation %s", messageID, toUser, conversationID)
	return message, nil
}

// getConversationID generates a conversation ID from two usernames
// Usernames are sorted alphabetically and joined with ~
func (c *SnapchatClient) getConversationID(user1, user2 string) string {
	users := []string{user1, user2}
	sort.Strings(users)
	return strings.Join(users, "~")
}

func (c *SnapchatClient) GetMessages() ([]*SnapchatMessage, error) {
	if err := c.rateLimiter.Wait(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	timestamp := time.Now().Unix()

	payload := map[string]interface{}{
		"checksums_dict": "",
		"features_map":   "{}",
		"ts":             timestamp * 1000,
		"seq":            1,
	}

	// Get signed request
	signed, err := c.getSignedRequest(c.username, c.authToken, "/loq/conversations")
	if err != nil {
		return nil, fmt.Errorf("failed to generate signed request: %w", err)
	}

	// Merge signed params into payload
	for k, v := range signed.Params {
		payload[k] = v
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conversations payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.Snapchat.BaseURL+"/loq/conversations", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create get conversations request: %w", err)
	}

	c.setHeaders(req, c.authToken)

	// Add signed headers
	for k, v := range signed.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get conversations request failed: %w", err)
	}
	defer resp.Body.Close()

	var msgResp MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, fmt.Errorf("failed to decode conversations response: %w", err)
	}

	return msgResp.Messages, nil
}

func (c *SnapchatClient) SendSnap(toUser, mediaPath string, viewTime int) (*SnapchatMessage, error) {
	if err := c.rateLimiter.Wait(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Read media file
	mediaData, err := os.ReadFile(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read media file: %w", err)
	}

	// Encrypt media
	encryptor, err := NewMediaEncryption()
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	encryptedData, metadata, err := encryptor.EncryptFile(mediaData)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt media: %w", err)
	}

	snapID := generateMessageID()
	timestamp := time.Now()

	payload := map[string]interface{}{
		"id":             snapID,
		"to_user":        toUser,
		"media_data":     base64.StdEncoding.EncodeToString(encryptedData),
		"encryption_key": metadata["encryption_key"],
		"encryption_iv":  metadata["encryption_iv"],
		"hash":           metadata["hash"],
		"view_time":      viewTime,
		"timestamp":      timestamp.Unix(),
		"device_id":      c.deviceID,
		"auth_token":     c.authToken,
		"ephemeral":      true,
		"media_type":     "image/jpeg",
	}

	// Get signed request
	signed, err := c.getSignedRequest(c.username, c.authToken, "/loq/send")
	if err != nil {
		return nil, fmt.Errorf("failed to generate signed request: %w", err)
	}

	// Merge signed params into payload
	for k, v := range signed.Params {
		payload[k] = v
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal snap payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.Snapchat.BaseURL+"/loq/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create send snap request: %w", err)
	}

	c.setHeaders(req, c.authToken)

	// Add signed headers
	for k, v := range signed.Headers {
		req.Header.Set(k, v)
	}

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

	log.Printf("Sent snap %s to %s (encrypted, %d bytes)", snapID, toUser, len(encryptedData))
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
	// If we already have an auth token, use it
	if c.authToken != "" {
		return c.authToken
	}

	// Otherwise, use the static token as a base
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()

	// Combine with static token for initial auth
	data := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		STATIC_TOKEN, username, password, c.deviceID, timestamp, nonce)

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
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	if rl.maxReqs <= 0 || rl.window <= 0 {
		return fmt.Errorf("invalid rate limit: requests and window must be positive")
	}

	for {
		now := time.Now()
		rl.requests = filterOldRequests(rl.requests, now.Add(-rl.window))
		if len(rl.requests) < rl.maxReqs {
			rl.requests = append(rl.requests, now)
			return nil
		}
		time.Sleep(time.Until(rl.requests[0].Add(rl.window)))
	}
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

func generateDeviceTokens() (string, string) {
	// Generate device token identifier (dtoken1i)
	dtoken1i := make([]byte, 20)
	rand.Read(dtoken1i)
	dtoken1iStr := hex.EncodeToString(dtoken1i)

	// Generate device token verifier (dtoken1v)
	dtoken1v := make([]byte, 32)
	rand.Read(dtoken1v)
	dtoken1vStr := hex.EncodeToString(dtoken1v)

	return dtoken1iStr, dtoken1vStr
}

func (c *SnapchatClient) generateDeviceSignature(username, password string, timestamp int64, requestToken string) string {
	// Device signature generation based on PHP SDK logic
	data := fmt.Sprintf("%s|%s|%d|%s", username, password, timestamp, requestToken)

	// HMAC-SHA256 with device token verifier as key
	h := hmac.New(sha256.New, []byte(c.dtoken1v))
	h.Write([]byte(data))

	// Return first 20 characters of the hash
	hash := hex.EncodeToString(h.Sum(nil))
	if len(hash) > 20 {
		return hash[:20]
	}
	return hash
}

// getSignedRequest generates signed headers and params for API requests
// This implements local request signing without external dependencies
func (c *SnapchatClient) getSignedRequest(username, authToken, endpoint string) (*SignedRequest, error) {
	timestamp := time.Now().UnixMilli() // Milliseconds

	// Increment sequence number
	c.seq++

	// Generate request token for this specific request
	reqToken := generateNonce()

	// Build base signature data
	sigData := fmt.Sprintf("%s|%s|%d|%d|%s", username, authToken, timestamp, c.seq, endpoint)

	// Sign with device token verifier
	h := hmac.New(sha256.New, []byte(c.dtoken1v))
	h.Write([]byte(sigData))
	signature := hex.EncodeToString(h.Sum(nil))[:20]

	// Generate additional auth headers
	headers := map[string]string{
		"X-Snapchat-Client-Auth":      fmt.Sprintf("Bearer %s", authToken),
		"X-Snapchat-Client-Auth-Type": "Bearer",
		"X-Snapchat-UUID":             c.dtoken1i,
		"X-Snapchat-Signature":        signature,
		"X-Snapchat-Request-Token":    reqToken,
	}

	// Generate additional params to add to request body
	params := map[string]interface{}{
		"req_token": reqToken,
		"timestamp": timestamp,
		"seq_num":   c.seq,
	}

	return &SignedRequest{
		Headers: headers,
		Params:  params,
		Seq:     c.seq,
	}, nil
}

func (c *SnapchatClient) Close() error {
	if c.sessionData != nil {
		log.Printf("Closing Snapchat session for user: %s", c.username)
	}
	return nil
}
