package main

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/util/configupgrade"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
)

type SnapchatConnector struct {
	br     *bridgev2.Bridge
	Config SnapchatConfig
	DB     *Database
}

// SnapchatConfig contains the Snapchat-specific configuration
type SnapchatConfig struct {
	BaseURL   string `yaml:"base_url"`
	UserAgent string `yaml:"user_agent"`
	DeviceID  string `yaml:"device_id"`
	AppVersion string `yaml:"app_version"`
	ClientAuth string `yaml:"client_auth"`
	RateLimit struct {
		Requests int `yaml:"requests"`
		Window   int `yaml:"window"`
	} `yaml:"rate_limit"`
}

var _ bridgev2.NetworkConnector = (*SnapchatConnector)(nil)

// Init initializes the connector with the bridge instance
func (sc *SnapchatConnector) Init(bridge *bridgev2.Bridge) {
	sc.br = bridge
}

// Start is called when the bridge starts
func (sc *SnapchatConnector) Start(ctx context.Context) error {
	// No special startup operations needed for Snapchat
	return nil
}

// GetCapabilities returns the bridge-wide capabilities
func (sc *SnapchatConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{
		DisappearingMessages: true,
	}
}

// GetBridgeInfoVersion returns version numbers for bridge info
func (sc *SnapchatConnector) GetBridgeInfoVersion() (info, capabilities int) {
	return 1, 1
}

// GetName returns the network name and identifiers
func (sc *SnapchatConnector) GetName() bridgev2.BridgeName {
	return bridgev2.BridgeName{
		DisplayName:      "Snapchat",
		NetworkURL:       "https://snapchat.com",
		NetworkIcon:      "mxc://snapchat.com/icon", // TODO: Replace with actual icon
		NetworkID:        "snapchat",
		BeeperBridgeType: "github.com/beeper/snapchat-bridge",
		DefaultPort:      29320,
	}
}

// GetConfig returns configuration information
func (sc *SnapchatConnector) GetConfig() (example string, data any, upgrader configupgrade.Upgrader) {
	return ExampleConfig, &sc.Config, configupgrade.SimpleUpgrader(upgradeConfig)
}

func upgradeConfig(helper configupgrade.Helper) {
	helper.Copy(configupgrade.Str, "base_url")
	helper.Copy(configupgrade.Str, "user_agent")
	helper.Copy(configupgrade.Str, "device_id")
	helper.Copy(configupgrade.Str, "app_version")
	helper.Copy(configupgrade.Str, "client_auth")
	helper.Copy(configupgrade.Int, "rate_limit", "requests")
	helper.Copy(configupgrade.Int, "rate_limit", "window")
}

const ExampleConfig = `# Snapchat-specific configuration
base_url: "https://app.snapchat.com"
user_agent: "Snapchat/11.45.0.33 CFNetwork/1240.0.4 Darwin/20.6.0"
device_id: "iPhone13,4"
app_version: "11.45.0.33"
client_auth: "your_client_auth_token_here"
rate_limit:
  requests: 10
  window: 60`

// GetDBMetaTypes returns database metadata types
func (sc *SnapchatConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Portal:   nil,
		Ghost:    nil,
		Message:  nil,
		Reaction: nil,
		UserLogin: func() any {
			return &UserLoginMetadata{}
		},
	}
}

// UserLoginMetadata stores Snapchat-specific login data
type UserLoginMetadata struct {
	Username     string `json:"username"`
	UserID       string `json:"user_id"`
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
	ExpiresAt    int64  `json:"expires_at"`
}

// LoadUserLogin initializes a user login with the Snapchat client
func (sc *SnapchatConnector) LoadUserLogin(ctx context.Context, login *bridgev2.UserLogin) error {
	meta := login.Metadata.(*UserLoginMetadata)

	// Create a simplified config for the SnapchatClient
	oldConfig := &Config{}
	oldConfig.Snapchat.BaseURL = sc.Config.BaseURL
	oldConfig.Snapchat.UserAgent = sc.Config.UserAgent
	oldConfig.Snapchat.DeviceID = sc.Config.DeviceID
	oldConfig.Snapchat.AppVersion = sc.Config.AppVersion
	oldConfig.Snapchat.ClientAuth = sc.Config.ClientAuth
	oldConfig.Snapchat.RateLimit.Requests = sc.Config.RateLimit.Requests
	oldConfig.Snapchat.RateLimit.Window = sc.Config.RateLimit.Window

	client := NewSnapchatClient(oldConfig)
	client.username = meta.Username
	client.authToken = meta.AuthToken
	client.deviceID = meta.DeviceID
	client.sessionData = &SessionData{
		Username:     meta.Username,
		UserID:       meta.UserID,
		AuthToken:    meta.AuthToken,
		RefreshToken: meta.RefreshToken,
		ExpiresAt:    meta.ExpiresAt,
		DeviceID:     meta.DeviceID,
	}

	login.Client = &SnapchatAPI{
		UserLogin: login,
		Client:    client,
	}

	return nil
}

// GetLoginFlows returns available login methods
func (sc *SnapchatConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return []bridgev2.LoginFlow{{
		Name:        "Username and Password",
		Description: "Log in with your Snapchat username and password",
		ID:          "username-password",
	}}
}

// CreateLogin creates a new login process
func (sc *SnapchatConnector) CreateLogin(ctx context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	if flowID != "username-password" {
		return nil, fmt.Errorf("unknown login flow ID")
	}
	return &SnapchatLogin{
		User:      user,
		Connector: sc,
	}, nil
}

// SnapchatAPI implements bridgev2.NetworkAPI
type SnapchatAPI struct {
	UserLogin *bridgev2.UserLogin
	Client    *SnapchatClient
}

var _ bridgev2.NetworkAPI = (*SnapchatAPI)(nil)

// Connect establishes connection to Snapchat
func (sa *SnapchatAPI) Connect(ctx context.Context) {
	// For Snapchat, we don't maintain persistent connections
	// We just validate the session is still valid
	// This would typically check if auth token is still valid
}

// Disconnect tears down the connection
func (sa *SnapchatAPI) Disconnect() {
	if sa.Client != nil {
		sa.Client.Close()
	}
}

// IsLoggedIn checks if the login is still valid
func (sa *SnapchatAPI) IsLoggedIn() bool {
	if sa.Client == nil || sa.Client.sessionData == nil {
		return false
	}
	// Check if session hasn't expired
	return sa.Client.sessionData.ExpiresAt > time.Now().Unix()
}

// LogoutRemote logs out from Snapchat
func (sa *SnapchatAPI) LogoutRemote(ctx context.Context) {
	// Snapchat doesn't have explicit logout, just invalidate local session
	if sa.Client != nil {
		sa.Client.sessionData = nil
		sa.Client.authToken = ""
	}
}

// GetCapabilities returns user/portal specific capabilities
func (sa *SnapchatAPI) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *event.RoomFeatures {
	return &event.RoomFeatures{
		MaxTextLength: 5000, // Snapchat message limit
	}
}

// IsThisUser checks if a user ID belongs to this login
func (sa *SnapchatAPI) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	meta := sa.UserLogin.Metadata.(*UserLoginMetadata)
	return makeUserID(meta.Username) == userID
}

// GetChatInfo returns information about a chat/portal
func (sa *SnapchatAPI) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	meta := sa.UserLogin.Metadata.(*UserLoginMetadata)

	return &bridgev2.ChatInfo{
		Members: &bridgev2.ChatMemberList{
			IsFull: true,
			Members: []bridgev2.ChatMember{
				{
					EventSender: bridgev2.EventSender{
						IsFromMe: true,
						Sender:   makeUserID(meta.Username),
					},
					Membership: event.MembershipJoin,
					PowerLevel: &[]int{50}[0],
				},
				{
					EventSender: bridgev2.EventSender{
						Sender: networkid.UserID(portal.ID),
					},
					Membership: event.MembershipJoin,
					PowerLevel: &[]int{50}[0],
				},
			},
		},
	}, nil
}

// GetUserInfo returns information about a user
func (sa *SnapchatAPI) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	return &bridgev2.UserInfo{
		Identifiers: []string{fmt.Sprintf("snapchat:%s", ghost.ID)},
		Name:        &[]string{string(ghost.ID)}[0],
	}, nil
}

// Helper functions for network identifiers
func makeUserID(username string) networkid.UserID {
	return networkid.UserID(username)
}

func makePortalID(username string) networkid.PortalID {
	return networkid.PortalID(username)
}

func makeUserLoginID(username string) networkid.UserLoginID {
	return networkid.UserLoginID(username)
}

func makeMessageID(snapchatID string) networkid.MessageID {
	return networkid.MessageID(snapchatID)
}