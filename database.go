package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

type StoredUser struct {
	MXID        string    `json:"mxid"`
	Username    string    `json:"username"`
	SessionData string    `json:"session_data"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type StoredMessage struct {
	ID          string    `json:"id"`
	RoomID      string    `json:"room_id"`
	EventID     string    `json:"event_id"`
	SnapchatID  string    `json:"snapchat_id"`
	FromUser    string    `json:"from_user"`
	ToUser      string    `json:"to_user"`
	Content     string    `json:"content"`
	MediaURL    string    `json:"media_url"`
	Ephemeral   bool      `json:"ephemeral"`
	Timestamp   time.Time `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
}

type StoredRoom struct {
	ID               string    `json:"id"`
	MXID             string    `json:"mxid"`
	SnapchatConvoID  string    `json:"snapchat_convo_id"`
	IsGroup          bool      `json:"is_group"`
	Name             string    `json:"name"`
	Participants     string    `json:"participants"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func NewDatabase(dbURI string) (*Database, error) {
	db, err := sql.Open("sqlite", dbURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return database, nil
}

func (d *Database) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			mxid TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			session_data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			event_id TEXT,
			snapchat_id TEXT UNIQUE NOT NULL,
			from_user TEXT NOT NULL,
			to_user TEXT NOT NULL,
			content TEXT,
			media_url TEXT,
			ephemeral BOOLEAN DEFAULT FALSE,
			timestamp DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS rooms (
			id TEXT PRIMARY KEY,
			mxid TEXT UNIQUE,
			snapchat_convo_id TEXT UNIQUE NOT NULL,
			is_group BOOLEAN DEFAULT FALSE,
			name TEXT,
			participants TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_room_id ON messages(room_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_snapchat_id ON messages(snapchat_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_rooms_snapchat_convo_id ON rooms(snapchat_convo_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %q: %w", query, err)
		}
	}

	return nil
}

func (d *Database) StoreUser(user *StoredUser) error {
	query := `INSERT OR REPLACE INTO users (mxid, username, session_data, updated_at)
			  VALUES (?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := d.db.Exec(query, user.MXID, user.Username, user.SessionData)
	if err != nil {
		return fmt.Errorf("failed to store user: %w", err)
	}

	return nil
}

func (d *Database) GetUser(mxid string) (*StoredUser, error) {
	query := `SELECT mxid, username, session_data, created_at, updated_at
			  FROM users WHERE mxid = ?`

	var user StoredUser
	err := d.db.QueryRow(query, mxid).Scan(
		&user.MXID, &user.Username, &user.SessionData,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (d *Database) GetUserByUsername(username string) (*StoredUser, error) {
	query := `SELECT mxid, username, session_data, created_at, updated_at
			  FROM users WHERE username = ?`

	var user StoredUser
	err := d.db.QueryRow(query, username).Scan(
		&user.MXID, &user.Username, &user.SessionData,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return &user, nil
}

func (d *Database) StoreMessage(message *StoredMessage) error {
	query := `INSERT OR REPLACE INTO messages
			  (id, room_id, event_id, snapchat_id, from_user, to_user, content, media_url, ephemeral, timestamp)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query,
		message.ID, message.RoomID, message.EventID, message.SnapchatID,
		message.FromUser, message.ToUser, message.Content, message.MediaURL,
		message.Ephemeral, message.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	return nil
}

func (d *Database) GetMessage(snapchatID string) (*StoredMessage, error) {
	query := `SELECT id, room_id, event_id, snapchat_id, from_user, to_user,
			  content, media_url, ephemeral, timestamp, created_at
			  FROM messages WHERE snapchat_id = ?`

	var message StoredMessage
	err := d.db.QueryRow(query, snapchatID).Scan(
		&message.ID, &message.RoomID, &message.EventID, &message.SnapchatID,
		&message.FromUser, &message.ToUser, &message.Content, &message.MediaURL,
		&message.Ephemeral, &message.Timestamp, &message.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &message, nil
}

func (d *Database) GetMessagesByRoom(roomID string, limit int) ([]*StoredMessage, error) {
	query := `SELECT id, room_id, event_id, snapchat_id, from_user, to_user,
			  content, media_url, ephemeral, timestamp, created_at
			  FROM messages WHERE room_id = ?
			  ORDER BY timestamp DESC LIMIT ?`

	rows, err := d.db.Query(query, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*StoredMessage
	for rows.Next() {
		var message StoredMessage
		err := rows.Scan(
			&message.ID, &message.RoomID, &message.EventID, &message.SnapchatID,
			&message.FromUser, &message.ToUser, &message.Content, &message.MediaURL,
			&message.Ephemeral, &message.Timestamp, &message.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &message)
	}

	return messages, nil
}

func (d *Database) StoreRoom(room *StoredRoom) error {
	query := `INSERT OR REPLACE INTO rooms
			  (id, mxid, snapchat_convo_id, is_group, name, participants, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := d.db.Exec(query,
		room.ID, room.MXID, room.SnapchatConvoID,
		room.IsGroup, room.Name, room.Participants,
	)

	if err != nil {
		return fmt.Errorf("failed to store room: %w", err)
	}

	return nil
}

func (d *Database) GetRoom(snapchatConvoID string) (*StoredRoom, error) {
	query := `SELECT id, mxid, snapchat_convo_id, is_group, name, participants, created_at, updated_at
			  FROM rooms WHERE snapchat_convo_id = ?`

	var room StoredRoom
	err := d.db.QueryRow(query, snapchatConvoID).Scan(
		&room.ID, &room.MXID, &room.SnapchatConvoID,
		&room.IsGroup, &room.Name, &room.Participants,
		&room.CreatedAt, &room.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	return &room, nil
}

func (d *Database) GetRoomByMXID(mxid string) (*StoredRoom, error) {
	query := `SELECT id, mxid, snapchat_convo_id, is_group, name, participants, created_at, updated_at
			  FROM rooms WHERE mxid = ?`

	var room StoredRoom
	err := d.db.QueryRow(query, mxid).Scan(
		&room.ID, &room.MXID, &room.SnapchatConvoID,
		&room.IsGroup, &room.Name, &room.Participants,
		&room.CreatedAt, &room.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get room by MXID: %w", err)
	}

	return &room, nil
}

func (d *Database) GetAllRooms() ([]*StoredRoom, error) {
	query := `SELECT id, mxid, snapchat_convo_id, is_group, name, participants, created_at, updated_at
			  FROM rooms ORDER BY updated_at DESC`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query rooms: %w", err)
	}
	defer rows.Close()

	var rooms []*StoredRoom
	for rows.Next() {
		var room StoredRoom
		err := rows.Scan(
			&room.ID, &room.MXID, &room.SnapchatConvoID,
			&room.IsGroup, &room.Name, &room.Participants,
			&room.CreatedAt, &room.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}
		rooms = append(rooms, &room)
	}

	return rooms, nil
}

func (d *Database) DeleteEphemeralMessages(olderThan time.Time) error {
	query := `DELETE FROM messages WHERE ephemeral = TRUE AND timestamp < ?`

	result, err := d.db.Exec(query, olderThan)
	if err != nil {
		return fmt.Errorf("failed to delete ephemeral messages: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("Deleted %d ephemeral messages\n", rowsAffected)
	}

	return nil
}

func (d *Database) UpdateUserSession(mxid string, sessionData *SessionData) error {
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	query := `UPDATE users SET session_data = ?, updated_at = CURRENT_TIMESTAMP WHERE mxid = ?`
	_, err = d.db.Exec(query, string(sessionJSON), mxid)
	if err != nil {
		return fmt.Errorf("failed to update user session: %w", err)
	}

	return nil
}

func (d *Database) GetUserSession(mxid string) (*SessionData, error) {
	user, err := d.GetUser(mxid)
	if err != nil {
		return nil, err
	}
	if user == nil || user.SessionData == "" {
		return nil, nil
	}

	var sessionData SessionData
	if err := json.Unmarshal([]byte(user.SessionData), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &sessionData, nil
}

func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}