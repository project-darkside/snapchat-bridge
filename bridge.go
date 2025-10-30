package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"maunium.net/go/mautrix/appservice"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type SimpleBridge struct {
	Config *Config
	AS     *appservice.AppService
	Bot    *appservice.IntentAPI
	DB     *Database

	usersByMXID     map[id.UserID]*SimpleUser
	usersByUsername map[string]*SimpleUser
	portalsByMXID   map[id.RoomID]*SimplePortal
	portalsByKey    map[string]*SimplePortal
	managementRooms map[id.UserID]id.RoomID

	userLock   sync.RWMutex
	portalLock sync.RWMutex
}

type SimpleUser struct {
	bridge   *SimpleBridge
	MXID     id.UserID
	Username string
	Client   *SnapchatClient

	managementRoom id.RoomID
	connected      bool
	lastSync       time.Time
}

type SimplePortal struct {
	bridge *SimpleBridge

	Key              string
	MXID             id.RoomID
	Name             string
	IsGroup          bool
	Participants     []string

	roomCreateLock sync.Mutex
}

func NewSimpleBridge(config *Config) (*SimpleBridge, error) {
	db, err := NewDatabase(config.Database.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	bridge := &SimpleBridge{
		Config:          config,
		DB:              db,
		usersByMXID:     make(map[id.UserID]*SimpleUser),
		usersByUsername: make(map[string]*SimpleUser),
		portalsByMXID:   make(map[id.RoomID]*SimplePortal),
		portalsByKey:    make(map[string]*SimplePortal),
		managementRooms: make(map[id.UserID]id.RoomID),
	}

	as, err := bridge.CreateAppService()
	if err != nil {
		return nil, fmt.Errorf("failed to create appservice: %w", err)
	}

	bridge.AS = as
	bridge.Bot = as.BotIntent()

	return bridge, nil
}

func (bridge *SimpleBridge) CreateAppService() (*appservice.AppService, error) {
	as := appservice.Create()
	as.HomeserverDomain = bridge.Config.Homeserver.Domain
	// Note: HomeserverURL may need to be set differently in actual mautrix version
	as.Host.Hostname = bridge.Config.AppService.Hostname
	as.Host.Port = uint16(bridge.Config.AppService.Port)

	registration := &appservice.Registration{
		ID:              bridge.Config.AppService.ID,
		URL:             bridge.Config.AppService.Address,
		AppToken:        bridge.Config.AppService.ASToken,
		ServerToken:     bridge.Config.AppService.HSToken,
		SenderLocalpart: bridge.Config.AppService.BotLocalID,
	}

	as.Registration = registration
	return as, nil
}

func (bridge *SimpleBridge) Start() error {
	log.Println("Starting Snapchat bridge...")

	bridge.AS.Start()
	// Note: AS.Start() may not return error in this mautrix version

	log.Println("Bridge started successfully")
	return nil
}

func (bridge *SimpleBridge) Stop() {
	log.Println("Stopping Snapchat bridge...")

	bridge.userLock.RLock()
	for _, user := range bridge.usersByMXID {
		if user.Client != nil {
			user.Client.Close()
		}
	}
	bridge.userLock.RUnlock()

	bridge.DB.Close()
	bridge.AS.Stop()

	log.Println("Bridge stopped")
}

func (bridge *SimpleBridge) GetUserByMXID(userID id.UserID) *SimpleUser {
	bridge.userLock.RLock()
	defer bridge.userLock.RUnlock()

	user, exists := bridge.usersByMXID[userID]
	if !exists {
		return bridge.loadUser(userID)
	}
	return user
}

func (bridge *SimpleBridge) loadUser(userID id.UserID) *SimpleUser {
	user := &SimpleUser{
		bridge: bridge,
		MXID:   userID,
	}

	storedUser, err := bridge.DB.GetUser(string(userID))
	if err != nil {
		log.Printf("Failed to load user %s: %v", userID, err)
		return user
	}

	if storedUser != nil {
		user.Username = storedUser.Username
		if storedUser.SessionData != "" {
			sessionData := unmarshalSessionData(storedUser.SessionData)
			if sessionData != nil && sessionData.ExpiresAt > time.Now().Unix() {
				client := NewSnapchatClient(bridge.Config)
				client.sessionData = sessionData
				client.username = sessionData.Username
				client.authToken = sessionData.AuthToken
				user.Client = client
				user.connected = true
			}
		}
	}

	bridge.userLock.Lock()
	bridge.usersByMXID[userID] = user
	if user.Username != "" {
		bridge.usersByUsername[user.Username] = user
	}
	bridge.userLock.Unlock()

	return user
}

func (user *SimpleUser) Reply(roomID id.RoomID, message string) {
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    message,
	}

	_, err := user.bridge.Bot.SendMessageEvent(context.Background(), roomID, event.EventMessage, content)
	if err != nil {
		log.Printf("Failed to send reply: %v", err)
	}
}

func marshalSessionData(data *SessionData) string {
	if data == nil {
		return ""
	}
	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

func unmarshalSessionData(data string) *SessionData {
	if data == "" {
		return nil
	}
	var sessionData SessionData
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil
	}
	return &sessionData
}