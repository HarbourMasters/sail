package twitchapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestEventSubClientStopReturnsWhileReadBlocked is the shutdown-hang
// regression. gorilla/websocket's ReadMessage ignores context cancellation, so
// while the read loop is parked waiting on a live connection, cancelling the
// context alone won't unblock it — runOnce must close the connection on
// ctx.Done. Before that fix, Stop() blocked on <-e.done forever and the app
// hung when closed on macOS. The fake server streams keepalives (as Twitch
// does) so nothing but the fix can end the read.
func TestEventSubClientStopReturnsWhileReadBlocked(t *testing.T) {
	upgrader := websocket.Upgrader{}

	mux := http.NewServeMux()
	// Helix subscribe endpoint: accept every subscription.
	mux.HandleFunc("/eventsub/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	// EventSub WebSocket: send the welcome, then keepalives forever, mimicking
	// Twitch holding the socket open so the client's ReadMessage stays parked.
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		welcome := `{"metadata":{"message_type":"session_welcome"},"payload":{"session":{"id":"test-session"}}}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(welcome)); err != nil {
			return
		}
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			keepalive := `{"metadata":{"message_type":"session_keepalive"},"payload":{}}`
			if err := conn.WriteMessage(websocket.TextMessage, []byte(keepalive)); err != nil {
				return // client closed the connection; stop streaming
			}
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var once sync.Once
	connected := make(chan struct{})
	client := &EventSubClient{
		ClientID:      "test-client",
		AccessToken:   "test-token",
		BroadcasterID: "12345",
		webSocketURL:  "ws" + strings.TrimPrefix(server.URL, "http") + "/ws",
		helixBaseURL:  server.URL,
		OnConnected:   func() { once.Do(func() { close(connected) }) },
	}

	client.Start(context.Background())

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("client never connected")
	}

	// The read loop is now parked in ReadMessage while the server streams
	// keepalives. Stop() must return promptly rather than hang.
	done := make(chan struct{})
	go func() {
		client.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() hung: the read loop was not interrupted by context cancellation (shutdown-hang regression)")
	}
}
