package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Homeserver struct {
		Address string `yaml:"address"`
		Domain  string `yaml:"domain"`
	} `yaml:"homeserver"`

	AppService struct {
		Address    string `yaml:"address"`
		Hostname   string `yaml:"hostname"`
		Port       int    `yaml:"port"`
		ID         string `yaml:"id"`
		BotToken   string `yaml:"bot_token"`
		HSToken    string `yaml:"hs_token"`
		ASToken    string `yaml:"as_token"`
		BotLocalID string `yaml:"bot_localpart"`
	} `yaml:"appservice"`

	Bridge struct {
		Username         string `yaml:"username"`
		DisplaynameTemplate string `yaml:"displayname_template"`
		CommandPrefix    string `yaml:"command_prefix"`
		MaxConnectionAge int    `yaml:"max_connection_age"`
		Encryption       struct {
			Allow   bool `yaml:"allow"`
			Default bool `yaml:"default"`
			Require bool `yaml:"require"`
		} `yaml:"encryption"`
	} `yaml:"bridge"`

	Database struct {
		Type string `yaml:"type"`
		URI  string `yaml:"uri"`
	} `yaml:"database"`

	Logging struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"logging"`

	Snapchat struct {
		BaseURL     string `yaml:"base_url"`
		UserAgent   string `yaml:"user_agent"`
		DeviceID    string `yaml:"device_id"`
		AppVersion  string `yaml:"app_version"`
		ClientAuth  string `yaml:"client_auth"`
		RateLimit   struct {
			Requests int `yaml:"requests"`
			Window   int `yaml:"window"`
		} `yaml:"rate_limit"`
	} `yaml:"snapchat"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
	if c.Homeserver.Address == "" {
		return fmt.Errorf("homeserver address is required")
	}
	if c.AppService.ID == "" {
		return fmt.Errorf("appservice ID is required")
	}
	if c.AppService.BotToken == "" {
		return fmt.Errorf("bot token is required")
	}
	if c.AppService.HSToken == "" {
		return fmt.Errorf("homeserver token is required")
	}
	if c.AppService.ASToken == "" {
		return fmt.Errorf("appservice token is required")
	}
	if c.Database.URI == "" {
		return fmt.Errorf("database URI is required")
	}
	if c.Snapchat.BaseURL == "" {
		c.Snapchat.BaseURL = "https://app.snapchat.com"
	}
	if c.Snapchat.UserAgent == "" {
		c.Snapchat.UserAgent = "Snapchat/12.51.0.35 (iPhone14,2; iOS 16.6; en_US) #4104006074"
	}
	return nil
}

func GenerateExampleConfig() *Config {
	return &Config{
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
			Address:    "http://localhost:29318",
			Hostname:   "0.0.0.0",
			Port:       29318,
			ID:         "snapchat",
			BotToken:   "MDAxOGxvY2F0aW9uIG1hdC5leGFtcGxlLmNvbQowMDEzaWRlbnRpZmllciBrZXkKMDAyZmNpZCBnZW4gPSAxCjAwMmRjaWQgdXNlcl9pZCA9IEBzbmFwY2hhdGJvdDpleGFtcGxlLmNvbQowMDE2Y2lkIHR5cGUgPSBhY2Nlc3MKMDAyMWNpZCBub25jZSA9IGFhYmJjY2RkZWVmZmdnCjAwMmZzaWduYXR1cmUg",
			HSToken:    "MDAxOGxvY2F0aW9uIG1hdC5leGFtcGxlLmNvbQowMDEzaWRlbnRpZmllciBrZXkKMDAyZmNpZCBnZW4gPSAxCjAwMmRjaWQgdXNlcl9pZCA9IEBzbmFwY2hhdGJvdDpleGFtcGxlLmNvbQowMDE2Y2lkIHR5cGUgPSBhY2Nlc3MKMDAyMWNpZCBub25jZSA9IGFhYmJjY2RkZWVmZmdnCjAwMmZzaWduYXR1cmUg",
			ASToken:    "MDAxOGxvY2F0aW9uIG1hdC5leGFtcGxlLmNvbQowMDEzaWRlbnRpZmllciBrZXkKMDAyZmNpZCBnZW4gPSAxCjAwMmRjaWQgdXNlcl9pZCA9IEBzbmFwY2hhdGJvdDpleGFtcGxlLmNvbQowMDE2Y2lkIHR5cGUgPSBhY2Nlc3MKMDAyMWNpZCBub25jZSA9IGFhYmJjY2RkZWVmZmdnCjAwMmZzaWduYXR1cmUg",
			BotLocalID: "snapchatbot",
		},
		Bridge: struct {
			Username         string `yaml:"username"`
			DisplaynameTemplate string `yaml:"displayname_template"`
			CommandPrefix    string `yaml:"command_prefix"`
			MaxConnectionAge int    `yaml:"max_connection_age"`
			Encryption       struct {
				Allow   bool `yaml:"allow"`
				Default bool `yaml:"default"`
				Require bool `yaml:"require"`
			} `yaml:"encryption"`
		}{
			Username:         "snapchat",
			DisplaynameTemplate: "{{.Username}} (Snapchat)",
			CommandPrefix:    "!sc",
			MaxConnectionAge: 3600,
			Encryption: struct {
				Allow   bool `yaml:"allow"`
				Default bool `yaml:"default"`
				Require bool `yaml:"require"`
			}{
				Allow:   true,
				Default: false,
				Require: false,
			},
		},
		Database: struct {
			Type string `yaml:"type"`
			URI  string `yaml:"uri"`
		}{
			Type: "sqlite3",
			URI:  "snapchat.db",
		},
		Logging: struct {
			Level string `yaml:"level"`
			File  string `yaml:"file"`
		}{
			Level: "info",
			File:  "",
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
			BaseURL:    "https://app.snapchat.com",
			UserAgent:  "Snapchat/12.51.0.35 (iPhone14,2; iOS 16.6; en_US) #4104006074",
			DeviceID:   "iPhone14,2",
			AppVersion: "12.51.0.35",
			ClientAuth: "",
			RateLimit: struct {
				Requests int `yaml:"requests"`
				Window   int `yaml:"window"`
			}{
				Requests: 30,
				Window:   60,
			},
		},
	}
}