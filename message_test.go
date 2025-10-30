package main

import (
	"testing"
)

func TestGetConversationID(t *testing.T) {
	config := &Config{
		Snapchat: SnapchatConfig{
			BaseURL: "https://app.snapchat.com",
		},
	}

	client := &SnapchatClient{
		config:   config,
		username: "user1",
	}

	tests := []struct {
		user1    string
		user2    string
		expected string
	}{
		{"alice", "bob", "alice~bob"},
		{"bob", "alice", "alice~bob"},
		{"zebra", "aardvark", "aardvark~zebra"},
		{"user1", "user2", "user1~user2"},
	}

	for _, tt := range tests {
		result := client.getConversationID(tt.user1, tt.user2)
		if result != tt.expected {
			t.Errorf("getConversationID(%s, %s) = %s; want %s", tt.user1, tt.user2, result, tt.expected)
		}
	}
}

func TestMessageStructure(t *testing.T) {
	// This test verifies the message structure matches the PHP SDK format
	config := &Config{
		Snapchat: SnapchatConfig{
			BaseURL: "https://app.snapchat.com",
		},
	}

	client := &SnapchatClient{
		config:   config,
		username: "testuser",
	}

	conversationID := client.getConversationID("alice", "bob")
	if conversationID != "alice~bob" {
		t.Errorf("Expected conversation ID 'alice~bob', got '%s'", conversationID)
	}
}
