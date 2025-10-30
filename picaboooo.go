package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	PICABOOOO_BASE_URL = "https://picaboooo.com/v1/"
)

// PicabooClient handles communication with the Picaboooo signing service
type PicabooClient struct {
	baseURL    string
	httpClient *http.Client
	ts         int64
	seq        int
}

// PicabooLoginRequest is the request to Picaboooo for login
type PicabooLoginRequest struct {
	Ts               int64  `json:"ts"`
	Seq              int    `json:"seq"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Dtoken1i         string `json:"dtoken1i"`
	Dtoken1v         string `json:"dtoken1v"`
	PreAuthToken     string `json:"pre_auth_token,omitempty"`
	OdlvPreAuthToken string `json:"odlv_pre_auth_token,omitempty"`
	OtpType          string `json:"otp_type,omitempty"`
}

// PicabooEndpointRequest is for getting signed headers for authenticated endpoints
type PicabooEndpointRequest struct {
	Ts        int64  `json:"ts"`
	Seq       int    `json:"seq"`
	Username  string `json:"username"`
	AuthToken string `json:"auth_token"`
	Endpoint  string `json:"endpoint"`
}

// PicabooResponse is the response from Picaboooo
type PicabooResponse struct {
	Seq     int                    `json:"seq"`
	URL     string                 `json:"url"`
	Headers map[string]string      `json:"headers"`
	Params  map[string]interface{} `json:"params"`
}

// NewPicabooClient creates a new Picaboooo client
func NewPicabooClient() *PicabooClient {
	return &PicabooClient{
		baseURL: PICABOOOO_BASE_URL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		ts:  time.Now().UnixMilli(),
		seq: 0,
	}
}

// GetAuthenticatedLogin gets signed request data for login from Picaboooo
func (p *PicabooClient) GetAuthenticatedLogin(username, password, dtoken1i, dtoken1v string) (*PicabooResponse, error) {
	p.seq++

	request := PicabooLoginRequest{
		Ts:       time.Now().UnixMilli(),
		Seq:      p.seq,
		Username: username,
		Password: password,
		Dtoken1i: dtoken1i,
		Dtoken1v: dtoken1v,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal picaboooo request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"login", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create picaboooo request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("picaboooo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read picaboooo response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("picaboooo returned status %d: %s", resp.StatusCode, string(body))
	}

	var picabooResp PicabooResponse
	if err := json.Unmarshal(body, &picabooResp); err != nil {
		return nil, fmt.Errorf("failed to parse picaboooo response: %w", err)
	}

	if picabooResp.Seq == 0 {
		return nil, fmt.Errorf("missing seq in picaboooo response")
	}

	p.seq = picabooResp.Seq

	return &picabooResp, nil
}

// GetAuthenticatedEndpoint gets signed request data for an authenticated endpoint
func (p *PicabooClient) GetAuthenticatedEndpoint(username, authToken, endpoint string) (*PicabooResponse, error) {
	p.seq++

	request := PicabooEndpointRequest{
		Ts:        time.Now().UnixMilli(),
		Seq:       p.seq,
		Username:  username,
		AuthToken: authToken,
		Endpoint:  endpoint,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal picaboooo request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"endpoint", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create picaboooo request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("picaboooo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read picaboooo response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("picaboooo returned status %d: %s", resp.StatusCode, string(body))
	}

	var picabooResp PicabooResponse
	if err := json.Unmarshal(body, &picabooResp); err != nil {
		return nil, fmt.Errorf("failed to parse picaboooo response: %w", err)
	}

	if picabooResp.Seq == 0 {
		return nil, fmt.Errorf("missing seq in picaboooo response")
	}

	p.seq = picabooResp.Seq

	return &picabooResp, nil
}
