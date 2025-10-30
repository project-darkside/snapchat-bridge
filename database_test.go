package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if db.db == nil {
		t.Error("Database connection should not be nil")
	}
}

func TestDatabase_StoreAndGetUser(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	user := &StoredUser{
		MXID:        "@testuser:example.com",
		Username:    "testuser",
		SessionData: `{"username":"testuser","auth_token":"token123"}`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = db.StoreUser(user)
	if err != nil {
		t.Fatalf("Failed to store user: %v", err)
	}

	retrieved, err := db.GetUser("@testuser:example.com")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved user should not be nil")
	}

	if retrieved.MXID != user.MXID {
		t.Errorf("Expected MXID %s, got %s", user.MXID, retrieved.MXID)
	}

	if retrieved.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, retrieved.Username)
	}

	if retrieved.SessionData != user.SessionData {
		t.Errorf("Expected session data %s, got %s", user.SessionData, retrieved.SessionData)
	}
}

func TestDatabase_GetUserByUsername(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	user := &StoredUser{
		MXID:        "@testuser:example.com",
		Username:    "testuser",
		SessionData: `{"username":"testuser"}`,
	}

	err = db.StoreUser(user)
	if err != nil {
		t.Fatalf("Failed to store user: %v", err)
	}

	retrieved, err := db.GetUserByUsername("testuser")
	if err != nil {
		t.Fatalf("Failed to get user by username: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved user should not be nil")
	}

	if retrieved.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", retrieved.Username)
	}
}

func TestDatabase_StoreAndGetMessage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	message := &StoredMessage{
		ID:         "msg123",
		RoomID:     "!room:example.com",
		EventID:    "$event123",
		SnapchatID: "snap123",
		FromUser:   "alice",
		ToUser:     "bob",
		Content:    "Hello World!",
		MediaURL:   "",
		Ephemeral:  false,
		Timestamp:  time.Now(),
		CreatedAt:  time.Now(),
	}

	err = db.StoreMessage(message)
	if err != nil {
		t.Fatalf("Failed to store message: %v", err)
	}

	retrieved, err := db.GetMessage("snap123")
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved message should not be nil")
	}

	if retrieved.SnapchatID != message.SnapchatID {
		t.Errorf("Expected Snapchat ID %s, got %s", message.SnapchatID, retrieved.SnapchatID)
	}

	if retrieved.Content != message.Content {
		t.Errorf("Expected content %s, got %s", message.Content, retrieved.Content)
	}

	if retrieved.FromUser != message.FromUser {
		t.Errorf("Expected from user %s, got %s", message.FromUser, retrieved.FromUser)
	}
}

func TestDatabase_GetMessagesByRoom(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	roomID := "!room:example.com"

	messages := []*StoredMessage{
		{
			ID:         "msg1",
			RoomID:     roomID,
			SnapchatID: "snap1",
			FromUser:   "alice",
			ToUser:     "bob",
			Content:    "Message 1",
			Timestamp:  time.Now().Add(-2 * time.Hour),
		},
		{
			ID:         "msg2",
			RoomID:     roomID,
			SnapchatID: "snap2",
			FromUser:   "bob",
			ToUser:     "alice",
			Content:    "Message 2",
			Timestamp:  time.Now().Add(-1 * time.Hour),
		},
		{
			ID:         "msg3",
			RoomID:     "!other:example.com",
			SnapchatID: "snap3",
			FromUser:   "charlie",
			ToUser:     "dave",
			Content:    "Message 3",
			Timestamp:  time.Now(),
		},
	}

	for _, msg := range messages {
		err = db.StoreMessage(msg)
		if err != nil {
			t.Fatalf("Failed to store message: %v", err)
		}
	}

	retrieved, err := db.GetMessagesByRoom(roomID, 10)
	if err != nil {
		t.Fatalf("Failed to get messages by room: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 messages for room, got %d", len(retrieved))
	}

	if retrieved[0].Content != "Message 2" {
		t.Errorf("Expected most recent message first, got %s", retrieved[0].Content)
	}

	if retrieved[1].Content != "Message 1" {
		t.Errorf("Expected older message second, got %s", retrieved[1].Content)
	}
}

func TestDatabase_StoreAndGetRoom(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	room := &StoredRoom{
		ID:              "room123",
		MXID:            "!room:example.com",
		SnapchatConvoID: "convo123",
		IsGroup:         false,
		Name:            "Test Room",
		Participants:    "alice,bob",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err = db.StoreRoom(room)
	if err != nil {
		t.Fatalf("Failed to store room: %v", err)
	}

	retrieved, err := db.GetRoom("convo123")
	if err != nil {
		t.Fatalf("Failed to get room: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved room should not be nil")
	}

	if retrieved.SnapchatConvoID != room.SnapchatConvoID {
		t.Errorf("Expected Snapchat convo ID %s, got %s", room.SnapchatConvoID, retrieved.SnapchatConvoID)
	}

	if retrieved.Name != room.Name {
		t.Errorf("Expected name %s, got %s", room.Name, retrieved.Name)
	}

	if retrieved.IsGroup != room.IsGroup {
		t.Errorf("Expected is_group %t, got %t", room.IsGroup, retrieved.IsGroup)
	}
}

func TestDatabase_GetRoomByMXID(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	room := &StoredRoom{
		ID:              "room123",
		MXID:            "!room:example.com",
		SnapchatConvoID: "convo123",
		IsGroup:         true,
		Name:            "Group Chat",
		Participants:    "alice,bob,charlie",
	}

	err = db.StoreRoom(room)
	if err != nil {
		t.Fatalf("Failed to store room: %v", err)
	}

	retrieved, err := db.GetRoomByMXID("!room:example.com")
	if err != nil {
		t.Fatalf("Failed to get room by MXID: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved room should not be nil")
	}

	if retrieved.MXID != room.MXID {
		t.Errorf("Expected MXID %s, got %s", room.MXID, retrieved.MXID)
	}
}

func TestDatabase_DeleteEphemeralMessages(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	now := time.Now()
	oldTime := now.Add(-2 * time.Hour)
	recentTime := now.Add(-30 * time.Minute)

	messages := []*StoredMessage{
		{
			ID:         "msg1",
			RoomID:     "!room:example.com",
			SnapchatID: "snap1",
			FromUser:   "alice",
			ToUser:     "bob",
			Content:    "Old ephemeral",
			Ephemeral:  true,
			Timestamp:  oldTime,
		},
		{
			ID:         "msg2",
			RoomID:     "!room:example.com",
			SnapchatID: "snap2",
			FromUser:   "alice",
			ToUser:     "bob",
			Content:    "Recent ephemeral",
			Ephemeral:  true,
			Timestamp:  recentTime,
		},
		{
			ID:         "msg3",
			RoomID:     "!room:example.com",
			SnapchatID: "snap3",
			FromUser:   "alice",
			ToUser:     "bob",
			Content:    "Persistent message",
			Ephemeral:  false,
			Timestamp:  oldTime,
		},
	}

	for _, msg := range messages {
		err = db.StoreMessage(msg)
		if err != nil {
			t.Fatalf("Failed to store message: %v", err)
		}
	}

	cutoff := now.Add(-1 * time.Hour)
	err = db.DeleteEphemeralMessages(cutoff)
	if err != nil {
		t.Fatalf("Failed to delete ephemeral messages: %v", err)
	}

	msg1, err := db.GetMessage("snap1")
	if err != nil {
		t.Fatalf("Failed to check message 1: %v", err)
	}
	if msg1 != nil {
		t.Error("Old ephemeral message should be deleted")
	}

	msg2, err := db.GetMessage("snap2")
	if err != nil {
		t.Fatalf("Failed to check message 2: %v", err)
	}
	if msg2 == nil {
		t.Error("Recent ephemeral message should not be deleted")
	}

	msg3, err := db.GetMessage("snap3")
	if err != nil {
		t.Fatalf("Failed to check message 3: %v", err)
	}
	if msg3 == nil {
		t.Error("Persistent message should not be deleted")
	}
}

func TestDatabase_UpdateUserSession(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	user := &StoredUser{
		MXID:        "@testuser:example.com",
		Username:    "testuser",
		SessionData: "",
	}

	err = db.StoreUser(user)
	if err != nil {
		t.Fatalf("Failed to store user: %v", err)
	}

	sessionData := &SessionData{
		Username:     "testuser",
		UserID:       "user123",
		AuthToken:    "token123",
		RefreshToken: "refresh123",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		DeviceID:     "device123",
	}

	err = db.UpdateUserSession("@testuser:example.com", sessionData)
	if err != nil {
		t.Fatalf("Failed to update user session: %v", err)
	}

	retrieved, err := db.GetUserSession("@testuser:example.com")
	if err != nil {
		t.Fatalf("Failed to get user session: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved session should not be nil")
	}

	if retrieved.Username != sessionData.Username {
		t.Errorf("Expected username %s, got %s", sessionData.Username, retrieved.Username)
	}

	if retrieved.AuthToken != sessionData.AuthToken {
		t.Errorf("Expected auth token %s, got %s", sessionData.AuthToken, retrieved.AuthToken)
	}
}

func TestDatabase_GetUserSession_NoSession(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	session, err := db.GetUserSession("@nonexistent:example.com")
	if err != nil {
		t.Fatalf("Failed to get user session: %v", err)
	}

	if session != nil {
		t.Error("Session should be nil for nonexistent user")
	}
}