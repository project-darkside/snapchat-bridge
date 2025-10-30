package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
homeserver:
  address: https://matrix.example.com
  domain: example.com

appservice:
  address: http://localhost:29318
  hostname: 0.0.0.0
  port: 29318
  id: snapchat
  bot_token: test_bot_token
  hs_token: test_hs_token
  as_token: test_as_token
  bot_localpart: snapchatbot

bridge:
  username: snapchat
  displayname_template: "{{.Username}} (Snapchat)"
  command_prefix: "!sc"
  max_connection_age: 3600
  encryption:
    allow: true
    default: false
    require: false

database:
  type: sqlite3
  uri: test.db

logging:
  level: info
  file: ""

snapchat:
  base_url: https://app.snapchat.com
  user_agent: "TestAgent"
  device_id: "iPhone14,2"
  app_version: "12.51.0.35"
  client_auth: ""
  rate_limit:
    requests: 30
    window: 60
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Homeserver.Address != "https://matrix.example.com" {
		t.Errorf("Expected homeserver address 'https://matrix.example.com', got '%s'", config.Homeserver.Address)
	}

	if config.AppService.Port != 29318 {
		t.Errorf("Expected port 29318, got %d", config.AppService.Port)
	}

	if config.Bridge.CommandPrefix != "!sc" {
		t.Errorf("Expected command prefix '!sc', got '%s'", config.Bridge.CommandPrefix)
	}

	if config.Snapchat.RateLimit.Requests != 30 {
		t.Errorf("Expected rate limit requests 30, got %d", config.Snapchat.RateLimit.Requests)
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := LoadConfig("nonexistent.yaml")
	if err == nil {
		t.Error("Expected error when loading nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `
homeserver:
  address: https://matrix.example.com
  invalid_yaml: [
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err = LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error when loading invalid YAML")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorText   string
	}{
		{
			name: "valid config",
			config: &Config{
				Homeserver: struct {
					Address string `yaml:"address"`
					Domain  string `yaml:"domain"`
				}{
					Address: "https://matrix.example.com",
					Domain:  "example.com",
				},
				AppService: struct {
					Address    string `yaml:"address"`
					Hostname   string `yaml:"hostname"`
					Port       int    `yaml:"port"`
					ID         string `yaml:"id"`
					BotToken   string `yaml:"bot_token"`
					HSToken    string `yaml:"hs_token"`
					ASToken    string `yaml:"as_token"`
					BotLocalID string `yaml:"bot_localpart"`
				}{
					ID:       "snapchat",
					BotToken: "token",
					HSToken:  "token",
					ASToken:  "token",
				},
				Database: struct {
					Type string `yaml:"type"`
					URI  string `yaml:"uri"`
				}{
					URI: "test.db",
				},
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
				}{},
			},
			expectError: false,
		},
		{
			name: "missing homeserver address",
			config: &Config{
				Homeserver: struct {
					Address string `yaml:"address"`
					Domain  string `yaml:"domain"`
				}{
					Address: "",
				},
			},
			expectError: true,
			errorText:   "homeserver address is required",
		},
		{
			name: "missing appservice ID",
			config: &Config{
				Homeserver: struct {
					Address string `yaml:"address"`
					Domain  string `yaml:"domain"`
				}{
					Address: "https://matrix.example.com",
				},
				AppService: struct {
					Address    string `yaml:"address"`
					Hostname   string `yaml:"hostname"`
					Port       int    `yaml:"port"`
					ID         string `yaml:"id"`
					BotToken   string `yaml:"bot_token"`
					HSToken    string `yaml:"hs_token"`
					ASToken    string `yaml:"as_token"`
					BotLocalID string `yaml:"bot_localpart"`
				}{
					ID: "",
				},
			},
			expectError: true,
			errorText:   "appservice ID is required",
		},
		{
			name: "missing database URI",
			config: &Config{
				Homeserver: struct {
					Address string `yaml:"address"`
					Domain  string `yaml:"domain"`
				}{
					Address: "https://matrix.example.com",
				},
				AppService: struct {
					Address    string `yaml:"address"`
					Hostname   string `yaml:"hostname"`
					Port       int    `yaml:"port"`
					ID         string `yaml:"id"`
					BotToken   string `yaml:"bot_token"`
					HSToken    string `yaml:"hs_token"`
					ASToken    string `yaml:"as_token"`
					BotLocalID string `yaml:"bot_localpart"`
				}{
					ID:       "snapchat",
					BotToken: "token",
					HSToken:  "token",
					ASToken:  "token",
				},
				Database: struct {
					Type string `yaml:"type"`
					URI  string `yaml:"uri"`
				}{
					URI: "",
				},
			},
			expectError: true,
			errorText:   "database URI is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
			if tt.expectError && err != nil && tt.errorText != "" {
				if err.Error() != tt.errorText {
					t.Errorf("Expected error text '%s', got '%s'", tt.errorText, err.Error())
				}
			}
		})
	}
}

func TestConfig_Validate_DefaultValues(t *testing.T) {
	config := &Config{
		Homeserver: struct {
			Address string `yaml:"address"`
			Domain  string `yaml:"domain"`
		}{
			Address: "https://matrix.example.com",
		},
		AppService: struct {
			Address    string `yaml:"address"`
			Hostname   string `yaml:"hostname"`
			Port       int    `yaml:"port"`
			ID         string `yaml:"id"`
			BotToken   string `yaml:"bot_token"`
			HSToken    string `yaml:"hs_token"`
			ASToken    string `yaml:"as_token"`
			BotLocalID string `yaml:"bot_localpart"`
		}{
			ID:       "snapchat",
			BotToken: "token",
			HSToken:  "token",
			ASToken:  "token",
		},
		Database: struct {
			Type string `yaml:"type"`
			URI  string `yaml:"uri"`
		}{
			URI: "test.db",
		},
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
			BaseURL:   "",
			UserAgent: "",
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Validation should succeed and set defaults: %v", err)
	}

	if config.Snapchat.BaseURL != "https://app.snapchat.com" {
		t.Errorf("Expected default base URL, got '%s'", config.Snapchat.BaseURL)
	}

	if config.Snapchat.UserAgent == "" {
		t.Error("Expected default user agent to be set")
	}
}

func TestGenerateExampleConfig(t *testing.T) {
	config := GenerateExampleConfig()

	if config == nil {
		t.Fatal("Generated config should not be nil")
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Generated config should be valid: %v", err)
	}

	if config.Homeserver.Address == "" {
		t.Error("Generated config should have homeserver address")
	}

	if config.AppService.ID == "" {
		t.Error("Generated config should have appservice ID")
	}

	if config.Database.URI == "" {
		t.Error("Generated config should have database URI")
	}
}