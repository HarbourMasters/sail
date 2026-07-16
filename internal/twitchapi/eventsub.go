package twitchapi

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const eventSubWebSocketURL = "wss://eventsub.wss.twitch.tv/ws"

// ChatMessageEvent is a chat message, trimmed to the fields this app uses.
// ChatterUserID lets the app recognize its own posted messages when they echo
// back (see the loop guard in app_twitch.go).
type ChatMessageEvent struct {
	ChatterUserID    string `json:"chatter_user_id"`
	ChatterUserLogin string `json:"chatter_user_login"`
	ChatterUserName  string `json:"chatter_user_name"`
	Message          struct {
		Text string `json:"text"`
	} `json:"message"`
}

// RewardRedemptionEvent is a channel point redemption, trimmed to the fields
// this app uses.
type RewardRedemptionEvent struct {
	UserLogin string `json:"user_login"`
	UserName  string `json:"user_name"`
	UserInput string `json:"user_input"`
	Reward    struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"reward"`
}

// EventSubClient maintains a Twitch EventSub WebSocket for one broadcaster,
// dispatching chat messages and channel point redemptions. It reconnects with
// backoff on any drop. Twitch's graceful "session_reconnect" handoff isn't
// implemented — it just resubscribes from scratch on the new connection.
type EventSubClient struct {
	ClientID      string
	AccessToken   string
	BroadcasterID string

	OnChatMessage func(ChatMessageEvent)
	OnRedemption  func(RewardRedemptionEvent)
	OnConnected   func()
	OnError       func(error)

	// webSocketURL and helixBaseURL override the Twitch endpoints for tests;
	// empty means the real ones (see the cmp.Or calls at their use sites).
	webSocketURL string
	helixBaseURL string

	cancel context.CancelFunc
	done   chan struct{}
}

// Start connects in the background and keeps reconnecting until Stop is
// called.
func (e *EventSubClient) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.done = make(chan struct{})
	go e.run(ctx)
}

// Stop disconnects and waits for the background loop to exit.
func (e *EventSubClient) Stop() {
	if e.cancel == nil {
		return
	}
	e.cancel()
	<-e.done
}

func (e *EventSubClient) run(ctx context.Context) {
	defer close(e.done)

	backoff := time.Second
	resetBackoff := func() { backoff = time.Second }

	for ctx.Err() == nil {
		if err := e.runOnce(ctx, resetBackoff); err != nil && ctx.Err() == nil {
			e.reportError(fmt.Errorf("eventsub connection dropped: %w", err))
		}
		if ctx.Err() != nil {
			return
		}

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (e *EventSubClient) runOnce(ctx context.Context, connected func()) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, cmp.Or(e.webSocketURL, eventSubWebSocketURL), nil)
	if err != nil {
		return fmt.Errorf("dial eventsub websocket: %w", err)
	}
	defer conn.Close()

	// gorilla/websocket reads don't observe context cancellation, so cancelling
	// ctx alone won't unblock the ReadMessage loop below: Twitch's keepalives
	// keep it returning without it ever checking ctx. Closing the connection is
	// what interrupts an in-flight read, so watch ctx here and close on cancel.
	// Without this, Stop() blocks on <-e.done forever and the app hangs on
	// shutdown. stopWatch ends the watcher when runOnce returns so reconnects
	// don't leak it. (Close may run concurrently with ReadMessage, which
	// gorilla/websocket explicitly permits.)
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stopWatch:
		}
	}()

	sessionID, err := awaitWelcome(conn)
	if err != nil {
		return err
	}

	if err := e.subscribe(ctx, sessionID); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	connected()
	if e.OnConnected != nil {
		e.OnConnected()
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			e.reportError(fmt.Errorf("decode eventsub message: %w", err))
			continue
		}

		switch msg.Metadata.MessageType {
		case "notification":
			e.handleNotification(msg.Payload)
		case "session_reconnect":
			return fmt.Errorf("twitch requested a reconnect")
		case "revocation":
			return fmt.Errorf("a subscription was revoked")
		}
	}
}

type wsMessage struct {
	Metadata struct {
		MessageType string `json:"message_type"`
	} `json:"metadata"`
	Payload json.RawMessage `json:"payload"`
}

func awaitWelcome(conn *websocket.Conn) (string, error) {
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	_, data, err := conn.ReadMessage()
	if err != nil {
		return "", fmt.Errorf("read welcome message: %w", err)
	}

	var msg struct {
		Metadata struct {
			MessageType string `json:"message_type"`
		} `json:"metadata"`
		Payload struct {
			Session struct {
				ID string `json:"id"`
			} `json:"session"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return "", fmt.Errorf("decode welcome message: %w", err)
	}
	if msg.Metadata.MessageType != "session_welcome" {
		return "", fmt.Errorf("expected session_welcome, got %q", msg.Metadata.MessageType)
	}
	return msg.Payload.Session.ID, nil
}

func (e *EventSubClient) subscribe(ctx context.Context, sessionID string) error {
	subscriptions := []struct {
		eventType string
		condition map[string]string
	}{
		{
			eventType: "channel.chat.message",
			condition: map[string]string{
				"broadcaster_user_id": e.BroadcasterID,
				"user_id":             e.BroadcasterID,
			},
		},
		{
			eventType: "channel.channel_points_custom_reward_redemption.add",
			condition: map[string]string{
				"broadcaster_user_id": e.BroadcasterID,
			},
		},
	}

	for _, sub := range subscriptions {
		if err := e.createSubscription(ctx, sub.eventType, sub.condition, sessionID); err != nil {
			return fmt.Errorf("%s: %w", sub.eventType, err)
		}
	}
	return nil
}

func (e *EventSubClient) createSubscription(ctx context.Context, eventType string, condition map[string]string, sessionID string) error {
	body, err := json.Marshal(map[string]any{
		"type":      eventType,
		"version":   "1",
		"condition": condition,
		"transport": map[string]string{
			"method":     "websocket",
			"session_id": sessionID,
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cmp.Or(e.helixBaseURL, helixBase)+"/eventsub/subscriptions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthHeaders(req, e.ClientID, e.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twitch returned %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

func (e *EventSubClient) handleNotification(payload json.RawMessage) {
	var envelope struct {
		Subscription struct {
			Type string `json:"type"`
		} `json:"subscription"`
		Event json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		e.reportError(fmt.Errorf("decode notification: %w", err))
		return
	}

	switch envelope.Subscription.Type {
	case "channel.chat.message":
		var event ChatMessageEvent
		if err := json.Unmarshal(envelope.Event, &event); err != nil {
			e.reportError(fmt.Errorf("decode chat message event: %w", err))
			return
		}
		if e.OnChatMessage != nil {
			e.OnChatMessage(event)
		}

	case "channel.channel_points_custom_reward_redemption.add":
		var event RewardRedemptionEvent
		if err := json.Unmarshal(envelope.Event, &event); err != nil {
			e.reportError(fmt.Errorf("decode redemption event: %w", err))
			return
		}
		if e.OnRedemption != nil {
			e.OnRedemption(event)
		}
	}
}

func (e *EventSubClient) reportError(err error) {
	if e.OnError != nil {
		e.OnError(err)
	}
}
