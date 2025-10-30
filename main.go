package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	Name        = "snapchat-bridge"
	Description = "A Matrix-Snapchat bridge for Beeper"
	Version     = "1.0.0"
	Author      = "Beeper Community"
	License     = "MIT"
)

func main() {
	var configPath = flag.String("config", "config.yaml", "Path to configuration file")
	var generateConfig = flag.Bool("generate-config", false, "Generate example configuration")
	var version = flag.Bool("version", false, "Show version information")
	var help = flag.Bool("help", false, "Show help")

	flag.Parse()

	if *help {
		printHelp()
		return
	}

	if *version {
		printVersion()
		return
	}

	if *generateConfig {
		generateExampleConfig(*configPath)
		return
	}

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Fatalf("Configuration file %s does not exist. Use --generate-config to create an example configuration.", *configPath)
	}

	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	setupLogging(config)

	log.Printf("Starting %s v%s", Name, Version)
	log.Printf("Configuration loaded from: %s", *configPath)

	bridge, err := NewSimpleBridge(config)
	if err != nil {
		log.Fatalf("Failed to create bridge: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		err := bridge.Start()
		if err != nil {
			log.Fatalf("Failed to start bridge: %v", err)
		}
	}()

	go cleanupTask(bridge)

	log.Println("Bridge is running. Press Ctrl+C to stop.")

	<-c
	log.Println("Received interrupt signal, shutting down...")

	bridge.Stop()

	log.Println("Bridge stopped successfully")
}

func printHelp() {
	fmt.Printf(`%s - %s

Usage:
  %s [options]

Options:
  -config string
        Path to configuration file (default "config.yaml")
  -generate-config
        Generate example configuration file
  -version
        Show version information
  -help
        Show this help message

Examples:
  # Start the bridge with default config
  %s

  # Start with custom config location
  %s -config /etc/snapchat-bridge/config.yaml

  # Generate example configuration
  %s -generate-config

For more information, visit: https://github.com/beeper/snapchat-bridge
`, Name, Description, Name, Name, Name, Name)
}

func printVersion() {
	fmt.Printf(`%s v%s

Description: %s
Author: %s
License: %s

Built with Go %s
`, Name, Version, Description, Author, License, "1.21+")
}

func generateExampleConfig(path string) {
	config := GenerateExampleConfig()

	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create config file: %v", err)
	}
	defer file.Close()

	configYAML := generateYAMLConfig(config)

	_, err = file.WriteString(configYAML)
	if err != nil {
		log.Fatalf("Failed to write config file: %v", err)
	}

	fmt.Printf("Example configuration written to: %s\n", path)
	fmt.Println("\nIMPORTANT: Please edit the configuration file and:")
	fmt.Println("1. Set your Matrix homeserver details")
	fmt.Println("2. Configure proper tokens for the appservice")
	fmt.Println("3. Update database path if needed")
	fmt.Println("4. Register the appservice with your homeserver")
}

func generateYAMLConfig(config *Config) string {
	return fmt.Sprintf(`# Snapchat Bridge Configuration
# Generated example - please customize for your setup

# Matrix homeserver connection
homeserver:
  address: %s
  domain: %s

# Application service configuration
appservice:
  address: %s
  hostname: %s
  port: %d
  id: %s
  bot_token: %s
  hs_token: %s
  as_token: %s
  bot_localpart: %s

# Bridge configuration
bridge:
  username: %s
  displayname_template: "%s"
  command_prefix: %s
  max_connection_age: %d
  encryption:
    allow: %t
    default: %t
    require: %t

# Database configuration
database:
  type: %s
  uri: %s

# Logging configuration
logging:
  level: %s
  file: "%s"

# Snapchat-specific configuration
snapchat:
  base_url: %s
  user_agent: %s
  device_id: %s
  app_version: %s
  client_auth: "%s"
  rate_limit:
    requests: %d
    window: %d
`,
		config.Homeserver.Address, config.Homeserver.Domain,
		config.AppService.Address, config.AppService.Hostname, config.AppService.Port,
		config.AppService.ID, config.AppService.BotToken, config.AppService.HSToken,
		config.AppService.ASToken, config.AppService.BotLocalID,
		config.Bridge.Username, config.Bridge.DisplaynameTemplate, config.Bridge.CommandPrefix,
		config.Bridge.MaxConnectionAge,
		config.Bridge.Encryption.Allow, config.Bridge.Encryption.Default, config.Bridge.Encryption.Require,
		config.Database.Type, config.Database.URI,
		config.Logging.Level, config.Logging.File,
		config.Snapchat.BaseURL, config.Snapchat.UserAgent, config.Snapchat.DeviceID,
		config.Snapchat.AppVersion, config.Snapchat.ClientAuth,
		config.Snapchat.RateLimit.Requests, config.Snapchat.RateLimit.Window,
	)
}

func setupLogging(config *Config) {
	if config.Logging.File != "" {
		err := os.MkdirAll(filepath.Dir(config.Logging.File), 0755)
		if err != nil {
			log.Printf("Warning: Failed to create log directory: %v", err)
			return
		}

		file, err := os.OpenFile(config.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Printf("Warning: Failed to open log file: %v", err)
			return
		}

		log.SetOutput(file)
		log.Printf("Logging to file: %s", config.Logging.File)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func cleanupTask(bridge *SimpleBridge) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Running cleanup task...")

		cutoff := time.Now().Add(-24 * time.Hour)
		err := bridge.DB.DeleteEphemeralMessages(cutoff)
		if err != nil {
			log.Printf("Failed to cleanup ephemeral messages: %v", err)
		}

		log.Println("Cleanup task completed")
	}
}