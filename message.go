package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"
)

// HandleMatrixMessage sends a message from Matrix to Snapchat
func (sa *SnapchatAPI) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (message *bridgev2.MatrixMessageResponse, err error) {
	if sa.Client == nil {
		return nil, fmt.Errorf("not connected to Snapchat")
	}

	// Extract the target user from the portal ID
	targetUser := string(msg.Portal.ID)

	// Send the message via Snapchat client
	snapMsg, err := sa.Client.SendMessage(targetUser, msg.Content.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to send message to Snapchat: %w", err)
	}

	return &bridgev2.MatrixMessageResponse{
		DB: &database.Message{
			ID:       makeMessageID(snapMsg.ID),
			SenderID: makeUserID(snapMsg.FromUser),
		},
	}, nil
}

// HandleSnapchatMessage processes incoming messages from Snapchat
func (sa *SnapchatAPI) HandleSnapchatMessage(msg *SnapchatMessage) {
	if sa.UserLogin == nil || sa.UserLogin.Bridge == nil {
		return
	}

	// Create portal key for the conversation
	portalKey := networkid.PortalKey{
		ID:       makePortalID(msg.FromUser),
		Receiver: sa.UserLogin.ID,
	}

	// Queue the message for processing by the bridge
	sa.UserLogin.Bridge.QueueRemoteEvent(sa.UserLogin, &simplevent.Message[*SnapchatMessage]{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventMessage,
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.
					Str("from_user", msg.FromUser).
					Str("message_id", msg.ID)
			},
			PortalKey:    portalKey,
			CreatePortal: true,
			Sender: bridgev2.EventSender{
				Sender: makeUserID(msg.FromUser),
			},
			Timestamp: msg.Timestamp,
		},
		Data:               msg,
		ID:                 makeMessageID(msg.ID),
		ConvertMessageFunc: sa.convertSnapchatMessage,
	})
}

// convertSnapchatMessage converts a SnapchatMessage to Matrix events
func (sa *SnapchatAPI) convertSnapchatMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, data *SnapchatMessage) (*bridgev2.ConvertedMessage, error) {
	if data.MediaURL != "" {
		// Handle media message (snap)
		return sa.convertMediaMessage(ctx, portal, intent, data)
	}

	// Handle text message
	return &bridgev2.ConvertedMessage{
		Parts: []*bridgev2.ConvertedMessagePart{{
			Type: event.EventMessage,
			Content: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    data.Content,
			},
		}},
	}, nil
}

// convertMediaMessage converts a Snapchat media message to Matrix
func (sa *SnapchatAPI) convertMediaMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, data *SnapchatMessage) (*bridgev2.ConvertedMessage, error) {
	// For now, just send the media URL as text
	// In a real implementation, you'd download the media and upload to Matrix
	body := fmt.Sprintf("📷 Snap from %s", data.FromUser)
	if data.ViewTime > 0 {
		body += fmt.Sprintf(" (view time: %ds)", data.ViewTime)
	}

	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
	}

	// If this is an ephemeral message, add metadata
	if data.Ephemeral && data.ViewTime > 0 {
		// Add disappearing message metadata
		// This would require implementing the disappearing messages extension
		content.Body = "🔥 " + content.Body + " (disappears after viewing)"
	}

	return &bridgev2.ConvertedMessage{
		Parts: []*bridgev2.ConvertedMessagePart{{
			Type:    event.EventMessage,
			Content: content,
		}},
	}, nil
}

// StartPolling begins polling for new messages from Snapchat
func (sa *SnapchatAPI) StartPolling(ctx context.Context) {
	if sa.Client == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages, err := sa.Client.GetMessages()
			if err != nil {
				// Log error but continue polling
				continue
			}

			for _, msg := range messages {
				// Only process messages we haven't seen before
				if sa.shouldProcessMessage(msg) {
					sa.HandleSnapchatMessage(msg)
				}
			}
		}
	}
}

// shouldProcessMessage determines if a message should be processed
func (sa *SnapchatAPI) shouldProcessMessage(msg *SnapchatMessage) bool {
	// In a real implementation, you'd track which messages have been processed
	// For now, just process messages from the last hour
	return time.Since(msg.Timestamp) < time.Hour
}

// IdentifierResolvingNetworkAPI implementation for starting new chats
var _ bridgev2.IdentifierResolvingNetworkAPI = (*SnapchatAPI)(nil)

// ResolveIdentifier resolves a Snapchat username to start a chat
func (sa *SnapchatAPI) ResolveIdentifier(ctx context.Context, identifier string, createChat bool) (*bridgev2.ResolveIdentifierResponse, error) {
	// Clean the identifier (remove @ if present, etc.)
	username := identifier
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}

	userID := makeUserID(username)
	portalKey := networkid.PortalKey{
		ID:       makePortalID(username),
		Receiver: sa.UserLogin.ID,
	}

	ghost, err := sa.UserLogin.Bridge.GetGhostByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ghost: %w", err)
	}

	portal, err := sa.UserLogin.Bridge.GetPortalByKey(ctx, portalKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal: %w", err)
	}

	ghostInfo, _ := sa.GetUserInfo(ctx, ghost)
	portalInfo, _ := sa.GetChatInfo(ctx, portal)

	return &bridgev2.ResolveIdentifierResponse{
		Ghost:    ghost,
		UserID:   userID,
		UserInfo: ghostInfo,
		Chat: &bridgev2.CreateChatResponse{
			Portal:     portal,
			PortalKey:  portalKey,
			PortalInfo: portalInfo,
		},
	}, nil
}