package main

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
)

// SnapchatLogin implements bridgev2.LoginProcess
type SnapchatLogin struct {
	User      *bridgev2.User
	Connector *SnapchatConnector
	Client    *SnapchatClient
	Username  string
	Password  string
}

var _ bridgev2.LoginProcess = (*SnapchatLogin)(nil)
var _ bridgev2.LoginProcessUserInput = (*SnapchatLogin)(nil)

// Start returns the first step of the login process
func (sl *SnapchatLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       "com.beeper.snapchat.enter_credentials",
		Instructions: "Enter your Snapchat credentials to log in",
		UserInputParams: &bridgev2.LoginUserInputParams{
			Fields: []bridgev2.LoginInputDataField{
				{
					Type:        bridgev2.LoginInputFieldTypeUsername,
					ID:          "username",
					Name:        "Snapchat Username",
					Description: "Your Snapchat username or email",
				},
				{
					Type:        bridgev2.LoginInputFieldTypePassword,
					ID:          "password",
					Name:        "Password",
					Description: "Your Snapchat password",
				},
			},
		},
	}, nil
}

// SubmitUserInput handles user input submission
func (sl *SnapchatLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
	username, ok := input["username"]
	if !ok || username == "" {
		return nil, fmt.Errorf("username is required")
	}

	password, ok := input["password"]
	if !ok || password == "" {
		return nil, fmt.Errorf("password is required")
	}

	sl.Username = username
	sl.Password = password

	// Create Snapchat client and attempt login
	oldConfig := &Config{}
	oldConfig.Snapchat.BaseURL = sl.Connector.Config.BaseURL
	oldConfig.Snapchat.UserAgent = sl.Connector.Config.UserAgent
	oldConfig.Snapchat.DeviceID = sl.Connector.Config.DeviceID
	oldConfig.Snapchat.AppVersion = sl.Connector.Config.AppVersion
	oldConfig.Snapchat.ClientAuth = sl.Connector.Config.ClientAuth
	oldConfig.Snapchat.RateLimit.Requests = sl.Connector.Config.RateLimit.Requests
	oldConfig.Snapchat.RateLimit.Window = sl.Connector.Config.RateLimit.Window

	client := NewSnapchatClient(oldConfig)
	err := client.Login(username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to login to Snapchat: %w", err)
	}

	sl.Client = client

	// Create the UserLogin
	return sl.finishLogin(ctx)
}

// finishLogin creates the UserLogin object and completes the process
func (sl *SnapchatLogin) finishLogin(ctx context.Context) (*bridgev2.LoginStep, error) {
	if sl.Client == nil || sl.Client.sessionData == nil {
		return nil, fmt.Errorf("no valid session data")
	}

	sessionData := sl.Client.sessionData

	ul, err := sl.User.NewLogin(ctx, &database.UserLogin{
		ID:         makeUserLoginID(sessionData.Username),
		RemoteName: sessionData.Username,
		Metadata: &UserLoginMetadata{
			Username:     sessionData.Username,
			UserID:       sessionData.UserID,
			AuthToken:    sessionData.AuthToken,
			RefreshToken: sessionData.RefreshToken,
			DeviceID:     sessionData.DeviceID,
			ExpiresAt:    sessionData.ExpiresAt,
		},
	}, &bridgev2.NewLoginParams{
		LoadUserLogin: func(ctx context.Context, login *bridgev2.UserLogin) error {
			login.Client = &SnapchatAPI{
				UserLogin: login,
				Client:    sl.Client,
			}
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user login: %w", err)
	}

	// Return the complete step
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeComplete,
		StepID:       "com.beeper.snapchat.complete",
		Instructions: fmt.Sprintf("Successfully logged in as %s", sessionData.Username),
		CompleteParams: &bridgev2.LoginCompleteParams{
			UserLoginID: ul.ID,
			UserLogin:   ul,
		},
	}, nil
}

// Cancel is called if the user cancels the login process
func (sl *SnapchatLogin) Cancel() {
	if sl.Client != nil {
		sl.Client.Close()
	}
}